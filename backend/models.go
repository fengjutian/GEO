package main

import "time"

type Site struct {
	ID uint `json:"id" gorm:"primaryKey"`
	Name string `json:"name" gorm:"size:160;not null"`
	URL string `json:"url" gorm:"size:2048;not null"`
	Description string `json:"description" gorm:"type:text"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type Scan struct {
	ID uint `json:"id" gorm:"primaryKey"`
	SiteID uint `json:"siteId" gorm:"index;not null"`
	Status string `json:"status" gorm:"size:32"`
	StartedAt time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	PageCount int `json:"pageCount"`
	Score int `json:"score"`
	Summary string `json:"summary" gorm:"type:text"`
	CreatedAt time.Time `json:"createdAt"`
}
type Finding struct {
	ID uint `json:"id" gorm:"primaryKey"`
	SiteID uint `json:"siteId" gorm:"index;not null"`
	ScanID uint `json:"scanId" gorm:"index;not null"`
	Severity string `json:"severity" gorm:"size:16;index"`
	RuleCode string `json:"ruleCode" gorm:"size:80"`
	Title string `json:"title" gorm:"size:255"`
	URL string `json:"url" gorm:"size:2048"`
	Evidence string `json:"evidence" gorm:"type:text"`
	Recommendation string `json:"recommendation" gorm:"type:text"`
	Status string `json:"status" gorm:"size:32;default:open"`
	CreatedAt time.Time `json:"createdAt"`
}
