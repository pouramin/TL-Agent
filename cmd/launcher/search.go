package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxProjectSearchQueryBytes   = 512
	maxProjectSearchFileBytes    = 2 * 1024 * 1024
	maxProjectSearchResults      = 500
	maxProjectSearchSnippetRunes = 240
)

var defaultProjectSearchIgnoredDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"dist":         {},
	"build":        {},
	"out":          {},
	"coverage":     {},
	".next":        {},
}

type projectSearchMatch struct {
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Start  int64  `json:"start"`
	End    int64  `json:"end"`
	Match  string `json:"match"`
	Text   string `json:"text"`
}

type projectSearchFileResult struct {
	Path    string               `json:"path"`
	Matches []projectSearchMatch `json:"matches"`
}

type projectSearchResponse struct {
	Query         string                    `json:"query"`
	CaseSensitive bool                      `json:"caseSensitive"`
	Files         []projectSearchFileResult `json:"files"`
	MatchCount    int                       `json:"matchCount"`
	FileCount     int                       `json:"fileCount"`
	Truncated     bool                      `json:"truncated"`
}

type projectSearchOptions struct {
	Query         string
	CaseSensitive bool
	Includes      []string
	Excludes      []string
	Limit         int
}

func registerProjectSearchRoutes(mux *http.ServeMux, state *appState) {
	mux.HandleFunc("GET /local/search", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		if strings.TrimSpace(query) == "" {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "search query cannot be empty"})
			return
		}
		if len([]byte(query)) > maxProjectSearchQueryBytes {
			writeJSON(w, http.StatusRequestEntityTooLarge, jsonError{Error: fmt.Sprintf("search query is too large (max %d bytes)", maxProjectSearchQueryBytes)})
			return
		}

		limit := maxProjectSearchResults
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 {
				writeJSON(w, http.StatusBadRequest, jsonError{Error: "limit must be a positive integer"})
				return
			}
			if parsed < limit {
				limit = parsed
			}
		}

		includes, excludes := parseProjectSearchPatterns(r.URL.Query().Get("include"), r.URL.Query().Get("exclude"))
		response, err := searchProject(r.Context(), state.projectPath(), projectSearchOptions{
			Query:         query,
			CaseSensitive: parseSearchBool(r.URL.Query().Get("caseSensitive")),
			Includes:      includes,
			Excludes:      excludes,
			Limit:         limit,
		})
		if err != nil {
			if r.Context().Err() != nil {
				return
			}
			writeJSON(w, http.StatusInternalServerError, jsonError{Error: err.Error()})
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, response)
	})
}

