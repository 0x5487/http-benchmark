package gateway

import (
	"fmt"
	"http-benchmark/pkg/config"
	"http-benchmark/pkg/log"
	"http-benchmark/pkg/provider/file"
	"http-benchmark/pkg/tracer/accesslog"
	"http-benchmark/pkg/tracer/prometheus"
	"http-benchmark/pkg/zero"
	"log/slog"
	"time"

	"github.com/cloudwego/hertz/pkg/common/tracer"
	"github.com/rs/dnscache"
)

type reloadFunc func(bifrost *Bifrost) error

type Bifrost struct {
	configPath   string
	opts         *config.Options
	fileProvider *file.FileProvider
	httpServers  map[string]*HTTPServer
	resolver     *dnscache.Resolver
	reloadCh     chan bool
	stopCh       chan bool
	onReload     reloadFunc
	zero         *zero.ZeroDownTime
}

func (b *Bifrost) Run() {
	i := 0
	for _, server := range b.httpServers {
		if i == len(b.httpServers)-1 {
			// last server need to blocked
			server.Run()
		}
		go server.Run()
		i++
	}
}

func (b *Bifrost) ZeroDownTime() *zero.ZeroDownTime {
	return b.zero
}

func (b *Bifrost) stop() {
	b.stopCh <- true
}

func (b *Bifrost) Shutdown() {
	b.stop()

}

func LoadFromConfig(path string) (*Bifrost, error) {
	return loadFromConfig(path, false)
}

func loadFromConfig(path string, isReload bool) (*Bifrost, error) {
	if !fileExist(path) {
		return nil, fmt.Errorf("config file not found, path: %s", path)
	}

	// main config file
	fileProviderOpts := config.FileProviderOptions{
		Paths: []string{path},
	}

	fileProvider := file.NewProvider(fileProviderOpts)

	cInfo, err := fileProvider.Open()
	if err != nil {
		return nil, err
	}

	mainOpts, err := parseContent(cInfo[0].Content)
	if err != nil {
		return nil, err
	}
	fileProvider.Reset()

	// file provider
	if mainOpts.Providers.File.Enabled && len(mainOpts.Providers.File.Paths) > 0 {
		for _, content := range mainOpts.Providers.File.Paths {
			fileProvider.Add(content)
		}

		cInfo, err = fileProvider.Open()
		if err != nil {
			return nil, err
		}

		for _, c := range cInfo {
			mainOpts, err = mergeOptions(mainOpts, c.Content)
			if err != nil {
				errMsg := fmt.Sprintf("path: %s, error: %s", c.Path, err.Error())
				return nil, fmt.Errorf(errMsg)
			}
		}
	}

	bifrost, err := load(mainOpts, isReload)
	if err != nil {
		return nil, err
	}

	if !isReload {
		reloadCh := make(chan bool)
		bifrost.fileProvider = fileProvider
		bifrost.configPath = path
		bifrost.onReload = reload
		bifrost.reloadCh = reloadCh

		if mainOpts.Providers.File.Watch {
			fileProvider.Add(path)
			fileProvider.OnChanged = func() error {
				reloadCh <- true
				return nil
			}
			_ = fileProvider.Watch()
			bifrost.watch()
		}
	}

	return bifrost, nil
}

func Load(opts config.Options) (*Bifrost, error) {
	return load(opts, false)
}

func load(opts config.Options, isReload bool) (*Bifrost, error) {
	// validate
	err := validateOptions(opts)
	if err != nil {
		return nil, err
	}

	zeroOptions := zero.Options{
		SocketPath: "./bifrost.sock",
		PIDFile:    "./bifrost.pid",
	}

	bifrsot := &Bifrost{
		resolver:    &dnscache.Resolver{},
		httpServers: make(map[string]*HTTPServer),
		opts:        &opts,
		stopCh:      make(chan bool),
		reloadCh:    make(chan bool),
		zero:        zero.New(zeroOptions),
	}

	go func() {
		t := time.NewTimer(1 * time.Hour)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				bifrsot.resolver.Refresh(true)
				slog.Info("refresh dns cache successfully")
			case <-bifrsot.stopCh:
				return
			}
		}
	}()

	// system logger
	logger, err := log.NewLogger(opts.Logging)
	if err != nil {
		return nil, err
	}
	slog.SetDefault(logger)

	tracers := []tracer.Tracer{}

	// prometheus tracer
	if opts.Metrics.Prometheus.Enabled && !isReload {
		promOpts := []prometheus.Option{}

		if len(opts.Metrics.Prometheus.Buckets) > 0 {
			promOpts = append(promOpts, prometheus.WithHistogramBuckets(opts.Metrics.Prometheus.Buckets))
		}

		promTracer := prometheus.NewTracer(promOpts...)
		tracers = append(tracers, promTracer)
	}

	// access log
	accessLogTracers := map[string]*accesslog.Tracer{}
	if len(opts.AccessLogs) > 0 && !isReload {

		for id, accessLogOptions := range opts.AccessLogs {
			if !accessLogOptions.Enabled {
				continue
			}

			accessLogTracer, err := accesslog.NewTracer(accessLogOptions)
			if err != nil {
				return nil, err
			}

			if accessLogTracer != nil {
				accessLogTracers[id] = accessLogTracer
			}
		}
	}

	for id, server := range opts.Servers {
		if id == "" {
			return nil, fmt.Errorf("http server id can't be empty")
		}

		server.ID = id

		if server.Bind == "" {
			return nil, fmt.Errorf("http server bind can't be empty")
		}

		_, found := bifrsot.httpServers[id]
		if found {
			return nil, fmt.Errorf("http server '%s' already exists", id)
		}

		if len(server.AccessLogID) > 0 {
			_, found := opts.AccessLogs[server.AccessLogID]
			if !found {
				return nil, fmt.Errorf("access log '%s' was not found in server '%s'", server.AccessLogID, server.ID)
			}

			accessLogTracer, found := accessLogTracers[server.AccessLogID]
			if found {
				tracers = append(tracers, accessLogTracer)
			}
		}

		httpServer, err := newHTTPServer(bifrsot, server, tracers)
		if err != nil {
			return nil, err
		}

		bifrsot.httpServers[id] = httpServer
	}

	return bifrsot, nil
}

func (b *Bifrost) watch() {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("bifrost: unknown error when reload config", "error", err)
				b.watch()
			}
		}()

		for {
			select {
			case <-b.reloadCh:
				err := b.onReload(b)
				if err != nil {
					slog.Error("bifrost: fail to reload config", "error", err)
				}
			case <-b.stopCh:
				return
			}
		}
	}()
}

func reload(bifrost *Bifrost) error {
	slog.Info("bifrost: reloading...")

	newBifrost, err := loadFromConfig(bifrost.configPath, true)
	if err != nil {
		return err
	}
	defer func() {
		newBifrost.stop()
	}()

	isReloaded := false

	for id, httpServer := range bifrost.httpServers {
		newServer, found := newBifrost.httpServers[id]
		if found && httpServer.serverOpts.Bind == newServer.serverOpts.Bind {
			httpServer.switcher.SetEngine(newServer.switcher.Engine())
			isReloaded = true
		}
	}

	slog.Info("bifrost is reloaded successfully", "isReloaded", isReloaded)

	return nil
}
