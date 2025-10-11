package controller

import (
	"net/http"

	"github.com/sj14/darts-o-mat/pkg/repos/postgres"
)

type Controller struct {
	repo *postgres.Store // TODO: replace with interface
}

func New(repo *postgres.Store) Controller {
	return Controller{
		repo: repo,
	}
}

func (interactor *Controller) InitializeRoutes(router *http.ServeMux) {

}
