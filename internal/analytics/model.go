package analytics

import "time"

// PageVisit records a public website page view. Rows are inserted by the
// public POST /api/v1/track/visit endpoint; created_at drives all
// day/week bucketing for the superadmin analytics pages endpoint.
type PageVisit struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Path      string    `gorm:"index;size:512;not null" json:"path"`
	Referrer  string    `gorm:"default:''" json:"referrer"`
	UserID    *uint     `gorm:"index" json:"user_id,omitempty"`
}

func (PageVisit) TableName() string { return "page_visits" }
