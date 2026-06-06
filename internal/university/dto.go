package university

type CreateUniversityRequest struct {
	Name           string      `json:"name" binding:"required"`
	Logo           string      `json:"logo"`
	Location       string      `json:"location"`
	Type           string      `json:"type"`
	Rank           int         `json:"rank"`
	Rating         float64     `json:"rating"`
	ReviewCount    int         `json:"review_count"`
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
	Rank           *int        `json:"rank"`
	Rating         *float64    `json:"rating"`
	ReviewCount    *int        `json:"review_count"`
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
	About           []byte   `json:"about,omitempty"`
	Contact         []byte   `json:"contact,omitempty"`
	Quick           []byte   `json:"quick,omitempty"`
	Overview        []byte   `json:"overview,omitempty"`
	Leadership      []byte   `json:"leadership,omitempty"`
	Courses         []byte   `json:"courses,omitempty"`
	Programs        []byte   `json:"programs,omitempty"`
	Scholarships    []byte   `json:"scholarships,omitempty"`
	Events          []byte   `json:"events,omitempty"`
	News            []byte   `json:"news,omitempty"`
	Downloads       []byte   `json:"downloads,omitempty"`
	Gallery         []byte   `json:"gallery,omitempty"`
	Faculties       []byte   `json:"faculties,omitempty"`
	Admissions      []byte   `json:"admissions,omitempty"`
	Reviews         []byte   `json:"reviews,omitempty"`
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
