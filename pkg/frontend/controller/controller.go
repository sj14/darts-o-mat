package controller

import (
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/repos/store"
)

type Controller struct {
	repo *store.Store // TODO: replace with interface
}

func New(repo *store.Store) Controller {
	return Controller{
		repo: repo,
	}
}

func (interactor *Controller) InitializeRoutes(router *http.ServeMux) {

}
