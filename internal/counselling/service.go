package counselling

import (
	"errors"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateBooking(userID uint, req CreateCounsellingBookingRequest) (*CounsellingBooking, error) {
	if req.SessionMode != "online" && req.SessionMode != "in_person" {
		return nil, errors.New("session_mode must be either 'online' or 'in_person'")
	}

	if s.repo.CheckDuplicateBooking(userID, req.SessionDate, req.SessionTime) {
		return nil, errors.New("you already booked this date and time slot")
	}

	booking := &CounsellingBooking{
		UserID:           userID,
		College:          req.College,
		ProgramLevel:     req.ProgramLevel,
		InterestedCourse: req.InterestedCourse,
		SessionMode:      req.SessionMode,
		SessionDate:      req.SessionDate,
		SessionTime:      req.SessionTime,
		StudentName:      req.StudentName,
		StudentPhone:     req.StudentPhone,
		StudentEmail:     req.StudentEmail,
		StudentNotes:     req.StudentNotes,
		Status:           "pending",
	}

	if err := s.repo.Create(booking); err != nil {
		return nil, errors.New("failed to create counselling booking")
	}

	return booking, nil
}

func (s *Service) GetMyBookings(userID uint) ([]CounsellingBooking, error) {
	return s.repo.FindByUserID(userID)
}
