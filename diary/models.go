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
	MapEntry   maps.MapEntry
	MapEntryID uint
}
