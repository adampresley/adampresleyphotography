package viewmodels

type HomePage struct {
	BaseViewModel
	Photos []HomePagePhoto
}

type HomePagePhoto struct {
	OriginalPath  string
	ThumbnailPath string
	FileName      string
}
