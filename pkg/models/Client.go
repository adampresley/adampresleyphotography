package models

import (
	"fmt"
)

var (
	ErrClientNotFound = fmt.Errorf("client not found")
)

type Client struct {
	BaseModel

	Password string
	Name     string
	Email    string
	Albums   []Album
}
