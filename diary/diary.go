package diary

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo"
	"github.com/pkg/errors"
)

const PerPage = 10

type Diary struct {
	DB     *gorm.DB
	render gongo.Render
}

func New() *Diary {
	return &Diary{}
}

func (c *Diary) Configure(app gongo.App) error {
	c.DB = app.DB
	c.render = app.Render

	c.render.AddTemplates(packr.NewBox("./templates"))

	c.render.AddContextFunc(func(r *http.Request, ctx gongo.Context) {
		var years []int
		// TODO: add error handling
		c.DB.Model(&DiaryEntry{}).
			Select("DISTINCT date_part('year', created_at) as year").
			Order("year desc").
			Pluck("year", &years)
		ctx["diaryYears"] = years
	})

	return nil
}

func (c Diary) Resources() []interface{} {
	return []interface{}{
		&DiaryEntry{},
	}
}

func (c Diary) Name() string {
	return "Diary"
}

func (c *Diary) ServeMux() http.Handler {
	router := chi.NewRouter()

	router.Get("/", c.ListHandler)

	router.Get("/{diaryID:[0-9]+}", c.ViewHandler)

	return router
}

func (c *Diary) ViewHandler(w http.ResponseWriter, r *http.Request) {
	var entry DiaryEntry

	if err := c.DB.First(&entry, chi.URLParam(r, "diaryID")).Error; err != nil {
		c.render.Error(w, r, err)
	}

	context := gongo.Context{
		"entry": entry,
	}

	c.render.Template(w, r, "diary_one.html", context)
}

func (c *Diary) ListHandler(w http.ResponseWriter, r *http.Request) {
	query := c.DB.Model(&DiaryEntry{}).Preload("Author").
		Order("created_at desc")

	var yearItf interface{}
	if r.URL.Query().Get("year") != "" {
		year, err := strconv.Atoi(r.URL.Query().Get("year"))
		if err != nil {
			c.render.Error(w, r, errors.Wrap(err, "invalid year"))
			return
		}
		query = query.Where("date_part('year', created_at) = ?", year)
		yearItf = year
	}

	var count int
	if err := query.Count(&count).Error; err != nil {
		c.render.Error(w, r, err)
		return
	}
	pages := make([]int, (count+PerPage-1)/PerPage)
	for i := range pages {
		pages[i] = PerPage * i
	}

	offset := 0
	if r.URL.Query().Get("offset") != "" {
		var err error
		offset, err = strconv.Atoi(r.URL.Query().Get("offset"))
		if err != nil {
			c.render.Error(w, r, errors.Wrap(err, "invalid offset"))
			return
		}
		query = query.Offset(offset)
	}
	nextOffset := offset + PerPage
	prevOffset := offset - PerPage

	var entries []DiaryEntry
	if err := query.Limit(PerPage).Find(&entries).Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	context := gongo.Context{
		"entries":    entries,
		"offset":     offset,
		"pages":      pages,
		"prevOffset": prevOffset,
		"nextOffset": nextOffset,
		"year":       yearItf,
	}

	c.render.Template(w, r, "diary_all.html", context)
}
