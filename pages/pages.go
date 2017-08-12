package pages

import (
	"log"
	"net/http"

	"github.com/flosch/pongo2"
	"github.com/go-chi/chi"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo/authorization"
)

type Pages struct {
	DB *gorm.DB

	id int
}

func New(id int) *Pages {
	return &Pages{
		id: id,
	}
}

func (l *Pages) Configure(DB *gorm.DB) error {
	l.DB = DB

	return nil
}

func (l *Pages) Resources() []interface{} {
	return []interface{}{
		&Page{},
	}
}

func (l *Pages) ServeMux() *chi.Mux {
	router := chi.NewRouter()

	router.Get("/", l.ViewHandler)

	return router
}

func (l *Pages) ViewHandler(w http.ResponseWriter, r *http.Request) {
	var page Page
	if err := l.DB.Debug().First(&page, l.id).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	context := pongo2.Context{
		"page": page,
		"ifPath": func(path, output string) string {
			if r.URL.Path == path {
				return output
			}
			return ""
		},
	}
	log.Println(context)

	if r.Context().Value("user") != nil {
		context["user"] = r.Context().Value("user").(authorization.User)
	}

	ts := pongo2.NewSet("test", pongo2.MustNewLocalFileSystemLoader("./pages/templates/"))
	t, err := ts.FromFile("page.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	err = t.ExecuteWriter(context, w)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}
