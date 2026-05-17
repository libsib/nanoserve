package nanoserve

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestContext(method, target string, body string, handlers ...HandlerFunction) (*Context, *httptest.ResponseRecorder) {
	var reqBody *strings.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	} else {
		reqBody = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, reqBody)
	w := httptest.NewRecorder()
	if len(handlers) == 0 {
		handlers = []HandlerFunction{func(c *Context) error { return nil }}
	}
	c := NewContext(w, req, handlers, nil)
	return c, w
}

// --- Abort ---

func TestAbortStopsChain(t *testing.T) {
	called := false
	second := func(c *Context) error {
		called = true
		return nil
	}
	first := func(c *Context) error {
		c.Abort()
		return c.Next()
	}
	c, _ := newTestContext("GET", "/", "", first, second)
	_ = c.handlers[0](c)
	if called {
		t.Fatal("second handler should not have been called after Abort")
	}
}

func TestAbortWithStatus(t *testing.T) {
	c, w := newTestContext("GET", "/", "")
	c.AbortWithStatus(http.StatusUnauthorized)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if !c.abort {
		t.Fatal("abort flag should be set")
	}
}

// --- Status + response writers ---

func TestTextSetsCharset(t *testing.T) {
	c, w := newTestContext("GET", "/", "")
	_ = c.Text("hello")
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "charset=utf-8") {
		t.Fatalf("expected charset=utf-8 in Content-Type, got %q", ct)
	}
	if w.Body.String() != "hello" {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}
}

func TestStringIsAliasForText(t *testing.T) {
	c, w := newTestContext("GET", "/", "")
	_ = c.String("hi")
	if w.Body.String() != "hi" {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/plain") {
		t.Fatalf("expected text/plain, got %q", ct)
	}
}

func TestJsonResponse(t *testing.T) {
	c, w := newTestContext("GET", "/", "")
	_ = c.JSON(map[string]string{"key": "value"})
	if !strings.Contains(w.Body.String(), `"key"`) {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("unexpected content type: %q", w.Header().Get("Content-Type"))
	}
}

func TestHTMLResponse(t *testing.T) {
	c, w := newTestContext("GET", "/", "")
	_ = c.HTML("<h1>hi</h1>")
	if w.Body.String() != "<h1>hi</h1>" {
		t.Fatalf("unexpected body: %q", w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html, got %q", ct)
	}
}

func TestStatusChaining(t *testing.T) {
	c, w := newTestContext("GET", "/", "")
	c.Status(http.StatusCreated).Text("created")
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

// --- Redirect ---

func TestRedirectUsesCodeParam(t *testing.T) {
	c, w := newTestContext("GET", "/", "")
	c.Redirect("/new", http.StatusFound)
	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
}

func TestRedirectPrefersStatusField(t *testing.T) {
	c, w := newTestContext("GET", "/", "")
	c.Status(http.StatusMovedPermanently).Redirect("/new", http.StatusFound)
	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301 (from Status()), got %d", w.Code)
	}
}

// --- BodyBytes + Bind coexistence ---

func TestBodyBytesAndBindShareCache(t *testing.T) {
	payload := `{"name":"test"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(payload))
	w := httptest.NewRecorder()
	c := NewContext(w, req, nil, nil)

	raw, err := c.BodyBytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != payload {
		t.Fatalf("BodyBytes got %q, want %q", raw, payload)
	}

	var data struct{ Name string `json:"name"` }
	if err := c.Bind(&data); err != nil {
		t.Fatal(err)
	}
	if data.Name != "test" {
		t.Fatalf("Bind got name=%q, want %q", data.Name, "test")
	}
}

func TestBindThenBodyBytes(t *testing.T) {
	payload := `{"x":1}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(payload))
	w := httptest.NewRecorder()
	c := NewContext(w, req, nil, nil)

	var data map[string]any
	if err := c.Bind(&data); err != nil {
		t.Fatal(err)
	}

	raw, err := c.BodyBytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != payload {
		t.Fatalf("BodyBytes after Bind got %q, want %q", raw, payload)
	}
}

// --- IP extraction ---

func TestIPFromXForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")
	ip, err := getIP(req)
	if err != nil {
		t.Fatal(err)
	}
	// first entry is the client
	if ip != "203.0.113.5" {
		t.Fatalf("expected 203.0.113.5, got %q", ip)
	}
}

func TestIPFromXRealIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "198.51.100.7")
	ip, err := getIP(req)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "198.51.100.7" {
		t.Fatalf("expected 198.51.100.7, got %q", ip)
	}
}

func TestIPFromRemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.0.2.1:12345"
	ip, err := getIP(req)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "192.0.2.1" {
		t.Fatalf("expected 192.0.2.1, got %q", ip)
	}
}

func TestIPLocalhostNormalized(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "[::1]:54321"
	ip, err := getIP(req)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1, got %q", ip)
	}
}

// --- Cookie helpers ---

func TestSetAndGetCookie(t *testing.T) {
	c, w := newTestContext("GET", "/", "")
	c.SetCookie(http.Cookie{Name: "token", Value: "abc123", HttpOnly: true})
	setCookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "token=abc123") {
		t.Fatalf("Set-Cookie header missing token: %q", setCookie)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "token", Value: "abc123"})
	c2 := NewContext(httptest.NewRecorder(), req, nil, nil)
	cookie, err := c2.GetCookie("token")
	if err != nil {
		t.Fatal(err)
	}
	if cookie.Value != "abc123" {
		t.Fatalf("expected abc123, got %q", cookie.Value)
	}
}

// --- Param / Query / Header ---

func TestParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/user/42", nil)
	c := NewContext(httptest.NewRecorder(), req, nil, Params{{Key: "id", Value: "42"}})
	if c.Param("id") != "42" {
		t.Fatalf("expected 42, got %q", c.Param("id"))
	}
}

func TestQuery(t *testing.T) {
	c, _ := newTestContext("GET", "/search?q=hello", "")
	if c.Query("q") != "hello" {
		t.Fatalf("expected hello, got %q", c.Query("q"))
	}
}

func TestSetAndGetHeader(t *testing.T) {
	c, w := newTestContext("GET", "/", "")
	c.SetHeader("X-Custom", "value")
	if w.Header().Get("X-Custom") != "value" {
		t.Fatalf("expected value, got %q", w.Header().Get("X-Custom"))
	}
}

func TestGetHeader(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer token")
	c := NewContext(httptest.NewRecorder(), req, nil, nil)
	if c.GetHeader("Authorization") != "Bearer token" {
		t.Fatalf("unexpected header value: %q", c.GetHeader("Authorization"))
	}
}

// --- Set / Get context data ---

func TestSetGet(t *testing.T) {
	c, _ := newTestContext("GET", "/", "")
	c.Set("user", "pradeep")
	if c.Get("user") != "pradeep" {
		t.Fatalf("expected pradeep, got %v", c.Get("user"))
	}
}

func TestGetMissingKeyReturnsNil(t *testing.T) {
	c, _ := newTestContext("GET", "/", "")
	if c.Get("missing") != nil {
		t.Fatal("expected nil for missing key")
	}
}
