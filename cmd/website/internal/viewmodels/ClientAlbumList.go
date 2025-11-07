package viewmodels

import (
	internalmodels "github.com/adampresley/adampresleyphotography/cmd/website/internal/models"
	"github.com/adampresley/adampresleyphotography/pkg/models"
)

type ClientAlbumList struct {
	BaseViewModel

	Client *models.Client
	Albums []internalmodels.Album
}
