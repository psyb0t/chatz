package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/psyb0t/aichteeteapee"
	"github.com/psyb0t/chatz/internal/pkg/webassets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testWebFS is a tiny stand-in for the built SPA: an index.html fallback plus a
// real asset under _app/.
func testWebFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":        {Data: []byte("<!doctype html><title>x</title>")},
		"_app/version.json": {Data: []byte(`{"version":"test"}`)},
		"favicon.png":       {Data: []byte("PNGDATA")},
	}
}

func TestSPA_ServesRealFile(t *testing.T) {
	t.Parallel()

	srv := New(Deps{WebFS: testWebFS()})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/_app/version.json", nil)
	srv.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"version":"test"}`, rec.Body.String())
	assert.Contains(
		t, rec.Header().Get(aichteeteapee.HeaderNameContentType),
		"application/json",
	)
}

func TestSPA_FallsBackToIndexForUnknownNonAPIPath(t *testing.T) {
	t.Parallel()

	srv := New(Deps{WebFS: testWebFS()})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/chats/some-client-route", nil)
	srv.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<title>x</title>")
	assert.Contains(
		t, rec.Header().Get(aichteeteapee.HeaderNameContentType), "text/html",
	)
}

func TestSPA_RootServesIndex(t *testing.T) {
	t.Parallel()

	srv := New(Deps{WebFS: testWebFS()})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/", nil)
	srv.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<title>x</title>")
}

func TestSPA_UnknownAPIPathReturnsJSON404(t *testing.T) {
	t.Parallel()

	srv := New(Deps{WebFS: testWebFS()})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/api/v1/does-not-exist", nil)
	srv.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(
		t, rec.Header().Get(aichteeteapee.HeaderNameContentType),
		"application/json",
	)
	assert.JSONEq(t,
		`{"code":"NOT_FOUND","message":"not found"}`, rec.Body.String())
}

// TestSPA_NilWebFSDisablesServing proves the nil-WebFS path stays API-only: an
// unknown non-API GET is echo's default 404, not an index.html body.
func TestSPA_NilWebFSDisablesServing(t *testing.T) {
	t.Parallel()

	srv := New(Deps{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/some-client-route", nil)
	srv.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.NotContains(t, rec.Body.String(), "<title>")
}

// TestSPA_EmbeddedPlaceholderServes proves the committed placeholder embed is a
// usable fs.FS: index.html is present and served at root.
func TestSPA_EmbeddedPlaceholderServes(t *testing.T) {
	t.Parallel()

	webFS, err := webassets.FS()
	require.NoError(t, err)
	require.True(t, spaFileExists(webFS, "index.html"))

	srv := New(Deps{WebFS: webFS})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(
		context.Background(), http.MethodGet, "/", nil)
	srv.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<html")
}
