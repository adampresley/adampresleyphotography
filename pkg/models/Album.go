package models

import (
	"time"
)

type Album struct {
	BaseModel

	Name            string `gorm:"type:varchar(255)"`
	PosterImagePath string `gorm:"column:poster_image_path;type:text"`
	Path            string `gorm:"type:text"`
	ClientID        uint
	Client          Client     `gorm:"foreignKey:ClientID"`
	ShootDate       time.Time  `gorm:"column:shoot_date"`
	Favorites       []Favorite `gorm:"foreignKey:AlbumID;references:ID"`
	PosterYPos      string     `gorm:"column:poster_y_pos;type:text"`
}
