package models

import (
	"time"
)

type Album struct {
	BaseModel

	Name            string
	PosterImagePath string
	Path            string
	ClientID        uint
	Client          Client `db:"client"`
	ShootDate       time.Time
	Favorites       []Favorite
	PosterYPos      string `db:"poster_y_pos"`
}
