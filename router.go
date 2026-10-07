package nanoserve

import (
	"slices"
	"strings"
)

type Param struct {
	Key   string
	Value string
}

type Params []Param

func (p Params) Get(key string) string {
	for _, param := range p {
		if param.Key == key {
			return param.Value
		}
	}
	return ""
}

type Router interface {
	Insert(method, path string, handlers ...HandlerFunction)
	Search(method, path string) *RouteMatch
	Find(method, path string) *RouteMatch
	AddMiddleware(path string, handlers ...HandlerFunction)
}

type RouteMatch struct {
	Handler []HandlerFunction
	Params  Params
	// HandlerFound is true when Handler ends with a real handler for the method (or ALL).
	// When false, Handler holds only middleware, if any, and the request is a 404.
	HandlerFound bool
}

// Our Node
type Node struct {
	children      map[string]*Node
	isEndOfWord   bool
	handlers      map[string][]HandlerFunction
	middlewares   []HandlerFunction
	params        map[string]string
	paramChild    *Node
	wildCardChild *Node
}

func newNode() *Node {
	return &Node{
		children:    make(map[string]*Node),
		handlers:    make(map[string][]HandlerFunction),
		middlewares: []HandlerFunction{},
		params:      make(map[string]string),
	}
}

// addChild creates the child for key and caches it on paramChild or wildCardChild
// so Find can reach them without a map lookup.
func (n *Node) addChild(key string) {
	child := newNode()
	n.children[key] = child
	switch key {
	case ":":
		n.paramChild = child
	case "*":
		n.wildCardChild = child
	}
}

type TrieRouter struct {
	root              *Node
	globalMiddlewares []HandlerFunction
	getStatic         map[string]*RouteMatch
	postStatic        map[string]*RouteMatch
	putStatic         map[string]*RouteMatch
	deleteStatic      map[string]*RouteMatch
	otherStatic       map[string]map[string]*RouteMatch

	staticPaths []string
}

func (r *TrieRouter) getStaticMapFor(method string) map[string]*RouteMatch {
	switch method {
	case "GET":
		return r.getStatic
	case "POST":
		return r.postStatic
	case "PUT":
		return r.putStatic
	case "DELETE":
		return r.deleteStatic
	default:
		return r.otherStatic[method]
	}
}

func (r *TrieRouter) getOrCreateStaticMapFor(method string) map[string]*RouteMatch {
	static := r.getStaticMapFor(method)
	if static != nil {
		return static
	}
	newMap := make(map[string]*RouteMatch)
	r.otherStatic[method] = newMap
	return newMap
}

func (r *TrieRouter) arrangeHandlers(path string) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "ALL"}

	for method := range r.otherStatic {
		if !slices.Contains(methods, method) {
			methods = append(methods, method)
		}
	}

	for _, mthd := range methods {
		searchedResult := r.Search(mthd, path)
		correctMap := r.getOrCreateStaticMapFor(mthd)
		if len(searchedResult.Handler) > 0 {
			correctMap[path] = searchedResult
		}
	}

}

func (r *TrieRouter) rebuildStatic() {
	for _, v := range r.staticPaths {
		r.arrangeHandlers(v)
	}
}

func NewTrieRouter() *TrieRouter {
	return &TrieRouter{
		root: &Node{
			children:    make(map[string]*Node),
			handlers:    make(map[string][]HandlerFunction),
			middlewares: []HandlerFunction{},
		},
		getStatic:    make(map[string]*RouteMatch),
		postStatic:   make(map[string]*RouteMatch),
		putStatic:    make(map[string]*RouteMatch),
		deleteStatic: make(map[string]*RouteMatch),
		otherStatic:  make(map[string]map[string]*RouteMatch),
		staticPaths:  make([]string, 0),
	}
}

func (r *TrieRouter) AddMiddleware(path string, handlers ...HandlerFunction) {
	node := r.root

	if path == "/" {
		r.globalMiddlewares = append(r.globalMiddlewares, handlers...)
		r.rebuildStatic()
		return
	}

	segments := strings.Split(path, "/")

	for _, element := range segments {
		if element == "" {
			continue
		}

		key := element
		if strings.HasPrefix(element, ":") {
			key = ":"
		}

		if node.children[key] == nil {
			node.addChild(key)
		}
		node = node.children[key]
	}

	node.middlewares = append(node.middlewares, handlers...)
	r.rebuildStatic()
}

// Insert registers the whole handler chain (route middleware first, final handler last)
// for the method, so middleware belongs to this route and not to the path.
func (r *TrieRouter) Insert(method string, path string, handlers ...HandlerFunction) {
	routeHandlers := slices.Clone(handlers)
	isStatic := !strings.Contains(path, ":") && !strings.Contains(path, "*")
	if isStatic {
		if !slices.Contains(r.staticPaths, path) {
			r.staticPaths = append(r.staticPaths, path)
		}
	}
	newMethod := r.getStaticMapFor(method) == nil
	r.getOrCreateStaticMapFor(method)

	node := r.root

	if path == "/" {
		node.isEndOfWord = true
		node.handlers[method] = routeHandlers
		if newMethod {
			// rebuild static cache
			r.rebuildStatic()
		}
		return
	}

	segments := strings.Split(path, "/")
	for _, element := range segments {
		if element == "" {
			continue
		}

		key := element
		cleanParam := ""
		if strings.HasPrefix(element, ":") {
			key = ":"
			cleanParam = element[1:]
		}

		if node.children[key] == nil {
			node.addChild(key)
		}
		node = node.children[key]
		if cleanParam != "" {
			node.params[method] = cleanParam
		}
	}
	node.isEndOfWord = true
	node.handlers[method] = routeHandlers

	if newMethod {
		// rebuild static
		r.rebuildStatic()
	}
	if isStatic {
		// re arrange handlers
		r.arrangeHandlers(path)
	}
}

