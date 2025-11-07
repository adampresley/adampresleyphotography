package viewmodels

import "github.com/adampresley/adampresleyphotography/pkg/models"

type ClientDownloadStarted struct {
	BaseViewModel

	Client *models.Client
	Album  *models.Album
}
