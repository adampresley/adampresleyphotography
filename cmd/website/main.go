package main

import (
	"context"
	"embed"
	"encoding/gob"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adampresley/adamgokit/awsconfig"
	"github.com/adampresley/adamgokit/httphelpers"
	"github.com/adampresley/adamgokit/mux"
	"github.com/adampresley/adamgokit/rendering"
	"github.com/adampresley/adamgokit/retrier"
	"github.com/adampresley/adamgokit/s3"
	"github.com/adampresley/adamgokit/sessions"
	"github.com/adampresley/adampresleyphotography/cmd/website/internal/cache"
	"github.com/adampresley/adampresleyphotography/cmd/website/internal/clientaccess"
	"github.com/adampresley/adampresleyphotography/cmd/website/internal/configuration"
	"github.com/adampresley/adampresleyphotography/cmd/website/internal/home"
	"github.com/adampresley/adampresleyphotography/pkg/models"
	"github.com/adampresley/adampresleyphotography/pkg/services"
	_ "github.com/glebarez/sqlite"
	"github.com/rfberaldo/sqlz"
	"github.com/rfberaldo/sqlz/binds"
)

var (
	Version string = "development"
	appName string = "adampresleyphotography"

	//go:embed app
	appFS embed.FS

	//go:embed sql-migrations
	sqlMigrationsFs embed.FS

	config configuration.Config

	/* Services */
	albumService        services.AlbumServicer
	cacheCreatorService cache.CacheCreator
	clientService       services.ClientServicer
	db                  *sqlz.DB
	renderer            rendering.TemplateRenderer
	sessionService      sessions.Session[*models.Client]
	zipService          services.ZipServicer

	/* Controllers */
	clientAccessController clientaccess.ClientAccessController
	homeController         home.HomeHandlers
)

