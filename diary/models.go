package diary

import (
	"github.com/jinzhu/gorm"
	"github.com/matematik7/camino-go/maps"
	"github.com/matematik7/gongo/authorization"
)

type DiaryEntry struct {
	gorm.Model
	Title      string
	Text       string `gorm:"type:text"`
	Author     authorization.User
	AuthorID   uint
	Comments   []Comment
	MapEntry   maps.MapEntry
	MapEntryID uint

	NumComments uint `gorm:"-"`
	Viewed      bool `gorm:"-"`
	NewComments bool `gorm:"-"`
}

type Comment struct {
	gorm.Model
	DiaryEntryID uint
	Comment      string             `gorm:"type:text" valid:"required"`
	Author       authorization.User `valid:"-"`
	AuthorID     uint
}

type EntryUserRead struct {
	gorm.Model
	DiaryEntryID uint
	UserID       uint
}
