package maps

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo"
)

type Maps struct {
	DB     *gorm.DB
	render gongo.Render
}

func New() *Maps {
	return &Maps{}
}

func (c *Maps) Configure(app gongo.App) error {
	c.DB = app.DB
	c.render = app.Render

	c.render.AddTemplates(packr.NewBox("./templates"))

	return nil
}

func (c Maps) Resources() []interface{} {
	return []interface{}{
		&MapEntry{},
		&MapGroup{},
		&GpsData{},
	}
}

func (c Maps) Name() string {
	return "Maps"
}

func (c *Maps) ServeMux() http.Handler {
	router := chi.NewRouter()

	// router.Get("/", c.ListHandler)

	return router
}
