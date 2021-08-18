package models

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"googlemaps.github.io/maps"

	"github.com/jinzhu/gorm"
	jsoniter "github.com/json-iterator/go"
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

		me.Lat = float64(dataEntries[len(dataEntries)-1].Latitude)
		me.Lon = float64(dataEntries[len(dataEntries)-1].Longitude)
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
	Latitude  Float     `json:"lat"`
	Longitude Float     `json:"lon"`
	Elevation Float     `json:"elevation"`
	Distance  Float     `json:"dist"`
}

// Old data in db sometimes has strings instead of floats, this accepts string when json unmarshaling
type Float float64

func (f *Float) UnmarshalJSON(data []byte) error {
	var flt float64
	err := json.Unmarshal(data, &flt)
	if err != nil {
		var str string
		err = json.Unmarshal(data, &str)
		if err != nil {
			return err
		}
		val, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return err
		}
		*f = Float(val)
		return nil
	}

	*f = Float(flt)
	return nil
}

func readFloat(value *jsoniter.Iterator) Float {
	switch value.WhatIsNext() {
	case jsoniter.StringValue:
		str := value.ReadString()
		val, err := strconv.ParseFloat(str, 64)
		if err != nil {
			value.ReportError("readFloat.parse", err.Error())
		}
		return Float(val)
	case jsoniter.NumberValue:
		return Float(value.ReadFloat64())
	default:
		value.ReportError("readFloat", "invalid next")
	}
	return Float(0)
}

func (g GpsData) OptimizedData() (string, error) {
	var entry DataEntry
	iter := jsoniter.ConfigFastest.BorrowIterator([]byte(g.Data))
	defer jsoniter.ConfigFastest.ReturnIterator(iter)

	stream := jsoniter.ConfigFastest.BorrowStream(nil)
	defer jsoniter.ConfigFastest.ReturnStream(stream)
	stream.WriteArrayStart()

	previousDistance := -1.0
	first := true
	iter.ReadArrayCB(func(item *jsoniter.Iterator) bool {
		item.ReadMapCB(func(value *jsoniter.Iterator, key string) bool {
			switch key {
			case "lat":
				entry.Latitude = readFloat(value)
			case "lon":
				entry.Longitude = readFloat(value)
			case "dist":
				entry.Distance = readFloat(value)
			case "elevation":
				entry.Elevation = readFloat(value)
			case "time.Time", "time":
				t, err := time.Parse(time.RFC3339, value.ReadString())
				if err != nil {
					value.ReportError("time.Parse", err.Error())
					return false
				}
				entry.Time = t
			default:
				value.Skip()
			}
			return true
		})

		if float64(entry.Distance)-previousDistance > 0.01 {
			previousDistance = float64(entry.Distance)
			if !first {
				stream.WriteMore()
			}
			stream.WriteVal(entry)
			first = false
		}
		return true
	})
	if err := iter.Error; err != nil {
		return "", err
	}

	stream.WriteArrayEnd()
	if err := stream.Error; err != nil {
		return "", err
	}

	return string(stream.Buffer()), nil
}
