package server

import (
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/psyb0t/ctxerrors"
	"github.com/psyb0t/ctxscope"
)

const (
	// apiPathPrefix guards the catch-all: requests under it are API routes, so
	// a miss must return the JSON 404 envelope — never HTML.
	apiPathPrefix = "/api/"

	// spaIndex is the client-side-routing fallback document.
	spaIndex = "index.html"
)

// registerSPA mounts a catch-all GET handler that serves the embedded SPA. It
// is registered AFTER the API routes so /api/v1/* and /healthz already matched;
// only unmatched GETs land here. Real embedded files are served with their
// content type; anything else falls back to index.html for client-side routing.
func (s *Server) registerSPA() {
	s.echo.GET("/*", s.serveSPA)
}

func (s *Server) serveSPA(c echo.Context) error {
	req := c.Request()
	ctx := req.Context()
	logger := ctxscope.GetLogger(ctx)

	// API paths never serve HTML: an unmatched /api/ request is a genuine 404
	// in the API envelope shape (via errorHandler).
	if strings.HasPrefix(req.URL.Path, apiPathPrefix) {
		return echo.NewHTTPError(http.StatusNotFound, "not found")
	}

	// Clean + strip the leading slash to an fs.FS-relative path. An empty path
	// (root "/") means index.html.
	name := strings.TrimPrefix(path.Clean(req.URL.Path), "/")
	if name == "" || name == "." {
		name = spaIndex
	}

	if served, err := s.tryServeFile(c, name); err != nil {
		return err
	} else if served {
		return nil
	}

	// Unknown non-API path => SPA fallback so client-side routing owns it.
	logger.Debug("spa fallback to index", "path", req.URL.Path)

	served, err := s.tryServeFile(c, spaIndex)
	if err != nil {
		return err
	}

	if !served {
		// The placeholder guarantees index.html exists; a miss means a broken
		// embed, which is an internal error, not a 404.
		return ctxerrors.New("spa index.html missing from embedded fs")
	}

	return nil
}

// tryServeFile serves name from the embedded FS if it resolves to a real file,
// returning (true, nil) when written. A non-existent path or a directory yields
// (false, nil) so the caller can fall back to index.html.
func (s *Server) tryServeFile(c echo.Context, name string) (bool, error) {
	f, err := s.deps.WebFS.Open(name)
	if err != nil {
		// A miss (no such file) is not an error here: the caller falls back to
		// index.html for SPA client-side routing.
		return false, nil
	}

	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return false, nil
	}

	seeker, ok := f.(io.ReadSeeker)
	if !ok {
		return false, ctxerrors.New("embedded file is not seekable: " + name)
	}

	// http.ServeContent sets Content-Type (from the extension), handles range
	// requests + conditional GETs, all off the in-memory embedded file.
	http.ServeContent(c.Response(), c.Request(), name, info.ModTime(), seeker)

	return true, nil
}

// spaFileExists reports whether name resolves to a real (non-dir) file in fsys.
// Kept as a small helper for tests/assertions against the embedded tree.
func spaFileExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)

	return err == nil && !info.IsDir()
}
