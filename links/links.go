package links

import (
	"net/http"

	"github.com/flosch/pongo2"
	"github.com/go-chi/chi"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo/authorization"
)

type Links struct {
	DB *gorm.DB
}

func New() *Links {
	return &Links{}
}

func (l *Links) Configure(DB *gorm.DB) error {
	l.DB = DB

	return nil
}

func (l *Links) Resources() []interface{} {
	return []interface{}{
		&Link{},
	}
}

func (l *Links) ServeMux() *chi.Mux {
	router := chi.NewRouter()

	router.Get("/", l.ListHandler)

	return router
}

func (l *Links) ListHandler(w http.ResponseWriter, r *http.Request) {
	var allLinks []Link
	if err := l.DB.Find(&allLinks).Error; err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	context := pongo2.Context{
		"links": allLinks,
		"ifPath": func(path, output string) string {
			if r.URL.Path == path {
				return output
			}
			return ""
		},
	}

	if r.Context().Value("user") != nil {
		context["user"] = r.Context().Value("user").(authorization.User)
	}

	ts := pongo2.NewSet("test", pongo2.MustNewLocalFileSystemLoader("./links/templates/"))
	t, err := ts.FromFile("links.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
	err = t.ExecuteWriter(context, w)
	if err != nil {
		http.Error(w, err.Error(), 500)
	}
}
