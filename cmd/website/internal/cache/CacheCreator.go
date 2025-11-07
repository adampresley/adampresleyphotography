package cache

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adampresley/adamgokit/s3"
	"github.com/adampresley/adamgokit/s3/createbucketoptions"
	"github.com/adampresley/adamgokit/s3/geturloptions"
	"github.com/adampresley/adamgokit/s3/listoptions"
	"github.com/adampresley/adamgokit/slices"
	"github.com/adampresley/adampresleyphotography/pkg/models"
	"github.com/adampresley/adampresleyphotography/pkg/services"
	"github.com/alitto/pond/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/nfnt/resize"
)

type CacheCreator interface {
	CreateCache()
}

type CacheCreatorConfig struct {
	AlbumService        services.AlbumServicer
	AwsBucket           string
	AwsRegion           string
	ClientsPhotoFolder  string
	ClientService       services.ClientServicer
	HomePagePhotoFolder string
	MaxCacheWorkers     int
	S3Client            s3.S3Client
	ShutdownCtx         context.Context
}

type CacheCreatorService struct {
	albumService        services.AlbumServicer
	awsBucket           string
	awsRegion           string
	clientsPhotoFolder  string
	clientService       services.ClientServicer
	homePagePhotoFolder string
	maxCacheWorkers     int
	s3Client            s3.S3Client
	shutdownCtx         context.Context
}

func NewCacheCreatorService(config CacheCreatorConfig) CacheCreatorService {
	return CacheCreatorService{
		albumService:        config.AlbumService,
		awsBucket:           config.AwsBucket,
		awsRegion:           config.AwsRegion,
		clientsPhotoFolder:  config.ClientsPhotoFolder,
		clientService:       config.ClientService,
		homePagePhotoFolder: config.HomePagePhotoFolder,
		maxCacheWorkers:     config.MaxCacheWorkers,
		s3Client:            config.S3Client,
		shutdownCtx:         config.ShutdownCtx,
	}
}

func (c CacheCreatorService) CreateCache() {
	var (
		err         error
		clients     []models.Client
		albums      []*models.Album
		albumImages []s3.Object
	)

	slog.Info("starting cache creation...")

	if err = c.ensureBucketExists(c.awsBucket); err != nil {
		slog.Error("error ensuring bucket exists. aborting", "bucket", c.awsBucket, "error", err)
		os.Exit(1)
	}

	/*
	 * First, retrieve all clients
	 */
	if clients, err = c.clientService.GetAll(); err != nil {
		slog.Error("error retrieving clients from database", "error", err)
		return
	}

	/*
	 * For each client, retrieve all their albums. We will use this information
	 * to get a list of images and create cache entries for each album.
	 */
	slog.Info("creating cache for clients...", "numClients", len(clients))

	pool := pond.NewPool(c.maxCacheWorkers, pond.WithContext(c.shutdownCtx))

	if err = c.updateHomePageCache(pool); err != nil {
		slog.Error("error updating home page cache", "error", err)
	}

	for _, client := range clients {
		if albums, err = c.albumService.GetAlbumList(client.ID); err != nil {
			slog.Error("error retrieving albums", "clientID", client.ID, "error", err)
			return
		}

		for _, album := range albums {
			pool.Submit(func() {
				if !c.doesHeroExist(album) {
					slog.Info("creating hero banner cache for album...", "clientID", client.ID, "albumID", album.ID)

					if err = c.createHeroBanner(album); err != nil {
						slog.Error("error creating hero banner for album", "clientID", client.ID, "albumID", album.ID, "error", err)
						return
					}
				}
			})

			if albumImages, err = c.getAlbumImageListing(album); err != nil {
				slog.Error("error retrieving image listing for album", "clientID", client.ID, "albumID", album.ID, "error", err)
				return
			}

			for _, imageObj := range albumImages {
				pool.Submit(func() {
					if !c.doesThumbnailExist(album, imageObj) {
						slog.Info("creating cache item for album...", "key", imageObj.Key)

						if err = c.createThumbnail(album, imageObj.Key); err != nil {
							slog.Error("error creating cache item for album", "clientID", client.ID, "albumID", album.ID, "imageName", imageObj, "error", err)
						}
					}
				})
			}
		}
	}

	_ = pool.Stop().Wait()
}

