package services

import (
	"context"
	"fmt"
	"time"

	"github.com/adampresley/adampresleyphotography/pkg/models"
	"github.com/rfberaldo/sqlz"
)

type AlbumServicer interface {
	GetAlbum(clientID uint, albumID uint) (*models.Album, error)
	GetAlbumList(clientID uint) ([]*models.Album, error)
	ToggleFavorite(clientID, albumID uint, key string) (bool, error)
}

type AlbumServiceConfig struct {
	DB *sqlz.DB
}

type AlbumService struct {
	db *sqlz.DB
}

func NewAlbumService(config AlbumServiceConfig) AlbumService {
	return AlbumService{
		db: config.DB,
	}
}

func (s AlbumService) GetAlbum(clientID, albumID uint) (*models.Album, error) {
	var (
		err error
	)

	result := &models.Album{}

	sql := `
SELECT
   a.id 
   , a.created_at 
   , a.updated_at
   , a.deleted_at
   , a.name 
   , a."path"
   , a.shoot_date
   , a.client_id
   , a.poster_image_path
	, COALESCE(a.poster_y_pos, '') AS poster_y_pos
   , c.id AS "client.id"
   , c.created_at AS "client.created_at"
   , c.updated_at AS "client.updated_at"
   , c.deleted_at AS "client.deleted_at"
   , c.name AS "client.name"
FROM albums AS a
   INNER JOIN clients AS c ON c.id=a.client_id
WHERE 1=1
   AND a.deleted_at IS NULL
   AND c.deleted_at IS NULL
   AND a.id=?
   AND a.client_id=?
   `

	params := []any{albumID, clientID}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err = s.db.QueryRow(ctx, result, sql, params...); err != nil {
		return result, fmt.Errorf("error querying for album %d, client %d: %w", albumID, clientID, err)
	}

	// Get favorites
	sql = `
SELECT
	album_id
	, client_id
	, image_path
FROM favorites
WHERE 1=1
	AND client_id=?
	AND album_id=?
	`
	params = []any{clientID, albumID}

	ctx, cancel = context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err = s.db.Query(ctx, &result.Favorites, sql, params...); err != nil {
		return result, fmt.Errorf("error querying for favorites for album %d, client %d: %w", albumID, clientID, err)
	}

	return result, nil
}

func (s AlbumService) GetAlbumList(clientID uint) ([]*models.Album, error) {
	var (
		err error
	)

	result := []*models.Album{}

	sql := `
SELECT
   a.id
   , a.created_at
   , a.updated_at
   , a.deleted_at
   , a.name
   , a."path"
   , a.client_id
   , a.shoot_date
   , a.poster_image_path
	, COALESCE(a.poster_y_pos, '') AS poster_y_pos
FROM albums AS a
WHERE 1=1
   AND a.deleted_at IS NULL
   AND a.client_id = ?
ORDER BY a.shoot_date DESC
   `

	params := []any{
		clientID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err = s.db.Query(ctx, &result, sql, params...); err != nil {
		return result, fmt.Errorf("error querying for albums by client ID %d: %w", clientID, err)
	}

	return result, nil
}

func (s AlbumService) ToggleFavorite(clientID, albumID uint, key string) (bool, error) {
	var (
		err      error
		exists   bool
		favorite models.Favorite
	)

	// First, check if the favorite already exists
	sql := `
SELECT 
    client_id,
    album_id,
    image_path
FROM favorites
WHERE 1=1
    AND client_id = ?
    AND album_id = ?
    AND image_path = ?
`

	params := []any{
		clientID,
		albumID,
		key,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err = s.db.QueryRow(ctx, &favorite, sql, params...)
	if err != nil {
		if sqlz.IsNotFound(err) {
			// Favorite doesn't exist, so we'll add it
			exists = false
		} else {
			// An actual error occurred
			return false, fmt.Errorf("error checking if favorite exists for client %d, album %d, image %s: %w",
				clientID, albumID, key, err)
		}
	} else {
		// Favorite exists
		exists = true
	}

	// Now either insert or delete based on existence
	if exists {
		// Delete the favorite
		sql = `
DELETE FROM favorites
WHERE 1=1
    AND client_id = ?
    AND album_id = ?
    AND image_path = ?
`
		ctx, cancel = context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		if _, err = s.db.Exec(ctx, sql, params...); err != nil {
			return false, fmt.Errorf("error removing favorite for client %d, album %d, image %s: %w",
				clientID, albumID, key, err)
		}
	} else {
		// Insert the favorite
		sql = `
INSERT INTO favorites (
    client_id,
    album_id,
    image_path
) VALUES (?, ?, ?)
`
		ctx, cancel = context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()

		if _, err = s.db.Exec(ctx, sql, params...); err != nil {
			return false, fmt.Errorf("error adding favorite for client %d, album %d, image %s: %w",
				clientID, albumID, key, err)
		}
	}

	return exists, nil
}
