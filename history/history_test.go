package history

import (
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewHistoryStoreCreatesDatabaseAndTables(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "history.db")
	store, err := NewHistoryStore(dbPath)
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}
	defer store.Close()

	for _, table := range []string{"searches", "searches_fts"} {
		var name string
		if err := store.db.QueryRow(`SELECT name FROM sqlite_master WHERE name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}
}

func TestSaveAndGetRoundTrip(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	parentID := mustSave(t, store, sampleRecord("existing parent"))
	record := sampleRecord("how to add middleware")
	record.ParentID = &parentID

	id, err := store.Save(record)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.Query != record.Query || got.Response != record.Response || got.ProjectStack != record.ProjectStack {
		t.Fatalf("unexpected roundtrip record: %#v", got)
	}
	if got.ParentID == nil || *got.ParentID != parentID {
		t.Fatalf("expected parent id %d, got %#v", parentID, got.ParentID)
	}
	if len(got.Sources) != len(record.Sources) || got.Sources[0].URL != record.Sources[0].URL {
		t.Fatalf("unexpected roundtrip sources: %#v", got.Sources)
	}
}

func TestSaveFollowUpWithParentID(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	parentID, err := store.Save(sampleRecord("parent"))
	if err != nil {
		t.Fatalf("save parent: %v", err)
	}

	followup := sampleRecord("follow-up")
	followup.IsFollowUp = true
	followup.ParentID = &parentID

	id, err := store.Save(followup)
	if err != nil {
		t.Fatalf("save follow-up: %v", err)
	}

	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ParentID == nil || *got.ParentID != parentID {
		t.Fatalf("expected parent id %d, got %#v", parentID, got.ParentID)
	}
	if !got.IsFollowUp {
		t.Fatalf("expected follow-up record")
	}
}

func TestSearchWithFTS(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	mustSave(t, store, sampleRecordWithResponse("TCP handshake", "A TCP handshake starts with SYN."))
	mustSave(t, store, sampleRecordWithResponse("UDP performance", "UDP is faster for some workloads."))
	mustSave(t, store, sampleRecordWithResponse("HTTP/2 multiplexing", "HTTP/2 multiplexes streams."))

	results, err := store.Search("TCP", 10)
	if err != nil {
		t.Fatalf("Search TCP: %v", err)
	}
	if len(results) == 0 || results[0].Query != "TCP handshake" {
		t.Fatalf("expected TCP result first, got %#v", results)
	}

	results, err = store.Search("performance", 10)
	if err != nil {
		t.Fatalf("Search performance: %v", err)
	}
	if len(results) == 0 || results[0].Query != "UDP performance" {
		t.Fatalf("expected UDP performance result, got %#v", results)
	}

	results, err = store.Search("nonexistent", 10)
	if err != nil {
		t.Fatalf("Search nonexistent: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no results, got %#v", results)
	}
}

func TestSearchWithDeveloperStyleTokens(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	mustSave(t, store, sampleRecordWithResponse("go/chi middleware", "Use chi middleware for request handling."))
	mustSave(t, store, sampleRecordWithResponse("config path", "See cmd/server/main.go for config loading."))

	results, err := store.Search("go/chi", 10)
	if err != nil {
		t.Fatalf("Search go/chi: %v", err)
	}
	if len(results) == 0 || results[0].Query != "go/chi middleware" {
		t.Fatalf("expected go/chi result, got %#v", results)
	}

	results, err = store.Search("main.go", 10)
	if err != nil {
		t.Fatalf("Search main.go: %v", err)
	}
	if len(results) == 0 || results[0].Query != "config path" {
		t.Fatalf("expected path-ish result, got %#v", results)
	}

	results, err = store.Search("a-b", 10)
	if err != nil {
		t.Fatalf("Search a-b should not error: %v", err)
	}
	if results == nil {
		t.Fatalf("expected non-nil results slice")
	}
}

func TestRecentReturnsNewestFirst(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	base := time.Now().Add(-5 * time.Hour)
	for i := 0; i < 5; i++ {
		record := sampleRecord("query")
		record.Query = "query " + string(rune('A'+i))
		record.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		mustSave(t, store, record)
	}

	results, err := store.Recent(3, "")
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].Query != "query E" || results[1].Query != "query D" || results[2].Query != "query C" {
		t.Fatalf("expected newest-first ordering, got %#v", results)
	}
}

func TestRecentFiltersByProjectDir(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	recordA := sampleRecord("a")
	recordA.ProjectDir = "/project/a"
	recordB := sampleRecord("b")
	recordB.ProjectDir = "/project/b"
	mustSave(t, store, recordA)
	mustSave(t, store, recordB)

	results, err := store.Recent(10, "/project/a")
	if err != nil {
		t.Fatalf("Recent filtered: %v", err)
	}
	if len(results) != 1 || results[0].ProjectDir != "/project/a" {
		t.Fatalf("expected only /project/a results, got %#v", results)
	}
}

func TestDeleteRemovesFromMainTableAndFTS(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	id := mustSave(t, store, sampleRecordWithResponse("tcp keepalive", "keepalive details"))
	results, err := store.Search("keepalive", 10)
	if err != nil {
		t.Fatalf("Search before delete: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected one result before delete, got %#v", results)
	}

	if err := store.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows after delete, got %v", err)
	}

	results, err = store.Search("keepalive", 10)
	if err != nil {
		t.Fatalf("Search after delete: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no search results after delete, got %#v", results)
	}
}

func TestClearRemovesAllRowsAndSearchIndex(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	for _, query := range []string{"tcp handshake", "udp performance", "http3"} {
		mustSave(t, store, sampleRecord(query))
	}

	deleted, err := store.Clear()
	if err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("expected 3 deleted records, got %d", deleted)
	}

	recent, err := store.Recent(10, "")
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(recent) != 0 {
		t.Fatalf("expected no recent records after clear, got %#v", recent)
	}

	results, err := store.Search("tcp", 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected no search results after clear, got %#v", results)
	}

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalSearches != 0 || stats.UniqueProjects != 0 || stats.AvgTotalMs != 0 {
		t.Fatalf("expected empty stats after clear, got %#v", stats)
	}
}

func TestStatsReturnsCorrectAggregates(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	records := []*SearchRecord{
		{Query: "q1", Response: "r1", ProjectDir: "/a", TotalMs: 100},
		{Query: "q2", Response: "r2", ProjectDir: "/a", TotalMs: 300},
		{Query: "q3", Response: "r3", ProjectDir: "/b", TotalMs: 500},
		{Query: "q4", Response: "r4", ProjectDir: "/b", TotalMs: 700},
		{Query: "q5", Response: "r5", ProjectDir: "/b", TotalMs: 900},
	}
	for _, record := range records {
		enriched := sampleRecord(record.Query)
		enriched.Response = record.Response
		enriched.ProjectDir = record.ProjectDir
		enriched.TotalMs = record.TotalMs
		mustSave(t, store, enriched)
	}

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalSearches != 5 || stats.UniqueProjects != 2 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
	if stats.AvgTotalMs != 500 {
		t.Fatalf("expected avg total ms 500, got %d", stats.AvgTotalMs)
	}
	if stats.MostSearchedDir != "/b" {
		t.Fatalf("expected most searched dir /b, got %q", stats.MostSearchedDir)
	}
}

func TestStatsRoundsFractionalAverageLatency(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	for _, totalMs := range []int64{6377, 6378} {
		record := sampleRecord("fractional avg")
		record.TotalMs = totalMs
		mustSave(t, store, record)
	}

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.AvgTotalMs != 6378 {
		t.Fatalf("expected rounded avg total ms 6378, got %d", stats.AvgTotalMs)
	}
}

func TestSourcesJSONSerializationRoundTrip(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	record := sampleRecord("with sources")
	record.Sources = []Source{
		{Title: "One", URL: "https://example.com/1", Domain: "example.com"},
		{Title: "Two", URL: "https://example.com/2", Domain: "example.com"},
		{Title: "Three", URL: "https://example.com/3", Domain: "example.com"},
	}

	id := mustSave(t, store, record)
	got, err := store.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.Sources) != 3 || got.Sources[2].Title != "Three" {
		t.Fatalf("unexpected source roundtrip: %#v", got.Sources)
	}
}

func TestConcurrentSavesDoNotCorrupt(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			record := sampleRecord("concurrent")
			record.Query = "concurrent " + string(rune('A'+idx))
			if _, err := store.Save(record); err != nil {
				t.Errorf("Save %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalSearches != 10 {
		t.Fatalf("expected 10 saved records, got %d", stats.TotalSearches)
	}
}

func newTestStore(t *testing.T) *HistoryStore {
	t.Helper()
	store, err := NewHistoryStore(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}
	return store
}

func sampleRecord(query string) *SearchRecord {
	return &SearchRecord{
		Query:        query,
		Response:     "response for " + query,
		Sources:      []Source{{Title: "Example", URL: "https://example.com", Domain: "example.com"}},
		ProjectDir:   "/workspace/project",
		ProjectStack: "go/chi",
		LLMBackend:   "groq/llama-3.3-70b",
		OutputFormat: "concise",
		SearchMs:     120,
		LLMMs:        340,
		TotalMs:      460,
	}
}

func sampleRecordWithResponse(query, response string) *SearchRecord {
	record := sampleRecord(query)
	record.Response = response
	return record
}

func mustSave(t *testing.T, store *HistoryStore, record *SearchRecord) int64 {
	t.Helper()
	id, err := store.Save(record)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	return id
}

func int64Ptr(value int64) *int64 {
	return &value
}
