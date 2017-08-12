package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/matematik7/camino-go/links"
	"github.com/matematik7/gongo/authentication"
	"github.com/matematik7/gongo/authorization"
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

	viper.SetDefault("cookie_key", "SESSION_SECRET")
	cookieKey := viper.GetString("cookie_key")

	Authentication := authentication.New()
	Authorization := authorization.New()
	Resources := resources.New()

	Links := links.New()

	DB, err := gorm.Open("postgres", "host=localhost user=postgres sslmode=disable password=postgres")
	if err != nil {
		log.Fatalf("could not open db: %v", err)
	}

	store := sessions.NewCookieStore([]byte(cookieKey))
	store.MaxAge(60 * 60 * 24 * 30)
	store.Options.Path = "/"
	store.Options.HttpOnly = true
	store.Options.Secure = isProd

	if err := Authentication.Configure(store, Authorization, url+"/auth"); err != nil {
		log.Fatalf("could not configure authentication: %v", err)
	}
	if err := Authorization.Configure(DB, store); err != nil {
		log.Fatalf("could not configure authorization: %v", err)
	}
	if err := Resources.Configure(DB, Authorization); err != nil {
		log.Fatalf("could not configure resources: %v", err)
	}
	if err := Links.Configure(DB); err != nil {
		log.Fatalf("could not configure links: %v", err)
	}

	if err := Resources.Register("Authorization", Authorization.Resources()...); err != nil {
		log.Fatalf("could not add resources for authorization: %v", err)
	}
	if err := Resources.Register("Links", Links.Resources()...); err != nil {
		log.Fatalf("could not add resources for links: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.StripSlashes)
	r.Use(middleware.WithValue("store", store))
	r.Use(Authorization.Middleware)

	r.Mount("/admin", Resources.ServeMux("/admin"))
	r.Mount("/auth", Authentication.ServeMux())
	r.Mount("/links", Links.ServeMux())

	r.Mount("/static", http.StripPrefix("/static", http.FileServer(packr.NewBox("./static"))))

	http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), r)
}

type History struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	Name      string
	User      authorization.User `gorm:"not null"`
}
