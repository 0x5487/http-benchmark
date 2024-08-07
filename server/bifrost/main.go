package main

import (
	"context"
	"http-benchmark/pkg/gateway"
	"http-benchmark/pkg/log"
	"log/slog"

	"github.com/cloudwego/hertz/pkg/app"
	_ "go.uber.org/automaxprocs"
)

type FindMyHome struct {
}

func (f *FindMyHome) ServeHTTP(c context.Context, ctx *app.RequestContext) {
	logger := log.FromContext(c)
	logger.Info("find my home")
	ctx.Set("$home", "default")
}

var bifrost *gateway.Bifrost

func main() {
	defer func() {
		if bifrost != nil {
			bifrost.Shutdown()
		}
	}()

	//runtime.GOMAXPROCS(5)

	var err error
	_ = gateway.DisableGopool()

	err = gateway.RegisterMiddleware("find_upstream", func(param map[string]any) (app.HandlerFunc, error) {
		m := FindMyHome{}
		return m.ServeHTTP, nil
	})
	if err != nil {
		panic(err)
	}

	bifrost, err = gateway.LoadFromConfig("./config.yaml")
	if err != nil {
		slog.Error("fail to start bifrost", "error", err)
		return
	}

	bifrost.Run()
}
