package nanoserve

import (
	"net/http"
	"strings"
)

type HandlerFunction func(*Context) error

type ErrorHandlerFunc func(*Context, error)

type NanoServe struct {
	router       Router
	ErrorHandler ErrorHandlerFunc
}

func New() *NanoServe {
	return &NanoServe{
		router: NewTrieRouter(),
		ErrorHandler: func(c *Context, err error) {
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
		},
	}
}

func (n *NanoServe) GET(path string, h ...HandlerFunction) *NanoServe {
	n.addRoute(http.MethodGet, path, h...)
	return n
}

func (n *NanoServe) POST(path string, h ...HandlerFunction) *NanoServe {
	n.addRoute(http.MethodPost, path, h...)
	return n

}

func (n *NanoServe) PUT(path string, h ...HandlerFunction) *NanoServe {
	n.addRoute(http.MethodPut, path, h...)
	return n

}

func (n *NanoServe) PATCH(path string, h ...HandlerFunction) *NanoServe {
	n.addRoute(http.MethodPatch, path, h...)
	return n

}

func (n *NanoServe) DELETE(path string, h ...HandlerFunction) *NanoServe {
	n.addRoute(http.MethodDelete, path, h...)
	return n

}

func (n *NanoServe) HEAD(path string, h ...HandlerFunction) *NanoServe {
	n.addRoute(http.MethodHead, path, h...)
	return n

}

func (n *NanoServe) OPTIONS(path string, h ...HandlerFunction) *NanoServe {
	n.addRoute(http.MethodOptions, path, h...)
	return n

}

func (n *NanoServe) CONNECT(path string, h ...HandlerFunction) *NanoServe {
	n.addRoute(http.MethodConnect, path, h...)
	return n
}

func (n *NanoServe) TRACE(path string, h ...HandlerFunction) *NanoServe {
	n.addRoute(http.MethodTrace, path, h...)
	return n
}

func (n *NanoServe) Handle(method, path string, h ...HandlerFunction) *NanoServe {
	n.addRoute(method, path, h...)
	return n
}

func (n *NanoServe) ALL(path string, h ...HandlerFunction) *NanoServe {
	n.addRoute("ALL", path, h...)
	return n
}

func (n *NanoServe) ANY(path string, h ...HandlerFunction) *NanoServe {
	n.addRoute("ALL", path, h...)
	return n
}

// for serving static files

func (n *NanoServe) Static(urlPrefix string, rootDir string) {
	fs := http.FileServer(http.Dir(rootDir))

	handler := func(ctx *Context) error {
		http.StripPrefix(urlPrefix, fs).ServeHTTP(ctx.Writer, ctx.Request)
		return nil
	}

	n.GET(urlPrefix+"/*", handler)
}

func (n *NanoServe) addRoute(method string, path string, handlers ...HandlerFunction) {
	if len(handlers) == 0 {
		panic("route must have at least one handler")
	}

	middlewareFunctions := handlers[:len(handlers)-1]
	if len(middlewareFunctions) > 0 {
		n.router.AddMiddleware(path, middlewareFunctions...)
	}

	handler := handlers[len(handlers)-1]
	n.router.Insert(method, path, handler)
}

func (n *NanoServe) Run(addr string) error {
	return http.ListenAndServe(addr, n)
}

func (n *NanoServe) Use(pathOrHandler any, handlers ...HandlerFunction) {
	switch v := pathOrHandler.(type) {
	case string:
		n.router.AddMiddleware(v, handlers...)
	case HandlerFunction:
		all := append([]HandlerFunction{v}, handlers...)
		n.router.AddMiddleware("/", all...)
	case func(*Context) error:
		all := append([]HandlerFunction{v}, handlers...)
		n.router.AddMiddleware("/", all...)
	}
}

// sub method for sub routing
func (n *NanoServe) Sub(prefix string, instance *NanoServe) {
	cleanPrefix := prefix
	if strings.HasSuffix(prefix, "/*") {
		cleanPrefix = prefix[:len(prefix)-2]
	}

	prefixLength := len(cleanPrefix)
	if cleanPrefix == "/" {
		prefixLength = 0
	}

	handler := func(ctx *Context) error {
		path := "/"
		if len(ctx.Request.URL.Path) >= prefixLength {
			path = ctx.Request.URL.Path[prefixLength:]
			if path == "" {
				path = "/"
			}
		}

		match := instance.router.Search(ctx.Request.Method, path)
		ctx.handlers = match.Handler
		ctx.Request.URL.Path = path
		ctx.params = match.Params
		// call child router's execute handler
		executeHandlers(ctx, instance.ErrorHandler)
		return nil
	}
	n.ALL(prefix, handler)
	n.ALL(cleanPrefix, handler)
}

// Our Main Handler which will handle the incoming request
func (n *NanoServe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	match := n.router.Search(r.Method, r.URL.Path)

	c := NewContext(w, r, match.Handler, match.Params)

	executeHandlers(c, n.ErrorHandler)
}

func executeHandlers(c *Context, errHandler ErrorHandlerFunc) {
	if len(c.handlers) > 0 {
		if err := c.handlers[0](c); err != nil {
			errHandler(c, err)
		}
		return
	}
	http.NotFound(c.Writer, c.Request)
}
