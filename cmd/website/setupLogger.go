package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/adampresley/adampresleyphotography/cmd/website/internal/configuration"
	"gorm.io/gorm/logger"
)

type DbSlogger struct{}

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

func setupDbLogger(config *configuration.Config) logger.Interface {
	return logger.New(
		DbSlogger{},
		logger.Config{
			SlowThreshold:             time.Second * 5,
			Colorful:                  false,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      false,
			LogLevel:                  getDbLevel(config.LogLevel),
		},
	)
}

func getDbLevel(level string) logger.LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return logger.Info
	case "warn":
		return logger.Warn
	default:
		return logger.Error
	}
}

func (d DbSlogger) Printf(format string, values ...interface{}) {
	slog.Info(fmt.Sprintf(format, values...))
}
