package httpapi

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
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
	// Files are immutable once written (new gens get new filenames); long cache is fine.
	w.Header().Set("cache-control", "private, max-age=31536000, immutable")
	w.Header().Set("x-content-type-options", "nosniff")
	w.WriteHeader(http.StatusOK)
	w.Write(img.Bytes)
}

// handleDeleteGenerated removes one persisted image from generated-images/.
// Only the on-disk file is deleted; the client must drop its own URL references.
func (s *Server) handleDeleteGenerated(w http.ResponseWriter, r *http.Request) {
	filename := path.Base(strings.TrimSpace(chi.URLParam(r, "filename")))
	if !generatedNamePattern.MatchString(filename) {
		writeJSON(w, http.StatusBadRequest, errorBody{Error: "無效的圖片檔名。"})
		return
	}
	// Confirm existence so the UI can distinguish already-gone vs deleted.
	if _, ok, err := s.Store.GetImage(filename); err != nil {
		logHandlerErr("generated/delete", err, "filename="+filename)
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	} else if !ok {
		log.Printf("[generated/delete] not found filename=%s", filename)
		writeJSON(w, http.StatusNotFound, errorBody{Error: "找不到該圖片檔案。"})
		return
	}
	if err := s.Store.DeleteImage(filename); err != nil {
		logHandlerErr("generated/delete", err, "filename="+filename+" | tip: 檢查 generated-images 寫入權限")
		writeErr(w, err, http.StatusServiceUnavailable)
		return
	}
	log.Printf("[generated/delete] ok filename=%s", filename)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "filename": filename})
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
