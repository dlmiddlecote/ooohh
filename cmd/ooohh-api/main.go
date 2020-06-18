package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ardanlabs/conf"
	"github.com/blendle/zapdriver"
	"github.com/boltdb/bolt"
	kitapi "github.com/dlmiddlecote/kit/api"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/dlmiddlecote/ooohh/pkg/api"
	"github.com/dlmiddlecote/ooohh/pkg/service"
)

const (
	// buildVersion is the git version of this program. It is set using build flags.
	buildVersion = "dev"
	// namespace is the prefix used for application configuration.
	namespace = "OOOHH"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stdout, "error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	//
	// Configuration
	//

	var cfg struct {
		Web struct {
			APIHost         string        `conf:"default:0.0.0.0:8080"`
			DebugHost       string        `conf:"default:0.0.0.0:8090"`
			EnableDebug     bool          `conf:"default:true"`
			ShutdownTimeout time.Duration `conf:"default:5s"`
		}
		DB struct {
			Path string `conf:"default:/tmp/ooohh.db"`
		}
	}

	// Parse configuration, showing usage if needed.
	if err := conf.Parse(os.Args[1:], namespace, &cfg); err != nil {
		if err == conf.ErrHelpWanted {
			usage, err := conf.Usage(namespace, &cfg)
			if err != nil {
				return errors.Wrap(err, "generating config usage")
			}

			fmt.Println(usage)
			return nil
		}
		return errors.Wrap(err, "parsing config")
	}

	//
	// Logging
	//

	var logger *zap.SugaredLogger
	{
		if l, err := zapdriver.NewProduction(); err != nil {
			return errors.Wrap(err, "creating logger")
		} else {
			logger = l.Sugar()
		}
	}
	// Flush logs at the end of the applications lifetime
	defer logger.Sync()

	logger.Infow("Application starting", "version", buildVersion)
	defer logger.Info("Application finished")

	//
	// DB
	//

	db, err := bolt.Open(cfg.DB.Path, 0600, nil)
	if err != nil {
		return errors.Wrap(err, "opening db")
	}
	defer db.Close()

	//
	// Debug listener
	//

	if cfg.Web.EnableDebug {

		// Expose Prometheus metrics at '/metrics'.
		http.Handle("/metrics", promhttp.Handler())

		// Start the debug listener in the background, we don't gracefully shut this down.
		go func() {
			logger.Infow("Debug listener starting", "addr", cfg.Web.DebugHost)
			err := http.ListenAndServe(cfg.Web.DebugHost, http.DefaultServeMux)
			logger.Infow("Debug listener closed", "err", err)
		}()
	}

	//
	// Application server setup
	//

	var app http.Server
	{
		now := func() time.Time {
			return time.Now()
		}

		// Initialise our ooohh service. This exposes all our desired interactions.
		s, err := service.NewService(db, logger.Named("service"), now)
		if err != nil {
			return errors.Wrap(err, "creating service")
		}

		// Create our API. This is an implementation of the kit API.
		// It has a dependency on the ooohh service, as it provides this service as a
		// HTTP API.
		oApi := api.NewAPI(logger.Named("api"), s)

		// Create our http.Server, exposing the account API on the given host.
		app = kitapi.NewServer(cfg.Web.APIHost, logger.Named("http"), oApi)
	}

	// Make a channel to listen for an interrupt or terminate signal from the OS.
	// Use a buffered channel because the signal package requires it.
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	// Make a channel to listen for errors coming from the listener. Use a
	// buffered channel so the goroutine can exit if we don't collect this error.
	serverErrors := make(chan error, 1)

	// Start the server listening for requests.
	go func() {
		logger.Infow("API listener starting", "addr", app.Addr)
		serverErrors <- app.ListenAndServe()
	}()

	//
	// Shutdown
	//

	// Blocking main and waiting for shutdown.
	select {
	case err := <-serverErrors:
		return errors.Wrap(err, "server error")

	case sig := <-shutdown:
		logger.Infow("Start shutdown", "signal", sig)

		// Give outstanding requests a deadline for completion.
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancel()

		// Asking listener to shutdown and load shed.
		err := app.Shutdown(ctx)
		if err != nil {
			logger.Infow("Graceful shutdown did not complete", "err", err)
			err = app.Close()
		}

		if err != nil {
			return errors.Wrap(err, "could not stop server gracefully")
		}
	}

	return nil
}
