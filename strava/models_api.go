package strava

import (
	"encoding/json"
	"time"
)

type Activity struct {
	ID                  int       `json:"id"`
	Name                string    `json:"name"`
	Distance            float64   `json:"distance"`
	MovingTime          int       `json:"moving_time"`
	ElapsedTime         int       `json:"elapsed_time"`
	TotalEleveationGain float64   `json:"total_elevation_gain"`
	Type                string    `json:"Walk"`
	StartDate           time.Time `json:"start_date"`
	AverageSpeed        float64   `json:"average_speed"`
}

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type Stream struct {
	Type         string `json:"type"`
	SeriesType   string `json:"series_type"`
	Resolution   string `json:"resolution"`
	OriginalSize int    `json:"original_size"`
	Data         json.RawMessage
}
