package pages

import (
	"net/http"

	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo"
)

type Pages struct {
	DB     *gorm.DB
	render gongo.Render

	id int
}

func New(id int) *Pages {
	return &Pages{
		id: id,
	}
}

func (l *Pages) Configure(app gongo.App) error {
	l.DB = app.DB
	l.render = app.Render

	l.render.AddTemplates(packr.NewBox("./templates"))

	return nil
}

func (l Pages) Resources() []interface{} {
	return []interface{}{
		&Page{},
	}
}

func (l Pages) Name() string {
	return "Pages"
}

func (l *Pages) ServeMux() http.Handler {
	return l
}

func (l *Pages) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var page Page
	if err := l.DB.First(&page, l.id).Error; err != nil {
		l.render.Error(w, r, err)
		return
	}

	context := gongo.Context{
		"page": page,
	}

	l.render.Template(w, r, "page.html", context)
}
