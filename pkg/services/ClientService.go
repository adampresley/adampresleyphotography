package services

import (
	"context"
	"fmt"
	"time"

	"github.com/adampresley/adampresleyphotography/pkg/models"
	"gorm.io/gorm"
)

type ClientServicer interface {
	GetAll() ([]models.Client, error)
	GetByPassword(password string) (*models.Client, error)
}

type ClientServiceConfig struct {
	DB *gorm.DB
}

type ClientService struct {
	db *gorm.DB
}

func NewClientService(config ClientServiceConfig) ClientService {
	return ClientService{
		db: config.DB,
	}
}

func (s ClientService) GetAll() ([]models.Client, error) {
	var (
		clients []models.Client
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := s.db.WithContext(ctx).Order("name").Find(&clients).Error; err != nil {
		return nil, fmt.Errorf("error querying for all clients: %w", err)
	}

	return clients, nil
}

func (s ClientService) GetByPassword(password string) (*models.Client, error) {
	result := &models.Client{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := s.db.WithContext(ctx).Where("password = ?", password).First(result).Error; err != nil {
		return result, fmt.Errorf("error querying for client by password: %w", err)
	}

	return result, nil
}
