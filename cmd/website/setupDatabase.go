package main

import (
	"log/slog"
	"os"
	"strings"

	"github.com/adampresley/adampresleyphotography/cmd/website/internal/configuration"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupDatabase(config *configuration.Config, dbLogger logger.Interface) *gorm.DB {
	db, err := gorm.Open(postgres.Open(config.DSN), &gorm.Config{Logger: dbLogger})
	if err != nil {
		slog.Error("error connecting to database", "error", err, "dsn", cleanCredentials(config.DSN))
		os.Exit(1)
	}

	slog.Info("connected to database", "dsn", cleanCredentials(config.DSN))
	migrateDatabase(db)

	return db
}

func cleanCredentials(dsn string) string {
	result := dsn

	for _, token := range strings.Fields(dsn) {
		if strings.HasPrefix(token, "password=") {
			result = strings.ReplaceAll(result, token, "password=********")
		}
	}

	return result
}
