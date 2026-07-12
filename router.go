package nanoserve

import (
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
	Insert(method, path string, handler HandlerFunction)
	Search(method, path string) *RouteMatch
	Find(method, path string) *RouteMatch
	AddMiddleware(path string, handlers ...HandlerFunction)
}

type RouteMatch struct {
	Handler []HandlerFunction
	Params  Params
}

type Node struct {
	children    map[string]*Node
	isEndOfWord bool
	handlers    map[string]HandlerFunction
	middlewares []HandlerFunction
	paramName   string
}

func newNode() *Node {
	return &Node{
		children:    make(map[string]*Node),
		handlers:    make(map[string]HandlerFunction),
		middlewares: []HandlerFunction{},
		paramName:   "",
	}
}

type TrieRouter struct {
	root              *Node
	globalMiddlewares []HandlerFunction
}

func NewTrieRouter() *TrieRouter {
	return &TrieRouter{
		root: &Node{
			children:    make(map[string]*Node),
			handlers:    make(map[string]HandlerFunction),
			middlewares: []HandlerFunction{},
		},
	}
}

func (r *TrieRouter) AddMiddleware(path string, handlers ...HandlerFunction) {
	node := r.root

	if path == "/" {
		r.globalMiddlewares = append(r.globalMiddlewares, handlers...)
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
			node.children[key] = newNode()
		}
		node = node.children[key]
	}

	node.middlewares = append(node.middlewares, handlers...)
}

func (r *TrieRouter) Insert(method string, path string, handler HandlerFunction) {
	node := r.root

	if path == "/" {
		node.isEndOfWord = true
		node.handlers[method] = handler
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
			node.children[key] = newNode()
		}

		node = node.children[key]
		if cleanParam != "" {
			node.paramName = cleanParam
		}
	}
	node.isEndOfWord = true
	node.handlers[method] = handler
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
			if node.paramName != "" {
				params = append(params, Param{Key: node.paramName, Value: element})
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
			return &RouteMatch{Params: params, Handler: collected}
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
	if handler := node.handlers[method]; handler != nil {
		if !copied {
			collected = append([]HandlerFunction{}, collected...)
		}
		collected = append(collected, handler)
		return &RouteMatch{Params: params, Handler: collected}
	}
	// if not then "ALL"
	if handler := node.handlers["ALL"]; handler != nil {
		if !copied {
			collected = append([]HandlerFunction{}, collected...)
		}
		collected = append(collected, handler)
		return &RouteMatch{Params: params, Handler: collected}
	}

	return &RouteMatch{Params: params, Handler: collected}
}

func (r *TrieRouter) Find(method string, path string) *RouteMatch {
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
			wildCardMatch := node.children["*"]

			if child := node.children[segment]; child != nil {
				node = child
				if wildCardMatch != nil && len(wildCardMatch.middlewares) > 0 {
					if !copied {
						collected = append([]HandlerFunction{}, collected...)
						copied = true
					}
					collected = append(collected, wildCardMatch.middlewares...)
				}
			} else if child := node.children[":"]; child != nil {
				node = child
				if node.paramName != "" {
					params = append(params, Param{Key: node.paramName, Value: segment})
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
				return &RouteMatch{Params: params, Handler: collected}
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
	if handler := node.handlers[method]; handler != nil {
		if !copied {
			collected = append([]HandlerFunction{}, collected...)
		}
		collected = append(collected, handler)
		return &RouteMatch{Params: params, Handler: collected}
	}
	// if not then "ALL"
	if handler := node.handlers["ALL"]; handler != nil {
		if !copied {
			collected = append([]HandlerFunction{}, collected...)
		}
		collected = append(collected, handler)
		return &RouteMatch{Params: params, Handler: collected}
	}

	return &RouteMatch{Params: params, Handler: collected}
}
