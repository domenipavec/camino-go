package maps

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/camino-go/diary/models"
	"github.com/matematik7/gongo"
	"github.com/matematik7/gongo/authorization"
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
	// TODO: move etag handling to render in gongo (important: binary.Write cannot handle generic uint, must be uint64)
	// can we just use gongo.Context to somehow calculate etag?
	etag := md5.New()

	var groups []models.MapGroup

	query := c.DB.Preload("Entries.DiaryEntry").Order("id desc").Find(&groups)
	if err := query.Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	if r.Context().Value("user") != nil {
		id := r.Context().Value("user").(authorization.User).ID
		err := binary.Write(etag, binary.BigEndian, uint64(id))
		if err != nil {
			c.render.Error(w, r, err)
			return
		}
	}

	gpsDataIDS := make([]uint, 0, 100)
	filteredGroups := groups[:0]
	for _, group := range groups {
		if len(group.Entries) > 0 {
			filteredGroups = append(filteredGroups, group)
		}
		binary.Write(etag, binary.BigEndian, uint64(group.ID))
		binary.Write(etag, binary.BigEndian, uint64(group.UpdatedAt.UnixNano()))
		for _, entry := range group.Entries {
			binary.Write(etag, binary.BigEndian, uint64(entry.ID))
			binary.Write(etag, binary.BigEndian, uint64(entry.UpdatedAt.UnixNano()))
			if entry.GpsDataID != 0 {
				gpsDataIDS = append(gpsDataIDS, entry.GpsDataID)
			}
		}
	}

	var gpsData []models.GpsData
	if err := c.DB.Where("id in (?)", gpsDataIDS).Find(&gpsData).Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	for _, gpsEntry := range gpsData {
		binary.Write(etag, binary.BigEndian, uint64(gpsEntry.ID))
		binary.Write(etag, binary.BigEndian, uint64(gpsEntry.UpdatedAt.UnixNano()))
	}

	etagStr := hex.EncodeToString(etag.Sum(nil))
	w.Header().Set("Etag", etagStr)
	if match := r.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, etagStr) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	context := render.Context{
		"groups":      filteredGroups,
		"gps_data":    gpsData,
		"browser_key": viper.GetString("GMAP_BROWSER_KEY"),
	}

	c.render.Template(w, r, "map.html", context)
}
