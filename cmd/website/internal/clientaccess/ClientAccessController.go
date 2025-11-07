package clientaccess

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adampresley/adamgokit/httphelpers"
	"github.com/adampresley/adamgokit/rendering"
	"github.com/adampresley/adamgokit/s3"
	"github.com/adampresley/adamgokit/s3/getoptions"
	"github.com/adampresley/adamgokit/s3/listoptions"
	"github.com/adampresley/adamgokit/sessions"
	"github.com/adampresley/adamgokit/slices"
	internalmodels "github.com/adampresley/adampresleyphotography/cmd/website/internal/models"
	"github.com/adampresley/adampresleyphotography/cmd/website/internal/viewmodels"
	"github.com/adampresley/adampresleyphotography/pkg/models"
	"github.com/adampresley/adampresleyphotography/pkg/services"
	"github.com/rfberaldo/sqlz"
)

type ClientAccessControllerConfig struct {
	AlbumService      services.AlbumServicer
	Bucket            string
	ClientPhotoFolder string
	ClientService     services.ClientServicer
	Renderer          rendering.TemplateRenderer
	S3Client          s3.S3Client
	SessionService    sessions.Session[*models.Client]
	ZipService        services.ZipServicer
}

type ClientAccessController struct {
	albumService      services.AlbumServicer
	bucket            string
	clientPhotoFolder string
	clientService     services.ClientServicer
	renderer          rendering.TemplateRenderer
	s3Client          s3.S3Client
	sessionService    sessions.Session[*models.Client]
	zipService        services.ZipServicer
}

func NewClientAccessController(config ClientAccessControllerConfig) ClientAccessController {
	return ClientAccessController{
		albumService:      config.AlbumService,
		bucket:            config.Bucket,
		clientPhotoFolder: config.ClientPhotoFolder,
		clientService:     config.ClientService,
		renderer:          config.Renderer,
		s3Client:          config.S3Client,
		sessionService:    config.SessionService,
		zipService:        config.ZipService,
	}
}

/*
GET /client
*/
func (c ClientAccessController) AlbumListPage(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		albums []*models.Album
	)

	viewData := viewmodels.ClientAlbumList{
		BaseViewModel: viewmodels.BaseViewModel{
			IsHtmx: httphelpers.IsHtmx(r),
			JavascriptIncludes: []rendering.JavascriptInclude{
				{Type: "module", Src: "/static/js/pages/album-list.js"},
			},
		},
		Albums: []internalmodels.Album{},
		Client: &models.Client{},
	}

	viewData.Client = viewmodels.GetClientFromContext(r)

	if albums, err = c.albumService.GetAlbumList(viewData.Client.ID); err != nil && !sqlz.IsNotFound(err) {
		slog.Error("error getting album list", "error", err, "clientID", viewData.Client.ID)
		viewData.IsError = true
		viewData.Message = "An unexpected error occurred. Please reach out for assistance."

		c.renderer.Render("pages/clientaccess/album-list", viewData, w)
		return
	}

	for _, album := range albums {
		converted := c.convertAlbumToViewModel(album, false)
		viewData.Albums = append(viewData.Albums, converted)
	}

	c.renderer.Render("pages/clientaccess/album-list", viewData, w)
}

/*
GET /client/library/{albumid}/download-all
*/
func (c ClientAccessController) DownloadAllImagesInAlbum(w http.ResponseWriter, r *http.Request) {
	var (
		err   error
		album *models.Album
	)

	client := viewmodels.GetClientFromContext(r)
	albumID := httphelpers.GetFromRequest[uint](r, "albumid")

	if album, err = c.albumService.GetAlbum(client.ID, albumID); err != nil {
		httphelpers.WriteText(w, http.StatusNotFound, "album not found")
		return
	}

	// Start the async zip creation process
	_, err = c.zipService.CreateZipAsync(album, client)
	if err != nil {
		slog.Error("failed to start zip creation", "error", err, "albumID", albumID)
		httphelpers.TextInternalServerError(w, "Failed to start download preparation")
		return
	}

	// Render a success message to the user
	viewData := viewmodels.ClientDownloadStarted{
		BaseViewModel: viewmodels.BaseViewModel{
			IsHtmx: httphelpers.IsHtmx(r),
		},
		Album:  album,
		Client: client,
	}

	c.renderer.Render("pages/clientaccess/download-started", viewData, w)
}

func (c ClientAccessController) DownloadImage(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		object s3.GetObjectResponse
	)

	key := httphelpers.GetFromRequest[string](r, "key")

	object, err = c.s3Client.Get(
		c.bucket,
		key,
		getoptions.WithContext(r.Context()),
		getoptions.WithTimeout(time.Minute*30),
	)

	if err != nil {
		slog.Error("error getting image object from S3", "error", err, "bucket", c.bucket, "key", key)
		httphelpers.WriteText(w, http.StatusInternalServerError, "Failed to download image")
		return
	}

	defer object.Body.Close()
	fileName := filepath.Base(key)

	w.Header().Set("Content-Type", object.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", object.Size))

	_, _ = io.Copy(w, object.Body)
}

