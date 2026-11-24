package manga

import (
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

// FilePrefix is where saved manga files are served in the app window. It
// differs from the novel reader's /library/ so the two never collide.
const FilePrefix = "/mangalib/"

// Handler serves saved files to the app window (in-process, no ports):
//
//	/mangalib/{slug}/cover.{ext}
//	/mangalib/{slug}/ch/{key}/{page}.{ext}
type Handler struct{ Store *Store }

var (
	coverName = regexp.MustCompile(`^cover\.(jpg|png|webp|gif|avif)$`)
	pagePath  = regexp.MustCompile(`^ch/\d{5}/\d{3}\.(jpg|png|webp|gif|avif)$`)
)

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, FilePrefix)
	slug, sub, ok := strings.Cut(rest, "/")
	if !ok || !validSlug(slug) || !(coverName.MatchString(sub) || pagePath.MatchString(sub)) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-cache") // revalidated via Last-Modified; a re-download replaces files
	http.ServeFile(w, r, filepath.Join(h.Store.Dir(slug), filepath.FromSlash(sub)))
}
