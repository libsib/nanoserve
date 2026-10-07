package nanoserve

import (
	"net/http"
	"testing"
)

// Route middleware belongs to its route: it must not become global when the path is "/",
// must not run for other methods on the same path, and must run exactly once.
func TestRouteMiddlewareScopedToRoute(t *testing.T) {
	runs := 0
	auth := func(c *Context) error { runs++; return c.Next() }
	ok := func(c *Context) error { return c.String("ok") }

	sub := New()
	sub.GET("/", auth, ok)
	sub.POST("/", auth, ok)
	sub.GET("/count", auth, ok)
	sub.GET("/validate", ok)
	app := New()
	app.Sub("/link/*", sub)

	cases := []struct {
		method, path string
		code, runs   int
	}{
		{http.MethodGet, "/link/validate", http.StatusOK, 0},
		{http.MethodGet, "/link/count", http.StatusOK, 1},
		{http.MethodGet, "/link", http.StatusOK, 1},
		{http.MethodPost, "/link", http.StatusOK, 1},
		{http.MethodDelete, "/link/count", http.StatusNotFound, 0},
	}
	for _, tc := range cases {
		runs = 0
		w := serve(app, tc.method, tc.path)
		if w.Code != tc.code || runs != tc.runs {
			t.Errorf("%s %s: got %d with %d auth runs, want %d with %d", tc.method, tc.path, w.Code, runs, tc.code, tc.runs)
		}
	}
}

func TestRouteMiddlewareDoesNotLeakAcrossMethods(t *testing.T) {
	var ran []string
	mw := func(name string) HandlerFunction {
		return func(c *Context) error { ran = append(ran, name); return c.Next() }
	}
	ok := func(c *Context) error { return c.String("ok") }

	app := New()
	app.GET("/x", mw("A"), ok)
	app.POST("/x", mw("B"), ok)

	serve(app, http.MethodGet, "/x")
	if len(ran) != 1 || ran[0] != "A" {
		t.Fatalf("GET /x ran %v, want [A]", ran)
	}
	ran = nil
	serve(app, http.MethodPost, "/x")
	if len(ran) != 1 || ran[0] != "B" {
		t.Fatalf("POST /x ran %v, want [B]", ran)
	}
}

func TestRouteMiddlewareOnRootIsNotGlobal(t *testing.T) {
	runs := 0
	auth := func(c *Context) error { runs++; return c.Next() }
	ok := func(c *Context) error { return c.String("ok") }

	app := New()
	app.POST("/", auth, ok)
	app.GET("/validate", ok)

	if w := serve(app, http.MethodGet, "/validate"); w.Code != http.StatusOK || runs != 0 {
		t.Fatalf("GET /validate: got %d with %d auth runs, want 200 with 0", w.Code, runs)
	}
}
