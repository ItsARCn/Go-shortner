package embedded

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:public
var publicFS embed.FS

// GetPublicFS returns a sub-filesystem rooted at 'public'.
func GetPublicFS() (fs.FS, error) {
	return fs.Sub(publicFS, "public")
}

// GetFileServer returns an http.Handler serving the embedded assets.
func GetFileServer() http.Handler {
	sub, err := fs.Sub(publicFS, "public")
	if err != nil {
		return http.FileServer(http.FS(publicFS))
	}
	return http.FileServer(http.FS(sub))
}

// ReadFile reads an embedded file by its relative path inside 'public'.
func ReadFile(name string) ([]byte, error) {
	sub, err := fs.Sub(publicFS, "public")
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(sub, name)
}
