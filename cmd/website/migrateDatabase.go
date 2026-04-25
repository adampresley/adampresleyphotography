package main

import (
	"log/slog"
	"os"

	"github.com/adampresley/adampresleyphotography/pkg/models"
	"gorm.io/gorm"
)

func migrateDatabase(db *gorm.DB) {
	if err := db.AutoMigrate(
		&models.Client{},
		&models.Album{},
		&models.Favorite{},
	); err != nil {
		slog.Error("error running database migrations", "error", err)
		os.Exit(1)
	}
}
