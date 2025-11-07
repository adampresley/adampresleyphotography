package models

type Favorite struct {
	BaseModel

	ClientID  uint
	AlbumID   uint
	ImagePath string
}
