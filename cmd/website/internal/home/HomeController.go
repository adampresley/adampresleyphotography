package home

import (
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/adampresley/adamgokit/httphelpers"
	"github.com/adampresley/adamgokit/rendering"
	"github.com/adampresley/adamgokit/s3"
	"github.com/adampresley/adamgokit/s3/listoptions"
	"github.com/adampresley/adampresleyphotography/cmd/website/internal/configuration"
	"github.com/adampresley/adampresleyphotography/cmd/website/internal/viewmodels"
)

type HomeHandlers interface {
	HomePage(w http.ResponseWriter, r *http.Request)
}

type HomeControllerConfig struct {
	AwsBucket           string
	HomePagePhotoFolder string
	Config              *configuration.Config
	Renderer            rendering.TemplateRenderer
	S3Client            s3.S3Client
}

type HomeController struct {
	awsBucket           string
	homePagePhotoFolder string
	config              *configuration.Config
	renderer            rendering.TemplateRenderer
	s3Client            s3.S3Client
}

func NewHomeController(config HomeControllerConfig) HomeController {
	return HomeController{
		awsBucket:           config.AwsBucket,
		homePagePhotoFolder: config.HomePagePhotoFolder,
		config:              config.Config,
		renderer:            config.Renderer,
		s3Client:            config.S3Client,
	}
}

/*
GET /
*/
func (c HomeController) HomePage(w http.ResponseWriter, r *http.Request) {
	pageName := "pages/home"

	viewData := viewmodels.HomePage{
		BaseViewModel: viewmodels.BaseViewModel{
			Message:            "",
			IsHtmx:             httphelpers.IsHtmx(r),
			JavascriptIncludes: []rendering.JavascriptInclude{},
		},
		Photos: []viewmodels.HomePagePhoto{},
	}

	thumbnails, err := c.s3Client.List(
		c.awsBucket,
		fmt.Sprintf("%s/thumbnail", c.homePagePhotoFolder),
		listoptions.WithGetUrls(),
	)

	if err != nil {
		slog.Error("error listing objects in S3 bucket", "error", err, "bucket", c.awsBucket, "prefix", c.homePagePhotoFolder)
		viewData.IsError = true
		viewData.Message = "There was a problem getting photo for this page."

		c.renderer.Render(pageName, viewData, w)
		return
	}

	originals, err := c.s3Client.List(
		c.awsBucket,
		fmt.Sprintf("%s/original", c.homePagePhotoFolder),
		listoptions.WithGetUrls(),
	)

	if err != nil {
		slog.Error("error listing objects in S3 bucket", "error", err, "bucket", c.awsBucket, "prefix", c.homePagePhotoFolder)
		viewData.IsError = true
		viewData.Message = "There was a problem getting photo for this page."

		c.renderer.Render(pageName, viewData, w)
		return
	}

	for index, obj := range thumbnails.Objects {
		fileName := filepath.Base(obj.Url)

		viewData.Photos = append(viewData.Photos, viewmodels.HomePagePhoto{
			ThumbnailPath: obj.Url,
			FileName:      fileName,
			OriginalPath:  originals.Objects[index].Url,
		})
	}

	c.renderer.Render(pageName, viewData, w)
}
