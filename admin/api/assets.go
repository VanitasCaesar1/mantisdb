package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed assets/dist
var assetsFS embed.FS

// GetAssetsFS returns the embedded assets filesystem
func GetAssetsFS() http.FileSystem {
	// Strip the "assets/dist" prefix so files are served from root
	stripped, err := fs.Sub(assetsFS, "assets/dist")
	if err != nil {
		panic("failed to create sub filesystem: " + err.Error())
	}
	return http.FS(stripped)
}

// AssetsAvailable checks if embedded assets are available
func AssetsAvailable() bool {
	// Try to read index.html to verify assets are embedded
	_, err := assetsFS.ReadFile("assets/dist/index.html")
	return err == nil
}
