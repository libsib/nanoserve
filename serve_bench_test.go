package nanoserve

import (
	"net/http/httptest"
	"testing"
)

// buildBenchApp returns a NanoServe app with a realistic mix of routes,
// for benchmarking the full ServeHTTP path (routing + context + handlers).
func buildBenchApp() *NanoServe {
	app := New()
	noop := func(c *Context) error { return nil }
	app.GET("/", noop)
	app.GET("/users", noop)
	app.GET("/users/:id", noop)
	app.GET("/orgs/:org/repos/:repo", noop)
	return app
}

func benchmarkServe(b *testing.B, path string) {
	app := buildBenchApp()
	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	for b.Loop() {
		app.ServeHTTP(w, req)
	}
}

func BenchmarkServeHTTP_Static(b *testing.B) {
	benchmarkServe(b, "/users")
}

func BenchmarkServeHTTP_Param(b *testing.B) {
	benchmarkServe(b, "/users/42")
}

func BenchmarkServeHTTP_MultiParam(b *testing.B) {
	benchmarkServe(b, "/orgs/nano/repos/serve")
}
