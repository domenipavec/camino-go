package stats

import (
	"net/http"

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
		var groups []models.DiaryGroup
		// TODO: handle errors
		c.DB.
			Joins("LEFT JOIN diary_entries de ON de.diary_group_id = diary_groups.id").
			Joins("LEFT JOIN map_entries me1 ON de.map_entry_id = me1.id").
			Joins("LEFT JOIN gps_data gd1 ON me1.gps_data_id = gd1.id").
			Group("diary_groups.id").
			// Where("gd1.id IS NOT NULL").
			Order("created_at desc").
			Find(&groups)
		ctx["statsDiaryGroups"] = groups
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

	router.Get("/{year}", c.ViewHandler)

	return router
}

func (c *Stats) ViewHandler(w http.ResponseWriter, r *http.Request) {
	year := chi.URLParam(r, "year")

	var diaryGroup models.DiaryGroup
	groupQuery := c.DB.Order("created_at desc").Limit(1).
		Where("slug = ?", year).
		First(&diaryGroup)
	if groupQuery.RecordNotFound() {
		c.render.NotFound(w, r)
		return
	} else if groupQuery.Error != nil {
		c.render.Error(w, r, groupQuery.Error)
		return
	}

	gpsData := []models.GpsData{}
	query := c.DB.
		Joins("LEFT JOIN map_entries me1 ON gps_data.id = me1.gps_data_id").
		Joins("LEFT JOIN diary_entries de1 ON de1.map_entry_id = me1.id").
		Where("de1.diary_group_id = ?", diaryGroup.ID).
		Where("de1.published = true").
		Order("de1.created_at").
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

	// TODO: separate year to fix active year marker
	context := render.Context{
		"diaryGroup": diaryGroup,
		"distances":  distances,
		"times":      times,
		"speeds":     speeds,
	}

	c.render.Template(w, r, "stats.html", context)
}
