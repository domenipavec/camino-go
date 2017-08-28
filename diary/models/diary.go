package models

import (
	"log"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/matematik7/gongo/authorization"
	"github.com/matematik7/gongo/files"
)

var location *time.Location

func init() {
	var err error
	location, err = time.LoadLocation("Europe/Ljubljana")
	if err != nil {
		log.Fatal(err)
	}
}

type DiaryEntry struct {
	gorm.Model
	Title      string             `valid:"required"`
	Text       string             `gorm:"type:text" valid:"required"`
	Author     authorization.User `valid:"-"`
	AuthorID   uint               `valid:"required"`
	Comments   []Comment          `valid:"-"`
	MapEntry   MapEntry           `valid:"-"`
	MapEntryID uint
	Images     []files.Image `gorm:"many2many:diary_image"`
	Published  bool

	NumComments uint `gorm:"-"`
	Viewed      bool `gorm:"-"`
	NewComments bool `gorm:"-"`
}

// TODO: handle timezone nicer
func (de *DiaryEntry) AfterFind() error {
	de.CreatedAt = de.CreatedAt.In(location)
	de.UpdatedAt = de.UpdatedAt.In(location)

	return nil
}

type Comment struct {
	gorm.Model
	DiaryEntryID uint
	Comment      string             `gorm:"type:text" valid:"required"`
	Author       authorization.User `valid:"-"`
	AuthorID     uint
}

func (c *Comment) AfterFind() error {
	c.CreatedAt = c.CreatedAt.In(location)

	return nil
}

type EntryUserRead struct {
	gorm.Model
	DiaryEntryID uint
	UserID       uint
}

type Workout struct {
	ID          string
	Description string
}
