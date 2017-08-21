package endomondo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"

	"golang.org/x/net/publicsuffix"

	"github.com/pkg/errors"
)

const (
	BASE_URL          = "https://www.endomondo.com/"
	LOGIN_URL         = "rest/session"
	SUBSCRIPTIONS_URL = "rest/v1/feeds/subscriptions?limit=20"
	WORKOUT_URL       = "rest/v1/users/%d/workouts/%d"
)

type Client struct {
	client *http.Client
}

func New(email, password string) (*Client, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, errors.Wrap(err, "could not init cookiejar")
	}

	c := &Client{
		client: &http.Client{
			Jar: jar,
		},
	}

	if err := c.Login(email, password); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) Login(email, password string) error {
	response := LoginResponse{}
	err := c.send("POST", LOGIN_URL, map[string]interface{}{
		"email":    email,
		"password": password,
		"remember": true,
	}, &response)
	if err != nil {
		return errors.Wrap(err, "could not login")
	}

	return nil
}

func (c *Client) Subscriptions() (*SubscriptionsResponse, error) {
	response := &SubscriptionsResponse{}
	err := c.send("GET", SUBSCRIPTIONS_URL, nil, response)
	if err != nil {
		return nil, errors.Wrap(err, "could not get subscriptions")
	}

	return response, nil
}

func (c *Client) Workout(userID, workoutID int) (*WorkoutResponse, error) {
	response := &WorkoutResponse{}
	err := c.send("GET", fmt.Sprintf(WORKOUT_URL, userID, workoutID), nil, response)
	if err != nil {
		return nil, errors.Wrap(err, "could not get workout")
	}

	return response, nil
}

func (c *Client) send(method, url string, body, output interface{}) error {
	fullURL := BASE_URL + url

	if method != "GET" {
		// TODO generate csrf token
		// https://github.com/fabulator/endomondo-api-base/blob/master/lib/Fabulator/Endomondo/EndomondoAPIBase.php#L87
	}

	csrfToken := "-"

	var bodyReader io.Reader
	if body != nil {
		buffer := &bytes.Buffer{}
		encoder := json.NewEncoder(buffer)
		err := encoder.Encode(body)
		if err != nil {
			return errors.Wrap(err, "could not json encode body")
		}
		bodyReader = buffer
	}

	r, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return errors.Wrap(err, "could not prepare request")
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Cookie", "CSRF_TOKEN="+csrfToken)
	r.Header.Add("X-CSRF-TOKEN", csrfToken)

	response, err := c.client.Do(r)
	if err != nil {
		return errors.Wrap(err, "could not do request")
	}
	defer response.Body.Close()

	if output != nil {
		decoder := json.NewDecoder(response.Body)
		err = decoder.Decode(output)
		if err != nil {
			return errors.Wrap(err, "could not decode response")
		}
	}

	return nil
}
