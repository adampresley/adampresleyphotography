package viewmodels

import (
	"net/http"

	"github.com/adampresley/adamgokit/rendering"
	"github.com/adampresley/adampresleyphotography/pkg/models"
)

type BaseViewModel struct {
	Message            string
	IsError            bool
	IsWarning          bool
	IsHtmx             bool
	JavascriptIncludes []rendering.JavascriptInclude
}

func GetClientFromContext(r *http.Request) *models.Client {
	if result, ok := r.Context().Value("client").(*models.Client); ok {
		return result
	}

	return &models.Client{}
}