/*
GET /client/login
*/
func (c ClientAccessController) LoginPage(w http.ResponseWriter, r *http.Request) {
	viewData := viewmodels.ClientLogin{
		BaseViewModel: viewmodels.BaseViewModel{
			IsHtmx: httphelpers.IsHtmx(r),
		},
		ClientCode: "",
	}

	c.renderer.Render("pages/clientaccess/login", viewData, w)
}

/*
POST /client/login
*/
func (c ClientAccessController) LoginAction(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		client *models.Client
	)

	pageName := "pages/clientaccess/login"

	viewData := viewmodels.ClientLogin{
		BaseViewModel: viewmodels.BaseViewModel{
			IsHtmx: httphelpers.IsHtmx(r),
		},
		ClientCode: httphelpers.GetFromRequest[string](r, "password"),
	}

	client, err = c.clientService.GetByPassword(viewData.ClientCode)

	if err != nil && !sqlz.IsNotFound(err) {
		slog.Error("error querying for client information", "error", err)
		viewData.IsError = true
		viewData.Message = "An unexpected error occurred. Please reach out for assistance."

		c.renderer.Render(pageName, viewData, w)
		return
	}

	if sqlz.IsNotFound(err) {
		viewData.IsWarning = true
		viewData.Message = "Your password was not correct. Please try again."

		c.renderer.Render(pageName, viewData, w)
		return
	}

	/*
	 * Setup the session and redirect to the happy place
	 */
	if err = c.sessionService.Set(r, client); err != nil {
		slog.Error("error setting client session", "error", err)
	}

	if err = c.sessionService.Save(w, r); err != nil {
		slog.Error("error saving session", "error", err)
	}

	http.Redirect(w, r, "/client", http.StatusFound)
}

/*
GET /client/logout
*/
func (c ClientAccessController) LogoutAction(w http.ResponseWriter, r *http.Request) {
	_ = c.sessionService.Destroy(w, r)
	_ = c.sessionService.Save(w, r)
	http.Redirect(w, r, "/client/login", http.StatusFound)
}

/*
GET /client/{id}
*/
func (c ClientAccessController) ViewAlbumPage(w http.ResponseWriter, r *http.Request) {
	var (
		err   error
		album *models.Album
	)

	viewData := viewmodels.ClientViewAlbum{
		BaseViewModel: viewmodels.BaseViewModel{
			IsHtmx: httphelpers.IsHtmx(r),
			JavascriptIncludes: []rendering.JavascriptInclude{
				{Type: "module", Src: "/static/js/pages/view-album.js"},
			},
		},
		Client:  &models.Client{},
		AlbumID: httphelpers.GetFromRequest[uint](r, "id"),
		Album:   internalmodels.Album{},
	}

	viewData.Client = viewmodels.GetClientFromContext(r)

	if album, err = c.albumService.GetAlbum(viewData.Client.ID, viewData.AlbumID); err != nil {
		slog.Error("an error occurred querying album in ViewAlbumPage", "error", err, "albumID", viewData.AlbumID)
		viewData.IsError = true
		viewData.Message = "An unexpected error occurred. Please reach out for assistance."

		c.renderer.Render("pages/clientaccess/view-album", viewData, w)
		return
	}

	viewData.Album = c.convertAlbumToViewModel(album, true)
	c.renderer.Render("pages/clientaccess/view-album", viewData, w)
}

func (c ClientAccessController) DownloadZip(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		object s3.GetObjectResponse
	)

	client := viewmodels.GetClientFromContext(r)
	filename := httphelpers.GetFromRequest[string](r, "filename")

	// Sanitize the filename to prevent directory traversal
	filename = filepath.Base(filename)

	/*
	 * This is brittle. It assumes the album ID is the last part of the filename
	 * separated by a hyphen. E.g. "My-Album-123.zip"
	 */
	parts := strings.Split(strings.TrimSuffix(filename, ".zip"), "-")
	albumID, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		slog.Error("error parsing album ID from filename", "error", err, "filename", filename)
		httphelpers.WriteText(w, http.StatusBadRequest, "Invalid download link")
		return
	}

	zipKey := filepath.Join(
		c.clientPhotoFolder,
		fmt.Sprint(client.ID),
		fmt.Sprint(albumID),
		"downloads",
		filename,
	)

	slog.Info("serving zip download from S3", "filename", filename, "key", zipKey, "clientID", client.ID)

	object, err = c.s3Client.Get(
		c.bucket,
		zipKey,
		getoptions.WithContext(r.Context()),
	)

	if err != nil {
		slog.Error("error getting zip object from S3", "error", err, "bucket", c.bucket, "key", zipKey)
		httphelpers.WriteText(w, http.StatusNotFound, "Download file not found")
		return
	}

	defer object.Body.Close()

	// Set appropriate headers for file download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", object.Size))

	// Stream the file to the response
	if _, err = io.Copy(w, object.Body); err != nil {
		slog.Error("error streaming zip file", "error", err, "key", zipKey)
		return
	}

	slog.Info("zip file download completed", "filename", filename, "clientID", client.ID)
}

