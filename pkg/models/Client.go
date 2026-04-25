package models

import (
	"fmt"
)

var (
	ErrClientNotFound = fmt.Errorf("client not found")
)

type Client struct {
	BaseModel

	Password string  `gorm:"type:varchar(255);uniqueIndex"`
	Name     string  `gorm:"type:varchar(255)"`
	Email    string  `gorm:"type:varchar(255)"`
	Albums   []Album `gorm:"foreignKey:ClientID"`
}
