package gateway

import (
	"context"
	"http-benchmark/pkg/config"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/stretchr/testify/assert"
)

const (
	backendResponse = "I am the backend"
)

func testServer() *server.Hertz {
	const backendResponse = "I am the backend"
	const backendStatus = 200
	h := server.Default(
		server.WithHostPorts("127.0.0.1:80"),
		server.WithExitWaitTime(1*time.Second),
		server.WithDisableDefaultDate(true),
		server.WithDisablePrintRoute(true),
		server.WithSenseClientDisconnection(true),
	)

	h.GET("/proxy/backend", func(cc context.Context, ctx *app.RequestContext) {
		ctx.Data(backendStatus, "application/json", []byte(backendResponse))
	})

	h.GET("/proxy/long-task", func(cc context.Context, ctx *app.RequestContext) {
		time.Sleep(5 * time.Second)
		ctx.Data(backendStatus, "application/json", []byte(backendResponse))
	})

	go h.Spin()
	time.Sleep(time.Second)
	return h
}

func TestServices(t *testing.T) {
	h := testServer()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = h.Shutdown(ctx)
	}()

	bifrost := &Bifrost{
		opts: &config.Options{
			Services: map[string]config.ServiceOptions{
				"testService": {
					Url: "http://localhost:80",
				},
			},
			Upstreams: map[string]config.UpstreamOptions{
				"testUpstream": {
					ID: "testUpstream",
					Targets: []config.TargetOptions{
						{
							Target: "127.0.0.1:80",
						},
					},
				},

				"test_upstream_no_port": {
					ID: "test_upstream_no_port",
					Targets: []config.TargetOptions{
						{
							Target: "127.0.0.1",
						},
					},
				},
			},
		},
	}

	ctx := context.Background()

	// direct proxy
	service, err := newService(bifrost, bifrost.opts.Services["testService"])
	assert.NoError(t, err)
	hzCtx := app.NewContext(0)
	hzCtx.Request.SetRequestURI("http://localhost:80/proxy/backend")
	service.ServeHTTP(ctx, hzCtx)
	assert.Equal(t, backendResponse, string(hzCtx.Response.Body()))

	// exist upstream
	serviceOpts := bifrost.opts.Services["testService"]
	serviceOpts.Url = "http://testUpstream"
	service, err = newService(bifrost, serviceOpts)
	assert.NoError(t, err)

	hzCtx = app.NewContext(0)
	hzCtx.Request.SetRequestURI("http://localhost:80/proxy/backend")
	service.ServeHTTP(ctx, hzCtx)
	assert.Equal(t, backendResponse, string(hzCtx.Response.Body()))

	serviceOpts.Url = "http://test_upstream_no_port"
	service, err = newService(bifrost, serviceOpts)
	assert.NoError(t, err)
	hzCtx = app.NewContext(0)
	hzCtx.Request.SetRequestURI("http://localhost/proxy/backend")
	service.ServeHTTP(ctx, hzCtx)
	assert.Equal(t, backendResponse, string(hzCtx.Response.Body()))

	// dynamic upstream
	serviceOpts = bifrost.opts.Services["testService"]
	serviceOpts.Url = "http://$test"
	service, err = newService(bifrost, serviceOpts)
	assert.NoError(t, err)

	hzCtx = app.NewContext(0)
	hzCtx.Set("$test", "testUpstream")
	hzCtx.Request.SetRequestURI("http://localhost:80/proxy/backend")
	service.ServeHTTP(ctx, hzCtx)
	assert.Equal(t, backendResponse, string(hzCtx.Response.Body()))
}

func TestDynamicService(t *testing.T) {
	h := testServer()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = h.Shutdown(ctx)
	}()

	bifrost := &Bifrost{
		opts: &config.Options{
			Services: map[string]config.ServiceOptions{
				"testService": {
					Url: "http://localhost:80",
				},
			},
			Upstreams: map[string]config.UpstreamOptions{
				"testUpstream": {
					ID: "testUpstream",
					Targets: []config.TargetOptions{
						{
							Target: "127.0.0.1:80",
						},
					},
				},

				"test_upstream_no_port": {
					ID: "test_upstream_no_port",
					Targets: []config.TargetOptions{
						{
							Target: "127.0.0.1",
						},
					},
				},
			},
		},
	}

	ctx := context.Background()
	services, err := loadServices(bifrost, nil)
	assert.NoError(t, err)

	dynamicService := newDynamicService("$dd", services)

	hzCtx := app.NewContext(0)
	hzCtx.Set("$dd", "testService")
	hzCtx.Request.SetRequestURI("http://localhost:80/proxy/backend")
	dynamicService.ServeHTTP(ctx, hzCtx)
	assert.Equal(t, backendResponse, string(hzCtx.Response.Body()))
}
