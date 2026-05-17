package nanoserve

import "testing"

// buildBenchRouter returns a router pre-loaded with a realistic mix of routes.
func buildBenchRouter() *TrieRouter {
	r := NewTrieRouter()
	r.Insert("GET", "/", dummyHandler)
	r.Insert("GET", "/users", dummyHandler)
	r.Insert("GET", "/users/:id", dummyHandler)
	r.Insert("GET", "/users/:id/posts", dummyHandler)
	r.Insert("GET", "/orgs/:org/repos/:repo", dummyHandler)
	r.Insert("POST", "/users", dummyHandler)
	r.Insert("DELETE", "/users/:id", dummyHandler)
	r.AddMiddleware("/", dummyHandler)
	r.AddMiddleware("/users", dummyHandler)
	return r
}

// --- static path ---

func BenchmarkSearch_Static(b *testing.B) {
	r := buildBenchRouter()
	for b.Loop() {
		r.Search("GET", "/users")
	}
}

func BenchmarkFind_Static(b *testing.B) {
	r := buildBenchRouter()
	for b.Loop() {
		r.Find("GET", "/users")
	}
}

// --- single param ---

func BenchmarkSearch_Param(b *testing.B) {
	r := buildBenchRouter()
	for b.Loop() {
		r.Search("GET", "/users/42")
	}
}

func BenchmarkFind_Param(b *testing.B) {
	r := buildBenchRouter()
	for b.Loop() {
		r.Find("GET", "/users/42")
	}
}

// --- multiple params (deeper path) ---

func BenchmarkSearch_MultiParam(b *testing.B) {
	r := buildBenchRouter()
	for b.Loop() {
		r.Search("GET", "/orgs/acme/repos/widget")
	}
}

func BenchmarkFind_MultiParam(b *testing.B) {
	r := buildBenchRouter()
	for b.Loop() {
		r.Find("GET", "/orgs/acme/repos/widget")
	}
}

// --- no match ---

func BenchmarkSearch_NoMatch(b *testing.B) {
	r := buildBenchRouter()
	for b.Loop() {
		r.Search("GET", "/does/not/exist")
	}
}

func BenchmarkFind_NoMatch(b *testing.B) {
	r := buildBenchRouter()
	for b.Loop() {
		r.Find("GET", "/does/not/exist")
	}
}
