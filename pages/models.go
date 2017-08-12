package pages

import "github.com/jinzhu/gorm"

type Page struct {
	gorm.Model
	Title   string
	Content string `gorm:"type:text"`
}
