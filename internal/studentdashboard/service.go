package studentdashboard

import (
	"errors"
	"time"

	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/shared/logger"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func parseTime(s string) (time.Time, error) {
	formats := []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, nil
}

func (s *Service) GetMessages(userID uint, page, limit int) ([]Message, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	return s.repo.GetMessages(userID, page, limit)
}

func (s *Service) GetMessageByID(msgID, userID uint) (*Message, error) {
	message, err := s.repo.GetMessageByID(msgID, userID)
	if err != nil {
		return nil, err
	}

	if !message.Read && message.ReceiverID == userID {
		if err := s.repo.MarkMessageRead(message.ID); err != nil {
			return nil, err
		}
		message.Read = true
	}

	return message, nil
}

func (s *Service) CreateMessage(userID, receiverID uint, subject, content string) (*Message, error) {
	message := &Message{
		SenderID:   userID,
		ReceiverID: receiverID,
		Subject:    subject,
		Content:    content,
		Direction:  "sent",
	}

	if err := s.repo.CreateMessage(message); err != nil {
		return nil, err
	}

	go s.repo.CreateInstitutionMessage(userID, receiverID, subject, content)

	return message, nil
}

func (s *Service) ReplyToMessage(msgID, userID uint, content string) (*Message, error) {
	original, err := s.repo.GetMessageForReply(msgID)
	if err != nil {
		return nil, err
	}

	receiverID := original.SenderID
	if original.SenderID == userID {
		receiverID = original.ReceiverID
	}

	reply := &Message{
		SenderID:   userID,
		ReceiverID: receiverID,
		Subject:    "Re: " + original.Subject,
		Content:    content,
		Direction:  "sent",
	}

	if err := s.repo.CreateMessage(reply); err != nil {
		return nil, err
	}

	return reply, nil
}

func (s *Service) GetMessageContacts(userID uint) ([]Contact, error) {
	return s.repo.GetContacts(userID)
}

func (s *Service) GetCalendarEvents(userID uint) ([]CalendarEvent, error) {
	return s.repo.GetCalendarEvents(userID)
}

func (s *Service) GetCalendarEventByID(eventID, userID uint) (*CalendarEvent, error) {
	return s.repo.GetCalendarEventByID(eventID, userID)
}

func (s *Service) CreateCalendarEvent(userID uint, req CalendarEventRequest) (*CalendarEvent, error) {
	startDate, _ := parseTime(req.StartDate)
	endDate, _ := parseTime(req.EndDate)

	event := &CalendarEvent{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		StartDate:   startDate,
		EndDate:     endDate,
		Location:    req.Location,
		Link:        req.Link,
		Color:       req.Color,
		Reminder:    req.Reminder,
		Type:        req.Type,
	}

	if err := s.repo.CreateCalendarEvent(event); err != nil {
		return nil, err
	}

	return event, nil
}

func (s *Service) UpdateCalendarEvent(eventID, userID uint, req CalendarEventUpdateRequest) (*CalendarEvent, error) {
	event, err := s.repo.GetCalendarEventByID(eventID, userID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.StartDate != "" {
		if t, _ := parseTime(req.StartDate); !t.IsZero() {
			updates["start_date"] = t
		}
	}
	if req.EndDate != "" {
		if t, _ := parseTime(req.EndDate); !t.IsZero() {
			updates["end_date"] = t
		}
	}
	if req.Location != "" {
		updates["location"] = req.Location
	}
	if req.Link != "" {
		updates["link"] = req.Link
	}
	if req.Color != "" {
		updates["color"] = req.Color
	}
	if req.Reminder != nil {
		updates["reminder"] = *req.Reminder
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}

	if err := s.repo.UpdateCalendarEvent(event, updates); err != nil {
		return nil, err
	}

	return event, nil
}

func (s *Service) DeleteCalendarEvent(eventID, userID uint) error {
	return s.repo.DeleteCalendarEvent(eventID, userID)
}

func (s *Service) GetInvites(userID uint, page, limit int) ([]SphereInvite, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}

	return s.repo.GetInvites(userID, page, limit)
}

func (s *Service) GetInviteByID(inviteID, userID uint) (*SphereInvite, error) {
	return s.repo.GetInviteByID(inviteID, userID)
}

func (s *Service) AcceptInvite(inviteID, userID uint) (*SphereInvite, error) {
	invite, err := s.repo.GetInviteByID(inviteID, userID)
	if err != nil {
		return nil, err
	}

	if invite.Status != "pending" {
		return nil, errors.New("invite already processed")
	}

	if err := s.repo.UpdateInviteStatus(invite, "accepted"); err != nil {
		return nil, err
	}

	return invite, nil
}

func (s *Service) DeclineInvite(inviteID, userID uint) (*SphereInvite, error) {
	invite, err := s.repo.GetInviteByID(inviteID, userID)
	if err != nil {
		return nil, err
	}

	if invite.Status != "pending" {
		return nil, errors.New("invite already processed")
	}

	if err := s.repo.UpdateInviteStatus(invite, "declined"); err != nil {
		return nil, err
	}

	return invite, nil
}

