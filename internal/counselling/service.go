package counselling

import (
	"context"
	"errors"
	"slices"
	"sort"

	"studsphere/backend/internal/notification"
)

type Service struct {
	repo     *Repository
	notifier notification.Notifier
}

func NewService(repo *Repository, notifier notification.Notifier) *Service {
	return &Service{repo: repo, notifier: notifier}
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

	// No counsellor role exists yet (doc 15 Q2) — platform bookings queue to
	// the superadmin audience.
	if s.notifier != nil {
		if audience, _ := s.notifier.ForRoles(context.Background(), "superadmin"); len(audience) > 0 {
			_ = s.notifier.Notify(context.Background(), notification.NotifyRequest{
				EventKey:   notification.EventCounsellingBookingCreated,
				Recipients: audience,
				Data:       map[string]any{"student_name": req.StudentName, "when": req.SessionDate + " " + req.SessionTime},
			})
		}
	}

	return booking, nil
}

func (s *Service) GetMyBookings(userID uint) ([]CounsellingBookingResponse, error) {
	regular, err := s.repo.FindByUserID(userID)
	if err != nil {
		return nil, err
	}

	institution, err := s.repo.FindInstitutionBookingsByUserID(userID)
	if err != nil {
		return nil, err
	}

	merged := make([]CounsellingBookingResponse, 0, len(regular)+len(institution))
	for i := range regular {
		merged = append(merged, toBookingResponse(&regular[i]))
	}
	merged = append(merged, institution...)

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].CreatedAt > merged[j].CreatedAt
	})

	merged = slices.CompactFunc(merged, func(a, b CounsellingBookingResponse) bool {
		return a.ID == b.ID && a.College == b.College && a.CreatedAt == b.CreatedAt
	})

	return merged, nil
}
