package maps

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	jsoniter "github.com/json-iterator/go"
	"github.com/matematik7/camino-go/diary/models"
	"github.com/matematik7/gongo"
	"github.com/matematik7/gongo/files"
	"github.com/matematik7/gongo/render"
	"github.com/spf13/viper"
)

type Maps struct {
	DB     *gorm.DB
	Files  *files.Files
	render *render.Render
}

func New() *Maps {
	return &Maps{}
}

func (c *Maps) Configure(app gongo.App) error {
	c.DB = app["DB"].(*gorm.DB)
	c.render = app["Render"].(*render.Render)
	c.Files = app["Files"].(*files.Files)

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
	router.Get("/group/{groupID:[0-9]+}", c.GroupJSONHandler)

	return router
}

func (c *Maps) ViewHandler(w http.ResponseWriter, r *http.Request) {
	var groups []models.MapGroup

	subQuery := c.DB.Select("distinct map_group_id").Table("map_entries").SubQuery()
	query := c.DB.Order("id desc").Where("id IN (?)", subQuery).Find(&groups)
	if err := query.Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	context := render.Context{
		"groups":      groups,
		"browser_key": viper.GetString("GMAP_BROWSER_KEY"),
	}

	c.render.Template(w, r, "map.html", context)
}

type MapEntryJSON struct {
	ID          uint            `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description,omitempty"`
	Lat         float64         `json:"latitude"`
	Lon         float64         `json:"longitude"`
	GpsDataID   uint            `json:"gps_id"`
	DiaryEntry  *DiaryEntryJSON `json:"diary,omitempty"`
}

type DiaryEntryJSON struct {
	ID    uint       `json:"id"`
	Title string     `json:"title"`
	Image *ImageJSON `json:"image,omitempty"`
}

type ImageJSON struct {
	Description string `json:"description"`
	URL         string `json:"url"`
}

func (c *Maps) GroupJSONHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "groupID"))
	if err != nil {
		c.render.NotFound(w, r)
		return
	}

	var entries []models.MapEntry
	result := c.DB.Preload("DiaryEntry.Images", func(db *gorm.DB) *gorm.DB {
		return db.Order("diary_entry_id, RANDOM()").Select("distinct on (diary_entry_id) *")
	}).
		Where("map_group_id = ?", id).Order("id desc").Find(&entries)
	if err := result.Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	var gpsData models.GpsData
	gpsRows, err := c.DB.Model(&gpsData).Joins("JOIN map_entries ON map_entries.gps_data_id = gps_data.id").Where("map_entries.map_group_id = ?", id).Rows()
	if err != nil {
		c.render.Error(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	stream := jsoniter.ConfigFastest.BorrowStream(w)
	defer jsoniter.ConfigFastest.ReturnStream(stream)
	stream.WriteObjectStart()

	stream.WriteObjectField("entries")
	stream.WriteArrayStart()
	for i, entry := range entries {
		if i != 0 {
			stream.WriteMore()
		}

		jsonEntry := MapEntryJSON{
			ID:          entry.ID,
			Title:       entry.City,
			Description: entry.Description,
			Lat:         entry.Lat,
			Lon:         entry.Lon,
			GpsDataID:   entry.GpsDataID,
		}
		if entry.DiaryEntry != nil {
			jsonEntry.DiaryEntry = &DiaryEntryJSON{
				ID:    entry.DiaryEntry.ID,
				Title: entry.DiaryEntry.Title,
			}
			if len(entry.DiaryEntry.Images) > 0 {
				url, err := c.Files.URL(entry.DiaryEntry.Images[0])
				if err != nil {
					c.render.Error(w, r, err)
					return
				}
				jsonEntry.DiaryEntry.Image = &ImageJSON{
					Description: entry.DiaryEntry.Images[0].Description,
					URL:         url,
				}
			}
		}

		stream.WriteVal(jsonEntry)
	}
	stream.WriteArrayEnd()

	stream.WriteMore()
	stream.WriteObjectField("gps")
	stream.WriteObjectStart()
	first := true
	for gpsRows.Next() {
		err := c.DB.ScanRows(gpsRows, &gpsData)
		if err != nil {
			c.render.Error(w, r, err)
			return
		}
		optimizedData, err := gpsData.OptimizedData()
		if err != nil {
			c.render.Error(w, r, err)
			return
		}

		if !first {
			stream.WriteMore()
		}
		stream.WriteObjectField(strconv.Itoa(int(gpsData.ID)))
		stream.WriteRaw(optimizedData)

		first = false
	}
	if err := gpsRows.Err(); err != nil {
		c.render.Error(w, r, err)
		return
	}
	stream.WriteObjectEnd()

	stream.WriteObjectEnd()
	stream.Flush()
}
