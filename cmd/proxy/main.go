package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/davidalecrim/extreme/config"
	"github.com/davidalecrim/extreme/proxy"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	logOpts := &slog.HandlerOptions{
		Level: cfg.Logging.GetLevel(),
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, logOpts))
	slog.SetDefault(logger)

	p, err := proxy.New(cfg, logger)
	if err != nil {
		logger.Error("failed to create proxy", "error", err)
		os.Exit(1)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := p.Start(); err != nil {
			logger.Error("failed to start proxy", "error", err)
			os.Exit(1)
		}
	}()

	<-sigChan

	if err := p.Shutdown(); err != nil {
		logger.Error("error during shutdown", "error", err)
	}
}
