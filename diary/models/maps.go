package models

import (
	"context"
	"time"

	"googlemaps.github.io/maps"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type MapEntry struct {
	gorm.Model
	City        string
	Description string
	MapGroup    MapGroup
	MapGroupID  uint
	Lon         float64
	Lat         float64
	GpsData     GpsData
	GpsDataID   uint

	DiaryEntry *DiaryEntry
}

func (me *MapEntry) BeforeSave() error {
	if me.City != "" {
		c, err := maps.NewClient(maps.WithAPIKey(viper.GetString("GMAP_SERVER_KEY")))
		if err != nil {
			return errors.Wrap(err, "could not get maps client")
		}

		result, err := c.Geocode(context.Background(), &maps.GeocodingRequest{
			Address: me.City,
		})
		if err != nil {
			return errors.Wrap(err, "could not get geocode result")
		}

		if len(result) < 1 {
			return errors.Wrap(err, "no results for geocode")
		}

		me.Lat = result[0].Geometry.Location.Lat
		me.Lon = result[0].Geometry.Location.Lng
	}

	return nil
}

type MapGroup struct {
	gorm.Model
	Name  string
	Color string

	Entries []MapEntry
}

type GpsData struct {
	gorm.Model
	Start  string
	End    string
	Date   time.Time
	Length float64
	Data   string `gorm:"type:text"`
	MapURL string
}
