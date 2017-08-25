package maps

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/camino-go/diary/models"
	"github.com/matematik7/gongo"
	"github.com/matematik7/gongo/render"
	"github.com/spf13/viper"
)

type Maps struct {
	DB     *gorm.DB
	render *render.Render
}

func New() *Maps {
	return &Maps{}
}

func (c *Maps) Configure(app gongo.App) error {
	c.DB = app["DB"].(*gorm.DB)
	c.render = app["Render"].(*render.Render)

	c.render.AddTemplates(packr.NewBox("./templates"))

	return nil
}

func (c Maps) Resources() []interface{} {
	return []interface{}{
		&models.MapEntry{},
		&models.MapGroup{},
		&models.GpsData{},
	}
}

func (c *Maps) ServeMux() http.Handler {
	router := chi.NewRouter()

	router.Get("/", c.ViewHandler)

	return router
}

func (c *Maps) ViewHandler(w http.ResponseWriter, r *http.Request) {
	var groups []models.MapGroup

	query := c.DB.Preload("Entries.DiaryEntry").Order("id desc").Find(&groups)
	if err := query.Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	filteredGroups := groups[:0]
	for _, group := range groups {
		if len(group.Entries) > 0 {
			filteredGroups = append(filteredGroups, group)
		}
	}

	var gpsData []models.GpsData
	if err := c.DB.Find(&gpsData).Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	context := render.Context{
		"groups":      filteredGroups,
		"gps_data":    gpsData,
		"browser_key": viper.GetString("GMAP_BROWSER_KEY"),
	}

	c.render.Template(w, r, "map.html", context)
}
