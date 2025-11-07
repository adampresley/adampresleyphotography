package services

import (
	"context"
	"fmt"
	"time"

	"github.com/adampresley/adampresleyphotography/pkg/models"
	"github.com/rfberaldo/sqlz"
)

type ClientServicer interface {
	GetAll() ([]models.Client, error)
	GetByPassword(password string) (*models.Client, error)
}

type ClientServiceConfig struct {
	DB *sqlz.DB
}

type ClientService struct {
	db *sqlz.DB
}

func NewClientService(config ClientServiceConfig) ClientService {
	return ClientService{
		db: config.DB,
	}
}

func (s ClientService) GetAll() ([]models.Client, error) {
	var (
		err     error
		clients []models.Client
	)

	sql := `
SELECT
   c.id
   , c.created_at
   , c.updated_at
   , c.deleted_at
   , c.password
   , c.name
   , c.email
FROM clients AS c
WHERE 1=1
   AND c.deleted_at IS NULL
ORDER BY c.name
`

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err = s.db.Query(ctx, &clients, sql); err != nil {
		return nil, fmt.Errorf("error querying for all clients: %w", err)
	}

	return clients, nil
}

func (s ClientService) GetByPassword(password string) (*models.Client, error) {
	var (
		err error
	)

	result := &models.Client{}

	sql := `
SELECT
   c.id
   , c.created_at
   , c.updated_at
   , c.deleted_at
   , c.password
   , c.name
	, c.email
FROM clients AS c
WHERE 1=1
   AND c.deleted_at IS NULL
   AND c.password=?
   `

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err = s.db.QueryRow(ctx, result, sql, password); err != nil {
		return result, fmt.Errorf("error querying for client by password: %w", err)
	}

	return result, nil
}
