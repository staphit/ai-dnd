package httpapi

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// mimeByExt matches the mime table in server.mjs.
var mimeByExt = map[string]string{
	".html":  "text/html; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".svg":   "image/svg+xml",
	".woff2": "font/woff2",
	".json":  "application/json; charset=utf-8",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".png":   "image/png",
	".webp":  "image/webp",
}

func mimeForPath(p string) string {
	if m, ok := mimeByExt[strings.ToLower(filepath.Ext(p))]; ok {
		return m
	}
	return "application/octet-stream"
}

// plainStatus writes a bare status + body, matching the Node original's
// response.writeHead(status).end(msg) (no trailing newline, no forced headers,
// unlike http.Error).
func plainStatus(w http.ResponseWriter, status int, body string) {
	w.WriteHeader(status)
	w.Write([]byte(body))
}

var generatedNamePattern = regexp.MustCompile(`(?i)^[a-zA-Z0-9-]+\.(?:png|jpe?g|webp)$`)

func (s *Server) serveGenerated(w http.ResponseWriter, r *http.Request) {
	urlPath := decodePath(r.URL)
	filename := path.Base(urlPath)
	if !generatedNamePattern.MatchString(filename) {
		plainStatus(w, http.StatusBadRequest, "Invalid image path")
		return
	}
	img, ok, err := s.Store.GetImage(filename)
	if err != nil || !ok {
		plainStatus(w, http.StatusNotFound, "Image not found")
		return
	}
	w.Header().Set("content-type", img.Mime)
	w.Header().Set("cache-control", "private, max-age=31536000, immutable")
	w.Header().Set("x-content-type-options", "nosniff")
	w.WriteHeader(http.StatusOK)
	w.Write(img.Bytes)
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	urlPath := decodePath(r.URL)
	requested := "index.html"
	if urlPath != "/" {
		requested = strings.TrimLeft(urlPath, "/")
	}
	filePath := filepath.Join(s.WebDist, requested)
	if filePath != s.WebDist && !strings.HasPrefix(filePath, s.WebDist+string(filepath.Separator)) {
		plainStatus(w, http.StatusForbidden, "Forbidden")
		return
	}

	info, err := os.Stat(filePath)
	if err == nil && info.IsDir() {
		filePath = filepath.Join(filePath, "index.html")
		info, err = os.Stat(filePath)
	}
	if err == nil && !info.IsDir() {
		if body, rerr := os.ReadFile(filePath); rerr == nil {
			w.Header().Set("content-type", mimeForPath(filePath))
			w.Header().Set("x-content-type-options", "nosniff")
			w.WriteHeader(http.StatusOK)
			w.Write(body)
			return
		}
	}

	if index, ierr := os.ReadFile(filepath.Join(s.WebDist, "index.html")); ierr == nil {
		w.Header().Set("content-type", mimeByExt[".html"])
		w.WriteHeader(http.StatusOK)
		w.Write(index)
		return
	}
	plainStatus(w, http.StatusNotFound, "Build the web app first (npm run build in frontend/)")
}

// decodePath mirrors decodeURIComponent(new URL(url).pathname).
func decodePath(u *url.URL) string {
	if decoded, err := url.PathUnescape(u.EscapedPath()); err == nil {
		return decoded
	}
	return u.Path
}