func (c CacheCreatorService) ensureBucketExists(bucketName string) error {
	var (
		err    error
		exists bool
	)

	exists, err = c.s3Client.BucketExists(bucketName)

	if err != nil {
		return fmt.Errorf("error ensuring bucket '%s' exists: %w", bucketName, err)
	}

	if exists {
		return nil
	}

	slog.Info("creating bucket", "bucketName", bucketName)

	err = c.s3Client.CreateBucket(
		bucketName,
		createbucketoptions.WithRegion(c.awsRegion),
	)

	if err != nil {
		return fmt.Errorf("error creating bucket '%s': %w", bucketName, err)
	}

	return nil
}

func (c CacheCreatorService) updateHomePageCache(pool pond.Pool) error {
	var (
		err           error
		originals     s3.ListResponse
		thumbnailStat *s3.ObjectMetadata
	)

	resizeWork := func(original s3.Object, thumbnailKey string) {
		var (
			err error
			img image.Image
			buf bytes.Buffer
		)

		img, err = c.resizeUrl(original.Url, 300)
		if err != nil {
			slog.Error("error resizing image", "image", original.Key, "error", err)
			return
		}

		if err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			slog.Error("error encoding image for thumbnail", "key", thumbnailKey, "error", err)
			return
		}

		if _, err = c.s3Client.Put(c.awsBucket, thumbnailKey, bytes.NewReader(buf.Bytes())); err != nil {
			slog.Error("error uploading resized image", "thumbnailKey", thumbnailKey, "error", err)
		}

		slog.Info("updated home page thumbnail", "thumbnailKey", thumbnailKey)
	}

	originalsKey := filepath.Join(c.homePagePhotoFolder, "original")
	originals, err = c.s3Client.List(
		c.awsBucket,
		originalsKey,
		listoptions.WithGetUrls(),
	)

	if err != nil {
		return fmt.Errorf("error listing home page images: %w", err)
	}

	slog.Info("checking for updated home page images...", "numImages", len(originals.Objects), "bucket", c.awsBucket, "path", originalsKey)

	for _, original := range originals.Objects {
		thumbnailKey := filepath.Join(c.homePagePhotoFolder, "thumbnail", filepath.Base(original.Key))

		if thumbnailStat, err = c.s3Client.StatObject(c.awsBucket, thumbnailKey); err != nil {
			slog.Error("error retrieving metadata for thumbnail", "thumbnailKey", thumbnailKey, "error", err)
			continue
		}

		if thumbnailStat == nil || thumbnailStat.LastModified.Before(original.LastModified) {
			pool.Submit(func() {
				resizeWork(original, thumbnailKey)
			})
		}
	}

	return nil
}

func (c CacheCreatorService) getAlbumImageListing(album *models.Album) ([]s3.Object, error) {
	var (
		err      error
		response s3.ListResponse
		validExt = []string{".jpg", ".jpeg"}
	)

	key := filepath.Join(
		c.clientsPhotoFolder,
		fmt.Sprint(album.ClientID),
		fmt.Sprint(album.ID),
		"originals",
	)

	response, err = c.s3Client.List(
		c.awsBucket,
		key,
		listoptions.WithGetUrls(),
		listoptions.WithGetAll(),
		listoptions.WithFilter(func(obj types.Object) bool {
			ext := strings.ToLower(filepath.Ext(aws.ToString(obj.Key)))
			result := slices.IsInSlice(ext, validExt)
			return result
		}),
		listoptions.WithGetUrlOptions(
			geturloptions.WithExpiration(time.Minute*30),
		),
	)

	if err != nil {
		return nil, fmt.Errorf("error listing album images: %w", err)
	}

	return response.Objects, nil
}

func (c CacheCreatorService) doesThumbnailExist(album *models.Album, original s3.Object) bool {
	var (
		err  error
		stat *s3.ObjectMetadata
	)

	imageName := filepath.Base(original.Key)

	key := filepath.Join(
		c.clientsPhotoFolder,
		fmt.Sprint(album.ClientID),
		fmt.Sprint(album.ID),
		"thumbnails",
		imageName,
	)

	if stat, err = c.s3Client.StatObject(c.awsBucket, key); err != nil {
		slog.Error("error retrieving metadata for thumbnail", "key", key, "error", err)
		return false
	}

	if stat == nil {
		return false
	}

	if stat.LastModified.Before(original.LastModified) {
		return false
	}

	return true
}

