package links

import (
	"net/http"

	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo"
	"github.com/matematik7/gongo/render"
)

type Links struct {
	DB     *gorm.DB
	render *render.Render
}

func New() *Links {
	return &Links{}
}

func (l *Links) Configure(app gongo.App) error {
	l.DB = app["DB"].(*gorm.DB)
	l.render = app["Render"].(*render.Render)

	l.render.AddTemplates(packr.NewBox("./templates"))

	return nil
}

func (l Links) Resources() []interface{} {
	return []interface{}{
		&Link{},
	}
}

func (l *Links) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var allLinks []Link
	if err := l.DB.Find(&allLinks).Error; err != nil {
		l.render.Error(w, r, err)
		return
	}

	context := render.Context{
		"links": allLinks,
	}

	l.render.Template(w, r, "links.html", context)
}
