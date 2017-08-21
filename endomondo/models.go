package endomondo

import (
	"encoding/json"
	"time"
)

type LoginResponse struct {
	ID               int       `json:"id"`
	CreatedDate      time.Time `json:"created_date"`
	Email            string    `json:"email"`
	FirstName        string    `json:"first_name"`
	LastName         string    `json:"last_name"`
	Premium          bool      `json:"premium"`
	HasAcceptedTerms bool      `json:"has_accepted_terms"`
	EmailVerified    bool      `json:"email_verified"`
	AccountStatus    int       `json:"account_status"`
	Locale           Locale    `json:"locale"`
}

type Locale struct {
	TimezoneOffset int     `json:"timezone_offset"`
	DatetimeFormat string  `json:"datetime_format"`
	Language       string  `json:"language"`
	Country        Country `json:"country"`
	Unit           string  `json:"unit"`
}

type Country struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SubscriptionsResponse struct {
	Data []SubscriptionEntry `json:"data"`
}

type SubscriptionEntry struct {
	ID           int       `json:"id"`
	Timestamp    time.Time `json:"timestamp"`
	Expand       string    `json:"expand"`
	Type         int       `json:"type"`
	ReadableType string    `json:"readable_type"`
	Author       User      `json:"author"`

	Friend  *User    `json:"friend"`
	Workout *Workout `json:"workout"`

	CanComment    bool              `json:"can_comment"`
	Comments      []json.RawMessage `json:"comments"`
	CommentsCount int               `json:"comments_count"`
	CanPeptalk    bool              `json:"can_peptalk"`
	Peptalks      []json.RawMessage `json:"peptalks"`
	PeptalksCount int               `json:"peptalks_count"`
	CanLike       bool              `json:"can_like"`
	LikeByMe      bool              `json:"like_by_me"`
	Likes         []json.RawMessage `json:"likes"`
	LikeCount     int               `json:"like_count"`
	Pictures      []json.RawMessage `json:"pictures"`
	TaggedWith    []json.RawMessage `json:"tagged_with"`
}

type Workout struct {
	ID                   int       `json:"id"`
	Expand               string    `json:"expand"`
	Sport                int       `json:"sport"`
	StartTime            time.Time `json:"start_time"`
	Distance             float64   `json:"distance"`
	Duration             float64   `json:"duration"`
	Calories             float64   `json:"calories"`
	IsLive               bool      `json:"is_live"`
	SmallEncodedPolyline string    `json:"small_encoded_polyline"`
	ShowMap              int       `json:"show_map"`
	ShowWorkout          int       `json:"show_workout"`
	MapKey               string    `json:"map_key"`
}

type User struct {
	ID               int     `json:"id"`
	Expand           string  `json:"expand"`
	Name             string  `json:"name"`
	FirstName        string  `json:"first_name"`
	LastName         string  `json:"last_name"`
	Picture          Picture `json:"picture"`
	Gender           int     `json:"gender"`
	IsPremium        bool    `json:"is_premium"`
	ViewerFriendship int     `json:"viewer_friendship"`
}

type Picture struct {
	URL string `json:"url"`
}

type WorkoutResponse struct {
	ID                   int               `json:"id"`
	Expand               string            `json:"expand"`
	Title                string            `json:"title"`
	Sport                int               `json:"sport"`
	StartTime            time.Time         `json:"start_time"`
	LocalStartTime       time.Time         `json:"local_start_time"`
	Distance             float64           `json:"distance"`
	Duration             float64           `json:"duration"`
	SpeedAvg             float64           `json:"speed_avg"`
	SpeedMax             float64           `json:"speed_max"`
	AltitudeMin          float64           `json:"altitude_min"`
	AltitudeMax          float64           `json:"altitude_max"`
	Ascent               float64           `json:"ascent"`
	Descent              float64           `json:"descent"`
	PbCount              float64           `json:"pb_count"`
	Calories             float64           `json:"calories"`
	IsLive               bool              `json:"is_live"`
	IncludeInStats       bool              `json:"include_in_stats"`
	Author               User              `json:"author"`
	IsPeptalkAllowed     bool              `json:"is_peptalk_allowed"`
	CanCopy              bool              `json:"can_copy"`
	Weather              json.RawMessage   `json:"weather"`
	FeedID               int               `json:"feed_id"`
	Laps                 json.RawMessage   `json:"laps"`
	SmallEncodedPolyline string            `json:"small_encoded_polyline"`
	Motivation           json.RawMessage   `json:"motivation"`
	Records              []json.RawMessage `json:"records"`
	Hashtags             []json.RawMessage `json:"hashtags"`
	TaggedUsers          []json.RawMessage `json:"tagged_users"`
	Pictures             []Picture         `json:"pictures"`
	Points               Points            `json:"points"`
	ShowMap              int               `json:"show_map"`
	ShowWorkout          int               `json:"show_workout"`
	AdminRejected        bool              `json:"admin_rejected"`
	Hydration            float64           `json:"hydration"`
	Route                Route             `json:"route"`
	PersonalBests        []json.RawMessage `json:"personal_bests"`
}

type Route struct {
	RouteID int    `json:"route_id"`
	Name    string `json:"string"`
}

type Points struct {
	ID     int     `json:"id"`
	Expand string  `json:"expand"`
	Points []Point `json:"points"`
}

type Point struct {
	Time        time.Time       `json:"time"`
	Instruction int             `json:"instruction"`
	Latitude    float64         `json:"latitude"`
	Longitude   float64         `json:"longitude"`
	Altitude    float64         `json:"altitude"`
	Distance    float64         `json:"distance"`
	Duration    float64         `json:"duration"`
	SensorData  json.RawMessage `json:"sensor_data"`
}
