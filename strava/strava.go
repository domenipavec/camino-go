package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo"
	"github.com/matematik7/gongo/authorization"
	"github.com/pkg/errors"
)

var NoTokenError = errors.New("current user doesn't have strava access tokens")

type Service struct {
	DB *gorm.DB
}

func New() *Service {
	return &Service{}
}

func (s *Service) Configure(app gongo.App) error {
	s.DB = app["DB"].(*gorm.DB)
	return nil
}

func (s Service) Resources() []interface{} {
	return []interface{}{
		&StravaUserTokens{},
	}
}

func (s Service) request(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	url := fmt.Sprintf("https://www.strava.com/api/%v", path)
	return http.NewRequestWithContext(ctx, method, url, body)
}

func (s Service) refreshToken(ctx context.Context, userTokens StravaUserTokens) error {
	form := url.Values{}
	form.Add("client_id", "62161")
	form.Add("client_secret", "c07e278cd4ca90dca169e66606b5ed942c8db334")
	form.Add("grant_type", "refresh_token")
	form.Add("refresh_token", userTokens.RefreshToken)

	req, err := s.request(ctx, "POST", "v3/oauth/token", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return errors.Errorf("strava error: %v", resp.Status)
	}

	tokens := Tokens{}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&tokens)
	if err != nil {
		return err
	}

	userTokens.AccessToken = tokens.AccessToken
	userTokens.RefreshToken = tokens.RefreshToken

	err = s.DB.Save(&userTokens).Error
	if err != nil {
		return errors.Wrap(err, "user token db write")
	}

	return nil
}

func (s Service) call(ctx context.Context, path string, response interface{}) error {
	req, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return err
	}

	userId := ctx.Value("user").(authorization.User).ID
	var userTokens StravaUserTokens
	query := s.DB.First(&userTokens, "user_id = ?", userId)
	if query.RecordNotFound() {
		return NoTokenError
	} else if query.Error != nil {
		return query.Error
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %v", userTokens.AccessToken))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		err := s.refreshToken(ctx, userTokens)
		if err != nil {
			return errors.Wrap(err, "refresh token")
		} else {
			return s.call(ctx, path, response)
		}
	} else if resp.StatusCode/100 != 2 {
		return errors.Errorf("strava error: %v", resp.Status)
	}

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(response)
	if err != nil {
		return err
	}

	return nil
}

func (s Service) RecentActivities(ctx context.Context) ([]Activity, error) {
	var response []Activity
	err := s.call(ctx, "v3/athlete/activities", &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (s Service) Activity(ctx context.Context, id int) (Activity, error) {
	var response Activity
	err := s.call(ctx, fmt.Sprintf("v3/activities/%v", id), &response)
	if err != nil {
		return response, err
	}
	return response, nil
}

type Point struct {
	TimeOffset int
	Latitude   float64
	Longitude  float64
	Altitude   float64
	Distance   float64
}

func (s Service) ActivityPoints(ctx context.Context, id int) ([]Point, error) {
	var streams []Stream
	err := s.call(ctx, fmt.Sprintf("v3/activities/%v/streams?keys=time,distance,latlng,altitude", id), &streams)
	if err != nil {
		return nil, err
	}

	var points []Point
	for _, stream := range streams {
		if stream.Resolution != "high" {
			return nil, errors.New("expected high resolution stream")
		}
		if len(points) != stream.OriginalSize {
			if len(points) == 0 {
				points = make([]Point, stream.OriginalSize)
			} else {
				return nil, errors.Errorf("expected original size %v, got %v", len(points), stream.OriginalSize)
			}
		}

		switch stream.Type {
		case "time":
			var data []int
			err := json.Unmarshal(stream.Data, &data)
			if err != nil {
				return nil, err
			}
			if len(data) != stream.OriginalSize {
				return nil, errors.New("data count does not match stream original size")
			}
			for i := range data {
				points[i].TimeOffset = data[i]
			}
		case "distance":
			var data []float64
			err := json.Unmarshal(stream.Data, &data)
			if err != nil {
				return nil, err
			}
			if len(data) != stream.OriginalSize {
				return nil, errors.New("data count does not match stream original size")
			}
			for i := range data {
				points[i].Distance = data[i]
			}
		case "latlng":
			var data [][2]float64
			err := json.Unmarshal(stream.Data, &data)
			if err != nil {
				return nil, err
			}
			if len(data) != stream.OriginalSize {
				return nil, errors.New("data count does not match stream original size")
			}
			for i := range data {
				points[i].Latitude = data[i][0]
				points[i].Longitude = data[i][1]
			}
		case "altitude":
			var data []float64
			err := json.Unmarshal(stream.Data, &data)
			if err != nil {
				return nil, err
			}
			if len(data) != stream.OriginalSize {
				return nil, errors.New("data count does not match stream original size")
			}
			for i := range data {
				points[i].Altitude = data[i]
			}
		}
	}

	return points, nil
}
