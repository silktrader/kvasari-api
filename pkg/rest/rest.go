package rest

import (
	"errors"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"net/http"
)

// Config is used to provide dependencies and configuration to the New function.
type Config struct {
	Logger logrus.FieldLogger
}

func New(cfg Config) (engine Engine, err error) {

	// assign a logger or fail
	if cfg.Logger == nil {
		return engine, errors.New("logger is required")
	}
	engine.baseLogger = cfg.Logger

	engine.router = httprouter.New()

	// disables redirections such as `/foo/` to `/foo`
	engine.router.RedirectTrailingSlash = false

	// disables attempts to fix common path issues and redirects them, i.e. `/FoO` redirects to `/foo`
	engine.router.RedirectFixedPath = false

	return engine, nil
}

// Engine contains the muxer, logger and middleware.
// tk enforce singleton? tk turn into an interface as with the original? what's the advantage; it won't be substituted in tests or with different implementations
type Engine struct {
	router *httprouter.Router

	// a middleware queue; invocation order follows insertion order
	middleware []func(http.Handler) http.Handler

	// baseLogger is a logger for non-requests contexts, like goroutines or background tasks not started by a request
	baseLogger logrus.FieldLogger
}

// Handler returns an instance of httprouter.Router that handle APIs registered here
func (e *Engine) Handler() http.Handler {
	// legacy routes were registered here
	return e.router
}

// Handle registers the path and method to the given handler. Also applies the middleware to the Handler
// Handle calls the base router, to register the method, path and handler.
func (e *Engine) Handle(method string, path string, handler http.Handler, middleware ...func(http.Handler) http.Handler) {

	// first apply the router's globally defined middleware
	for _, mw := range e.middleware {
		handler = mw(handler)
	}

	// then apply the per-route specific middleware
	for _, mw := range middleware {
		handler = mw(handler)
	}

	// associate the final composed handler to the selected path and method pair
	e.router.Handler(method, path, handler)
}

// Use specifies one or multiple new handlers that will be evaluated for every specified route (ie. logger).
func (e *Engine) Use(mw ...func(http.Handler) http.Handler) {
	e.middleware = append(e.middleware, mw...)
}

// Get defines a new GET method handler for the specified path.
// The variadic arguments can include middleware that will be exclusively evaluated for the path.
func (e *Engine) Get(path string, handlerFunc http.HandlerFunc, middleware ...func(http.Handler) http.Handler) {
	e.Handle(http.MethodGet, path, handlerFunc, middleware...)
}

func (e *Engine) Post(path string, handlerFunc http.HandlerFunc, middleware ...func(http.Handler) http.Handler) {
	e.Handle(http.MethodPost, path, handlerFunc, middleware...)
}

func (e *Engine) Put(path string, handlerFunc http.HandlerFunc, middleware ...func(http.Handler) http.Handler) {
	e.Handle(http.MethodPut, path, handlerFunc, middleware...)
}
