package maps

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo"
	"github.com/spf13/viper"
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

	router.Get("/", c.ViewHandler)

	return router
}

func (c *Maps) ViewHandler(w http.ResponseWriter, r *http.Request) {
	var groups []MapGroup

	query := c.DB.Preload("Entries").Order("id desc").Find(&groups)
	if err := query.Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	var gpsData []GpsData
	if err := c.DB.Find(&gpsData).Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	context := gongo.Context{
		"groups":      groups,
		"gps_data":    gpsData,
		"browser_key": viper.GetString("GMAP_BROWSER_KEY"),
	}

	c.render.Template(w, r, "map.html", context)
}
