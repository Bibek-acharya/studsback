package university

import "encoding/json"

type CreateUniversityRequest struct {
	Name           string      `json:"name" binding:"required"`
	Logo           string      `json:"logo"`
	Location       string      `json:"location"`
	Type           string      `json:"type"`
	IsNepali       bool        `json:"is_nepali"`
	Rank           int         `json:"rank"`
	Rating         float64     `json:"rating"`
	ReviewCount    int         `json:"review_count"`
	ProgramsCount  int         `json:"programsCount"`
	CollegesCount  int         `json:"collegesCount"`
	Verified       bool        `json:"verified"`
	Popular        bool        `json:"popular"`
	Status         string      `json:"status"`
	Description    string      `json:"description"`
	Established    string      `json:"established"`
	Students       string      `json:"students"`
	Chancellor     string      `json:"chancellor"`
	ViceChancellor string      `json:"vice_chancellor"`
	Founder        string      `json:"founder"`
	Website        string      `json:"website"`
	Cover          string      `json:"cover"`
	About          interface{} `json:"about"`
	Contact        interface{} `json:"contact"`
	Quick          interface{} `json:"quick"`
	Overview       interface{} `json:"overview"`
	Leadership     interface{} `json:"leadership"`
	Courses        interface{} `json:"courses"`
	Programs       interface{} `json:"programs"`
	Scholarships   interface{} `json:"scholarships"`
	Events         interface{} `json:"events"`
	News           interface{} `json:"news"`
	Downloads      interface{} `json:"downloads"`
	Gallery        interface{} `json:"gallery"`
	Faculties      interface{} `json:"faculties"`
	Admissions     interface{} `json:"admissions"`
	Reviews        interface{} `json:"reviews"`
}

type UpdateUniversityRequest struct {
	Name           *string     `json:"name"`
	Logo           *string     `json:"logo"`
	Location       *string     `json:"location"`
	Type           *string     `json:"type"`
	IsNepali       *bool       `json:"is_nepali"`
	Rank           *int        `json:"rank"`
	Rating         *float64    `json:"rating"`
	ReviewCount    *int        `json:"review_count"`
	ProgramsCount  *int        `json:"programsCount"`
	CollegesCount  *int        `json:"collegesCount"`
	Verified       *bool       `json:"verified"`
	Popular        *bool       `json:"popular"`
	Status         *string     `json:"status"`
	Description    *string     `json:"description"`
	Established    *string     `json:"established"`
	Students       *string     `json:"students"`
	Chancellor     *string     `json:"chancellor"`
	ViceChancellor *string     `json:"vice_chancellor"`
	Founder        *string     `json:"founder"`
	Website        *string     `json:"website"`
	Cover          *string     `json:"cover"`
	About          interface{} `json:"about"`
	Contact        interface{} `json:"contact"`
	Quick          interface{} `json:"quick"`
	Overview       interface{} `json:"overview"`
	Leadership     interface{} `json:"leadership"`
	Courses        interface{} `json:"courses"`
	Programs       interface{} `json:"programs"`
	Scholarships   interface{} `json:"scholarships"`
	Events         interface{} `json:"events"`
	News           interface{} `json:"news"`
	Downloads      interface{} `json:"downloads"`
	Gallery        interface{} `json:"gallery"`
	Faculties      interface{} `json:"faculties"`
	Admissions     interface{} `json:"admissions"`
	Reviews        interface{} `json:"reviews"`
}

type UniversityResponse struct {
	ID              uint     `json:"id"`
	Name            string   `json:"name"`
	Logo            string   `json:"logo"`
	Location        string   `json:"location"`
	Rating          float64  `json:"rating"`
	ReviewCount     int      `json:"review_count"`
	Type            string   `json:"type"`
	IsNepali        bool     `json:"is_nepali"`
	Rank            int      `json:"rank"`
	Verified        bool     `json:"verified"`
	IsPopular       bool     `json:"isPopular"`
	Status          string   `json:"status"`
	ProgramsCount   int      `json:"programsCount"`
	CollegesCount   int      `json:"collegesCount"`
	PopularPrograms []string `json:"popularPrograms"`
	Description     string   `json:"description,omitempty"`
	Established     string   `json:"established,omitempty"`
	Students        string   `json:"students,omitempty"`
	Chancellor      string   `json:"chancellor"`
	ViceChancellor  string   `json:"vice_chancellor"`
	Founder         string   `json:"founder"`
	Website         string   `json:"website,omitempty"`
	Cover           string   `json:"cover"`
	About           json.RawMessage `json:"about,omitempty"`
	Contact         json.RawMessage `json:"contact,omitempty"`
	Quick           json.RawMessage `json:"quick,omitempty"`
	Overview        json.RawMessage `json:"overview,omitempty"`
	Leadership      json.RawMessage `json:"leadership,omitempty"`
	Courses         json.RawMessage `json:"courses,omitempty"`
	Programs        json.RawMessage `json:"programs,omitempty"`
	Scholarships    json.RawMessage `json:"scholarships,omitempty"`
	Events          json.RawMessage `json:"events,omitempty"`
	News            json.RawMessage `json:"news,omitempty"`
	Downloads       json.RawMessage `json:"downloads,omitempty"`
	Gallery         json.RawMessage `json:"gallery,omitempty"`
	Faculties       json.RawMessage `json:"faculties,omitempty"`
	Admissions      json.RawMessage `json:"admissions,omitempty"`
	Reviews         json.RawMessage `json:"reviews,omitempty"`
}

type UniversityCollegeResponse struct {
	ID           uint    `json:"id"`
	UniversityID uint    `json:"universityId"`
	Name         string  `json:"name"`
	Logo         string  `json:"logo"`
	Rating       float64 `json:"rating"`
	Reviews      int     `json:"reviews"`
	Affiliation  string  `json:"affiliation"`
	Type         string  `json:"type"`
}

type UniversityTabResponse struct {
	Tab  string `json:"tab"`
	Data []byte `json:"data,omitempty"`
}

type UniversityFilterCountsResponse struct {
	Total          int64            `json:"total"`
	TypeCounts     map[string]int64 `json:"type_counts"`
	TypeCountsByID map[string]int64 `json:"type_counts_by_id"`
	RatingCounts   map[string]int64 `json:"rating_counts"`
	AcademicCounts map[string]int64 `json:"academic_counts"`
}
