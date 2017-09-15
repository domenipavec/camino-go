package stats

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/flosch/pongo2"
	"github.com/go-chi/chi"
	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo"
	"github.com/matematik7/gongo/render"

	"github.com/matematik7/camino-go/diary/models"
)

type Stats struct {
	DB     *gorm.DB
	render *render.Render
}

func New() *Stats {
	return &Stats{}
}

func (c *Stats) Configure(app gongo.App) error {
	c.DB = app["DB"].(*gorm.DB)
	c.render = app["Render"].(*render.Render)

	c.render.AddTemplates(packr.NewBox("./templates"))

	c.render.AddContextFunc(func(r *http.Request, ctx render.Context) {
		var years []int
		// TODO: handle errors
		c.DB.Model(&models.DiaryEntry{}).
			Select("DISTINCT date_part('year', diary_entries.created_at) as year").
			Joins("LEFT JOIN map_entries me1 ON diary_entries.map_entry_id = me1.id").
			Joins("LEFT JOIN gps_data gd1 ON me1.gps_data_id = gd1.id").
			Where("gd1.id IS NOT NULL").
			Order("year desc").
			Pluck("year", &years)
		ctx["statsYears"] = years
	})

	pongo2.RegisterFilter("sum", func(in *pongo2.Value, param *pongo2.Value) (out *pongo2.Value, err *pongo2.Error) {
		sum := 0.0
		in.Iterate(func(idx, count int, item, none *pongo2.Value) bool {
			sum += item.Float()
			return true
		}, func() {})
		return pongo2.AsValue(sum), nil
	})

	pongo2.RegisterFilter("average", func(in *pongo2.Value, param *pongo2.Value) (out *pongo2.Value, err *pongo2.Error) {
		sum := 0.0
		in.Iterate(func(idx, count int, item, none *pongo2.Value) bool {
			sum += item.Float()
			return true
		}, func() {})
		return pongo2.AsValue(sum / float64(in.Len())), nil
	})

	return nil
}

func (c *Stats) ServeMux() http.Handler {
	router := chi.NewRouter()

	router.Get("/{year:[0-9]+}", c.ViewHandler)

	return router
}

func (c *Stats) ViewHandler(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		// TODO: introduce system not found
		c.render.Error(w, r, errors.New("Not Found"))
		return
	}

	gpsData := []models.GpsData{}
	query := c.DB.
		Joins("LEFT JOIN map_entries me1 ON gps_data.id = me1.gps_data_id").
		Joins("LEFT JOIN diary_entries de1 ON de1.map_entry_id = me1.id").
		Where("date_part('year', de1.created_at) = ?", year).
		Where("de1.published = true").
		Find(&gpsData)
	if query.Error != nil {
		c.render.Error(w, r, query.Error)
		return
	}

	distances := make([]float64, len(gpsData))
	times := make([]float64, len(gpsData))
	speeds := make([]float64, len(gpsData))
	for i := range gpsData {
		distances[i] = gpsData[i].Length
		times[i] = gpsData[i].Duration
		speeds[i] = gpsData[i].AvgSpeed
	}

	context := render.Context{
		"year":      year,
		"distances": distances,
		"times":     times,
		"speeds":    speeds,
	}

	c.render.Template(w, r, "stats.html", context)
}
