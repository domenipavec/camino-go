package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
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

	app := gongo.App{
		Authentication: Authentication,
		Authorization:  Authorization,
		DB:             DB,
		Render:         Render,
		Resources:      Resources,
		Store:          store,
		Controllers: []gongo.Controller{
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
	r.Use(middleware.WithValue("store", store))
	r.Use(Authorization.Middleware)

	r.Mount("/admin", app.Resources.ServeMux())
	r.Mount("/auth", app.Authentication.ServeMux())

	r.Mount("/links", Links.ServeMux())
	r.Mount("/camino", CaminoPage.ServeMux())

	r.Mount("/static", http.StripPrefix("/static", http.FileServer(packr.NewBox("./static"))))

	http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), r)
}
