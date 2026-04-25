package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/adampresley/adampresleyphotography/pkg/models"
	"gorm.io/gorm"
)

type AlbumServicer interface {
	GetAlbum(clientID uint, albumID uint) (*models.Album, error)
	GetAlbumList(clientID uint) ([]*models.Album, error)
	ToggleFavorite(clientID, albumID uint, key string) (bool, error)
}

type AlbumServiceConfig struct {
	DB *gorm.DB
}

type AlbumService struct {
	db *gorm.DB
}

func NewAlbumService(config AlbumServiceConfig) AlbumService {
	return AlbumService{
		db: config.DB,
	}
}

func (s AlbumService) GetAlbum(clientID, albumID uint) (*models.Album, error) {
	result := &models.Album{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := s.db.WithContext(ctx).
		Preload("Client").
		Where("id = ? AND client_id = ?", albumID, clientID).
		First(result).Error; err != nil {
		return result, fmt.Errorf("error querying for album %d, client %d: %w", albumID, clientID, err)
	}

	if err := s.db.WithContext(ctx).
		Where("client_id = ? AND album_id = ?", clientID, albumID).
		Find(&result.Favorites).Error; err != nil {
		return result, fmt.Errorf("error querying for favorites for album %d, client %d: %w", albumID, clientID, err)
	}

	return result, nil
}

func (s AlbumService) GetAlbumList(clientID uint) ([]*models.Album, error) {
	result := []*models.Album{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := s.db.WithContext(ctx).
		Where("client_id = ?", clientID).
		Order("shoot_date DESC").
		Find(&result).Error; err != nil {
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	err = s.db.WithContext(ctx).
		Where("client_id = ? AND album_id = ? AND image_path = ?", clientID, albumID, key).
		First(&favorite).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			exists = false
		} else {
			return false, fmt.Errorf("error checking if favorite exists for client %d, album %d, image %s: %w",
				clientID, albumID, key, err)
		}
	} else {
		exists = true
	}

	if exists {
		if err = s.db.WithContext(ctx).Delete(&favorite).Error; err != nil {
			return false, fmt.Errorf("error removing favorite for client %d, album %d, image %s: %w",
				clientID, albumID, key, err)
		}
	} else {
		favorite = models.Favorite{
			ClientID:  clientID,
			AlbumID:   albumID,
			ImagePath: key,
		}

		if err = s.db.WithContext(ctx).Create(&favorite).Error; err != nil {
			return false, fmt.Errorf("error adding favorite for client %d, album %d, image %s: %w",
				clientID, albumID, key, err)
		}
	}

	return exists, nil
}
