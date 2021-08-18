package diary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	mailgun "gopkg.in/mailgun/mailgun-go.v1"

	"googlemaps.github.io/maps"

	"github.com/asaskevich/govalidator"
	"github.com/flosch/pongo2"
	"github.com/go-chi/chi"
	"github.com/gobuffalo/packr"
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo"
	"github.com/matematik7/gongo/authorization"
	"github.com/matematik7/gongo/files"
	"github.com/matematik7/gongo/render"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/twpayne/go-polyline"

	"github.com/matematik7/camino-go/diary/models"
	"github.com/matematik7/camino-go/strava"
)

const PerPage = 10

type Diary struct {
	DB     *gorm.DB
	render *render.Render
	files  *files.Files
	maps   *maps.Client
	log    *logrus.Logger
	mg     mailgun.Mailgun
	strava *strava.Service
}

func New() *Diary {
	return &Diary{}
}

func (c *Diary) Configure(app gongo.App) error {
	c.DB = app["DB"].(*gorm.DB)
	c.render = app["Render"].(*render.Render)
	c.files = app["Files"].(*files.Files)
	c.log = app["Log"].(*logrus.Logger)
	c.strava = app["Strava"].(*strava.Service)

	c.mg = mailgun.NewMailgun("ipavec.net", viper.GetString("mailgun_apikey"), viper.GetString("mailgun_publicapikey"))

	c.render.AddTemplates(packr.NewBox("./templates"))

	c.render.AddContextFunc(func(r *http.Request, ctx render.Context) {
		// TODO: add this as helper to authorization
		userID := -1
		if r.Context().Value("user") != nil {
			userID = int(r.Context().Value("user").(authorization.User).ID)
		}
		var years []int
		// TODO: add error handling
		c.DB.Model(&models.DiaryEntry{}).
			Select("DISTINCT date_part('year', created_at) as year").
			Where("published = ? or author_id = ?", true, userID).
			Order("year desc").
			Pluck("year", &years)
		ctx["diaryYears"] = years
	})

	client, err := maps.NewClient(maps.WithAPIKey(viper.GetString("GMAP_SERVER_KEY")))
	if err != nil {
		return errors.Wrap(err, "could not get maps client")
	}
	c.maps = client

	pongo2.RegisterFilter("durationformat", func(in *pongo2.Value, param *pongo2.Value) (out *pongo2.Value, err *pongo2.Error) {
		output := ""
		duration := in.Integer()
		if duration > 3600 {
			output += fmt.Sprintf("%d ur", duration/3600)
			duration %= 3600
		}
		if duration > 60 {
			if output != "" {
				output += " "
			}
			output += fmt.Sprintf("%d minut", duration/60)
		}

		return pongo2.AsValue(output), nil
	})

	app["Authorization"].(*authorization.Authorization).OnNewUser.Add(func(ctx context.Context) error {
		return c.markAllRead(ctx.Value("DB").(*gorm.DB), ctx.Value("user").(authorization.User).ID)
	})

	return nil
}

func (c Diary) Resources() []interface{} {
	return []interface{}{
		&models.DiaryEntry{},
		&models.Comment{},
		&models.EntryUserRead{},
	}
}

// TODO: can we simplify this can functions
func (c Diary) CanEdit(entry models.DiaryEntry, userItf interface{}) bool {
	if userItf == nil {
		return false
	}
	user := userItf.(authorization.User)
	if user.HasPermissions("create_diary_entries") && entry.AuthorID == user.ID {
		return true
	}
	return user.HasPermissions("update_diary_entries")
}

func (c Diary) CanSeeUnpublished(userItf interface{}) bool {
	if userItf == nil {
		return false
	}
	user := userItf.(authorization.User)
	return user.HasPermissions("update_diary_entries")
}

func (c Diary) CanCreate(userItf interface{}) bool {
	if userItf == nil {
		return false
	}
	user := userItf.(authorization.User)
	return user.HasPermissions("create_diary_entries")
}