func (r *TrieRouter) Find(method string, path string) *RouteMatch {
	staticmap := r.getStaticMapFor(method)
	if staticmap != nil {
		cachedresult := staticmap[path]
		if cachedresult != nil {
			return cachedresult
		}
	}
	// Path - /user/me
	node := r.root

	var collected []HandlerFunction
	collected = r.globalMiddlewares
	copied := false

	var params Params

	start := 0
	for i := 0; i <= len(path); i++ {
		// range of /user/me
		if i == len(path) || path[i] == '/' {
			//
			if start == i {
				start = i + 1
				continue
			}
			// strip the seg
			// like "/user/me" , start=0, and when path[i]== / second time at /me
			// so we do path[0:5] which will return user, thats what we need.
			segment := path[start:i]
			next := node.children[segment]
			if next != nil {
				if node.wildCardChild != nil && len(node.wildCardChild.middlewares) > 0 {
					if !copied {
						collected = append([]HandlerFunction{}, collected...)
						copied = true
					}
					collected = append(collected, node.wildCardChild.middlewares...)
				}
				node = next
			} else if node.paramChild != nil {
				if node.wildCardChild != nil && len(node.wildCardChild.middlewares) > 0 {
					if !copied {
						collected = append([]HandlerFunction{}, collected...)
						copied = true
					}
					collected = append(collected, node.wildCardChild.middlewares...)
				}
				node = node.paramChild

				param := node.params[method]
				if param == "" {
					param = node.params["ALL"]
				}
				if param != "" {
					params = append(params, Param{Key: param, Value: segment})
				}
			} else if node.wildCardChild != nil {
				node = node.wildCardChild
				break
			} else {
				return &RouteMatch{Params: params, Handler: collected, HandlerFound: false}
			}

			start = i + 1
		}
	}

	if len(node.middlewares) > 0 {
		if !copied {
			collected = append([]HandlerFunction{}, collected...)
			copied = true
		}
		collected = append(collected, node.middlewares...)
	}
	// first check for given method
	if routeHandlers := node.handlers[method]; routeHandlers != nil {
		if !copied {
			collected = append([]HandlerFunction{}, collected...)
		}
		collected = append(collected, routeHandlers...)
		return &RouteMatch{Params: params, Handler: collected, HandlerFound: true}
	}
	// if not then "ALL"
	if routeHandlers := node.handlers["ALL"]; routeHandlers != nil {
		if !copied {
			collected = append([]HandlerFunction{}, collected...)
		}
		collected = append(collected, routeHandlers...)
		return &RouteMatch{Params: params, Handler: collected, HandlerFound: true}
	}

	return &RouteMatch{Params: params, Handler: collected, HandlerFound: false}
}

// deprecated.
// use find method for better performance.
func (r *TrieRouter) Search(method string, path string) *RouteMatch {
	node := r.root
	segments := strings.Split(path, "/")
	var collected []HandlerFunction
	collected = r.globalMiddlewares
	copied := false

	var params Params

	for _, element := range segments {
		if element == "" {
			continue
		}

		wildCardMatch := node.children["*"]
		if child := node.children[element]; child != nil {
			if wildCardMatch != nil && len(wildCardMatch.middlewares) > 0 {
				if !copied {
					collected = append([]HandlerFunction{}, collected...)
					copied = true
				}
				collected = append(collected, wildCardMatch.middlewares...)
			}
			node = child
		} else if child := node.children[":"]; child != nil {
			node = child
			param := node.params[method]
			if param == "" {
				param = node.params["ALL"]
			}
			if param != "" {
				params = append(params, Param{Key: param, Value: element})
			}
			if wildCardMatch != nil && len(wildCardMatch.middlewares) > 0 {
				if !copied {
					collected = append([]HandlerFunction{}, collected...)
					copied = true
				}
				collected = append(collected, wildCardMatch.middlewares...)
			}
		} else if child := node.children["*"]; child != nil {
			node = child
			break
		} else {
			return &RouteMatch{Params: params, Handler: collected, HandlerFound: false}
		}
	}
	if len(node.middlewares) > 0 {
		if !copied {
			collected = append([]HandlerFunction{}, collected...)
			copied = true
		}
		collected = append(collected, node.middlewares...)
	}
	// first check for given method
	if routeHandlers := node.handlers[method]; routeHandlers != nil {
		if !copied {
			collected = append([]HandlerFunction{}, collected...)
		}
		collected = append(collected, routeHandlers...)
		return &RouteMatch{Params: params, Handler: collected, HandlerFound: true}
	}
	// if not then "ALL"
	if routeHandlers := node.handlers["ALL"]; routeHandlers != nil {
		if !copied {
			collected = append([]HandlerFunction{}, collected...)
		}
		collected = append(collected, routeHandlers...)
		return &RouteMatch{Params: params, Handler: collected, HandlerFound: true}
	}

	return &RouteMatch{Params: params, Handler: collected, HandlerFound: false}
}