func (s *Service) SaveInvite(inviteID, userID uint) (*SphereInvite, error) {
	invite, err := s.repo.GetInviteByID(inviteID, userID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateInviteStatus(invite, "saved"); err != nil {
		return nil, err
	}

	return invite, nil
}

func (s *Service) GetBookmarks(userID uint, page, limit int) ([]Bookmark, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	return s.repo.GetBookmarks(userID, page, limit)
}

func (s *Service) CreateBookmark(userID uint, req BookmarkRequest) (*Bookmark, error) {
	if s.repo.BookmarkExists(userID, req.ItemID, req.ItemType) {
		return nil, errors.New("already bookmarked")
	}

	bookmark := &Bookmark{
		UserID:   userID,
		ItemID:   req.ItemID,
		ItemType: req.ItemType,
	}

	if err := s.repo.CreateBookmark(bookmark); err != nil {
		return nil, err
	}

	return bookmark, nil
}

func (s *Service) DeleteBookmark(bookmarkID, userID uint) error {
	return s.repo.DeleteBookmark(bookmarkID, userID)
}

func (s *Service) GetBookmarksByType(userID uint, itemType string) ([]Bookmark, error) {
	return s.repo.GetBookmarksByType(userID, itemType)
}

func (s *Service) GetNotifications(userID uint, page, limit int) ([]Notification, int64, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	return s.repo.GetNotifications(userID, page, limit)
}

func (s *Service) MarkNotificationRead(notifID, userID uint) error {
	_, err := s.repo.GetNotificationByID(notifID, userID)
	if err != nil {
		return err
	}

	return s.repo.MarkNotificationRead(notifID)
}

func (s *Service) MarkAllNotificationsRead(userID uint) error {
	return s.repo.MarkAllNotificationsRead(userID)
}

func (s *Service) CreateNotification(userID uint, title, message, notifType, link string) {
	notification := &Notification{
		UserID:  userID,
		Title:   title,
		Message: message,
		Type:    notifType,
		Link:    link,
		Read:    false,
	}
	if err := s.repo.CreateNotification(notification); err != nil {
		logger.Warn("Failed to create notification", "error", err)
	}
}

func (s *Service) GetDashboardStats(userID uint) (*DashboardStats, error) {
	applicationsSubmitted, err := s.repo.CountAdmissions(userID)
	if err != nil {
		return nil, err
	}

	savedColleges, err := s.repo.CountBookmarksByType(userID, "college")
	if err != nil {
		return nil, err
	}

	savedScholarships, err := s.repo.CountBookmarksByType(userID, "scholarship")
	if err != nil {
		return nil, err
	}

	scholarshipsApplied, err := s.repo.CountScholarshipApplications(userID)
	if err != nil {
		return nil, err
	}

	activeInvites, err := s.repo.CountActiveInvites(userID)
	if err != nil {
		return nil, err
	}

	unreadMessages, err := s.repo.CountUnreadMessages(userID)
	if err != nil {
		return nil, err
	}

	upcomingDeadlines, err := s.repo.CountUpcomingEvents(userID)
	if err != nil {
		return nil, err
	}

	profileCompletion := 0
	user, err := s.repo.GetUserByID(userID)
	if err == nil && user != nil {
		educationCount, _ := s.repo.CountEducationEntries(userID)
		profileCompletion = computeProfileCompletion(user, educationCount)
	}

	return &DashboardStats{
		ApplicationsSubmitted: int(applicationsSubmitted),
		SavedColleges:         int(savedColleges),
		SavedScholarships:     int(savedScholarships),
		ScholarshipsApplied:   int(scholarshipsApplied),
		ActiveInvites:         int(activeInvites),
		UnreadMessages:        int(unreadMessages),
		UpcomingDeadlines:     int(upcomingDeadlines),
		ProfileCompletion:     profileCompletion,
	}, nil
}

func computeProfileCompletion(user *auth.User, educationCount int64) int {
	score := 0
	totalChecks := 12

	if user.FirstName != "" {
		score++
	}
	if user.LastName != "" {
		score++
	}
	if user.Phone != "" {
		score++
	}
	if user.DateOfBirth != "" {
		score++
	}
	if user.Gender != "" {
		score++
	}
	if user.Nationality != "" {
		score++
	}
	if user.Address != "" {
		score++
	}
	if user.Bio != "" {
		score++
	}
	if user.Preferences != nil && user.Preferences.OnboardingCompleted {
		score++
	}
	if user.ImageURL != "" {
		score++
	}
	if educationCount > 0 {
		score++
	}
	if user.Email != "" {
		score++
	}

	percent := (score * 100) / totalChecks
	if percent > 100 {
		percent = 100
	}
	return percent
}

func (s *Service) GetRecentApplications(userID uint) ([]RecentApplication, error) {
	admissions, err := s.repo.GetRecentAdmissions(userID, 5)
	if err != nil {
		return nil, err
	}

	apps := make([]RecentApplication, len(admissions))
	for i, a := range admissions {
		institution := ""
		if a.College.Name != "" {
			institution = a.College.Name
		}
		apps[i] = RecentApplication{
			ID:          a.ID,
			Institution: institution,
			Program:     a.ProgramName,
			Type:        a.ProgramLevel,
			Status:      a.Status,
			UpdatedAt:   a.UpdatedAt.Format(time.RFC3339),
		}
	}
	return apps, nil
}

func (s *Service) GetMyApplications(userID uint, page, limit int) ([]MyApplication, int64, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}

	admissions, total, err := s.repo.GetUserAdmissions(userID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	apps := make([]MyApplication, len(admissions))
	for i, a := range admissions {
		institution := ""
		if a.College.Name != "" {
			institution = a.College.Name
		}
		apps[i] = MyApplication{
			ID:          a.ID,
			Institution: institution,
			Program:     a.ProgramName,
			Type:        a.ProgramLevel,
			Status:      a.Status,
			AppliedDate: a.CreatedAt.Format(time.RFC3339),
		}
	}
	return apps, total, nil
}
