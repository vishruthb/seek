package history

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type SearchRecord struct {
	ID           int64
	Query        string
	Response     string
	Sources      []Source
	ProjectDir   string
	ProjectStack string
	LLMBackend   string
	OutputFormat string
	SearchMs     int64
	LLMMs        int64
	TotalMs      int64
	IsFollowUp   bool
	ParentID     *int64
	CreatedAt    time.Time
}

type Source struct {
	Title  string `json:"title"`
	URL    string `json:"url"`
	Domain string `json:"domain"`
}

type HistoryStats struct {
	TotalSearches   int
	UniqueProjects  int
	AvgTotalMs      int64
	MostSearchedDir string
}

type HistoryStore struct {
	db *sql.DB
}

func NewHistoryStore(dbPath string) (*HistoryStore, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, errors.New("history database path is empty")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return nil, fmt.Errorf("create history directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := ensureSchema(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("initialize schema: %w", err)
	}

	return &HistoryStore{db: db}, nil
}

func (h *HistoryStore) Save(record *SearchRecord) (int64, error) {
	if h == nil || h.db == nil {
		return 0, errors.New("history store is not initialized")
	}
	if record == nil {
		return 0, errors.New("search record is nil")
	}

	sourcesJSON, err := json.Marshal(record.Sources)
	if err != nil {
		return 0, fmt.Errorf("marshal sources: %w", err)
	}

	var (
		result sql.Result
		args   = []any{
			record.Query,
			record.Response,
			string(sourcesJSON),
			record.ProjectDir,
			record.ProjectStack,
			record.LLMBackend,
			record.OutputFormat,
			record.SearchMs,
			record.LLMMs,
			record.TotalMs,
			boolToInt(record.IsFollowUp),
			record.ParentID,
		}
	)

	if record.CreatedAt.IsZero() {
		result, err = h.db.Exec(`
			INSERT INTO searches (
				query, response, sources, project_dir, project_stack,
				llm_backend, output_format, search_ms, llm_ms, total_ms,
				is_followup, parent_id
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			args...,
		)
	} else {
		result, err = h.db.Exec(`
			INSERT INTO searches (
				query, response, sources, project_dir, project_stack,
				llm_backend, output_format, search_ms, llm_ms, total_ms,
				is_followup, parent_id, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			append(args, record.CreatedAt.UTC().Format(time.RFC3339Nano))...,
		)
	}
	if err != nil {
		return 0, fmt.Errorf("insert search record: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("read inserted id: %w", err)
	}
	return id, nil
}

func (h *HistoryStore) Search(query string, maxResults int) ([]SearchRecord, error) {
	if h == nil || h.db == nil {
		return nil, errors.New("history store is not initialized")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	if maxResults <= 0 {
		maxResults = 10
	}

	ftsQuery := buildFTSQuery(query)
	rows, err := h.db.Query(`
		SELECT
			s.id, s.query, s.response, s.sources, s.project_dir, s.project_stack,
			s.llm_backend, s.output_format, s.search_ms, s.llm_ms, s.total_ms,
			s.is_followup, s.parent_id, s.created_at
		FROM searches AS s
		JOIN searches_fts ON searches_fts.rowid = s.id
		WHERE searches_fts MATCH ?
		ORDER BY bm25(searches_fts), s.created_at DESC, s.id DESC
		LIMIT ?`,
		ftsQuery,
		maxResults,
	)
	if err != nil {
		return nil, fmt.Errorf("search history: %w", err)
	}
	defer rows.Close()

	return scanRecords(rows)
}

func (h *HistoryStore) Recent(n int, projectDir string) ([]SearchRecord, error) {
	if h == nil || h.db == nil {
		return nil, errors.New("history store is not initialized")
	}
	if n <= 0 {
		n = 10
	}

	projectDir = strings.TrimSpace(projectDir)
	var (
		rows *sql.Rows
		err  error
	)
	if projectDir == "" {
		rows, err = h.db.Query(`
			SELECT
				id, query, response, sources, project_dir, project_stack,
				llm_backend, output_format, search_ms, llm_ms, total_ms,
				is_followup, parent_id, created_at
			FROM searches
			ORDER BY created_at DESC, id DESC
			LIMIT ?`, n)
	} else {
		rows, err = h.db.Query(`
			SELECT
				id, query, response, sources, project_dir, project_stack,
				llm_backend, output_format, search_ms, llm_ms, total_ms,
				is_followup, parent_id, created_at
			FROM searches
			WHERE project_dir = ?
			ORDER BY created_at DESC, id DESC
			LIMIT ?`, projectDir, n)
	}
	if err != nil {
		return nil, fmt.Errorf("query recent history: %w", err)
	}
	defer rows.Close()

	return scanRecords(rows)
}

func (h *HistoryStore) Get(id int64) (*SearchRecord, error) {
	if h == nil || h.db == nil {
		return nil, errors.New("history store is not initialized")
	}

	row := h.db.QueryRow(`
		SELECT
			id, query, response, sources, project_dir, project_stack,
			llm_backend, output_format, search_ms, llm_ms, total_ms,
			is_followup, parent_id, created_at
		FROM searches
		WHERE id = ?`, id)

	record, err := scanRecord(row)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (h *HistoryStore) Stats() (*HistoryStats, error) {
	if h == nil || h.db == nil {
		return nil, errors.New("history store is not initialized")
	}

	stats := &HistoryStats{}
	var avgTotalMs float64
	if err := h.db.QueryRow(`
		SELECT COUNT(*), COUNT(DISTINCT project_dir), COALESCE(AVG(total_ms), 0)
		FROM searches`).Scan(&stats.TotalSearches, &stats.UniqueProjects, &avgTotalMs); err != nil {
		return nil, fmt.Errorf("query history stats: %w", err)
	}
	stats.AvgTotalMs = int64(math.Round(avgTotalMs))

	var mostDir sql.NullString
	if err := h.db.QueryRow(`
		SELECT project_dir
		FROM searches
		WHERE project_dir <> ''
		GROUP BY project_dir
		ORDER BY COUNT(*) DESC, project_dir ASC
		LIMIT 1`).Scan(&mostDir); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("query most searched dir: %w", err)
	}
	if mostDir.Valid {
		stats.MostSearchedDir = mostDir.String
	}

	return stats, nil
}

func (h *HistoryStore) Clear() (int64, error) {
	if h == nil || h.db == nil {
		return 0, errors.New("history store is not initialized")
	}

	tx, err := h.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin clear history: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`DELETE FROM searches`)
	if err != nil {
		return 0, fmt.Errorf("delete history rows: %w", err)
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read deleted history rows: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM sqlite_sequence WHERE name = 'searches'`); err != nil {
		return 0, fmt.Errorf("reset history sequence: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit clear history: %w", err)
	}
	return deleted, nil
}

func (h *HistoryStore) Delete(id int64) error {
	if h == nil || h.db == nil {
		return errors.New("history store is not initialized")
	}
	if _, err := h.db.Exec(`DELETE FROM searches WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete search record: %w", err)
	}
	return nil
}

func (h *HistoryStore) Close() error {
	if h == nil || h.db == nil {
		return nil
	}
	return h.db.Close()
}

func scanRecords(rows *sql.Rows) ([]SearchRecord, error) {
	records := make([]SearchRecord, 0)
	for rows.Next() {
		record, err := scanRecordFromRows(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate history rows: %w", err)
	}
	return records, nil
}

func scanRecord(row *sql.Row) (*SearchRecord, error) {
	var (
		record      SearchRecord
		sourcesJSON string
		isFollowUp  int
		parentID    sql.NullInt64
		createdAt   string
	)

	err := row.Scan(
		&record.ID,
		&record.Query,
		&record.Response,
		&sourcesJSON,
		&record.ProjectDir,
		&record.ProjectStack,
		&record.LLMBackend,
		&record.OutputFormat,
		&record.SearchMs,
		&record.LLMMs,
		&record.TotalMs,
		&isFollowUp,
		&parentID,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, fmt.Errorf("scan history record: %w", err)
	}

	if err := hydrateRecord(&record, sourcesJSON, isFollowUp, parentID, createdAt); err != nil {
		return nil, err
	}
	return &record, nil
}

func scanRecordFromRows(rows *sql.Rows) (*SearchRecord, error) {
	var (
		record      SearchRecord
		sourcesJSON string
		isFollowUp  int
		parentID    sql.NullInt64
		createdAt   string
	)

	if err := rows.Scan(
		&record.ID,
		&record.Query,
		&record.Response,
		&sourcesJSON,
		&record.ProjectDir,
		&record.ProjectStack,
		&record.LLMBackend,
		&record.OutputFormat,
		&record.SearchMs,
		&record.LLMMs,
		&record.TotalMs,
		&isFollowUp,
		&parentID,
		&createdAt,
	); err != nil {
		return nil, fmt.Errorf("scan history row: %w", err)
	}

	if err := hydrateRecord(&record, sourcesJSON, isFollowUp, parentID, createdAt); err != nil {
		return nil, err
	}
	return &record, nil
}

func hydrateRecord(record *SearchRecord, sourcesJSON string, isFollowUp int, parentID sql.NullInt64, createdAt string) error {
	record.IsFollowUp = isFollowUp == 1
	if parentID.Valid {
		value := parentID.Int64
		record.ParentID = &value
	}
	if strings.TrimSpace(sourcesJSON) == "" {
		sourcesJSON = "[]"
	}
	if err := json.Unmarshal([]byte(sourcesJSON), &record.Sources); err != nil {
		return fmt.Errorf("unmarshal history sources: %w", err)
	}
	parsedTime, err := parseSQLiteTime(createdAt)
	if err != nil {
		return fmt.Errorf("parse history created_at: %w", err)
	}
	record.CreatedAt = parsedTime
	return nil
}

func parseSQLiteTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}

	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			return ts, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q", value)
}

func buildFTSQuery(query string) string {
	normalized := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r + ('a' - 'A')
		case r >= '0' && r <= '9':
			return r
		case r == '_':
			return r
		default:
			return ' '
		}
	}, query)

	tokens := strings.Fields(normalized)
	parts := make([]string, 0, len(tokens))
	for _, token := range tokens {
		parts = append(parts, token+"*")
	}
	if len(parts) == 0 {
		return strings.TrimSpace(query)
	}
	return strings.Join(parts, " AND ")
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
