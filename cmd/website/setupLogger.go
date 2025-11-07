package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/adampresley/adampresleyphotography/cmd/website/internal/configuration"
)

func setupLogger(config *configuration.Config, version string) {
	var (
		logger *slog.Logger
	)

	level := slog.LevelInfo

	switch strings.ToLower(config.LogLevel) {
	case "debug":
		level = slog.LevelDebug

	case "error":
		level = slog.LevelError

	default:
		level = slog.LevelInfo
	}

	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}).WithAttrs([]slog.Attr{
		slog.String("version", version),
	})

	logger = slog.New(h)
	slog.SetDefault(logger)
}
