package links

import "github.com/jinzhu/gorm"

type Link struct {
	gorm.Model
	URL         string
	Title       string
	Description string `gorm:"type:text"`
}
