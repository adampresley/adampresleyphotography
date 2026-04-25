package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	pathpkg "path"
	"time"

	"github.com/adampresley/adampresleyphotography/pkg/models"
	_ "github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultPostgresDSN = "host=localhost user=adampresleyphotography password=password dbname=adampresleyphotography port=5432 sslmode=disable"
	defaultSQLiteDSN   = "file:./data/adampresleyphotography.db"
)

func main() {
	sqliteDSN := getenv("SQLITE_DSN", defaultSQLiteDSN)
	postgresDSN := getenv("DSN", defaultPostgresDSN)

	sqliteDB, err := sql.Open("sqlite", sqliteDSN)
	if err != nil {
		log.Fatalf("open sqlite database: %v", err)
	}
	defer sqliteDB.Close()

	postgresDB, err := gorm.Open(postgres.Open(postgresDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}

	if err = postgresDB.AutoMigrate(&models.Client{}, &models.Album{}, &models.Favorite{}); err != nil {
		log.Fatalf("auto migrate postgres schema: %v", err)
	}

	clientCount, err := migrateClients(sqliteDB, postgresDB)
	if err != nil {
		log.Fatalf("migrate clients: %v", err)
	}

	albumCount, err := migrateAlbums(sqliteDB, postgresDB)
	if err != nil {
		log.Fatalf("migrate albums: %v", err)
	}

	favoriteCount, err := migrateFavorites(sqliteDB, postgresDB)
	if err != nil {
		log.Fatalf("migrate favorites: %v", err)
	}

	if err = resetSequence(postgresDB, "clients", "id"); err != nil {
		log.Fatalf("reset clients sequence: %v", err)
	}

	if err = resetSequence(postgresDB, "albums", "id"); err != nil {
		log.Fatalf("reset albums sequence: %v", err)
	}

	log.Printf("migration complete: clients=%d albums=%d favorites=%d", clientCount, albumCount, favoriteCount)
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func migrateClients(sqliteDB *sql.DB, postgresDB *gorm.DB) (int, error) {
	rows, err := sqliteDB.Query(`
SELECT id, created_at, updated_at, deleted_at, password, name, email
FROM clients
`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			client   models.Client
			created  sql.NullString
			updated  sql.NullString
			deleted  sql.NullString
			password sql.NullString
			name     sql.NullString
			email    sql.NullString
		)

		if err = rows.Scan(&client.ID, &created, &updated, &deleted, &password, &name, &email); err != nil {
			return count, err
		}

		client.CreatedAt = nullableTime(created)
		client.UpdatedAt = nullableTime(updated)
		client.DeletedAt = gorm.DeletedAt{Time: nullableTime(deleted), Valid: deleted.Valid}
		client.Password = password.String
		client.Name = name.String
		client.Email = email.String

		if err = postgresDB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&client).Error; err != nil {
			return count, err
		}

		count++
	}

	return count, rows.Err()
}

func migrateAlbums(sqliteDB *sql.DB, postgresDB *gorm.DB) (int, error) {
	rows, err := sqliteDB.Query(`
SELECT id, created_at, updated_at, deleted_at, name, path, client_id, shoot_date, poster_image_path, COALESCE(poster_y_pos, '')
FROM albums
`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var (
			album       models.Album
			created     sql.NullString
			updated     sql.NullString
			deleted     sql.NullString
			name        sql.NullString
			path        sql.NullString
			shootDate   sql.NullString
			posterImage sql.NullString
		)

		if err = rows.Scan(
			&album.ID,
			&created,
			&updated,
			&deleted,
			&name,
			&path,
			&album.ClientID,
			&shootDate,
			&posterImage,
			&album.PosterYPos,
		); err != nil {
			return count, err
		}

		album.CreatedAt = nullableTime(created)
		album.UpdatedAt = nullableTime(updated)
		album.DeletedAt = gorm.DeletedAt{Time: nullableTime(deleted), Valid: deleted.Valid}
		album.Name = name.String
		album.Path = normalizeAlbumPath(album.ClientID, album.ID, path.String)
		album.ShootDate = nullableTime(shootDate)
		album.PosterImagePath = normalizePosterImagePath(posterImage.String)

		if err = postgresDB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&album).Error; err != nil {
			return count, err
		}

		count++
	}

	return count, rows.Err()
}

func normalizeAlbumPath(clientID uint, albumID uint, originalPath string) string {
	if originalPath == "" {
		return ""
	}

	return fmt.Sprintf("clients/%d/%d/originals", clientID, albumID)
}

func normalizePosterImagePath(originalPath string) string {
	return pathpkg.Base(originalPath)
}

func migrateFavorites(sqliteDB *sql.DB, postgresDB *gorm.DB) (int, error) {
	rows, err := sqliteDB.Query(`
SELECT client_id, album_id, image_path
FROM favorites
`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var favorite models.Favorite
		if err = rows.Scan(&favorite.ClientID, &favorite.AlbumID, &favorite.ImagePath); err != nil {
			return count, err
		}

		if err = postgresDB.Clauses(clause.OnConflict{DoNothing: true}).Create(&favorite).Error; err != nil {
			return count, err
		}

		count++
	}

	return count, rows.Err()
}

func nullableTime(value sql.NullString) time.Time {
	if value.Valid {
		result, err := time.Parse(time.RFC3339, value.String)
		if err == nil {
			return result
		}
	}

	return time.Time{}
}

func resetSequence(db *gorm.DB, tableName, columnName string) error {
	sql := fmt.Sprintf(
		"SELECT setval(pg_get_serial_sequence('%s', '%s'), COALESCE((SELECT MAX(%s) FROM %s), 1), (SELECT COUNT(*) > 0 FROM %s))",
		tableName,
		columnName,
		columnName,
		tableName,
		tableName,
	)

	return db.Exec(sql).Error
}
