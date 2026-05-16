package nanoserve

import (
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

	params map[string]string

	handlers []HandlerFunction
	index    int

	contextData map[string]any
	statusCode  int

	abort bool
}

func NewContext(w http.ResponseWriter, r *http.Request, handlers []HandlerFunction, params map[string]string) *Context {
	return &Context{
		Writer:   w,
		Request:  r,
		handlers: handlers,
		params:   params,
		index:    0,
	}
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

// Abort stops the middleware chain.
func (c *Context) Abort() {
	c.abort = true
}

// Status sets the HTTP response status code. Returns the context for chaining.
func (c *Context) Status(code int) *Context {
	c.statusCode = code
	return c
}

// Internally used for writing status code in Reesponse
func (c *Context) writeStatus() {
	if c.statusCode != 0 {
		c.Writer.WriteHeader(c.statusCode)
	}
}

// Set stores a value in the request scoped context data.
func (c *Context) Set(key string, value any) {
	if c.contextData == nil {
		c.contextData = make(map[string]any)
	}
	c.contextData[key] = value
}

// Get retrieves a value previously stored with Set.
// any type cause you can literally store anything in context datat
func (c *Context) Get(key string) any {
	return c.contextData[key]
}

// Url returns the request URL.
// eg - "/user/me"
func (c *Context) Url() *url.URL {
	return c.Request.URL
}

// Query returns the value of a URL query parameter.
// eg :- "/user/778?name=example"
// c.Query("name")
// Output = example
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// Param returns the value of a dynamic route parameter (e.g. :id).
// /user/:id -> /user/6777
// c.Param("id")
// Output = 6777
func (c *Context) Param(key string) string {
	return c.params[key]
}

// shouldn't exist cause user can do this themselves
// Example:
// Request with GET method
// c.Method()
// Output - GET
func (c *Context) Method() string {
	return c.Request.Method
}

// SetHeader sets a response header.
// eg := c.SetHeader("powered-by", "nanoServe")
func (c *Context) SetHeader(key string, val string) {
	c.Writer.Header().Set(key, val)
}

// GetHeader returns a request header value.
// request has -> "Authorization: Bearer token something..."
// c.GetHeader("Authorization")
// Output - Bearer token something...
func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

// Text writes a plain text response.
func (c *Context) Text(text string) error {
	c.Writer.Header().Set("Content-Type", "text/plain")
	c.writeStatus()
	_, err := c.Writer.Write([]byte(text))
	return err
}

// String is an alias for Text.
// it shouldn't exist but dunno why it's here.
func (c *Context) String(s string) error {
	return c.Text(s)
}

// Json writes a JSON response.
// Encodes data using encoding/json.
func (c *Context) Json(data any) error {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.writeStatus()
	return json.NewEncoder(c.Writer).Encode(data)
}

// HTML writes an HTML response.
//
// Example:
//
//	c.HTML("<h1>Hello</h1>")
func (c *Context) HTML(s string) error {
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	c.writeStatus()
	_, err := c.Writer.Write([]byte(s))
	return err
}

// GetCookie returns the named cookie from the request.
//
// Example:
//
//	cookie, err := c.GetCookie("access_token")
func (c *Context) GetCookie(cookieName string) (*http.Cookie, error) {
	return c.Request.Cookie(cookieName)
}

// SetCookie adds a Set-Cookie header to the response.
//
// Example:
//
//	c.SetCookie(http.Cookie{Name: "token", Value: "abc", HttpOnly: true})
func (c *Context) SetCookie(cookie http.Cookie) {
	http.SetCookie(c.Writer, &cookie)
}

// Redirect sends an HTTP redirect to the given URL.
//
// Example:
//
//	c.Redirect("/login", http.StatusFound)
func (c *Context) Redirect(url string, code int) {
	http.Redirect(c.Writer, c.Request, url, code)
}

// BodyBytes reads and returns the raw request body.
// The body can only be read once — calling this after Bind will return empty bytes.
func (c *Context) BodyBytes() ([]byte, error) {
	return io.ReadAll(c.Request.Body)
}

// Bind decodes the JSON request body into v. v must be a pointer to a struct.
//
// Example:
//
//	var payload struct { Name string `json:"name"` }
//	if err := c.Bind(&payload); err != nil { ... }
func (c *Context) Bind(v any) error {
	return json.NewDecoder(c.Request.Body).Decode(v)
}

func (c *Context) IP() (string, error) {
	return getIP(c.Request)
}

func getIP(r *http.Request) (string, error) {
	ips := r.Header.Get("X-Forwarded-For")
	splitIps := strings.Split(ips, ",")

	// first check for X-Forwarded-For
	if len(splitIps) > 0 {
		// get last IP in list since ELB prepends other user defined IPs, meaning the last one is the actual client IP.
		netIP := net.ParseIP(strings.TrimSpace(splitIps[len(splitIps)-1]))
		if netIP != nil {
			return netIP.String(), nil
		}
	}

	// if not then check for X-Real-IP
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		netIP := net.ParseIP(strings.TrimSpace(realIP))
		if netIP != nil {
			return netIP.String(), nil
		}
	}

	// in last fallback for r.RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}

	netIP := net.ParseIP(ip)
	if netIP != nil {
		ip := netIP.String()
		if ip == "::1" {
			return "127.0.0.1", nil
		}
		return ip, nil
	}

	return "", errors.New("IP not found")
}
