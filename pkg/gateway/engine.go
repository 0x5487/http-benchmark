package gateway

import (
	"context"
	"fmt"
	"http-benchmark/pkg/config"
	"http-benchmark/pkg/log"

	"github.com/cloudwego/hertz/pkg/app"
	hzconfig "github.com/cloudwego/hertz/pkg/common/config"
	"github.com/hertz-contrib/obs-opentelemetry/provider"
	"github.com/hertz-contrib/obs-opentelemetry/tracing"
)

type Engine struct {
	ID              string
	opts            config.Options
	handlers        app.HandlersChain
	middlewares     app.HandlersChain
	notFoundHandler app.HandlerFunc

	options []hzconfig.Option
}

func newEngine(bifrost *Bifrost, entryOpts config.EntryOptions) (*Engine, error) {

	// middlewares
	middlewares, err := loadMiddlewares(bifrost.opts.Middlewares)
	if err != nil {
		return nil, err
	}

	// services
	services, err := loadServices(bifrost, middlewares)
	if err != nil {
		return nil, err
	}

	// routes
	router, err := loadRouter(bifrost, entryOpts, services, middlewares)
	if err != nil {
		return nil, err
	}

	// engine
	engine := &Engine{
		opts:            *bifrost.opts,
		handlers:        make([]app.HandlerFunc, 0),
		notFoundHandler: nil,
		options:         make([]hzconfig.Option, 0),
	}

	// tracing
	if bifrost.opts.Tracing.Enabled {

		provider.NewOpenTelemetryProvider(
			provider.WithEnableMetrics(false),
			provider.WithServiceName("bifrost"),
		)

		tracer, cfg := tracing.NewServerTracer()
		engine.options = append(engine.options, tracer)
		tracingServerMiddleware := tracing.ServerMiddleware(cfg)
		engine.Use(tracingServerMiddleware)
	}

	// init middlewares
	logger, err := log.NewLogger(entryOpts.Logging)
	if err != nil {
		return nil, err
	}
	initMiddleware := newInitMiddleware(entryOpts.ID, logger)
	engine.Use(initMiddleware.ServeHTTP)

	// set entry's middlewares
	for _, middleware := range entryOpts.Middlewares {

		if len(middleware.Use) > 0 {
			val, found := middlewares[middleware.Use]
			if !found {
				return nil, fmt.Errorf("middleware '%s' was not found in entry id: '%s'", middleware.Use, entryOpts.ID)
			}

			engine.Use(val)
			continue
		}

		if len(middleware.Type) == 0 {
			return nil, fmt.Errorf("middleware kind can't be empty in entry id: '%s'", entryOpts.ID)
		}

		handler, found := middlewareFactory[middleware.Type]
		if !found {
			return nil, fmt.Errorf("middleware handler '%s' was not found in entry id: '%s'", middleware.Type, entryOpts.ID)
		}

		m, err := handler(middleware.Params)
		if err != nil {
			return nil, err
		}

		engine.Use(m)
	}

	engine.Use(router.ServeHTTP)

	return engine, nil
}

func (e *Engine) ServeHTTP(c context.Context, ctx *app.RequestContext) {
	ctx.SetIndex(-1)
	ctx.SetHandlers(e.middlewares)
	ctx.Next(c)
	ctx.Abort()
}

func (e *Engine) OnShutdown() {

}

func (e *Engine) Use(middleware ...app.HandlerFunc) {
	e.handlers = append(e.handlers, middleware...)
	e.middlewares = e.handlers

	if e.notFoundHandler != nil {
		e.middlewares = append(e.handlers, e.notFoundHandler)
	}
}
