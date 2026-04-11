package gin

import (
	"encoding/json"
	"net/http"
	"strings"
)

type H map[string]any

type HandlerFunc func(*Context)

type Context struct {
	Request *http.Request
	Writer  http.ResponseWriter
	values  map[string]any
	aborted bool
}

func (c *Context) JSON(code int, obj any) {
	if c.Writer == nil {
		return
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(code)
	_ = json.NewEncoder(c.Writer).Encode(obj)
}
func (c *Context) ShouldBindJSON(obj any) error                { return nil }
func (c *Context) Param(key string) string {
	if c.values == nil {
		return ""
	}
	v, _ := c.values["param:"+key].(string)
	return v
}
func (c *Context) Query(key string) string                     { return "" }
func (c *Context) DefaultQuery(key, def string) string         { return def }
func (c *Context) GetHeader(key string) string                 { return c.Request.Header.Get(key) }
func (c *Context) Abort()                                      { c.aborted = true }
func (c *Context) Next()                                       {}
func (c *Context) File(path string)                            { http.ServeFile(c.Writer, c.Request, path) }
func (c *Context) Set(key string, value any)                   { if c.values == nil { c.values = map[string]any{} }; c.values[key] = value }
func (c *Context) Get(key string) (any, bool)                  { v, ok := c.values[key]; return v, ok }

type route struct {
	method   string
	path     string
	handlers []HandlerFunc
}

type RouterGroup struct {
	engine *Engine
	prefix string
}

func (g *RouterGroup) Use(middleware ...HandlerFunc) {
	if g.engine != nil {
		g.engine.middleware = append(g.engine.middleware, middleware...)
	}
}
func (g *RouterGroup) Group(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	child := &RouterGroup{engine: g.engine, prefix: joinPaths(g.prefix, relativePath)}
	if len(handlers) > 0 {
		child.Use(handlers...)
	}
	return child
}
func (g *RouterGroup) GET(relativePath string, handlers ...HandlerFunc) {
	g.addRoute(http.MethodGet, relativePath, handlers...)
}
func (g *RouterGroup) POST(relativePath string, handlers ...HandlerFunc) {
	g.addRoute(http.MethodPost, relativePath, handlers...)
}
func (g *RouterGroup) PUT(relativePath string, handlers ...HandlerFunc) {
	g.addRoute(http.MethodPut, relativePath, handlers...)
}
func (g *RouterGroup) DELETE(relativePath string, handlers ...HandlerFunc) {
	g.addRoute(http.MethodDelete, relativePath, handlers...)
}

func (g *RouterGroup) addRoute(method, relativePath string, handlers ...HandlerFunc) {
	if g.engine == nil {
		return
	}
	full := joinPaths(g.prefix, relativePath)
	allHandlers := make([]HandlerFunc, 0, len(g.engine.middleware)+len(handlers))
	allHandlers = append(allHandlers, g.engine.middleware...)
	allHandlers = append(allHandlers, handlers...)
	g.engine.routes = append(g.engine.routes, route{method: method, path: full, handlers: allHandlers})
}

type Engine struct {
	RouterGroup
	routes      []route
	middleware  []HandlerFunc
	noRouteFunc []HandlerFunc
}

func Default() *Engine {
	e := &Engine{}
	e.RouterGroup = RouterGroup{engine: e, prefix: ""}
	return e
}
func (e *Engine) Use(middleware ...HandlerFunc) { e.middleware = append(e.middleware, middleware...) }
func (e *Engine) Group(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return e.RouterGroup.Group(relativePath, handlers...)
}
func (e *Engine) GET(relativePath string, handlers ...HandlerFunc)    { e.RouterGroup.GET(relativePath, handlers...) }
func (e *Engine) POST(relativePath string, handlers ...HandlerFunc)   { e.RouterGroup.POST(relativePath, handlers...) }
func (e *Engine) PUT(relativePath string, handlers ...HandlerFunc)    { e.RouterGroup.PUT(relativePath, handlers...) }
func (e *Engine) DELETE(relativePath string, handlers ...HandlerFunc) { e.RouterGroup.DELETE(relativePath, handlers...) }
func (e *Engine) Static(relativePath, root string) {}
func (e *Engine) NoRoute(handlers ...HandlerFunc)  { e.noRouteFunc = handlers }
func (e *Engine) Run(addr ...string) error {
	listenAddr := ":8080"
	if len(addr) > 0 && addr[0] != "" {
		listenAddr = addr[0]
	}
	return http.ListenAndServe(listenAddr, e)
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range e.routes {
		if route.method != r.Method {
			continue
		}
		if ok, params := matchPath(route.path, r.URL.Path); ok {
			c := &Context{Request: r, Writer: w, values: map[string]any{}}
			for k, v := range params {
				c.values["param:"+k] = v
			}
			for _, h := range route.handlers {
				h(c)
				if c.aborted {
					return
				}
			}
			return
		}
	}
	c := &Context{Request: r, Writer: w}
	if len(e.noRouteFunc) > 0 {
		for _, h := range e.noRouteFunc {
			h(c)
			if c.aborted {
				return
			}
		}
		return
	}
	w.WriteHeader(http.StatusNotFound)
}

func joinPaths(base, path string) string {
	if base == "" {
		if strings.HasPrefix(path, "/") {
			return path
		}
		return "/" + path
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func matchPath(pattern, path string) (bool, map[string]string) {
	pSeg := strings.Split(strings.Trim(pattern, "/"), "/")
	uSeg := strings.Split(strings.Trim(path, "/"), "/")
	if pattern == "/" && path == "/" {
		return true, map[string]string{}
	}
	if len(pSeg) != len(uSeg) {
		return false, nil
	}
	params := map[string]string{}
	for i := range pSeg {
		if strings.HasPrefix(pSeg[i], ":") {
			params[strings.TrimPrefix(pSeg[i], ":")] = uSeg[i]
			continue
		}
		if pSeg[i] != uSeg[i] {
			return false, nil
		}
	}
	return true, params
}
