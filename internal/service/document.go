package service

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"wiki-go/internal/auth"
	"wiki-go/internal/config"
	"wiki-go/internal/i18n"
	"wiki-go/internal/logger"
	"wiki-go/internal/roles"
	"wiki-go/internal/utils"
)

// Typed errors for service layer functions
var (
	ErrUnauthorized = fmt.Errorf("unauthorized")
	ErrNotFound     = fmt.Errorf("not found")
	ErrConflict     = fmt.Errorf("conflict")
	ErrHasChildren  = fmt.Errorf("document has child pages and cannot be deleted")
)

// Document represents a document in the wiki
type Document struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

// SearchResult represents a search result
type SearchResult struct {
	Title   string `json:"title"`
	Path    string `json:"path"`
	Excerpt string `json:"excerpt"`
}

// CreateDocumentRequest represents the request to create a new document
type CreateDocumentRequest struct {
	Title string `json:"title"`
	Path  string `json:"path"`
	Type  string `json:"type"`
}

// ResolveDocPath resolves a wiki path to its filesystem locations.
// path is the wiki path (e.g., "" or "/" for homepage, "my-page" for regular pages).
// Returns:
//   - docPath: full path to document.md
//   - dirPath: full path to the page's directory
//   - relativePath: path relative to RootDir, used for versioning
//     (e.g., "documents/my-page" or "pages/home")
func ResolveDocPath(cfg *config.Config, path string) (docPath, dirPath, relativePath string) {
	if path == "" || path == "/" {
		docPath = filepath.Join(cfg.Wiki.RootDir, "pages", "home", "document.md")
		dirPath = filepath.Join(cfg.Wiki.RootDir, "pages", "home")
		relativePath = "pages/home"
		return
	}

	path = filepath.Clean(path)
	path = strings.TrimSuffix(path, "/")
	path = strings.ReplaceAll(path, "\\", "/")

	dirPath = filepath.Join(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir, path)
	docPath = filepath.Join(dirPath, "document.md")
	relativePath = "documents/" + strings.TrimPrefix(path, "/")
	return
}

// checkEditorRole verifies the session has admin or editor role
func checkEditorRole(session *auth.Session) error {
	if session == nil || (session.Role != roles.RoleAdmin && session.Role != roles.RoleEditor) {
		return ErrUnauthorized
	}
	return nil
}

// isHomepage checks if the resolved paths correspond to the homepage
func isHomepage(relativePath string) bool {
	return relativePath == "pages/home"
}

// GetSource returns the markdown content of a page.
// If the directory exists but document.md is missing, returns placeholder content.
// Returns ErrNotFound only if the directory doesn't exist.
func GetSource(cfg *config.Config, session *auth.Session, path string) (string, error) {
	if err := checkEditorRole(session); err != nil {
		return "", err
	}

	docPath, dirPath, _ := ResolveDocPath(cfg, path)

	content, err := os.ReadFile(docPath)
	if err != nil {
		if os.IsNotExist(err) {
			dirInfo, dirErr := os.Stat(dirPath)
			if dirErr == nil && dirInfo.IsDir() {
				dirName := filepath.Base(path)
				if dirName == "" || dirName == "." {
					dirName = "Home"
				}
				formattedName := utils.FormatDirName(dirName)
				return fmt.Sprintf("# %s\n\nEnter content here", formattedName), nil
			}
			return "", ErrNotFound
		}
		return "", err
	}

	return string(content), nil
}