func (c *Diary) getCity(latitude, longitude float64) (string, error) {
	result, err := c.maps.Geocode(context.Background(), &maps.GeocodingRequest{
		LatLng: &maps.LatLng{
			Lat: latitude,
			Lng: longitude,
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "could not get geocode result")
	}

	if len(result) < 1 {
		return "", errors.Wrap(err, "no results for geocode")
	}

	for _, ac := range result[0].AddressComponents {
		for _, typ := range ac.Types {
			if typ == "locality" {
				return ac.LongName, nil
			}
		}
	}
	return result[0].FormattedAddress, nil
}

func (c *Diary) markAllRead(DB *gorm.DB, userID uint) error {
	if err := DB.Model(&models.EntryUserRead{}).Where("user_id = ?", userID).Update("updated_at", "NOW()").Error; err != nil {
		return errors.Wrap(err, "could not update entry user read entries")
	}

	query := DB.Exec(
		`INSERT INTO entry_user_reads
		(created_at, updated_at, diary_entry_id, user_id)
		(
			SELECT NOW(), NOW(), id, ?
			FROM diary_entries
			LEFT JOIN (
				SELECT 1 as already_exists, diary_entry_id
				FROM entry_user_reads
				WHERE user_id = ?
			) ae ON ae.diary_entry_id = diary_entries.id
			WHERE ae.already_exists IS DISTINCT FROM 1
		)`,
		userID,
		userID,
	)
	if query.Error != nil {
		return errors.Wrap(query.Error, "could not create new entry user read entries")
	}

	return nil
}

func (c *Diary) ServeMux() http.Handler {
	router := chi.NewRouter()

	router.Get("/", c.ListHandler)

	router.Get("/read", c.ReadHandler)

	router.Get("/subscribe", c.SubscribeHandler)
	router.Post("/subscribe", c.SubscribeHandler)

	router.Get("/new", c.EditHandler)
	router.Post("/new", c.EditHandler)

	router.Route("/{diaryID:[0-9]+}", func(r chi.Router) {
		r.Get("/", c.ViewHandler)
		r.Post("/comment", c.CommentHandler)

		r.Get("/edit", c.EditHandler)
		r.Post("/edit", c.EditHandler)

		r.Get("/publish", c.PublishHandler)

		r.Route("/pictures", func(r chi.Router) {
			r.Get("/", c.PicturesHandler)
			r.Post("/", c.AddPictureHandler)

			r.Route("/{imageID:[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}}", func(r chi.Router) {
				r.Delete("/", c.DeletePictureHandler)
				r.Get("/delete", c.DeletePictureHandler)
			})
		})
	})

	return router
}

func (c *Diary) PublishHandler(w http.ResponseWriter, r *http.Request) {
	diaryEntry := models.DiaryEntry{}

	id, err := strconv.Atoi(chi.URLParam(r, "diaryID"))
	if err != nil {
		c.render.NotFound(w, r)
		return
	}

	query := c.DB.Preload("Author").First(&diaryEntry, id)
	if query.RecordNotFound() {
		c.render.NotFound(w, r)
		return
	} else if query.Error != nil {
		c.render.Error(w, r, query.Error)
		return
	}

	if !c.CanEdit(diaryEntry, r.Context().Value("user")) {
		c.render.Forbidden(w, r)
		return
	}

	now := time.Now()
	updates := models.DiaryEntry{
		Model: gorm.Model{
			CreatedAt: now,
			UpdatedAt: now,
		},
		Published: true,
	}

	if err := c.DB.Model(&diaryEntry).UpdateColumns(updates).Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	txt := `Živjo,
%s je na %s spletni strani objavil: %s
Preberi več: https://%s.ipavec.net/diary/%d

Lep pozdrav

Odjava od prejemanja teh sporočil:
%%mailing_list_unsubscribe_url%%`
	subdomain := viper.GetString("subdomain")

	txt = fmt.Sprintf(txt, diaryEntry.Author.DisplayName(), subdomain, diaryEntry.Title, subdomain, diaryEntry.ID)

	msg := c.mg.NewMessage(
		fmt.Sprintf("%s@ipavec.net", subdomain),
		fmt.Sprintf("Nova objava na %s.ipavec.net", subdomain),
		txt,
		fmt.Sprintf("%s-subscribers@ipavec.net", subdomain),
	)
	if _, _, err := c.mg.Send(msg); err != nil {
		c.render.Error(w, r, err)
		return
	}

	if err := c.render.AddFlash(w, r, FlashInfo("Vnos objavljen!")); err != nil {
		c.render.Error(w, r, errors.Wrap(err, "could not set flash"))
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/diary/%d", diaryEntry.ID), http.StatusFound)
}

func (c *Diary) SubscribeHandler(w http.ResponseWriter, r *http.Request) {
	subpage := "Naroči se"

	if r.Method == "POST" {
		email := r.FormValue("email")
		if !govalidator.IsEmail(email) {
			c.render.AddFlash(w, r, FlashError("Neveljaven email naslov!"))
		} else {
			mailingList := fmt.Sprintf("%s-subscribers@ipavec.net", viper.GetString("subdomain"))
			err := c.mg.CreateMember(true, mailingList, mailgun.Member{
				Address:    email,
				Subscribed: mailgun.Subscribed,
			})
			if err != nil {
				c.log.Error(err.Error())
				c.render.AddFlash(w, r, FlashError("Nekaj je šlo narobe, poskusite znova!"))
			} else {
				c.render.AddFlash(w, r, FlashInfo("Uspešno ste naročeni!"))
				http.Redirect(w, r, "/diary", http.StatusFound)
			}
		}
	}

	context := render.Context{
		"subpage": subpage,
	}

	c.render.Template(w, r, "diary_subscribe.html", context)
}

func reverseEntries(entries []models.DataEntry) {
	startTime := entries[0].Time
	for i := len(entries)/2 - 1; i >= 0; i-- {
		opp := len(entries) - 1 - i
		entries[i], entries[opp] = entries[opp], entries[i]
	}
	endTime := entries[0].Time
	totalDistance := entries[0].Distance
	for i := range entries {
		entries[i].Time = startTime.Add(endTime.Sub(entries[i].Time))
		entries[i].Distance = totalDistance - entries[i].Distance
	}
}

func (c *Diary) EditHandler(w http.ResponseWriter, r *http.Request) {
	entryID := chi.URLParam(r, "diaryID")
	diaryEntry := models.DiaryEntry{}
	subpage := "Nov vnos"

	if entryID != "" {
		id, err := strconv.Atoi(chi.URLParam(r, "diaryID"))
		if err != nil {
			c.render.NotFound(w, r)
			return
		}

		query := c.DB.Preload("MapEntry.GpsData").First(&diaryEntry, id)
		if !query.RecordNotFound() && query.Error != nil {
			c.render.Error(w, r, query.Error)
			return
		}
		subpage = "Urejanje"

		if !c.CanEdit(diaryEntry, r.Context().Value("user")) {
			c.render.Forbidden(w, r)
			return
		}
	} else {
		if !c.CanCreate(r.Context().Value("user")) {
			c.render.Forbidden(w, r)
			return
		}
	}

	if r.Method == "POST" {
		// TODO: can we use some kind of apply for this (like gobuffalo)
		diaryEntry.Title = r.FormValue("title")
		diaryEntry.Text = r.FormValue("content")
		diaryEntry.MapEntry.City = r.FormValue("city")

		if entryID == "" {
			diaryEntry.AuthorID = r.Context().Value("user").(authorization.User).ID
		}

		workout := r.FormValue("workout")

		if (diaryEntry.MapEntry.City != "" || workout != "") && diaryEntry.MapEntry.MapGroupID == 0 {
			mapGroupIDs := []uint{}
			if err := c.DB.Model(&models.MapGroup{}).Order("id desc").Limit(1).Pluck("id", &mapGroupIDs).Error; err != nil {
				c.render.Error(w, r, err)
				return
			}
			diaryEntry.MapEntry.MapGroupID = mapGroupIDs[0]
		}

		if workout != "" && workout != diaryEntry.MapEntry.GpsData.WorkoutID {
			activityID, err := strconv.Atoi(workout)
			if err != nil {
				c.render.Error(w, r, err)
				return
			}

			activity, err := c.strava.Activity(r.Context(), activityID)
			if err != nil {
				c.render.Error(w, r, err)
				return
			}

			points, err := c.strava.ActivityPoints(r.Context(), activityID)
			if err != nil {
				c.render.Error(w, r, err)
				return
			}

			dataEntries := make([]models.DataEntry, 0, len(points))
			for _, point := range points {
				if point.Latitude == 0 || point.Longitude == 0 {
					continue
				}
				dataEntries = append(dataEntries, models.DataEntry{
					Time:      activity.StartDate.Add(time.Second * time.Duration(point.TimeOffset)),
					Latitude:  models.Float(point.Latitude),
					Longitude: models.Float(point.Longitude),
					Elevation: models.Float(point.Altitude),
					Distance:  models.Float(point.Distance / 1000),
				})
			}
			// Reverse for biking (going down)
			if activity.Type == "Ride" {
				reverseEntries(dataEntries)
			}
			dataJSON, err := json.Marshal(dataEntries)
			if err != nil {
				c.render.Error(w, r, err)
				return
			}

			inc := 1
			mapURL := ""
			for {
				coords := [][]float64{}
				for i := 0; i < len(dataEntries); i += inc {
					coords = append(coords, []float64{
						float64(dataEntries[i].Latitude),
						float64(dataEntries[i].Longitude),
					})
				}

				mapURL = url.QueryEscape(string(polyline.EncodeCoords(coords)))
				if len(mapURL) <= 1800 {
					break
				}

				inc *= 2
			}

			start, err := c.getCity(
				float64(dataEntries[0].Latitude),
				float64(dataEntries[0].Longitude),
			)
			if err != nil {
				c.render.Error(w, r, err)
				return
			}
			end, err := c.getCity(
				float64(dataEntries[len(dataEntries)-1].Latitude),
				float64(dataEntries[len(dataEntries)-1].Longitude),
			)
			if err != nil {
				c.render.Error(w, r, err)
				return
			}

			diaryEntry.MapEntry.GpsData.Start = start
			diaryEntry.MapEntry.GpsData.End = end
			diaryEntry.MapEntry.GpsData.Date = activity.StartDate
			diaryEntry.MapEntry.GpsData.Length = activity.Distance / 1000
			diaryEntry.MapEntry.GpsData.Duration = float64(activity.ElapsedTime)
			diaryEntry.MapEntry.GpsData.AvgSpeed = activity.AverageSpeed * 3.6
			diaryEntry.MapEntry.GpsData.WorkoutID = workout
			diaryEntry.MapEntry.GpsData.Data = string(dataJSON)
			diaryEntry.MapEntry.GpsData.MapURL = mapURL
		}

		if err := c.DB.Save(&diaryEntry).Error; err != nil {
			if err := c.render.AddFlash(w, r, FlashError(err.Error())); err != nil {
				c.render.Error(w, r, err)
				return
			}
		} else {
			if err := c.render.AddFlash(w, r, FlashInfo("Vnos shranjen!")); err != nil {
				c.render.Error(w, r, err)
				return
			}

			http.Redirect(w, r, fmt.Sprintf("/diary/%d", diaryEntry.ID), http.StatusFound)
			return
		}
	}

	activities, err := c.strava.RecentActivities(r.Context())
	if err != nil && err != strava.NoTokenError {
		c.render.Error(w, r, err)
		return
	}

	workouts := []models.Workout{}
	for _, activity := range activities {
		workouts = append(workouts, models.Workout{
			ID: strconv.Itoa(activity.ID),
			Description: fmt.Sprintf("%s: %s %.1f km v %.1f urah",
				activity.Name,
				activity.StartDate.Format("2. 1. 2006"),
				activity.Distance/1000,
				float64(activity.ElapsedTime)/3600,
			),
		})
	}

	context := render.Context{
		"entry":       diaryEntry,
		"subpage":     subpage,
		"workouts":    workouts,
		"browser_key": viper.GetString("GMAP_BROWSER_KEY"),
	}

	c.render.Template(w, r, "diary_edit.html", context)
}

func (c *Diary) DeletePictureHandler(w http.ResponseWriter, r *http.Request) {
	diaryEntry := models.DiaryEntry{}
	diaryID, err := strconv.Atoi(chi.URLParam(r, "diaryID"))
	if err != nil {
		c.render.NotFound(w, r)
		return
	}
	query := c.DB.First(&diaryEntry, diaryID)
	if query.RecordNotFound() {
		c.render.NotFound(w, r)
		return
	} else if query.Error != nil {
		c.render.Error(w, r, query.Error)
		return
	}

	if !c.CanEdit(diaryEntry, r.Context().Value("user")) {
		c.render.Forbidden(w, r)
		return
	}

	id, err := uuid.FromString(chi.URLParam(r, "imageID"))
	if err != nil {
		c.render.Error(w, r, err)
		return
	}

	image := files.Image{
		File: files.File{
			ID: id,
		},
	}

	if err := c.DB.Model(&diaryEntry).Association("Images").Delete(&image).Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	if err := c.files.Delete(image); err != nil {
		c.render.Error(w, r, err)
		return
	}

	if err := c.render.AddFlash(w, r, FlashInfo("Slike izbrisana!")); err != nil {
		c.render.Error(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/diary/%d/pictures", diaryEntry.ID), http.StatusFound)
}

func (c *Diary) AddPictureHandler(w http.ResponseWriter, r *http.Request) {
	diaryEntry := models.DiaryEntry{}
	id, err := strconv.Atoi(chi.URLParam(r, "diaryID"))
	if err != nil {
		c.render.NotFound(w, r)
		return
	}
	query := c.DB.First(&diaryEntry, id)
	if query.RecordNotFound() {
		c.render.NotFound(w, r)
		return
	} else if query.Error != nil {
		c.render.Error(w, r, query.Error)
		return
	}

	if !c.CanEdit(diaryEntry, r.Context().Value("user")) {
		c.render.Forbidden(w, r)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 20*1024*1024)
	r.ParseMultipartForm(20 * 1024 * 1024)
	defer r.Body.Close()

	file, handler, err := r.FormFile("userfile")
	if err != nil {
		// TODO: use flashes for these errors that are form related
		c.render.Error(w, r, err)
		return
	}
	defer file.Close()

	img, err := c.files.NewImage(file, handler.Filename, r.FormValue("description"))
	if err != nil {
		c.render.Error(w, r, err)
		return
	}

	if err := c.DB.Model(&diaryEntry).Association("Images").Append(img).Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	if err := c.render.AddFlash(w, r, FlashInfo("Slika dodana!")); err != nil {
		c.render.Error(w, r, err)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/diary/%d/pictures", diaryEntry.ID), http.StatusFound)
}

func (c *Diary) PicturesHandler(w http.ResponseWriter, r *http.Request) {
	diaryEntry := models.DiaryEntry{}

	id, err := strconv.Atoi(chi.URLParam(r, "diaryID"))
	if err != nil {
		c.render.NotFound(w, r)
		return
	}

	query := c.DB.
		Preload("Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("images.created_at")
		}).
		First(&diaryEntry, id)
	if query.RecordNotFound() {
		c.render.NotFound(w, r)
		return
	} else if query.Error != nil {
		c.render.Error(w, r, query.Error)
		return
	}

	if !c.CanEdit(diaryEntry, r.Context().Value("user")) {
		c.render.Forbidden(w, r)
		return
	}

	context := render.Context{
		"subpage": "Slike",
		"entry":   diaryEntry,
	}

	c.render.Template(w, r, "diary_pictures.html", context)
}

func (c *Diary) CommentHandler(w http.ResponseWriter, r *http.Request) {
	//TODO: move this get user to authorization to perform interface is nil validation and have proper context key
	userItf := r.Context().Value("user")
	if userItf == nil {
		c.render.Forbidden(w, r)
		return
	}
	user := userItf.(authorization.User)

	diaryEntry := models.DiaryEntry{}
	id, err := strconv.Atoi(chi.URLParam(r, "diaryID"))
	if err != nil {
		c.render.NotFound(w, r)
		return
	}
	query := c.DB.First(&diaryEntry, id)
	if query.RecordNotFound() {
		c.render.NotFound(w, r)
		return
	} else if query.Error != nil {
		c.render.Error(w, r, query.Error)
		return
	}

	comment := models.Comment{
		DiaryEntryID: diaryEntry.ID,
		AuthorID:     user.ID,
		Comment:      r.FormValue("comment"),
	}

	err = c.DB.Save(&comment).Error
	if err != nil {
		// TODO: figure better way for handling validation errors for bigger forms
		if _, ok := err.(govalidator.Errors); ok {
			if err := c.render.AddFlash(w, r, FlashError("Vpisati morate vaš komentar!")); err != nil {
				c.render.Error(w, r, err)
				return
			}
			http.Redirect(w, r, r.Referer(), http.StatusFound)
		}
		c.render.Error(w, r, err)
		return
	}

	if err := c.render.AddFlash(w, r, FlashInfo("Komentar objavljen!")); err != nil {
		c.render.Error(w, r, err)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusFound)
}

func (c *Diary) ReadHandler(w http.ResponseWriter, r *http.Request) {
	userItf := r.Context().Value("user")
	if userItf == nil {
		c.render.Forbidden(w, r)
		return
	}
	user := userItf.(authorization.User)

	if err := c.markAllRead(c.DB, user.ID); err != nil {
		c.render.Error(w, r, err)
		return
	}

	if err := c.render.AddFlash(w, r, FlashInfo("Vsi vnosi označeni kot prebrani!")); err != nil {
		c.render.Error(w, r, err)
		return
	}

	http.Redirect(w, r, r.Referer(), http.StatusFound)
}

func (c *Diary) ViewHandler(w http.ResponseWriter, r *http.Request) {
	var entry models.DiaryEntry

	id, err := strconv.Atoi(chi.URLParam(r, "diaryID"))
	if err != nil {
		c.render.NotFound(w, r)
		return
	}

	query := c.DB.Preload("Author").LogMode(true).
		Preload("Comments", func(db *gorm.DB) *gorm.DB {
			return db.Order("comments.created_at desc")
		}).
		Preload("Comments.Author").
		Preload("MapEntry.GpsData").
		Preload("Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("images.created_at")
		}).
		First(&entry, id)

	if query.RecordNotFound() {
		c.render.NotFound(w, r)
		return
	} else if err := query.Error; err != nil {
		c.render.Error(w, r, errors.Wrap(err, "could not get diary entry"))
		return
	}

	var totalDistance []float64
	query = c.DB.Model(&models.DiaryEntry{}).
		Select("COALESCE(SUM(gd1.length), 0) as total_distance").
		Where("date_part('year', diary_entries.created_at) = ?", entry.CreatedAt.Year()).
		Where("diary_entries.created_at <= ?", entry.CreatedAt).
		Joins("LEFT JOIN map_entries me1 ON diary_entries.map_entry_id = me1.id").
		Joins("LEFT JOIN gps_data gd1 ON me1.gps_data_id = gd1.id").
		Pluck("total_distance", &totalDistance)

	if err := query.Error; err != nil {
		c.render.Error(w, r, errors.Wrap(err, "could not get total distance"))
		return
	}

	if len(totalDistance) != 1 {
		c.render.Error(w, r, errors.Errorf("invalid total distance len %d", len(totalDistance)))
		return
	}

	// Mark as read if logged in
	if user := r.Context().Value("user"); user != nil {
		user := user.(authorization.User)

		entryUserRead := models.EntryUserRead{
			DiaryEntryID: entry.ID,
			UserID:       user.ID,
		}

		query := c.DB.First(&entryUserRead, entryUserRead)
		if !query.RecordNotFound() && query.Error != nil {
			c.render.Error(w, r, query.Error)
			return
		}

		if err := c.DB.Save(&entryUserRead).Error; err != nil {
			c.render.Error(w, r, err)
			return
		}
	}

	mapCenter := struct {
		Lat float64
		Lon float64
	}{}
	c.DB.Raw("SELECT AVG(lat) as lat, AVG(lon) as lon FROM map_entries").Scan(&mapCenter)

	context := render.Context{
		"entry":          entry,
		"browser_key":    viper.GetString("GMAP_BROWSER_KEY"),
		"total_distance": totalDistance[0],
		"map_center":     mapCenter,
		"CanEdit":        c.CanEdit,
	}

	c.render.Template(w, r, "diary_one.html", context)
}

func (c *Diary) ListHandler(w http.ResponseWriter, r *http.Request) {
	yearStr := r.URL.Query().Get("year")

	userID := -1
	if r.Context().Value("user") != nil {
		userID = int(r.Context().Value("user").(authorization.User).ID)
	}

	// show latest year on main page
	if r.URL.Path == "/" {
		var year []int
		query := c.DB.Model(&models.DiaryEntry{}).
			Select("DISTINCT date_part('year', created_at) as year").
			Where("published = ? or author_id = ?", true, userID).
			Order("year desc").
			Limit(1).
			Pluck("year", &year)
		if query.Error != nil {
			c.render.Error(w, r, query.Error)
			return
		}
		if len(year) > 0 {
			yearStr = strconv.Itoa(year[0])
		}
	}

	query := c.DB.Model(&models.DiaryEntry{}).
		Select("*").
		Preload("Author").
		Preload("Images", func(db *gorm.DB) *gorm.DB {
			return db.Order("diary_entry_id, RANDOM()").Select("distinct on (diary_entry_id) *")
		}).
		Order("created_at desc")

	if !c.CanSeeUnpublished(r.Context().Value("user")) {
		query = query.Where("published = ? or author_id = ?", true, userID)
	}

	var yearItf interface{}
	if yearStr != "" {
		year, err := strconv.Atoi(yearStr)
		if err != nil {
			c.render.Error(w, r, errors.Wrap(err, "invalid year"))
			return
		}
		query = query.Where("date_part('year', created_at) = ?", year)
		yearItf = year
	}

	// paging
	var count int
	if err := query.Count(&count).Error; err != nil {
		c.render.Error(w, r, err)
		return
	}
	pages := make([]int, (count+PerPage-1)/PerPage)
	for i := range pages {
		pages[i] = PerPage * i
	}

	query = query.Joins(
		`natural left join (
			SELECT diary_entry_id as id, count(*) as num_comments
			FROM comments
			WHERE comments.deleted_at IS NULL
			GROUP BY diary_entry_id
		) c`,
	)

	// get new status for user if logged in
	if user := r.Context().Value("user"); user != nil {
		user := user.(authorization.User)

		query = query.Joins(
			`natural left join (
				SELECT
					entry_user_reads.diary_entry_id as id,
					true as viewed,
					rc.created_at > entry_user_reads.updated_at as new_comments
				FROM entry_user_reads
				LEFT JOIN (
					SELECT diary_entry_id, MAX(created_at) as created_at
					FROM comments
					WHERE comments.deleted_at IS NULL
					GROUP BY diary_entry_id
				) rc ON rc.diary_entry_id = entry_user_reads.diary_entry_id
				WHERE user_id = ?
			) r`,
			user.ID,
		)
	} else {
		query = query.Select("*, true as viewed")
	}

	offset := 0
	if r.URL.Query().Get("offset") != "" {
		var err error
		offset, err = strconv.Atoi(r.URL.Query().Get("offset"))
		if err != nil {
			c.render.Error(w, r, errors.Wrap(err, "invalid offset"))
			return
		}
		query = query.Offset(offset)
	}
	nextOffset := offset + PerPage
	prevOffset := offset - PerPage

	var entries []models.DiaryEntry
	if err := query.Limit(PerPage).Find(&entries).Error; err != nil {
		c.render.Error(w, r, err)
		return
	}

	hasUnread := false
	for _, entry := range entries {
		if !entry.Viewed || entry.NewComments {
			hasUnread = true
		}
	}

	context := render.Context{
		"multiView":  true,
		"entries":    entries,
		"hasUnread":  hasUnread,
		"offset":     offset,
		"pages":      pages,
		"prevOffset": prevOffset,
		"nextOffset": nextOffset,
		"year":       yearItf,
	}

	c.render.Template(w, r, "diary_all.html", context)
}
