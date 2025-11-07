package configuration

import "github.com/adampresley/configinator"

type Config struct {
	AwsEndpointUrl         string `flag:"awsep" env:"AWS_ENDPOINT_URL" default:"http://localhost:4566" description:"AWS endpoint URL"`
	AwsRegion              string `flag:"awsregion" env:"AWS_REGION" default:"us-central-1" description:"AWS region"`
	AwsAccessKeyId         string `flag:"awsaccesskeyid" env:"AWS_ACCESS_KEY_ID" default:"" description:"AWS access key ID"`
	AwsSecretAccessKey     string `flag:"awssecretaccesskey" env:"AWS_SECRET_ACCESS_KEY" default:"" description:"AWS secret access key"`
	AwsBucket              string `flag:"awsbucket" env:"AWS_BUCKET" default:"adampresleyphotography.com" description:"S3 bucket"`
	ClientsPhotoFolder     string `flag:"cpf" env:"CLIENTS_PHOTO_FOLDER" default:"clients" description:"S3 folder for clients' photos"`
	CookieSecret           string `flag:"cookiesecret" env:"COOKIE_SECRET" default:"password" description:"Secret for encoding coodies"`
	DataMigrationDir       string `flag:"dmd" env:"DATA_MIGRATION_DIR" default:"../../sql-migrations" description:"Migration folder"`
	DownloadBaseURL        string `flag:"dlb" env:"DOWNLOAD_BASE_URL" default:"http://localhost:8080" description:"Base URL for downloading images"`
	DownloadExpirationDays int    `flag:"dle" env:"DOWNLOAD_EXPIRATION_DAYS" default:"30" description:"Number of days before images expire in the download directory"`
	DSN                    string `flag:"dsn" env:"DSN" default:"file:./data/adampresleyphotography.db" description:"Data source name"`
	EmailApiKey            string `flag:"emailapikey" env:"EMAIL_API_KEY" default:"" description:"API key for sending emails"`
	HomePagePhotoFolder    string `flag:"hppf" env:"HOME_PAGE_PHOTO_FOLDER" default:"home-page" description:"S3 folder for home page photos"`
	Host                   string `flag:"host" env:"HOST" default:"localhost:8081" description:"The address and port to bind the HTTP server to"`
	LogLevel               string `flag:"loglevel" env:"LOG_LEVEL" default:"debug" description:"The log level to use. Valid values are 'debug', 'info', 'warn', and 'error'"`
	MaxCacheWorkers        int    `flag:"mcc" env:"MAX_CACHE_WORKERS" default:"20" description:"Maximum number of concurrent cache workers"`
}

func LoadConfig() Config {
	config := Config{}
	configinator.Behold(&config)
	return config
}
