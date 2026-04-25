package models

type Favorite struct {
	ClientID  uint   `gorm:"primaryKey;autoIncrement:false"`
	AlbumID   uint   `gorm:"primaryKey;autoIncrement:false"`
	ImagePath string `gorm:"primaryKey;column:image_path;type:text;autoIncrement:false"`
}
