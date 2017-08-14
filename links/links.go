package links

import (
	"net/http"

	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo"
)

type Links struct {
	DB     *gorm.DB
	render gongo.Render
}

func New() *Links {
	return &Links{}
}

func (l *Links) Configure(app gongo.App) error {
	l.DB = app.DB
	l.render = app.Render

	l.render.AddTemplates(packr.NewBox("./templates"))

	return nil
}

func (l Links) Resources() []interface{} {
	return []interface{}{
		&Link{},
	}
}

func (l Links) Name() string {
	return "Links"
}

func (l *Links) ServeMux() http.Handler {
	return l
}

func (l *Links) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var allLinks []Link
	if err := l.DB.Find(&allLinks).Error; err != nil {
		l.render.Error(w, r, err)
		return
	}

	context := gongo.Context{
		"links": allLinks,
	}

	l.render.Template(w, r, "links.html", context)
}
