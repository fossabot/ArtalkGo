package model

import (
	"strings"

	"github.com/ArtalkJS/ArtalkGo/lib"
	"gorm.io/gorm"
)

type Site struct {
	gorm.Model
	Name string `gorm:"uniqueIndex"`
	Urls string

	Users string // 分配给指定 UserIDs
}

func (s Site) IsEmpty() bool {
	return s.ID == 0
}

type CookedSite struct {
	ID       uint     `json:"id"`
	Name     string   `json:"name"`
	Urls     []string `json:"urls"`
	UrlsRaw  string   `json:"urls_raw"`
	FirstUrl string   `json:"first_url"`
}

func (s Site) ToCooked() CookedSite {
	splitUrls := lib.RemoveBlankStrings(strings.Split(s.Urls, ","))
	firstUrl := ""
	if len(splitUrls) > 0 {
		firstUrl = splitUrls[0]
	}

	return CookedSite{
		ID:       s.ID,
		Name:     s.Name,
		Urls:     splitUrls,
		UrlsRaw:  s.Urls,
		FirstUrl: firstUrl,
	}
}
