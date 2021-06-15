package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/evalphobia/logrus_sentry"
	"github.com/flosch/pongo2"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/csrf"
	"github.com/gorilla/sessions"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/matematik7/camino-go/diary"
	"github.com/matematik7/camino-go/gallery"
	"github.com/matematik7/camino-go/links"
	"github.com/matematik7/camino-go/maps"
	"github.com/matematik7/camino-go/pages"
	"github.com/matematik7/camino-go/stats"
	"github.com/matematik7/camino-go/strava"
	"github.com/matematik7/gongo"
	"github.com/matematik7/gongo/admin"
	"github.com/matematik7/gongo/authentication"
	"github.com/matematik7/gongo/authorization"
	"github.com/matematik7/gongo/files"
	"github.com/matematik7/gongo/files/storage/s3storage"
	"github.com/matematik7/gongo/render"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func main() {

	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("port", 8000)
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

	viper.SetDefault("images_bucket", "images-camino")
	viper.SetDefault("subdomain", "camino")

	// TODO: to something similar to goth auto init
	log := logrus.New()

	if viper.GetString("sentry_dsn") != "" {
		hook, err := logrus_sentry.NewSentryHook(viper.GetString("sentry_dsn"), []logrus.Level{
			logrus.PanicLevel,
			logrus.FatalLevel,
			logrus.ErrorLevel,
		})
		if err != nil {
			log.Fatal(err)
		}
		log.Hooks.Add(hook)
	}

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

	AwsSession, err := session.NewSession(
		aws.NewConfig().WithRegion("eu-central-1"),
	)
	if err != nil {
		log.Fatal(err)
	}
	Storage, err := s3storage.New(AwsSession, viper.GetString("images_bucket"), false)
	if err != nil {
		log.Fatal(err)
	}
	Files := files.New(Storage)

	Render.AddTemplates(packr.NewBox("./templates"))
	caminoBox := packr.NewBox("./templates-camino")
	hribiBox := packr.NewBox("./templates-hribi")
	dartsBox := packr.NewBox("./templates-darts")
	if viper.GetString("subdomain") == "camino" {
		Render.AddTemplates(caminoBox)
	} else if viper.GetString("subdomain") == "hribi" {
		Render.AddTemplates(hribiBox)
	} else if viper.GetString("subdomain") == "darts" {
		Render.AddTemplates(dartsBox)
	}

	Diary := diary.New()
	Links := links.New()
	CaminoPage := pages.New(1)
	Maps := maps.New()
	Gallery := gallery.New()
	Stats := stats.New()

	app := gongo.App{
		"Admin":          Admin,
		"Authentication": Authentication,
		"Authorization":  Authorization,
		"DB":             DB,
		"Files":          Files,
		"Log":            log,
		"Render":         Render,
		"Storage":        Storage,
		"Store":          store,

		"Diary":      Diary,
		"Links":      Links,
		"CaminoPage": CaminoPage,
		"Maps":       Maps,
		"Gallery":    Gallery,
		"Stats":      Stats,

		"Strava": strava.New(),
	}

	for _, itf := range app {
		if resourcer, ok := itf.(gongo.Resourcer); ok {
			if err := DB.AutoMigrate(resourcer.Resources()...).Error; err != nil {
				log.Fatal(errors.Wrap(err, "could not auto migrate models"))
			}
		}
	}

	if err := app.Configure(); err != nil {
		log.Fatalln(err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	// r.Use(NewStructuredLogger(log))
	r.Use(middleware.Recoverer) // TODO: proper error page
	r.Use(middleware.StripSlashes)
	r.Use(middleware.Compress(5))
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
			csrf.ErrorHandler(http.HandlerFunc(Render.Forbidden)),
		))

		r.Mount("/diary", Diary.ServeMux())
		r.Mount("/links", Links)
		r.Mount("/camino", CaminoPage)
		r.Mount("/map", Maps.ServeMux())
		r.Mount("/gallery", Gallery.ServeMux())
		r.Mount("/stats", Stats.ServeMux())
		r.Mount("/", Diary.ServeMux())
	})

	r.Mount("/static", http.StripPrefix("/static", http.FileServer(packr.NewBox("./static"))))

	r.NotFound(Render.NotFound)
	r.MethodNotAllowed(Render.MethodNotAllowed)

	serveAddr := fmt.Sprintf("%s:%d", host, port)
	log.Printf("Serving on %s", serveAddr)
	log.Fatal(http.ListenAndServe(serveAddr, r))
}

func NewStructuredLogger(logger *logrus.Logger) func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&StructuredLogger{logger})
}

type StructuredLogger struct {
	Logger *logrus.Logger
}

func (l *StructuredLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	entry := &StructuredLoggerEntry{Logger: logrus.NewEntry(l.Logger)}
	logFields := logrus.Fields{}

	logFields["TimeStamp"] = time.Now().UTC().Format(time.RFC1123)

	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		logFields["RequestID"] = reqID
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	logFields["HttpScheme"] = scheme
	logFields["HttpProtocol"] = r.Proto
	logFields["HttpMethod"] = r.Method

	logFields["RemoteAddr"] = r.RemoteAddr
	logFields["UserAgent"] = r.UserAgent()

	logFields["URL"] = fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	entry.Logger = entry.Logger.WithFields(logFields)

	entry.Logger.Infoln("request started")

	return entry
}

type StructuredLoggerEntry struct {
	Logger logrus.FieldLogger
}

func (l *StructuredLoggerEntry) Write(status, bytes int, elapsed time.Duration) {
	l.Logger = l.Logger.WithFields(logrus.Fields{
		"resp_status": status, "resp_bytes_length": bytes,
		"resp_elasped_ms": float64(elapsed.Nanoseconds()) / 1000000.0,
	})

	l.Logger.Infoln("request complete")
}

func (l *StructuredLoggerEntry) Panic(v interface{}, stack []byte) {
	l.Logger = l.Logger.WithFields(logrus.Fields{
		"stack": string(stack),
	})

	l.Logger.Error(fmt.Sprintf("%+v", v))
}

// Helper methods used by the application to get the request-scoped
// logger entry and set additional fields between handlers.
//
// This is a useful pattern to use to set state on the entry as it
// passes through the handler chain, which at any point can be logged
// with a call to .Print(), .Info(), etc.

func GetLogEntry(r *http.Request) logrus.FieldLogger {
	entry := middleware.GetLogEntry(r).(*StructuredLoggerEntry)
	return entry.Logger
}

func LogEntrySetField(r *http.Request, key string, value interface{}) {
	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*StructuredLoggerEntry); ok {
		entry.Logger = entry.Logger.WithField(key, value)
	}
}

func LogEntrySetFields(r *http.Request, fields map[string]interface{}) {
	if entry, ok := r.Context().Value(middleware.LogEntryCtxKey).(*StructuredLoggerEntry); ok {
		entry.Logger = entry.Logger.WithFields(fields)
	}
}
