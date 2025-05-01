package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flashbots/go-utils/rpcclient"
	"github.com/google/uuid"
	"github.com/holisticode/mev-rpc/blocktrace"
	"github.com/holisticode/mev-rpc/common"
	"github.com/holisticode/mev-rpc/database"
	"github.com/holisticode/mev-rpc/httpserver"
	"github.com/urfave/cli/v2" // imports as package "cli"
)

var flags []cli.Flag = []cli.Flag{
	&cli.StringFlag{
		Name:  "listen-addr",
		Value: "127.0.0.1:8080",
		Usage: "address to listen on for API",
	},
	&cli.StringFlag{
		Name:  "metrics-addr",
		Value: "127.0.0.1:8090",
		Usage: "address to listen on for Prometheus metrics",
	},
	&cli.BoolFlag{
		Name:  "log-json",
		Value: false,
		Usage: "log in JSON format",
	},
	&cli.BoolFlag{
		Name:  "log-debug",
		Value: true,
		Usage: "log debug messages",
	},
	&cli.BoolFlag{
		Name:  "log-uid",
		Value: false,
		Usage: "generate a uuid and add to all log messages",
	},
	&cli.StringFlag{
		Name:  "log-service",
		Value: "MEV-Block-Tracer",
		Usage: "add 'service' tag to logs",
	},
	&cli.BoolFlag{
		Name:  "pprof",
		Value: false,
		Usage: "enable pprof debug endpoint",
	},
	&cli.Int64Flag{
		Name:  "drain-seconds",
		Value: 45,
		Usage: "seconds to wait in drain HTTP request",
	},
	&cli.StringFlag{
		Name:  "db-connection-string",
		Value: "",
		Usage: "postgres database backend",
	},
	&cli.StringFlag{
		Name:     "rpc-endpoint",
		Value:    "",
		Usage:    "chain rpc endpoint",
		Required: true,
	},
}

func main() {
	app := &cli.App{
		Name:  "httpserver",
		Usage: "Serve API, and metrics",
		Flags: flags,
		Action: func(cCtx *cli.Context) error {
			listenAddr := cCtx.String("listen-addr")
			metricsAddr := cCtx.String("metrics-addr")
			logJSON := cCtx.Bool("log-json")
			logDebug := cCtx.Bool("log-debug")
			logUID := cCtx.Bool("log-uid")
			logService := cCtx.String("log-service")
			enablePprof := cCtx.Bool("pprof")
			drainDuration := time.Duration(cCtx.Int64("drain-seconds")) * time.Second

			log := common.SetupLogger(&common.LoggingOpts{
				Debug:   logDebug,
				JSON:    logJSON,
				Service: logService,
				Version: common.Version,
			})

			if logUID {
				id := uuid.Must(uuid.NewRandom())
				log = log.With("uid", id.String())
			}

			cfg := &httpserver.HTTPServerConfig{
				ListenAddr:  listenAddr,
				MetricsAddr: metricsAddr,
				Log:         log,
				EnablePprof: enablePprof,

				DrainDuration:            drainDuration,
				GracefulShutdownDuration: 30 * time.Second,
				ReadTimeout:              60 * time.Second,
				WriteTimeout:             30 * time.Second,
			}

			rpcEndpoint := cCtx.String("rpc-endpoint")
			dbConn := cCtx.String("db-connection-string")

			log.Debug("Creating DB backend connection...")
			storage, err := database.NewStorage(dbConn)
			if err != nil {
				cfg.Log.Error("failed to create database service", "err", err)
				return err
			}

			log.Debug("Creating Block Tracer...")
			rpcClient := rpcclient.NewClient(rpcEndpoint)
			tracer := blocktrace.NewBlockTracer(rpcClient, storage, log)
			// TODO cleanup
			log.Info("Starting tracer...")
			ctx, cancel := context.WithCancel(context.Background())
			go tracer.Start(ctx, blocktrace.POLLING_INTERVAL)

			log.Info("Starting RPC server...")
			srv, err := httpserver.New(cfg)
			if err != nil {
				cfg.Log.Error("failed to create server", "err", err)
				return err
			}

			exit := make(chan os.Signal, 1)
			signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
			srv.RunInBackground()
			<-exit
			cancel()

			// Shutdown server once termination signal is received
			log.Info("Shutting down the application")
			srv.Shutdown()
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
