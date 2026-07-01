package counselling

import (
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindByUserID(userID uint) ([]CounsellingBooking, error) {
	var bookings []CounsellingBooking
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&bookings).Error
	return bookings, err
}

type institutionBookingRow struct {
	ID                 uint
	CreatedAt          time.Time
	Status             string
	Notes              string
	StudentName        string
	StudentPhone       string
	StudentEmail       string
	ProgramLevel       string
	InterestedCourse   string
	SessionMode        string
	SessionScheduledAt time.Time
	InstitutionName    string
}

func (r *Repository) FindInstitutionBookingsByUserID(userID uint) ([]CounsellingBookingResponse, error) {
	var rows []institutionBookingRow
	err := r.db.Table("institution_counselling_bookings").
		Select(`institution_counselling_bookings.id,
			institution_counselling_bookings.created_at,
			institution_counselling_bookings.status,
			COALESCE(institution_counselling_bookings.notes, '') as notes,
			institution_counselling_bookings.student_name,
			institution_counselling_bookings.student_phone,
			institution_counselling_bookings.student_email,
			institution_counselling_bookings.program_level,
			institution_counselling_bookings.interested_course,
			institution_counselling_bookings.session_mode,
			institution_counselling_sessions.scheduled_at as session_scheduled_at,
			COALESCE(institution_users.institution_name, 'College') as institution_name`).
		Joins("JOIN institution_counselling_sessions ON institution_counselling_sessions.id = institution_counselling_bookings.session_id").
		Joins("JOIN institution_users ON institution_users.id = institution_counselling_sessions.institution_id").
		Where("institution_counselling_bookings.user_id = ?", userID).
		Order("institution_counselling_bookings.created_at DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	responses := make([]CounsellingBookingResponse, len(rows))
	for i, r := range rows {
		name := r.StudentName
		if name == "" {
			name = "Student"
		}
		responses[i] = CounsellingBookingResponse{
			ID:               r.ID,
			College:          r.InstitutionName,
			ProgramLevel:     r.ProgramLevel,
			InterestedCourse: r.InterestedCourse,
			SessionMode:      r.SessionMode,
			SessionDate:      r.SessionScheduledAt.Format("2006-01-02"),
			SessionTime:      r.SessionScheduledAt.Format("03:04 PM"),
			StudentName:      name,
			StudentPhone:     r.StudentPhone,
			StudentEmail:     r.StudentEmail,
			StudentNotes:     r.Notes,
			Status:           r.Status,
			CreatedAt:        r.CreatedAt.Format(time.RFC3339),
		}
	}
	return responses, nil
}

func (r *Repository) Create(booking *CounsellingBooking) error {
	return r.db.Create(booking).Error
}

func (r *Repository) CheckDuplicateBooking(userID uint, sessionDate, sessionTime string) bool {
	var count int64
	r.db.Model(&CounsellingBooking{}).Where(
		"user_id = ? AND session_date = ? AND session_time = ?",
		userID, sessionDate, sessionTime,
	).Count(&count)
	return count > 0
}
