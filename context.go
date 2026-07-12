package nanoserve

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request

	params Params

	handlers []HandlerFunction
	index    int

	contextData map[string]any
	statusCode  int

	abort      bool
	bodyCache  []byte
	bodyCached bool
}

func NewContext(w http.ResponseWriter, r *http.Request, matchedResult *RouteMatch) *Context {
	return &Context{
		Writer:   w,
		Request:  r,
		handlers: matchedResult.Handler,
		params:   matchedResult.Params,
		index:    0,
	}
}

// resetWith clears all the state and applies new values for the next request.
func (c *Context) resetWith(w http.ResponseWriter, r *http.Request, matchedResult *RouteMatch) {
	c.Writer = w
	c.Request = r
	c.handlers = matchedResult.Handler
	c.params = matchedResult.Params
	c.index = 0
	c.contextData = nil
	c.statusCode = 0
	c.abort = false
	c.bodyCache = nil
	c.bodyCached = false
}

// Next calls the next handler in the middleware chain.
// If Abort was called, it stops execution immediately.
func (c *Context) Next() error {
	if c.abort {
		return nil
	}
	c.index++
	if c.index >= len(c.handlers) {
		return nil
	}
	return c.handlers[c.index](c)
}

// Abort stops the middleware chain. Write a response before calling this,
// otherwise the client receives an empty 200 OK.
func (c *Context) Abort() {
	c.abort = true
}

// AbortWithStatus stops the middleware chain and writes the given status code.
func (c *Context) AbortWithStatus(code int) {
	c.abort = true
	c.Status(code)
	c.writeStatus()
}

// Status sets the HTTP response status code. Returns the context for chaining.
func (c *Context) Status(code int) *Context {
	c.statusCode = code
	return c
}

func (c *Context) writeStatus() {
	if c.statusCode != 0 {
		c.Writer.WriteHeader(c.statusCode)
	}
}

// Set stores a value in the request-scoped context data.
func (c *Context) Set(key string, value any) {
	if c.contextData == nil {
		c.contextData = make(map[string]any)
	}
	c.contextData[key] = value
}

// Get retrieves a value previously stored with Set.
func (c *Context) Get(key string) any {
	return c.contextData[key]
}

// URL returns the request URL.
func (c *Context) URL() *url.URL {
	return c.Request.URL
}

// Query returns the value of a URL query parameter.
//
//	c.Query("name") // "/user?name=example" → "example"
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// Param returns the value of a dynamic route parameter.
//
//	c.Param("id") // route "/user/:id", request "/user/42" → "42"
func (c *Context) Param(key string) string {
	return c.params.Get(key)
}

// Method returns the HTTP method of the request (GET, POST, etc.).
func (c *Context) Method() string {
	return c.Request.Method
}

// SetHeader sets a response header.
func (c *Context) SetHeader(key string, val string) {
	c.Writer.Header().Set(key, val)
}

// GetHeader returns the value of a request header.
func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

// Text writes a plain text response.
func (c *Context) Text(text string) error {
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.writeStatus()
	_, err := c.Writer.Write([]byte(text))
	return err
}

// String is an alias for Text.
func (c *Context) String(s string) error {
	return c.Text(s)
}

// JSON writes a JSON response.
func (c *Context) JSON(data any) error {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.writeStatus()
	return json.NewEncoder(c.Writer).Encode(data)
}

// HTML writes an HTML response.
func (c *Context) HTML(s string) error {
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	c.writeStatus()
	_, err := c.Writer.Write([]byte(s))
	return err
}

// Send writes data to the response with the given Content-Type.
// data may be a string, []byte, or io.Reader; any other type is JSON-encoded.
// If contentType is empty, it defaults to application/octet-stream.
func (c *Context) Send(data any, contentType string) error {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Writer.Header().Set("Content-Type", contentType)
	c.writeStatus()
	switch d := data.(type) {
	case nil:
		return nil
	case []byte:
		_, err := c.Writer.Write(d)
		return err
	case string:
		_, err := c.Writer.Write([]byte(d))
		return err
	case io.Reader:
		_, err := io.Copy(c.Writer, d)
		return err
	default:
		return json.NewEncoder(c.Writer).Encode(data)
	}
}

// NoContent writes a response with the given status code and no body.
func (c *Context) NoContent(code int) error {
	c.statusCode = code
	c.Writer.WriteHeader(code)
	return nil
}

// Send204 sends a 204 No Content response.
func (c *Context) Send204() error {
	return c.NoContent(http.StatusNoContent)
}

// GetCookie returns the named cookie from the request.
func (c *Context) GetCookie(cookieName string) (*http.Cookie, error) {
	return c.Request.Cookie(cookieName)
}

// SetCookie adds a Set-Cookie header to the response.
func (c *Context) SetCookie(cookie http.Cookie) {
	http.SetCookie(c.Writer, &cookie)
}

// Redirect sends an HTTP redirect. Uses the status code set via Status() if
// provided, otherwise falls back to the code argument.
func (c *Context) Redirect(url string, code int) {
	if c.statusCode != 0 {
		code = c.statusCode
	}
	http.Redirect(c.Writer, c.Request, url, code)
}

// readBody reads the request body once and caches it so that BodyBytes and
// Bind can both be called without consuming the stream twice.
func readBody(c *Context) ([]byte, error) {
	if c.bodyCached {
		return c.bodyCache, nil
	}
	b, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	c.bodyCache = b
	c.bodyCached = true
	c.Request.Body = io.NopCloser(bytes.NewReader(b))
	return b, nil
}

// BodyBytes reads and returns the raw request body.
// Safe to call alongside Bind — both share the same cached read.
func (c *Context) BodyBytes() ([]byte, error) {
	return readBody(c)
}

// Bind decodes a JSON request body. Expects Content-Type: application/json.
// v must be a pointer.
// Safe to call alongside BodyBytes — both share the same cached read.
func (c *Context) Bind(v any) error {
	b, err := readBody(c)
	if err != nil {
		return err
	}
	return json.NewDecoder(bytes.NewReader(b)).Decode(v)
}

// IP returns the best-guess client IP address.
// It reads X-Forwarded-For (first entry), then X-Real-IP, then RemoteAddr.
// Only trust this behind a reverse proxy you control — these headers are
// trivially spoofable on a direct internet-facing server.
func (c *Context) IP() (string, error) {
	return getIP(c.Request)
}

func getIP(r *http.Request) (string, error) {
	// X-Forwarded-For: client, proxy1, proxy2 — first entry is original client.
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first := strings.TrimSpace(strings.SplitN(forwarded, ",", 2)[0])
		if ip := net.ParseIP(first); ip != nil {
			return ip.String(), nil
		}
	}

	// X-Real-IP is set by nginx and similar proxies.
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		if ip := net.ParseIP(strings.TrimSpace(realIP)); ip != nil {
			return ip.String(), nil
		}
	}

	// Fall back to the direct connection address.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() {
			return "127.0.0.1", nil
		}
		return ip.String(), nil
	}

	return "", errors.New("IP not found")
}
