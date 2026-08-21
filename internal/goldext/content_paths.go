package goldext

import (
	"path/filepath"
	"sync"
	"time"
)

var contentPaths = struct {
	sync.RWMutex
	rootDir      string
	documentsDir string
}{
	rootDir:      "data",
	documentsDir: "documents",
}

// ConfigureContentPaths sets the filesystem locations used by Markdown
// preprocessors. It must be called after the application configuration loads.
func ConfigureContentPaths(rootDir, documentsDir string) {
	contentPaths.Lock()
	changed := contentPaths.rootDir != rootDir || contentPaths.documentsDir != documentsDir
	contentPaths.rootDir = rootDir
	contentPaths.documentsDir = documentsDir
	contentPaths.Unlock()

	if changed {
		slugCache.mu.Lock()
		slugCache.index = nil
		slugCache.modTime = time.Time{}
		slugCache.mu.Unlock()
	}
}

// DocumentsRoot returns the configured directory that contains wiki documents.
func DocumentsRoot() string {
	contentPaths.RLock()
	defer contentPaths.RUnlock()
	return filepath.Join(contentPaths.rootDir, contentPaths.documentsDir)
}

func homepageRoot() string {
	contentPaths.RLock()
	defer contentPaths.RUnlock()
	return filepath.Join(contentPaths.rootDir, "pages", "home")
}
