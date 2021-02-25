package models

import (
	"context"
	"encoding/json"
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
	if len(me.GpsData.Data) > 0 && me.City == "" {
		dataEntries := []DataEntry{}
		err := json.Unmarshal([]byte(me.GpsData.Data), &dataEntries)
		if err != nil {
			return errors.Wrap(err, "could not unmarshal gps data")
		}

		me.Lat = dataEntries[len(dataEntries)-1].Latitude
		me.Lon = dataEntries[len(dataEntries)-1].Longitude
		me.City = me.GpsData.End
	}
	if me.City != "" && me.City != me.GpsData.End {
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
	Start     string
	End       string
	Date      time.Time
	Length    float64
	Duration  float64
	AvgSpeed  float64
	WorkoutID string `gorm:"column:endomondo_id"`
	Data      string `gorm:"type:text"`
	MapURL    string
}

type DataEntry struct {
	Time      time.Time `json:"time.Time"`
	Latitude  float64   `json:"lat"`
	Longitude float64   `json:"lon"`
	Elevation float64   `json:"elevation"`
	Distance  float64   `json:"dist"`
}