// Save saves markdown content to a page, creating version history if the document exists.
func Save(cfg *config.Config, session *auth.Session, path, content string) error {
	if err := checkEditorRole(session); err != nil {
		return err
	}

	docPath, _, relativePath := ResolveDocPath(cfg, path)

	if _, err := os.Stat(docPath); err == nil && cfg.Wiki.MaxVersions > 0 {
		currentContent, err := os.ReadFile(docPath)
		if err == nil && len(currentContent) > 0 {
			timestamp := time.Now().Format("20060102150405")
			versionDir := filepath.Join(cfg.Wiki.RootDir, "versions", relativePath)

			if err := os.MkdirAll(versionDir, 0755); err == nil {
				versionPath := filepath.Join(versionDir, timestamp+".md")
				_ = os.WriteFile(versionPath, currentContent, 0644)
				logger.Info("Created version: %s", versionPath)
				utils.CleanupOldVersions(versionDir, cfg.Wiki.MaxVersions)
			}
		}
	}

	dir := filepath.Dir(docPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := os.WriteFile(docPath, []byte(content), 0644); err != nil {
		return err
	}

	logger.Info("User %s edited document %s", session.Username, filepath.Join(cfg.Wiki.RootDir, relativePath))
	return nil
}

// Create creates a new document with the given title, path, and type.
// Returns the URL path of the created document.
// Returns ErrConflict if the document already exists.
func Create(cfg *config.Config, session *auth.Session, req CreateDocumentRequest) (string, error) {
	if err := checkEditorRole(session); err != nil {
		return "", err
	}

	if req.Title == "" {
		return "", fmt.Errorf("title is required")
	}

	if req.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	cleanPath := utils.SanitizePath(req.Path)
	if cleanPath == "" {
		return "", fmt.Errorf("invalid path after sanitization")
	}

	logger.Debug("Creating document: Title=%s, Path=%s, CleanPath=%s", req.Title, req.Path, cleanPath)

	documentDir := filepath.Join(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir)
	fullPath := filepath.Join(documentDir, cleanPath)

	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return "", err
	}

	docFile := filepath.Join(fullPath, "document.md")

	if _, err := os.Stat(docFile); err == nil {
		return "", ErrConflict
	}

	content := fmt.Sprintf("# %s\n\n%s", req.Title, i18n.Translate("new_doc.default_content"))
	if req.Type == "kanban" {
		content = fmt.Sprintf(
			"---\nlayout: kanban\n---\n\n# %s\n\n%s\n\n#### %s\n\n##### %s\n- [ ] %s\n\n##### %s\n\n##### %s",
			req.Title, i18n.Translate("new_doc.default_content"),
			i18n.Translate("new_doc.kanban_title"),
			i18n.Translate("new_doc.kanban_todo"),
			i18n.Translate("new_doc.kanban_task_example"),
			i18n.Translate("new_doc.kanban_in_progress"),
			i18n.Translate("new_doc.kanban_done"),
		)
	} else if req.Type == "links" {
		content = fmt.Sprintf(
			"---\nlayout: links\n---\n\n# %s\n\n## Web Tools\n- [Example Link](https://example.com) - Sample link description | %s\n\n## Documentation\n- [MDN Docs](https://developer.mozilla.org) - Web development reference | %s",
			req.Title, time.Now().Format("2006-01-02"), time.Now().Format("2006-01-02"),
		)
	}

	if err := os.WriteFile(docFile, []byte(content), 0644); err != nil {
		return "", err
	}

	logger.Info("User %s created document %s", session.Username, filepath.Join(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir, cleanPath))
	return "/" + cleanPath, nil
}

// Delete deletes a page and its versions/comments.
// If protectChildren is true, returns ErrHasChildren when the page has child pages.
// Returns ErrUnauthorized if the session lacks permissions.
// Returns ErrNotFound if the page doesn't exist.
// Refuses to delete the homepage (returns ErrNotFound).
func Delete(cfg *config.Config, session *auth.Session, path string, protectChildren bool) error {
	if err := checkEditorRole(session); err != nil {
		return err
	}

	docPath, _, relativePath := ResolveDocPath(cfg, path)
	if isHomepage(relativePath) {
		return ErrNotFound
	}

	fullPath := filepath.Dir(docPath)

	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}

	if !fileInfo.IsDir() {
		if err := os.Remove(fullPath); err != nil {
			return err
		}
	} else {
		if protectChildren {
			entries, readErr := os.ReadDir(fullPath)
			if readErr == nil {
				for _, e := range entries {
					if e.IsDir() {
						return ErrHasChildren
					}
				}
			}
		}

		if err := os.RemoveAll(fullPath); err != nil {
			return err
		}
	}

	logger.Info("User %s deleted document %s", session.Username, filepath.Join(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir, path))

	versionsPath := filepath.Join(cfg.Wiki.RootDir, "versions", relativePath)

	if _, err := os.Stat(versionsPath); err == nil {
		if err := os.RemoveAll(versionsPath); err != nil {
			logger.Warn("Failed to delete versions directory: %s - %v", versionsPath, err)
		} else {
			logger.Info("Deleted versions directory: %s", versionsPath)
		}
	}

	commentsPath := filepath.Join(cfg.Wiki.RootDir, "comments", relativePath)
	if _, err := os.Stat(commentsPath); err == nil {
		if err := os.RemoveAll(commentsPath); err != nil {
			logger.Warn("Failed to delete comments directory: %s - %v", commentsPath, err)
		} else {
			logger.Info("Deleted comments directory: %s", commentsPath)
		}
	}

	return nil
}