/*
PUT /client/library/{albumid}/toggle-favorite/{imagepath}
*/
func (c ClientAccessController) ToggleFavorite(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		exists bool
	)

	client := viewmodels.GetClientFromContext(r)
	albumID := httphelpers.GetFromRequest[uint](r, "albumid")
	key := filepath.Base(httphelpers.GetFromRequest[string](r, "key"))

	if exists, err = c.albumService.ToggleFavorite(client.ID, albumID, key); err != nil {
		slog.Error("error toggling favorite", "error", err, "albumID", albumID, "imagePath", key)
		httphelpers.TextInternalServerError(w, "Error toggling favorite")
		return
	}

	icon := "icon"

	/*
	 * Update the view model to reflect the favorite status.
	 * Exist returns true if the image is already a favorite, false otherwise.
	 */
	if !exists {
		icon += " icon-heart"
	} else {
		icon += " icon-empty-heart"
	}

	markup := fmt.Sprintf("<i class='%s'></i>", icon)
	httphelpers.WriteHtml(w, http.StatusOK, markup)
}

func (c ClientAccessController) convertAlbumToViewModel(album *models.Album, getImages bool) internalmodels.Album {
	var (
		err error
		u   string
	)

	result := internalmodels.Album{
		ID:             album.ID,
		Name:           album.Name,
		PosterImageURL: "",
		Client: internalmodels.Client{
			ID:    album.ClientID,
			Name:  album.Client.Name,
			Email: album.Client.Email,
		},
		ShootDate:  album.ShootDate.Format("Jan _2, 2006"),
		Favorites:  []internalmodels.Favorite{},
		PosterYPos: album.PosterYPos,
		ImageURLs:  []internalmodels.Image{},
	}

	key := filepath.Join(
		c.clientPhotoFolder,
		fmt.Sprint(album.ClientID),
		fmt.Sprint(album.ID),
		"thumbnails",
		album.PosterImagePath,
	)

	u, err = c.s3Client.GetUrl(c.bucket, key)

	if err == nil {
		slog.Info("got poster image URL", "clientID", album.ClientID, "albumID", album.ID, "imagePath", album.PosterImagePath, "url", u)
		result.PosterImageURL = u
	} else {
		slog.Error("error getting poster image URL", "error", err, "clientID", album.ClientID, "albumID", album.ID, "imagePath", album.PosterImagePath)
	}

	if getImages {
		thumbnails, err := c.s3Client.List(
			c.bucket,
			fmt.Sprintf("%s/%d/%d/thumbnails/", c.clientPhotoFolder, album.ClientID, album.ID),
			listoptions.WithGetUrls(),
		)

		if err != nil {
			slog.Error("error getting thumbnail image URLs", "error", err, "clientID", album.ClientID, "albumID", album.ID)
		}

		originals, err := c.s3Client.List(
			c.bucket,
			fmt.Sprintf("%s/%d/%d/originals/", c.clientPhotoFolder, album.ClientID, album.ID),
			listoptions.WithGetUrls(),
		)

		if err != nil {
			slog.Error("error getting image URLs", "error", err, "clientID", album.ClientID, "albumID", album.ID)
		}

		for index, thumbnail := range thumbnails.Objects {
			newImage := internalmodels.Image{
				ThumbnailURL: thumbnail.Url,
				OriginalURL:  originals.Objects[index].Url,
				OriginalPath: fmt.Sprintf("%s/%d/%d/originals/", c.clientPhotoFolder, album.ClientID, album.ID),
				OriginalKey:  originals.Objects[index].Key,
			}

			// Is this image a favorite?
			uu, _ := url.Parse(originals.Objects[index].Url)
			baseImage := filepath.Base(uu.Path)

			favImagePaths := slices.Map(album.Favorites, func(input models.Favorite, index int) string {
				return input.ImagePath
			})

			isFav := slices.IsInSlice(baseImage, favImagePaths)

			if isFav {
				newImage.IsFavorite = true
			}

			result.ImageURLs = append(result.ImageURLs, newImage)
		}
	}

	return result
}
