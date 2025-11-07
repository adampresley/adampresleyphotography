package services

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adampresley/adamgokit/s3"
	"github.com/adampresley/adamgokit/s3/listoptions"
	"github.com/adampresley/adamgokit/s3/putoptions"
	"github.com/adampresley/adampresleyphotography/pkg/models"
)

type ZipServiceConfig struct {
	AlbumService      AlbumServicer
	BaseDownloadURL   string
	Bucket            string
	ClientPhotoFolder string
	ClientService     ClientServicer
	ExpirationDays    int
	S3Client          s3.S3Client
	EmailApiKey       string
	FromName          string
	FromEmail         string
}

type ZipServicer interface {
	CreateZipAsync(album *models.Album, client *models.Client) (string, error)
	StartCleanupRoutine(interval time.Duration)
	StopCleanupRoutine()
}

type ZipService struct {
	config        ZipServiceConfig
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
	wg            *sync.WaitGroup
}

func NewZipService(config ZipServiceConfig) ZipService {
	// Default expiration to 7 days if not specified
	if config.ExpirationDays <= 0 {
		config.ExpirationDays = 7
	}

	return ZipService{
		config:      config,
		stopCleanup: make(chan struct{}),
		wg:          &sync.WaitGroup{},
	}
}

func (s ZipService) CreateZipAsync(album *models.Album, client *models.Client) (string, error) {
	var (
		err        error
		objectData *s3.ObjectMetadata
	)

	jobID := fmt.Sprintf("%s-%d", strings.ReplaceAll(album.Name, " ", "-"), album.ID)
	zipFilename := fmt.Sprintf("%s.zip", jobID)

	zipKey := filepath.Join(
		s.config.ClientPhotoFolder,
		fmt.Sprint(client.ID),
		fmt.Sprint(album.ID),
		"downloads",
		zipFilename,
	)

	// Check if the file already exists
	if objectData, err = s.config.S3Client.StatObject(s.config.Bucket, zipKey); err == nil && objectData != nil {
		slog.Info("zip file already exists, sending email only", "zipKey", zipKey, "albumID", album.ID)
		downloadURL := fmt.Sprintf("%s/client/downloads/%s", s.config.BaseDownloadURL, zipFilename)

		err = SendEmail(
			s.config.EmailApiKey,
			client.Name,
			client.Email,
			s.config.FromName,
			s.config.FromEmail,
			map[string]any{
				"downloadURL":    downloadURL,
				"name":           client.Name,
				"albumName":      album.Name,
				"expirationDays": s.config.ExpirationDays,
			},
		)

		if err != nil {
			slog.Error("failed to send email notification", "error", err, "email", client.Email, "albumID", album.ID)
			return jobID, err
		}

		return jobID, nil
	}

	// Start the background job to create the zip
	go s.processZip(zipKey, zipFilename, album, client)

	return jobID, nil
}

