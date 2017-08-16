package models

import (
	"time"

	"github.com/jinzhu/gorm"
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
