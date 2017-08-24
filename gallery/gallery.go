package gallery

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi"
	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo"
	"github.com/matematik7/gongo/files/storage"
	"github.com/matematik7/gongo/render"
	"github.com/pkg/errors"
)

const PerPage = 10

type Gallery struct {
	DB      *gorm.DB
	render  *render.Render
	storage storage.Storage
}

func New() *Gallery {
	return &Gallery{}
}

func (c *Gallery) Configure(app gongo.App) error {
	c.DB = app["DB"].(*gorm.DB)
	c.render = app["Render"].(*render.Render)
	c.storage = app["Storage"].(storage.Storage)

	c.render.AddTemplates(packr.NewBox("./templates"))

	return nil
}

func (c *Gallery) ServeMux() http.Handler {
	router := chi.NewRouter()

	router.Get("/", c.ListHandler)

	router.Route("/{galleryID:[a-zA-Z0-9]+}", func(r chi.Router) {
		r.Get("/", c.ViewHandler)
	})

	return router
}

func (c *Gallery) getTitle(galleryID string) (string, error) {
	url, err := c.storage.URL(fmt.Sprintf("gallery/%s/title", galleryID))
	if err != nil {
		return "", errors.Wrap(err, "could not get title url")
	}

	response, err := http.Get(url)
	if err != nil {
		return "", errors.Wrap(err, "could not get title")
	}
	defer response.Body.Close()

	title, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return "", errors.Wrap(err, "could not read title")
	}

	return string(title), nil
}

func (c *Gallery) ListHandler(w http.ResponseWriter, r *http.Request) {
	entries, err := c.storage.List("gallery/")
	if err != nil {
		c.render.Error(w, r, err)
		return
	}

	galleries := make([]GalleryEntry, len(entries))
	for i, entry := range entries {
		galleries[i].ID = path.Base(entry)
		galleries[i].Title, err = c.getTitle(galleries[i].ID)
		if err != nil {
			c.render.Error(w, r, err)
			return
		}
	}

	context := render.Context{
		"galleries": galleries,
	}

	c.render.Template(w, r, "galleries.html", context)
}

func (c *Gallery) ViewHandler(w http.ResponseWriter, r *http.Request) {
	galleryID := chi.URLParam(r, "galleryID")
	title, err := c.getTitle(galleryID)
	if err != nil {
		c.render.Error(w, r, err)
		return
	}

	imageNames, err := c.storage.List(fmt.Sprintf("gallery/%s/", galleryID))
	if err != nil {
		c.render.Error(w, r, err)
		return
	}

	images := make([]string, 0, len(imageNames))
	for _, name := range imageNames {
		if !strings.HasSuffix(strings.ToLower(name), ".jpg") {
			continue
		}
		url, err := c.storage.URL(name)
		if err != nil {
			c.render.Error(w, r, err)
			return
		}
		images = append(images, url)
	}

	context := render.Context{
		"title":  title,
		"images": images,
	}

	c.render.Template(w, r, "gallery.html", context)
}