func (s ZipService) processZip(zipKey, zipFilename string, album *models.Album, client *models.Client) {
	l := slog.With("albumID", album.ID, "zipKey", zipKey)
	l.Info("starting zip creation process with io.Pipe")

	originalsKey := filepath.Join(
		s.config.ClientPhotoFolder,
		fmt.Sprint(album.ClientID),
		fmt.Sprint(album.ID),
		"originals",
	)

	addFile := func(zipWriter *zip.Writer, key string) error {
		imageName := filepath.Base(key)
		l.Info("adding image to zip", "image", imageName)

		src, err := s.config.S3Client.Get(s.config.Bucket, key)

		if err != nil {
			return fmt.Errorf("failed to get source file from '%s' S3: %w", key, err)
		}

		dest, err := zipWriter.Create(imageName)

		if err != nil {
			return fmt.Errorf("failed to create file '%s' in zip: %w", imageName, err)
		}

		defer src.Body.Close()

		if _, err := io.Copy(dest, src.Body); err != nil {
			return fmt.Errorf("failed to copy file '%s' to zip: %w", imageName, err)
		}

		return nil
	}

	stream, err := s.config.S3Client.PutStream(s.config.Bucket, zipKey, putoptions.WithContentType("application/zip"))

	if err != nil {
		l.Error("failed to setup s3 stream", "error", err)
		return
	}

	zipWriter := zip.NewWriter(stream.Writer)
	listResponse, err := s.config.S3Client.List(s.config.Bucket, originalsKey, listoptions.WithGetAll())

	if err != nil {
		l.Error("error listing album images", "error", err)
		return
	}

	for _, img := range listResponse.Objects {
		if err = addFile(zipWriter, img.Key); err != nil {
			l.Error("failed to add image to zip", "error", err, "image", img.Key)
			continue
		}
	}

	if err = zipWriter.Close(); err != nil {
		l.Error("failed to close zip writer", "error", err)
		return
	}

	if err = stream.Writer.Close(); err != nil {
		l.Error("failed to close s3 stream writer", "error", err)
		return
	}

	_, err = stream.Wait()

	if err != nil {
		l.Error("failed to wait for s3 stream", "error", err)
		return
	}

	l.Info("finished uploading zip file to S3")

	// Generate download URL
	downloadURL := fmt.Sprintf("%s/client/downloads/%s", s.config.BaseDownloadURL, zipFilename)

	err = SendEmail(
		s.config.EmailApiKey,
		client.Name,
		client.Email,
		s.config.FromName,
		s.config.FromEmail,
		map[string]any{
			"downloadURL":    downloadURL,
			"name":           client.Name,
			"albumName":      album.Name,
			"expirationDays": s.config.ExpirationDays,
		},
	)

	if err != nil {
		l.Error("failed to send email notification", "error", err, "email", client.Email)
		return
	}

	l.Info("zip creation completed successfully", "downloadURL", downloadURL)
}

// StartCleanupRoutine starts a periodic routine to clean up expired zip files
func (s ZipService) StartCleanupRoutine(interval time.Duration) {
	s.stopCleanup = make(chan struct{})
	s.cleanupTicker = time.NewTicker(interval)

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			select {
			case <-s.cleanupTicker.C:
				s.cleanupExpiredZips()
			case <-s.stopCleanup:
				s.cleanupTicker.Stop()
				return
			}
		}
	}()

	slog.Info("zip cleanup routine started", "interval", interval)
}

// StopCleanupRoutine stops the cleanup routine
func (s ZipService) StopCleanupRoutine() {
	if s.cleanupTicker != nil {
		close(s.stopCleanup)
		s.wg.Wait()
		slog.Info("zip cleanup routine stopped")
	}
}

// cleanupExpiredZips removes zip files older than the expiration period
func (s ZipService) cleanupExpiredZips() {
	var (
		err     error
		clients []models.Client
		albums  []*models.Album
	)

	l := slog.With("function", "cleanupExpiredZips")
	l.Info("starting cleanup of expired zip files")

	// Calculate the cutoff time
	cutoffTime := time.Now().AddDate(0, 0, -s.config.ExpirationDays)
	var removedCount int

	if clients, err = s.config.ClientService.GetAll(); err != nil {
		l.Error("error retrieving clients from database", "error", err)
		return
	}

	for _, client := range clients {
		if albums, err = s.config.AlbumService.GetAlbumList(client.ID); err != nil {
			l.Error("error retrieving albums", "clientID", client.ID, "error", err)
			return
		}

		for _, album := range albums {
			downloadsKey := filepath.Join(
				s.config.ClientPhotoFolder,
				fmt.Sprint(album.ClientID),
				fmt.Sprint(album.ID),
				"downloads",
			)

			listResponse, err := s.config.S3Client.List(s.config.Bucket, downloadsKey)
			if err != nil {
				l.Error("failed to list S3 directory", "error", err, "path", downloadsKey)
				continue
			}

			// Check each file
			for _, file := range listResponse.Objects {
				// Only process zip files
				if !strings.HasSuffix(strings.ToLower(file.Key), ".zip") {
					continue
				}

				// Check if the file is older than the cutoff time
				if file.LastModified.Before(cutoffTime) {
					l.Info("removing expired zip file from S3", "path", file.Key, "modTime", file.LastModified)

					if _, err := s.config.S3Client.Delete(s.config.Bucket, []string{file.Key}); err != nil {
						l.Error("failed to remove expired zip file from S3", "error", err, "path", file.Key)
					} else {
						removedCount++
					}
				}
			}
		}
	}

	l.Info("completed cleanup of expired zip files", "removed", removedCount)
}