// List returns all documents in the wiki.
func List(cfg *config.Config, session *auth.Session) ([]Document, error) {
	if err := checkEditorRole(session); err != nil {
		return nil, err
	}

	documentsPath := filepath.Join(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir)
	var documents []Document

	err := filepath.WalkDir(documentsPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && d.Name() == "document.md" {
			docDir := filepath.Dir(p)
			relPath, err := filepath.Rel(cfg.Wiki.RootDir, docDir)
			if err != nil {
				return nil
			}

			relPath = strings.ReplaceAll(relPath, "\\", "/")

			title := extractTitleFromMarkdown(p)
			if title == "" {
				title = filepath.Base(docDir)
			}

			if strings.HasPrefix(relPath, cfg.Wiki.DocumentsDir) {
				relPath = strings.TrimPrefix(relPath, cfg.Wiki.DocumentsDir)
				relPath = strings.TrimPrefix(relPath, "/")
			}

			documents = append(documents, Document{
				Title: title,
				Path:  "/" + relPath,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return documents, nil
}

// Search searches for pages matching the query.
// Does NOT perform role checks (matching current SearchHandler behavior).
// The session is used for CanAccessDocument filtering within results.
func Search(cfg *config.Config, session *auth.Session, query string) []SearchResult {
	var results []SearchResult
	searchTerms := parseSearchQuery(query)

	docsPath := filepath.Join(cfg.Wiki.RootDir, cfg.Wiki.DocumentsDir)

	_ = filepath.Walk(docsPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".md") {
			cleanPath := path
			cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")
			prefix := strings.ReplaceAll(docsPath, "\\", "/") + "/"
			cleanPath = strings.TrimPrefix(cleanPath, prefix)
			cleanPath = strings.TrimSuffix(strings.Replace(cleanPath, "document.md", "", 1), ".md")
			urlPath := "/" + cleanPath

			if !auth.CanAccessDocument(urlPath, session, cfg) {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			if matches := matchContent(string(content), searchTerms); matches {
				title := extractTitle(string(content))
				excerpt := extractExcerpt(string(content), searchTerms)

				results = append(results, SearchResult{
					Title:   title,
					Path:    urlPath,
					Excerpt: excerpt,
				})
			}
		}
		return nil
	})

	return results
}

// --- Internal helpers extracted from search.go ---

type SearchTerms struct {
	ExactPhrases []string
	IncludeWords []string
	ExcludeWords []string
}

func parseSearchQuery(query string) SearchTerms {
	var terms SearchTerms

	exactMatches := strings.Count(query, "\"")
	if exactMatches >= 2 {
		for {
			start := strings.Index(query, "\"")
			if start == -1 {
				break
			}
			end := strings.Index(query[start+1:], "\"")
			if end == -1 {
				break
			}
			end += start + 1

			phrase := query[start+1 : end]
			if phrase != "" {
				terms.ExactPhrases = append(terms.ExactPhrases, strings.ToLower(phrase))
			}

			query = query[:start] + query[end+1:]
		}
	}

	words := strings.Fields(query)
	for i := 0; i < len(words); i++ {
		word := strings.ToLower(words[i])

		if word == "not" && i+1 < len(words) {
			terms.ExcludeWords = append(terms.ExcludeWords, strings.ToLower(words[i+1]))
			i++
		} else if word == "and" {
			continue
		} else {
			terms.IncludeWords = append(terms.IncludeWords, word)
		}
	}

	return terms
}

func matchContent(content string, terms SearchTerms) bool {
	content = strings.ToLower(content)

	for _, phrase := range terms.ExactPhrases {
		if !strings.Contains(content, phrase) {
			return false
		}
	}

	for _, word := range terms.IncludeWords {
		if !strings.Contains(content, word) {
			return false
		}
	}

	for _, word := range terms.ExcludeWords {
		if strings.Contains(content, word) {
			return false
		}
	}

	return true
}

func extractTitle(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return "Untitled"
}

func extractExcerpt(content string, terms SearchTerms) string {
	const excerptLength = 200
	content = strings.ToLower(content)

	var matchIndex int
	if len(terms.ExactPhrases) > 0 {
		for _, phrase := range terms.ExactPhrases {
			if idx := strings.Index(content, phrase); idx != -1 {
				matchIndex = idx
				break
			}
		}
	} else if len(terms.IncludeWords) > 0 {
		for _, word := range terms.IncludeWords {
			if idx := strings.Index(content, word); idx != -1 {
				matchIndex = idx
				break
			}
		}
	}

	start := matchIndex - excerptLength/2
	if start < 0 {
		start = 0
	}
	end := start + excerptLength
	if end > len(content) {
		end = len(content)
	}

	excerpt := content[start:end]
	if start > 0 {
		if idx := strings.Index(excerpt, " "); idx != -1 {
			excerpt = "..." + excerpt[idx:]
		}
	}
	if end < len(content) {
		if idx := strings.LastIndex(excerpt, " "); idx != -1 {
			excerpt = excerpt[:idx] + "..."
		}
	}

	return excerpt
}

func extractTitleFromMarkdown(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}

	return ""
}
