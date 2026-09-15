package main

import (
	"bytes"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const maxLocalPreviewBytes int64 = 2 * 1024 * 1024

type localFileEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	Size     int64  `json:"size,omitempty"`
	Modified string `json:"modified,omitempty"`
}

type localFileList struct {
	Path    string           `json:"path"`
	Entries []localFileEntry `json:"entries"`
}

type localFilePreview struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Mime     string `json:"mime"`
	Binary   bool   `json:"binary"`
	Content  string `json:"content,omitempty"`
	Modified string `json:"modified,omitempty"`
}

func registerLocalFileRoutes(mux *http.ServeMux, state *appState) {
	mux.HandleFunc("GET /local/files", func(w http.ResponseWriter, r *http.Request) {
		project := state.projectPath()
		target, rel, err := resolveProjectEntry(project, r.URL.Query().Get("path"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: err.Error()})
			return
		}
		info, err := os.Stat(target)
		if err != nil {
			writeJSON(w, http.StatusNotFound, jsonError{Error: err.Error()})
			return
		}
		if !info.IsDir() {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "requested path is not a directory"})
			return
		}
		entries, err := os.ReadDir(target)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, jsonError{Error: err.Error()})
			return
		}
		sort.SliceStable(entries, func(i, j int) bool {
			if entries[i].IsDir() != entries[j].IsDir() {
				return entries[i].IsDir()
			}
			return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
		})
		result := localFileList{Path: rel, Entries: make([]localFileEntry, 0, len(entries))}
		for _, entry := range entries {
			if entry.Name() == ".git" && entry.IsDir() {
				continue
			}
			entryType := "file"
			if entry.IsDir() {
				entryType = "directory"
			} else if entry.Type()&os.ModeSymlink != 0 {
				entryType = "symlink"
			}
			item := localFileEntry{
				Name: entry.Name(),
				Path: slashJoin(rel, entry.Name()),
				Type: entryType,
			}
			if fileInfo, infoErr := entry.Info(); infoErr == nil {
				if !fileInfo.IsDir() {
					item.Size = fileInfo.Size()
				}
				item.Modified = fileInfo.ModTime().UTC().Format("2006-01-02T15:04:05Z")
			}
			result.Entries = append(result.Entries, item)
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /local/file", func(w http.ResponseWriter, r *http.Request) {
		project := state.projectPath()
		target, rel, err := resolveProjectEntry(project, r.URL.Query().Get("path"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: err.Error()})
			return
		}
		info, err := os.Stat(target)
		if err != nil {
			writeJSON(w, http.StatusNotFound, jsonError{Error: err.Error()})
			return
		}
		if !info.Mode().IsRegular() {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "requested path is not a regular file"})
			return
		}
		if info.Size() > maxLocalPreviewBytes {
			writeJSON(w, http.StatusRequestEntityTooLarge, jsonError{Error: fmt.Sprintf("file is too large to preview (max %d MiB)", maxLocalPreviewBytes/(1024*1024))})
			return
		}
		data, err := os.ReadFile(target)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, jsonError{Error: err.Error()})
			return
		}
		binary := bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data)
		mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(target)))
		if mimeType == "" {
			if binary {
				mimeType = "application/octet-stream"
			} else {
				mimeType = "text/plain; charset=utf-8"
			}
		}
		preview := localFilePreview{
			Name: filepath.Base(target),
			Path: rel,
			Size: info.Size(),
			Mime: mimeType,
			Binary: binary,
			Modified: info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		}
		if !binary {
			preview.Content = string(data)
		}
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, preview)
	})
}

func slashJoin(parent, name string) string {
	parent = strings.Trim(strings.ReplaceAll(parent, "\\", "/"), "/")
	name = strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
	if parent == "" {
		return name
	}
	if name == "" {
		return parent
	}
	return parent + "/" + name
}

func resolveProjectEntry(project, requested string) (string, string, error) {
	if strings.TrimSpace(project) == "" {
		return "", "", fmt.Errorf("no project is selected")
	}
	root, err := filepath.Abs(project)
	if err != nil {
		return "", "", err
	}
	if evaluated, evalErr := filepath.EvalSymlinks(root); evalErr == nil {
		root = evaluated
	}

	raw := strings.TrimSpace(requested)
	if raw == "" || raw == "." {
		return root, "", nil
	}
	candidate := filepath.FromSlash(raw)
	if filepath.IsAbs(candidate) || filepath.VolumeName(candidate) != "" {
		return "", "", fmt.Errorf("path must be relative to the selected project")
	}
	clean := filepath.Clean(candidate)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("path escapes the selected project")
	}
	target := filepath.Join(root, clean)
	evaluated, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(root, evaluated)
	if err != nil {
		return "", "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("path escapes the selected project")
	}
	if rel == "." {
		rel = ""
	}
	return evaluated, filepath.ToSlash(rel), nil
}
