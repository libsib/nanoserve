package nanoserve

import (
	"strings"
)

type Router interface {
	Insert(method, path string, handler HandlerFunction)
	Search(method, path string) *RouteMatch
	AddMiddleware(path string, handlers ...HandlerFunction)
}

type RouteMatch struct {
	Handler []HandlerFunction
	Params  map[string]string
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
		} else if strings.HasPrefix(element, "*") {
			node.middlewares = append(node.middlewares, handlers...)
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
	routeParams := map[string]interface{}{}
	for i, element := range segments {
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
			routeParams[cleanParam] = i
			node.paramName = cleanParam
		}
	}
	node.isEndOfWord = true
	node.handlers[method] = handler
}

func (r *TrieRouter) Search(method string, path string) *RouteMatch {
	node := r.root
	segments := strings.Split(path, "/")
	var collected []HandlerFunction
	collected = r.globalMiddlewares
	copied := false

	var params map[string]string

	for _, element := range segments {
		if element == "" {
			continue
		}

		if node.children[element] != nil {
			node = node.children[element]
		} else if node.children[":"] != nil {
			node = node.children[":"]
			if node.paramName != "" {
				if params == nil {
					params = map[string]string{}
				}
				params[node.paramName] = element
			}
		} else if node.children["*"] != nil {
			node = node.children["*"]
			break
		} else {
			return &RouteMatch{Params: params, Handler: collected}
		}
		if len(node.middlewares) > 0 {
			if !copied {
				collected = append([]HandlerFunction{}, collected...)
			}
			collected = append(collected, node.middlewares...)
		}
	}
	if h := node.handlers[method]; h != nil {
		if !copied {
			collected = append([]HandlerFunction{}, collected...)
		}
		collected = append(collected, h)
		return &RouteMatch{Params: params, Handler: collected}
	}

	return &RouteMatch{Params: params, Handler: collected}
}
