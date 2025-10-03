package controller

import (
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/repos/sqlite"
)

type Controller struct {
	repo *sqlite.SQLiteStore // TODO: replace with interface
}

func New(repo *sqlite.SQLiteStore) Controller {
	return Controller{
		repo: repo,
	}
}

func (interactor *Controller) InitializeRoutes(router *http.ServeMux) {

}