func main() {
	var (
		err error
	)

	config = configuration.LoadConfig()
	setupLogger(&config, Version)

	slog.Info("configuration loaded",
		slog.String("app", appName),
		slog.String("version", Version),
		slog.String("loglevel", config.LogLevel),
		slog.String("host", config.Host),
		slog.String("awsEndpointUrl", config.AwsEndpointUrl),
		slog.String("awsRegion", config.AwsRegion),
	)

	slog.Debug("setting up...")

	shutdownCtx, cancel := context.WithCancel(context.Background())

	/*
	 * Setup services
	 */
	binds.Register("sqlite", binds.BindByDriver("sqlite3"))
	if db, err = sqlz.Connect("sqlite", config.DSN); err != nil {
		panic(err)
	}

	migrateDatabase()
	gob.Register(&models.Client{})

	cookieStore := sessions.NewCookieStore(config.CookieSecret)
	sessionService = sessions.NewSessionWrapper[*models.Client](cookieStore, "adamphotographyclients", "client")

	awsConfig := &awsconfig.Config{
		Endpoint:        config.AwsEndpointUrl,
		Region:          config.AwsRegion,
		AccessKeyID:     config.AwsAccessKeyId,
		SecretAccessKey: config.AwsSecretAccessKey,
	}

	retrier.Retry(func() error {
		if err = awsConfig.Load(); err != nil {
			slog.Error("failed to load AWS config. trying again", "error", err)
			return err
		}

		return nil
	})

	if err != nil {
		panic(err)
	}

	s3Client, err := s3.NewClient(awsConfig)

	if err != nil {
		panic(err)
	}

	renderer, err = rendering.NewGoTemplateRenderer(rendering.GoTemplateRendererConfig{
		TemplateDir:       "app",
		TemplateExtension: ".html",
		TemplateFS:        appFS,
		PagesDir:          "pages",
	})

	if err != nil {
		panic(err)
	}

	albumService = services.NewAlbumService(services.AlbumServiceConfig{
		DB: db,
	})

	clientService = services.NewClientService(services.ClientServiceConfig{
		DB: db,
	})

	zipService = services.NewZipService(services.ZipServiceConfig{
		AlbumService:      albumService,
		BaseDownloadURL:   config.DownloadBaseURL,
		Bucket:            config.AwsBucket,
		ClientPhotoFolder: config.ClientsPhotoFolder,
		ClientService:     clientService,
		ExpirationDays:    config.DownloadExpirationDays,
		S3Client:          s3Client,
		EmailApiKey:       config.EmailApiKey,
		FromName:          "Adam Presley",
		FromEmail:         "noreply@adampresleyphotography.com",
	})

	cacheCreatorService = cache.NewCacheCreatorService(cache.CacheCreatorConfig{
		AlbumService:        albumService,
		AwsBucket:           config.AwsBucket,
		AwsRegion:           config.AwsRegion,
		ClientsPhotoFolder:  config.ClientsPhotoFolder,
		ClientService:       clientService,
		HomePagePhotoFolder: config.HomePagePhotoFolder,
		MaxCacheWorkers:     config.MaxCacheWorkers,
		S3Client:            s3Client,
		ShutdownCtx:         shutdownCtx,
	})

	/*
	 * Setup controllers
	 */
	clientAccessController = clientaccess.NewClientAccessController(clientaccess.ClientAccessControllerConfig{
		AlbumService:      albumService,
		Bucket:            config.AwsBucket,
		ClientPhotoFolder: config.ClientsPhotoFolder,
		ClientService:     clientService,
		Renderer:          renderer,
		S3Client:          s3Client,
		SessionService:    sessionService,
		ZipService:        zipService,
	})

	homeController = home.NewHomeController(home.HomeControllerConfig{
		AwsBucket:           config.AwsBucket,
		HomePagePhotoFolder: config.HomePagePhotoFolder,
		Config:              &config,
		Renderer:            renderer,
		S3Client:            s3Client,
	})

	/*
	 * Setup router and http server
	 */
	slog.Debug("setting up routes...")

	clientAccessMiddleware := newClientAccessMiddleware(
		sessionService,
		[]string{
			"/static",
			"/client/login",
		},
	)

	routes := []mux.Route{
		{Path: "GET /heartbeat", HandlerFunc: heartbeat},
		{Path: "GET /", HandlerFunc: homeController.HomePage},
		{Path: "GET /client/login", HandlerFunc: clientAccessController.LoginPage},
		{Path: "POST /client/login", HandlerFunc: clientAccessController.LoginAction},
		{Path: "GET /client/logout", HandlerFunc: clientAccessController.LogoutAction},
		{Path: "GET /client", HandlerFunc: clientAccessController.AlbumListPage, Middlewares: []mux.MiddlewareFunc{clientAccessMiddleware}},
		{Path: "GET /client/", HandlerFunc: clientAccessController.AlbumListPage, Middlewares: []mux.MiddlewareFunc{clientAccessMiddleware}},
		{Path: "GET /client/{id}", HandlerFunc: clientAccessController.ViewAlbumPage, Middlewares: []mux.MiddlewareFunc{clientAccessMiddleware}},
		{Path: "GET /client/download-image", HandlerFunc: clientAccessController.DownloadImage, Middlewares: []mux.MiddlewareFunc{clientAccessMiddleware}},
		{Path: "GET /client/library/{albumid}/download-all", HandlerFunc: clientAccessController.DownloadAllImagesInAlbum, Middlewares: []mux.MiddlewareFunc{clientAccessMiddleware}},
		{Path: "GET /client/downloads/{filename}", HandlerFunc: clientAccessController.DownloadZip, Middlewares: []mux.MiddlewareFunc{clientAccessMiddleware}},
		{Path: "PUT /client/library/{albumid}/toggle-favorite", HandlerFunc: clientAccessController.ToggleFavorite, Middlewares: []mux.MiddlewareFunc{clientAccessMiddleware}},
	}

	routerConfig := mux.RouterConfig{
		Address:              config.Host,
		Debug:                Version == "development",
		ServeStaticContent:   true,
		StaticContentRootDir: "app",
		StaticContentPrefix:  "/static/",
		StaticFS:             appFS,
		HttpWriteTimeout:     60,
	}

	m := mux.SetupRouter(routerConfig, routes)
	httpServer, quit := mux.SetupServer(routerConfig, m)

	/*
	 * Start the zip cleanup job
	 */
	zipService.StartCleanupRoutine(24 * time.Hour)
	defer zipService.StopCleanupRoutine()

	/*
	 * Start the cache creator job
	 */
	setupCacheCreator(quit)

	/*
	 * Wait for graceful shutdown
	 */
	slog.Info("server started")

	<-quit

	cancel()
	mux.Shutdown(httpServer)
	slog.Info("server stopped")
}

func heartbeat(w http.ResponseWriter, r *http.Request) {
	httphelpers.TextOK(w, "OK")
}

func migrateDatabase() {
	var (
		err  error
		dirs []fs.DirEntry
		b    []byte
	)

	if dirs, err = sqlMigrationsFs.ReadDir("sql-migrations"); err != nil {
		panic(err)
	}

	for _, d := range dirs {
		if d.IsDir() {
			continue
		}

		if strings.HasPrefix(d.Name(), "commit") {
			if b, err = fs.ReadFile(sqlMigrationsFs, filepath.Join("sql-migrations", d.Name())); err != nil {
				panic(err)
			}

			if err = runSqlScript(b); err != nil {
				if !isIgnorableError(err) {
					panic(err)
				}
			}
		}
	}
}

func runSqlScript(script []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	_, err := db.Exec(ctx, string(script))
	return err
}

func isIgnorableError(err error) bool {
	if strings.Contains(err.Error(), "duplicate column") {
		return true
	}

	return false
}

func setupCacheCreator(quit chan os.Signal) {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		running := true

		runner := func() {
			defer func() {
				running = false
			}()

			cacheCreatorService.CreateCache()
			slog.Info("cache creator finished.")
		}

		runner()

		for {
			select {
			case <-quit:
				return

			case <-ticker.C:
				if running {
					slog.Info("cache creator already running. skipping...")
					continue
				}

				runner()
			}
		}
	}()
}