func searchProject(ctx context.Context, project string, options projectSearchOptions) (projectSearchResponse, error) {
	root, err := canonicalProjectRoot(project)
	if err != nil {
		return projectSearchResponse{}, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return projectSearchResponse{}, err
	}
	if !info.IsDir() {
		return projectSearchResponse{}, fmt.Errorf("selected project is not a directory")
	}

	limit := options.Limit
	if limit <= 0 || limit > maxProjectSearchResults {
		limit = maxProjectSearchResults
	}
	response := projectSearchResponse{
		Query:         options.Query,
		CaseSensitive: options.CaseSensitive,
		Files:         make([]projectSearchFileResult, 0),
	}
	needle := options.Query
	var insensitivePattern *regexp.Regexp
	if !options.CaseSensitive {
		insensitivePattern, err = regexp.Compile(`(?i:` + regexp.QuoteMeta(needle) + `)`)
		if err != nil {
			return projectSearchResponse{}, fmt.Errorf("compile search query: %w", err)
		}
	}

	walkErr := filepath.WalkDir(root, func(target string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if target == root {
			return nil
		}

		relNative, err := filepath.Rel(root, target)
		if err != nil {
			return nil
		}
		rel := filepath.ToSlash(relNative)
		if rel == "." || rel == "" {
			return nil
		}

		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if _, ignored := defaultProjectSearchIgnoredDirs[strings.ToLower(entry.Name())]; ignored {
				return filepath.SkipDir
			}
			if excludedByProjectSearch(rel+"/", options.Excludes) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if !includedByProjectSearch(rel, options.Includes) || excludedByProjectSearch(rel, options.Excludes) {
			return nil
		}

		fileInfo, err := entry.Info()
		if err != nil || fileInfo.Size() > maxProjectSearchFileBytes {
			return nil
		}
		data, err := os.ReadFile(target)
		if err != nil {
			return nil
		}
		if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
			return nil
		}

		text := string(data)
		if options.CaseSensitive {
			if !strings.Contains(text, needle) {
				return nil
			}
		} else if insensitivePattern.FindStringIndex(text) == nil {
			return nil
		}

		fileResult := projectSearchFileResult{Path: rel, Matches: make([]projectSearchMatch, 0)}
		lineStartByte := 0
		lineNumber := 1
		for lineStartByte <= len(text) {
			if err := ctx.Err(); err != nil {
				return err
			}
			lineEndByte := strings.IndexByte(text[lineStartByte:], '\n')
			if lineEndByte < 0 {
				lineEndByte = len(text)
			} else {
				lineEndByte += lineStartByte
			}
			lineText := strings.TrimSuffix(text[lineStartByte:lineEndByte], "\r")
			matchRanges := make([][]int, 0)
			if options.CaseSensitive {
				searchFrom := 0
				for searchFrom <= len(lineText) {
					matchIndex := strings.Index(lineText[searchFrom:], needle)
					if matchIndex < 0 {
						break
					}
					matchIndex += searchFrom
					matchRanges = append(matchRanges, []int{matchIndex, matchIndex + len(needle)})
					searchFrom = matchIndex + len(needle)
				}
			} else {
				matchRanges = insensitivePattern.FindAllStringIndex(lineText, -1)
			}

			for _, matchRange := range matchRanges {
				matchIndex, matchEnd := matchRange[0], matchRange[1]
				matched := lineText[matchIndex:matchEnd]
				column := utf8.RuneCountInString(lineText[:matchIndex]) + 1
				fileResult.Matches = append(fileResult.Matches, projectSearchMatch{
					Line:   lineNumber,
					Column: column,
					Start:  int64(lineStartByte + matchIndex),
					End:    int64(lineStartByte + matchEnd),
					Match:  matched,
					Text:   projectSearchSnippet(lineText),
				})
				response.MatchCount++
				if response.MatchCount >= limit {
					response.Truncated = true
					response.Files = append(response.Files, fileResult)
					response.FileCount = len(response.Files)
					return errSearchLimitReached
				}
			}

			if lineEndByte >= len(text) {
				break
			}
			lineStartByte = lineEndByte + 1
			lineNumber++
		}
		if len(fileResult.Matches) > 0 {
			response.Files = append(response.Files, fileResult)
		}
		return nil
	})
	if walkErr != nil && walkErr != errSearchLimitReached {
		if walkErr == context.Canceled || walkErr == context.DeadlineExceeded {
			return projectSearchResponse{}, walkErr
		}
		return projectSearchResponse{}, walkErr
	}
	response.FileCount = len(response.Files)
	return response, nil
}

var errSearchLimitReached = fmt.Errorf("project search result limit reached")

func parseSearchBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseProjectSearchPatterns(includeRaw, excludeRaw string) ([]string, []string) {
	includes := make([]string, 0)
	excludes := make([]string, 0)
	for _, token := range splitSearchPatterns(includeRaw) {
		if strings.HasPrefix(token, "!") {
			if trimmed := strings.TrimSpace(strings.TrimPrefix(token, "!")); trimmed != "" {
				excludes = append(excludes, trimmed)
			}
			continue
		}
		includes = append(includes, token)
	}
	excludes = append(excludes, splitSearchPatterns(excludeRaw)...)
	return includes, excludes
}

func splitSearchPatterns(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		if value := normalizeSearchPattern(field); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func normalizeSearchPattern(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.TrimPrefix(value, "./")
	value = strings.TrimLeft(value, "/")
	return value
}

func includedByProjectSearch(rel string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, pattern := range patterns {
		if projectSearchPatternMatches(pattern, rel) {
			return true
		}
	}
	return false
}

func excludedByProjectSearch(rel string, patterns []string) bool {
	for _, pattern := range patterns {
		if projectSearchPatternMatches(pattern, rel) {
			return true
		}
	}
	return false
}

func projectSearchPatternMatches(pattern, rel string) bool {
	pattern = normalizeSearchPattern(pattern)
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	if pattern == "" || rel == "" {
		return false
	}
	if strings.Contains(pattern, "..") {
		return false
	}
	if !strings.Contains(pattern, "/") {
		if matched, err := path.Match(pattern, path.Base(strings.TrimSuffix(rel, "/"))); err == nil && matched {
			return true
		}
	}
	if matched, err := path.Match(pattern, rel); err == nil && matched {
		return true
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(rel, prefix)
	}
	return false
}

func projectSearchSnippet(line string) string {
	runes := []rune(strings.TrimSpace(line))
	if len(runes) <= maxProjectSearchSnippetRunes {
		return string(runes)
	}
	return string(runes[:maxProjectSearchSnippetRunes-1]) + "…"
}
