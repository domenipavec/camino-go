package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/asaskevich/govalidator"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/flosch/pongo2"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/matematik7/camino-go/diary"
	"github.com/matematik7/camino-go/endomondo"
	"github.com/matematik7/camino-go/gallery"
	"github.com/matematik7/camino-go/links"
	"github.com/matematik7/camino-go/maps"
	"github.com/matematik7/camino-go/pages"
	"github.com/matematik7/gongo"
	"github.com/matematik7/gongo/admin"
	"github.com/matematik7/gongo/authentication"
	"github.com/matematik7/gongo/authorization"
	"github.com/matematik7/gongo/files"
	"github.com/matematik7/gongo/files/storage/s3storage"
	"github.com/matematik7/gongo/render"
	"github.com/pkg/errors"
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

	viper.SetDefault("csrf_key", "01234567890123456789012345678901")

	viper.SetDefault("psql_host", "localhost")
	viper.SetDefault("psql_dbname", "postgres")
	viper.SetDefault("psql_user", "postgres")
	viper.SetDefault("psql_password", "postgres")
	psqlConfig := fmt.Sprintf(
		"host=%s dbname=%s user=%s password=%s sslmode=disable",
		viper.GetString("psql_host"),
		viper.GetString("psql_dbname"),
		viper.GetString("psql_user"),
		viper.GetString("psql_password"),
	)
	DB, err := gorm.Open("postgres", psqlConfig)
	if err != nil {
		log.Fatalf("could not open db: %v", err)
	}

	if isProd {
		// TODO: automate this somehow
		if viper.GetString("psql_password") == "postgres" {
			log.Fatal("Cannot use default password in production")
		}
		if viper.GetString("csrf_key") == "01234567890123456789012345678901" {
			log.Fatal("Cannot use default csrf_key in production")
		}
		if viper.GetString("cookie_key") == "SESSION_SECRET" {
			log.Fatal("Cannot use default cookie key in production")
		}
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
	Admin := admin.New("/admin")

	AwsSession, err := session.NewSession()
	if err != nil {
		log.Fatal(err)
	}
	// TODO: make bucket configurable
	Storage, err := s3storage.New(AwsSession, "images-camino", false)
	if err != nil {
		log.Fatal(err)
	}
	Files := files.New(Storage)

	Render.AddTemplates(packr.NewBox("./templates"))

	Diary := diary.New()
	Links := links.New()
	CaminoPage := pages.New(1)
	Maps := maps.New()
	Gallery := gallery.New()

	Endomondo, err := endomondo.New(viper.GetString("ENDOMONDO_EMAIL"), viper.GetString("ENDOMONDO_PASSWORD"))
	if err != nil {
		log.Fatal(err)
	}

	app := gongo.App{
		"Admin":          Admin,
		"Authentication": Authentication,
		"Authorization":  Authorization,
		"DB":             DB,
		"Files":          Files,
		"Render":         Render,
		"Storage":        Storage,
		"Store":          store,

		"Diary":      Diary,
		"Links":      Links,
		"CaminoPage": CaminoPage,
		"Maps":       Maps,
		"Gallery":    Gallery,

		"Endomondo": Endomondo,
	}

	if err := app.Configure(); err != nil {
		log.Fatalln(err)
	}

	for _, itf := range app {
		if resourcer, ok := itf.(gongo.Resourcer); ok {
			if err := DB.AutoMigrate(resourcer.Resources()...).Error; err != nil {
				log.Fatal(errors.Wrap(err, "could not auto migrate models"))
			}
		}
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer) // TODO: proper error page
	r.Use(middleware.StripSlashes)
	r.Use(middleware.WithValue("store", store)) // TODO: do we need this anywhere?
	r.Use(Authorization.Middleware)

	r.Mount("/admin", Admin.ServeMux()) // TODO: figure out if this uses some sort of csrf
	r.Mount("/auth", Authentication.ServeMux())

	r.Group(func(r chi.Router) { // web is protected with csrf
		// TODO: move csrf stuff to gongo package
		Render.AddContextFunc(func(r *http.Request, ctx render.Context) {
			// TODO: add safe value to render
			ctx["csrf_token"] = pongo2.AsSafeValue(csrf.TemplateField(r))
		})
		r.Use(csrf.Protect(
			[]byte(viper.GetString("csrf_key")),
			csrf.HttpOnly(true),
			csrf.Secure(isProd),
			csrf.FieldName("csrfmiddlewaretoken"),
			csrf.ErrorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Render.Error(w, r, errors.New("Forbidden")) // TODO: move this handler to render
			})),
		))

		r.Mount("/diary", Diary.ServeMux())
		r.Mount("/", Diary.ServeMux())
		r.Mount("/links", Links)
		r.Mount("/camino", CaminoPage)
		r.Mount("/map", Maps.ServeMux())
		r.Mount("/gallery", Gallery.ServeMux())
	})

	r.Mount("/static", http.StripPrefix("/static", http.FileServer(packr.NewBox("./static"))))

	//TODO: Move these handlers to render
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		Render.Template(w, r, "error.html", render.Context{
			"title": "Not Found",
			"msg":   "This is not the web page you are looking for.",
		})
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		Render.Template(w, r, "error.html", render.Context{
			"title": "Method Not Allowed",
			"msg":   "Your position's correct, except... not this method.",
		})
	})

	http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), r)
}
