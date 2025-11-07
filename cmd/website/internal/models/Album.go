package models

type Album struct {
	ID             uint
	Name           string
	PosterImageURL string
	Client         Client
	ShootDate      string
	Favorites      []Favorite
	PosterYPos     string
	ImageURLs      []Image
}

type Image struct {
	ThumbnailURL string
	OriginalURL  string
	IsFavorite   bool
	OriginalKey  string
	OriginalPath string
}
