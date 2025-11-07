package viewmodels

import (
	internalmodels "github.com/adampresley/adampresleyphotography/cmd/website/internal/models"
	"github.com/adampresley/adampresleyphotography/pkg/models"
)

type ClientViewAlbum struct {
	BaseViewModel

	Client  *models.Client
	AlbumID uint
	Album   internalmodels.Album
}
