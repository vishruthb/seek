package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	attachmentPerFileLimit   = 24 * 1024
	attachmentTotalByteLimit = 96 * 1024
	attachmentFileIndexLimit = 4000
)

var (
	attachmentTokenRE      = regexp.MustCompile(`@\[(.*?)\]`)
	errAttachmentFileLimit = errors.New("attachment file index limit reached")
)

type AttachedFile struct {
	DisplayPath string
	FullPath    string
	Content     string
	Language    string
	Truncated   bool
}

type attachmentCompletion struct {
	Start  int
	End    int
	Prefix string
}

func prepareAttachedFiles(query, workingDir string) (string, []AttachedFile, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", nil, nil
	}

	matches := attachmentTokenRE.FindAllStringSubmatchIndex(query, -1)
	if len(matches) == 0 {
		return query, nil, nil
	}

	baseDir, err := attachmentBaseDir(workingDir)
	if err != nil {
		return "", nil, err
	}

	var (
		attachments []AttachedFile
		seen        = make(map[string]struct{}, len(matches))
		totalBytes  int
		builder     strings.Builder
		last        int
	)

	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		builder.WriteString(query[last:match[0]])

		rawPath := strings.TrimSpace(query[match[2]:match[3]])
		if rawPath == "" {
			return "", nil, errors.New("attachment path is empty")
		}

		fullPath, displayPath, err := resolveAttachmentPath(baseDir, rawPath)
		if err != nil {
			return "", nil, err
		}

		builder.WriteString(displayPath)
		last = match[1]

		if _, ok := seen[fullPath]; ok {
			continue
		}

		file, err := loadAttachedFile(fullPath, displayPath, &totalBytes)
		if err != nil {
			return "", nil, err
		}

		seen[fullPath] = struct{}{}
		attachments = append(attachments, file)
	}

	builder.WriteString(query[last:])
	return strings.TrimSpace(builder.String()), attachments, nil
}

func activeAttachmentCompletion(value string, cursor int) (attachmentCompletion, bool) {
	runes := []rune(value)
	cursor = max(0, min(cursor, len(runes)))

	start := -1
	for idx := cursor - 2; idx >= 0; idx-- {
		if runes[idx] == '@' && runes[idx+1] == '[' {
			start = idx
			break
		}
	}
	if start < 0 {
		return attachmentCompletion{}, false
	}
	for idx := start + 2; idx < cursor; idx++ {
		if runes[idx] == ']' {
			return attachmentCompletion{}, false
		}
	}

	end := cursor
	for idx := cursor; idx < len(runes); idx++ {
		if runes[idx] == ']' {
			end = idx + 1
			break
		}
	}

	return attachmentCompletion{
		Start:  start,
		End:    end,
		Prefix: string(runes[start+2 : cursor]),
	}, true
}

func insertAttachmentValue(value string, completion attachmentCompletion, suggestion string) (string, int) {
	suggestion = strings.TrimSpace(suggestion)
	runes := []rune(value)
	replacement := []rune("@[" + suggestion + "]")
	if completion.End >= len(runes) {
		replacement = append(replacement, ' ')
	}

	next := make([]rune, 0, len(runes)-max(0, completion.End-completion.Start)+len(replacement))
	next = append(next, runes[:completion.Start]...)
	next = append(next, replacement...)
	next = append(next, runes[completion.End:]...)
	return string(next), completion.Start + len(replacement)
}

func listLocalFiles(rootDir string) ([]string, error) {
	rootDir, err := attachmentBaseDir(rootDir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, 256)
	err = filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if path == rootDir {
			return nil
		}

		name := d.Name()
		if d.IsDir() {
			if shouldSkipAttachmentDir(name) {
				return filepath.SkipDir
			}
			return nil
		}

		if shouldSkipAttachmentFile(name) {
			return nil
		}

		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return nil
		}

		files = append(files, filepath.ToSlash(rel))
		if len(files) >= attachmentFileIndexLimit {
			return errAttachmentFileLimit
		}
		return nil
	})
	if err != nil && !errors.Is(err, errAttachmentFileLimit) {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

func attachmentBaseDir(workingDir string) (string, error) {
	baseDir := strings.TrimSpace(workingDir)
	if baseDir == "" {
		var err error
		baseDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve working directory for attachments: %w", err)
		}
	}
	baseDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve attachment root: %w", err)
	}
	baseDir = filepath.Clean(baseDir)
	realBaseDir, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve attachment root %q: %w", baseDir, err)
	}
	return filepath.Clean(realBaseDir), nil
}

func resolveAttachmentPath(baseDir, rawPath string) (string, string, error) {
	path := filepath.Clean(strings.TrimSpace(rawPath))
	if path == "" || path == "." {
		return "", "", errors.New("attachment path is empty")
	}

	fullPath := path
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(baseDir, fullPath)
	}

	fullPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve attachment path %q: %w", rawPath, err)
	}
	fullPath = filepath.Clean(fullPath)

	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve attachment path %q: %w", rawPath, err)
	}
	fullPath = filepath.Clean(realPath)

	rel, err := filepath.Rel(baseDir, fullPath)
	if err != nil {
		return "", "", fmt.Errorf("resolve attachment path %q: %w", rawPath, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("attachment %q is outside %s", rawPath, baseDir)
	}

	return fullPath, filepath.ToSlash(rel), nil
}

func loadAttachedFile(fullPath, displayPath string, totalBytes *int) (AttachedFile, error) {
	info, err := os.Stat(fullPath)
	if err != nil {
		return AttachedFile{}, fmt.Errorf("open attachment %q: %w", displayPath, err)
	}
	if info.IsDir() {
		return AttachedFile{}, fmt.Errorf("attachment %q is a directory", displayPath)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return AttachedFile{}, fmt.Errorf("open attachment %q: %w", displayPath, err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, attachmentPerFileLimit+1))
	if err != nil {
		return AttachedFile{}, fmt.Errorf("read attachment %q: %w", displayPath, err)
	}

	truncated := len(data) > attachmentPerFileLimit
	if truncated {
		data = data[:attachmentPerFileLimit]
	}

	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return AttachedFile{}, fmt.Errorf("attachment %q is not a UTF-8 text file", displayPath)
	}

	if totalBytes != nil && *totalBytes+len(data) > attachmentTotalByteLimit {
		return AttachedFile{}, fmt.Errorf("attachments exceed %d KB", attachmentTotalByteLimit/1024)
	}
	if totalBytes != nil {
		*totalBytes += len(data)
	}

	return AttachedFile{
		DisplayPath: displayPath,
		FullPath:    fullPath,
		Content:     strings.TrimRight(string(data), "\n"),
		Language:    languageFromFilename(displayPath),
		Truncated:   truncated,
	}, nil
}

func shouldSkipAttachmentDir(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, ".") {
		return true
	}

	switch name {
	case "node_modules", "vendor", "dist", "build", "out", ".next", "coverage":
		return true
	default:
		return false
	}
}

func shouldSkipAttachmentFile(name string) bool {
	name = strings.TrimSpace(name)
	return name == "" || strings.HasPrefix(name, ".")
}

func languageFromFilename(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return "go"
	case ".js", ".cjs", ".mjs":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".jsx":
		return "jsx"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".sh", ".bash", ".zsh":
		return "bash"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".md":
		return "markdown"
	case ".sql":
		return "sql"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".c":
		return "c"
	case ".cc", ".cpp", ".cxx", ".hpp", ".hh", ".hxx":
		return "cpp"
	default:
		return ""
	}
}
