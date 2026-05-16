package nanoserve

import (
	"encoding/json"
	"net/http"
	"net/url"
)

type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request

	params map[string]string

	handlers []HandlerFunction
	index    int

	contextData map[string]any
	statusCode  int
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

func (c *Context) Next() error {
	c.index++
	if c.index >= len(c.handlers) {
		return nil
	}
	return c.handlers[c.index](c)
}

func (c *Context) Status(code int) *Context {
	c.statusCode = code
	return c
}

func (c *Context) writeStatus() {
	if c.statusCode != 0 {
		c.Writer.WriteHeader(c.statusCode)
	}
}

func (c *Context) Set(key string, value any) {
	if c.contextData == nil {
		c.contextData = make(map[string]any)
	}
	c.contextData[key] = value
}

func (c *Context) Get(key string) any {
	return c.contextData[key]
}

func (c *Context) Url() *url.URL {
	return c.Request.URL
}

func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

func (c *Context) Param(key string) string {
	val := c.params[key]
	if val != "" {
		return val
	}
	return ""
}

func (c *Context) SetHeader(key string, val string) {
	c.Writer.Header().Set(key, val)
}

func (c *Context) GetHeader(key string) string {
	return c.Request.Header.Get(key)
}

func (c *Context) Text(text string) error {
	c.Writer.Header().Set("Content-Type", "text/plain")
	c.writeStatus()
	_, err := c.Writer.Write([]byte(text))
	return err
}

// same as Text, shouldn't exist but still i have kept this, dunno whyyy
func (c *Context) String(s string) error {
	return c.Text(s)
}

func (c *Context) Json(data any) error {
	c.Writer.Header().Set("Content-Type", "application/json")
	c.writeStatus()
	return json.NewEncoder(c.Writer).Encode(data)
}

func (c *Context) HTML(s string) error {
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	c.writeStatus()
	_, err := c.Writer.Write([]byte(s))
	return err
}

func (c *Context) GetCookie(cookieName string) (*http.Cookie, error) {
	return c.Request.Cookie(cookieName)
}

func (c *Context) SetCookie(cookie http.Cookie) {
	http.SetCookie(c.Writer, &cookie)
}

func (c *Context) Redirect(url string, code int) {
	http.Redirect(c.Writer, c.Request, url, code)
}
