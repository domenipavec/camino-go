package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/flosch/pongo2"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/matematik7/camino-go/diary"
	"github.com/matematik7/camino-go/links"
	"github.com/matematik7/camino-go/pages"
	"github.com/matematik7/gongo"
	"github.com/matematik7/gongo/authentication"
	"github.com/matematik7/gongo/authorization"
	"github.com/matematik7/gongo/render"
	"github.com/matematik7/gongo/resources"
	"github.com/spf13/viper"
)

func main() {
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("port", 3000)
	port := viper.GetInt("port")

	viper.SetDefault("host", "localhost")
	host := viper.GetString("host")

	viper.SetDefault("url", fmt.Sprintf("http://%s:%d", host, port))
	url := viper.GetString("url")

	viper.SetDefault("prod", false)
	isProd := viper.GetBool("prod")

	viper.SetDefault("cookie_key", "SESSION_SECRET") // TODO: generate secret in generator
	cookieKey := viper.GetString("cookie_key")

	DB, err := gorm.Open("postgres", "host=localhost user=postgres sslmode=disable password=postgres")
	if err != nil {
		log.Fatalf("could not open db: %v", err)
	}

	validate := func(scope *gorm.Scope) {
		ok, err := govalidator.ValidateStruct(scope.Value)
		if !ok {
			scope.Err(err)
		}
	}

	DB.Callback().Create().Before("gorm:before_create").Register("govalidator:before_create", validate)
	DB.Callback().Update().Before("gorm:before_update").Register("govalidator:before_update", validate)

	store := sessions.NewCookieStore([]byte(cookieKey))
	store.MaxAge(60 * 60 * 24 * 30)
	store.Options.Path = "/"
	store.Options.HttpOnly = true
	store.Options.Secure = isProd

	Authentication := authentication.New(url + "/auth")
	Authorization := authorization.New()
	Render := render.New(isProd)
	Resources := resources.New("/admin")

	Render.AddTemplates(packr.NewBox("./templates"))

	Links := links.New()
	CaminoPage := pages.New(1)
	Diary := diary.New()

	app := gongo.App{
		Authentication: Authentication,
		Authorization:  Authorization,
		DB:             DB,
		Render:         Render,
		Resources:      Resources,
		Store:          store,
		Controllers: []gongo.Controller{
			Diary,
			Links,
			CaminoPage,
		},
	}

	if err := app.Configure(); err != nil {
		log.Fatalln(err)
	}

	if err := app.RegisterResources(); err != nil {
		log.Fatalln(err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)
	r.Use(middleware.WithValue("store", store)) // TODO: do we need this anywhere?
	r.Use(Authorization.Middleware)

	r.Mount("/admin", app.Resources.ServeMux()) // TODO: figure out if this uses some sort of csrf
	r.Mount("/auth", app.Authentication.ServeMux())

	r.Group(func(r chi.Router) { // web is protected with csrf
		// TODO: move csrf stuff to gongo package
		app.Render.AddContextFunc(func(r *http.Request, ctx gongo.Context) {
			// TODO: add safe value to render
			ctx["csrf_token"] = pongo2.AsSafeValue(csrf.TemplateField(r))
		})
		r.Use(csrf.Protect(
			[]byte("01234567890123456789012345678901"),
			csrf.HttpOnly(true),
			csrf.Secure(isProd),
			csrf.FieldName("csrfmiddlewaretoken"),
			csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				app.Render.Error(w, r, errors.New("Forbidden")) // TODO: move this handler to render
			})),
		)) // TODO: pass as param and generate in generator

		r.Mount("/diary", Diary.ServeMux())
		r.Mount("/", Diary.ServeMux())
		r.Mount("/links", Links.ServeMux())
		r.Mount("/camino", CaminoPage.ServeMux())
	})

	r.Mount("/static", http.StripPrefix("/static", http.FileServer(packr.NewBox("./static"))))

	//TODO: Move these handlers to render
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		app.Render.Template(w, r, "error.html", gongo.Context{
			"title": "Not Found",
			"msg":   "This is not the web page you are looking for.",
		})
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		app.Render.Template(w, r, "error.html", gongo.Context{
			"title": "Method Not Allowed",
			"msg":   "Your position's correct, except... not this method.",
		})
	})

	http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), r)
}
