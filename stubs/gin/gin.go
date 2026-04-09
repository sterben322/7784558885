package gin

import "net/http"

type H map[string]any

type HandlerFunc func(*Context)

type Context struct {
	Request *http.Request
	values  map[string]any
}

func (c *Context) JSON(code int, obj any)                      {}
func (c *Context) ShouldBindJSON(obj any) error                { return nil }
func (c *Context) Param(key string) string                     { return "" }
func (c *Context) Query(key string) string                     { return "" }
func (c *Context) DefaultQuery(key, def string) string         { return def }
func (c *Context) GetHeader(key string) string                 { return "" }
func (c *Context) Abort()                                      {}
func (c *Context) Next()                                       {}
func (c *Context) File(path string)                            {}
func (c *Context) Set(key string, value any)                   { if c.values == nil { c.values = map[string]any{} }; c.values[key] = value }
func (c *Context) Get(key string) (any, bool)                  { v, ok := c.values[key]; return v, ok }

type RouterGroup struct{}

func (g *RouterGroup) Use(middleware ...HandlerFunc)                 {}
func (g *RouterGroup) Group(relativePath string, handlers ...HandlerFunc) *RouterGroup { return &RouterGroup{} }
func (g *RouterGroup) GET(relativePath string, handlers ...HandlerFunc)    {}
func (g *RouterGroup) POST(relativePath string, handlers ...HandlerFunc)   {}
func (g *RouterGroup) PUT(relativePath string, handlers ...HandlerFunc)    {}
func (g *RouterGroup) DELETE(relativePath string, handlers ...HandlerFunc) {}

type Engine struct{ RouterGroup }

func Default() *Engine                                               { return &Engine{} }
func (e *Engine) Use(middleware ...HandlerFunc)                      {}
func (e *Engine) Group(relativePath string, handlers ...HandlerFunc) *RouterGroup { return &RouterGroup{} }
func (e *Engine) GET(relativePath string, handlers ...HandlerFunc)   {}
func (e *Engine) POST(relativePath string, handlers ...HandlerFunc)  {}
func (e *Engine) PUT(relativePath string, handlers ...HandlerFunc)   {}
func (e *Engine) DELETE(relativePath string, handlers ...HandlerFunc) {}
func (e *Engine) Static(relativePath, root string)                   {}
func (e *Engine) NoRoute(handlers ...HandlerFunc)                    {}
func (e *Engine) Run(addr ...string) error                           { return nil }
