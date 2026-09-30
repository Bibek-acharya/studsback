package jobs

import (
	"time"

	"gorm.io/gorm"
)

// Job is a posting on the platform's own careers page. There is deliberately no
// tenant column here: `/careers` is a first-party surface, every posting on it
// belongs to the platform, and nothing in this module creates a job on behalf of
// an institution or a scholarship provider. See access.go, which depends on
// that being true.
type Job struct {
	ID                  uint           `gorm:"primarykey" json:"id"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"-"`
	Title               string         `gorm:"type:varchar(255);not null" json:"title"`
	Department          string         `gorm:"type:varchar(100);not null" json:"department"`
	Description         string         `gorm:"type:text;not null" json:"description"`
	Requirements        string         `gorm:"type:text" json:"requirements"`
	Location            string         `gorm:"type:varchar(255)" json:"location"`
	JobType             string         `gorm:"type:varchar(50);not null" json:"job_type"`
	PositionsOpen       int            `gorm:"not null;default:1" json:"positions_open"`
	SalaryRange         string         `gorm:"type:varchar(100)" json:"salary_range"`
	ApplicationDeadline *time.Time     `json:"application_deadline,omitempty"`
	Status              string         `gorm:"type:varchar(20);not null;default:'draft'" json:"status"`
}

type JobApplication struct {
	ID             uint           `gorm:"primarykey" json:"id"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
	JobID          uint           `gorm:"index;not null" json:"job_id"`
	FullName       string         `gorm:"type:varchar(255);not null" json:"full_name"`
	Email          string         `gorm:"type:varchar(255);not null" json:"email"`
	Phone          string         `gorm:"type:varchar(50);not null" json:"phone"`
	ResumeURL      string         `gorm:"type:varchar(500);not null" json:"resume_url"`
	CoverLetterURL string         `gorm:"type:varchar(500)" json:"cover_letter_url"`
	Status         string         `gorm:"type:varchar(20);not null;default:'pending'" json:"status"`
	Notes          string         `gorm:"type:text" json:"notes"`

	// ApplicantUserID is the authenticated account that submitted this
	// application, recorded when the submitter presented a valid token to the
	// public apply form. It is the only thing that makes an application
	// attributable to a person: Email is a contact field that a guest types
	// into a form, so it cannot carry ownership.
	//
	// Nullable and unbackfilled on purpose. Applications submitted while signed
	// out have no account to point at, and those stay admin-only rather than
	// being guessed at by matching Email against the users table — matching on
	// email would let anyone who knows an applicant's address claim their
	// application by submitting a second one with that address.
	//
	// json:"-" because this is an authorization key, not applicant data.
	ApplicantUserID *uint `gorm:"index" json:"-"`

	Job Job `gorm:"foreignKey:JobID" json:"job,omitempty"`
}

// SubmittedBy returns the account that submitted this application, or 0 when it
// was submitted by a guest. 0 is never a real users.id, so it can be compared
// against an authenticated viewer's id directly.
func (a *JobApplication) SubmittedBy() uint {
	if a.ApplicantUserID == nil {
		return 0
	}
	return *a.ApplicantUserID
}

func (Job) TableName() string {
	return "jobs"
}

func (JobApplication) TableName() string {
	return "job_applications"
}
