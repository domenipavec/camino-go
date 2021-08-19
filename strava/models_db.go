package strava

import (
	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo/authorization"
)

type StravaUserTokens struct {
	gorm.Model

	User   *authorization.User
	UserID uint

	AccessToken  string
	RefreshToken string
}