func (c CacheCreatorService) doesHeroExist(album *models.Album) bool {
	var (
		err          error
		originalStat *s3.ObjectMetadata
		heroStat     *s3.ObjectMetadata
	)

	heroKey := filepath.Join(
		c.clientsPhotoFolder,
		fmt.Sprint(album.ClientID),
		fmt.Sprint(album.ID),
		"hero-banner",
		album.PosterImagePath,
	)

	if heroStat, err = c.s3Client.StatObject(c.awsBucket, heroKey); err != nil {
		slog.Error("error retrieving metadata for hero banner", "key", heroKey, "error", err)
		return false
	}

	originalKey := filepath.Join(
		c.clientsPhotoFolder,
		fmt.Sprint(album.ClientID),
		fmt.Sprint(album.ID),
		"originals",
		album.PosterImagePath,
	)

	if originalStat, err = c.s3Client.StatObject(c.awsBucket, originalKey); err != nil {
		slog.Error("error retrieving metadata for original poster image", "key", originalKey, "error", err)
		return false
	}

	if originalStat == nil || heroStat == nil || heroStat.LastModified.Before(originalStat.LastModified) {
		return false
	}

	return true
}

func (c CacheCreatorService) createThumbnail(album *models.Album, originalKey string) error {
	var (
		err      error
		img      image.Image
		maxSize  uint = 400
		original s3.GetObjectResponse
		buf      bytes.Buffer
	)

	original, err = c.s3Client.Get(
		c.awsBucket,
		originalKey,
	)

	if err != nil {
		return fmt.Errorf("error retrieving original image %s: %w", originalKey, err)
	}

	if img, err = c.resizeReader(original.Body, maxSize); err != nil {
		return fmt.Errorf("error resizing image: %w", err)
	}

	if err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("error encoding image for thumbnail: %w", err)
	}

	putKey := filepath.Join(
		c.clientsPhotoFolder,
		fmt.Sprint(album.ClientID),
		fmt.Sprint(album.ID),
		"thumbnails",
		filepath.Base(originalKey),
	)

	_, err = c.s3Client.Put(
		c.awsBucket,
		putKey,
		&buf,
	)

	if err != nil {
		return fmt.Errorf("error uploading thumbnail to S3: %w", err)
	}

	return nil
}

func (c CacheCreatorService) createHeroBanner(album *models.Album) error {
	var (
		err      error
		img      image.Image
		maxSize  uint = 400
		original s3.GetObjectResponse
		buf      bytes.Buffer
	)

	originalKey := filepath.Join(
		c.clientsPhotoFolder,
		fmt.Sprint(album.ClientID),
		fmt.Sprint(album.ID),
		"originals",
		album.PosterImagePath,
	)

	original, err = c.s3Client.Get(
		c.awsBucket,
		originalKey,
	)

	if err != nil {
		return fmt.Errorf("error retrieving original image %s: %w", originalKey, err)
	}

	if img, err = c.resizeReader(original.Body, maxSize); err != nil {
		return fmt.Errorf("error resizing image: %w", err)
	}

	if err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("error encoding image for hero banner: %w", err)
	}

	putKey := filepath.Join(
		c.clientsPhotoFolder,
		fmt.Sprint(album.ClientID),
		fmt.Sprint(album.ID),
		"hero-banner",
		album.PosterImagePath,
	)

	_, err = c.s3Client.Put(
		c.awsBucket,
		putKey,
		&buf,
	)

	if err != nil {
		return fmt.Errorf("error uploading hero banner to S3: %w", err)
	}

	return nil
}

func (c CacheCreatorService) resizeUrl(url string, maxSize uint) (image.Image, error) {
	var (
		err      error
		response *http.Response
	)

	if response, err = http.Get(url); err != nil {
		return nil, fmt.Errorf("error downloading image from '%s': %w", url, err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error downloading image from '%s', status: %s", url, response.Status)
	}

	return c.resizeReader(response.Body, maxSize)
}

func (c CacheCreatorService) resizeReader(r io.Reader, maxSize uint) (image.Image, error) {
	var (
		err error
		img image.Image
	)

	if img, _, err = image.Decode(r); err != nil {
		return nil, fmt.Errorf("error decoding image: %w", err)
	}

	resizedImage := c.resize(img, maxSize)
	return resizedImage, nil
}

func (c CacheCreatorService) resize(img image.Image, maxSize uint) image.Image {
	var (
		resizedImage image.Image
	)

	/*
	 * Determine which dimension to resize based on the longest edge
	 */
	bounds := img.Bounds()
	width := uint(bounds.Dx())
	height := uint(bounds.Dy())

	var newWidth, newHeight uint
	if width > height {
		// Landscape orientation
		newWidth = maxSize
		newHeight = uint(float64(height) * (float64(maxSize) / float64(width)))
	} else {
		// Portrait orientation or square
		newHeight = maxSize
		newWidth = uint(float64(width) * (float64(maxSize) / float64(height)))
	}

	resizedImage = resize.Resize(newWidth, newHeight, img, resize.Lanczos3)
	return resizedImage
}
