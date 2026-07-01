package studentdashboard

import (
	"fmt"
	"studsphere/backend/internal/admission"
	"studsphere/backend/internal/auth"
	"studsphere/backend/internal/scholarship"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetMessages(userID uint, page, limit int) ([]Message, int64, error) {
	var total int64
	if err := r.db.Model(&Message{}).Where("sender_id = ? OR receiver_id = ?", userID, userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var messages []Message
	offset := (page - 1) * limit
	if err := r.db.Where("sender_id = ? OR receiver_id = ?", userID, userID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&messages).Error; err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

func (r *Repository) GetMessageByID(msgID uint, userID uint) (*Message, error) {
	var message Message
	if err := r.db.Where("id = ? AND (sender_id = ? OR receiver_id = ?)", msgID, userID, userID).
		First(&message).Error; err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *Repository) CreateMessage(message *Message) error {
	return r.db.Create(message).Error
}

func (r *Repository) CreateInstitutionMessage(userID, receiverID uint, subject, content string) error {
	var instID uint
	r.db.Table("institution_users").Where("id = ?", receiverID).Select("id").Take(&instID)
	if instID == 0 {
		return nil
	}
	return r.db.Table("institution_messages").Create(map[string]interface{}{
		"institution_id": instID,
		"user_id":        userID,
		"subject":        subject,
		"content":        content,
		"direction":      "inbound",
	}).Error
}

func (r *Repository) GetMessageForReply(msgID uint) (*Message, error) {
	var message Message
	if err := r.db.Where("id = ?", msgID).First(&message).Error; err != nil {
		return nil, err
	}
	return &message, nil
}

func (r *Repository) GetContacts(userID uint) ([]Contact, error) {
	var messages []Message
	if err := r.db.Where("sender_id = ? OR receiver_id = ?", userID, userID).
		Order("created_at desc").Find(&messages).Error; err != nil {
		return nil, err
	}

	contactMap := map[uint]*Contact{}
	for _, msg := range messages {
		var contactID uint
		if msg.SenderID == userID {
			contactID = msg.ReceiverID
		} else {
			contactID = msg.SenderID
		}

		if _, exists := contactMap[contactID]; !exists {
			var user struct {
				FirstName string
				LastName  string
			}
			r.db.Model(&User{}).Where("id = ?", contactID).First(&user)
			name := user.FirstName + " " + user.LastName
			if name == " " {
				var instUser struct {
					InstitutionName string
				}
				r.db.Table("institution_users").Where("id = ?", contactID).First(&instUser)
				name = instUser.InstitutionName
				if name == "" {
					name = fmt.Sprintf("User #%d", contactID)
				}
			}
			contactMap[contactID] = &Contact{
				UserID:      contactID,
				Name:        name,
				LastMessage: msg.Content,
			}
		}

		if msg.ReceiverID == userID && !msg.Read {
			contactMap[contactID].Unread++
		}
	}

	contacts := make([]Contact, 0, len(contactMap))
	for _, c := range contactMap {
		contacts = append(contacts, *c)
	}

	return contacts, nil
}

func (r *Repository) MarkMessageRead(msgID uint) error {
	return r.db.Model(&Message{}).Where("id = ?", msgID).Update("read", true).Error
}

func (r *Repository) GetCalendarEvents(userID uint) ([]CalendarEvent, error) {
	var events []CalendarEvent
	if err := r.db.Where("user_id = ?", userID).Order("start_date asc").Find(&events).Error; err != nil {
		return nil, err
	}
	return events, nil
}

func (r *Repository) GetCalendarEventByID(eventID uint, userID uint) (*CalendarEvent, error) {
	var event CalendarEvent
	if err := r.db.Where("id = ? AND user_id = ?", eventID, userID).First(&event).Error; err != nil {
		return nil, err
	}
	return &event, nil
}

func (r *Repository) CreateCalendarEvent(event *CalendarEvent) error {
	return r.db.Create(event).Error
}

func (r *Repository) UpdateCalendarEvent(event *CalendarEvent, updates map[string]interface{}) error {
	return r.db.Model(event).Updates(updates).Error
}

func (r *Repository) DeleteCalendarEvent(eventID uint, userID uint) error {
	result := r.db.Where("id = ? AND user_id = ?", eventID, userID).Delete(&CalendarEvent{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) GetInvites(userID uint, page, limit int) ([]SphereInvite, int64, error) {
	var total int64
	if err := r.db.Model(&SphereInvite{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var invites []SphereInvite
	offset := (page - 1) * limit
	if err := r.db.Where("user_id = ?", userID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&invites).Error; err != nil {
		return nil, 0, err
	}

	return invites, total, nil
}

func (r *Repository) GetInviteByID(inviteID uint, userID uint) (*SphereInvite, error) {
	var invite SphereInvite
	if err := r.db.Where("id = ? AND user_id = ?", inviteID, userID).First(&invite).Error; err != nil {
		return nil, err
	}
	return &invite, nil
}

func (r *Repository) UpdateInviteStatus(invite *SphereInvite, status string) error {
	return r.db.Model(invite).Update("status", status).Error
}

func (r *Repository) GetBookmarks(userID uint, page, limit int) ([]Bookmark, int64, error) {
	var total int64
	if err := r.db.Model(&Bookmark{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var bookmarks []Bookmark
	offset := (page - 1) * limit
	if err := r.db.Where("user_id = ?", userID).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&bookmarks).Error; err != nil {
		return nil, 0, err
	}

	return bookmarks, total, nil
}

func (r *Repository) BookmarkExists(userID, itemID uint, itemType string) bool {
	var existing Bookmark
	return r.db.Where("user_id = ? AND item_id = ? AND item_type = ?", userID, itemID, itemType).
		First(&existing).Error == nil
}

func (r *Repository) CreateBookmark(bookmark *Bookmark) error {
	return r.db.Create(bookmark).Error
}

func (r *Repository) DeleteBookmark(bookmarkID uint, userID uint) error {
	result := r.db.Where("id = ? AND user_id = ?", bookmarkID, userID).Delete(&Bookmark{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) GetBookmarksByType(userID uint, itemType string) ([]Bookmark, error) {
	var bookmarks []Bookmark
	if err := r.db.Where("user_id = ? AND item_type = ?", userID, itemType).
		Order("created_at desc").Find(&bookmarks).Error; err != nil {
		return nil, err
	}
	return bookmarks, nil
}

func (r *Repository) GetNotifications(userID uint, page, limit int) ([]Notification, int64, int64, error) {
	var total int64
	if err := r.db.Model(&Notification{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}

	var notifications []Notification
	offset := (page - 1) * limit
	if err := r.db.Where("user_id = ?", userID).
		Order("read asc, created_at desc").Offset(offset).Limit(limit).Find(&notifications).Error; err != nil {
		return nil, 0, 0, err
	}

	var unreadCount int64
	if err := r.db.Model(&Notification{}).Where("user_id = ? AND read = ?", userID, false).Count(&unreadCount).Error; err != nil {
		return nil, 0, 0, err
	}

	return notifications, total, unreadCount, nil
}

func (r *Repository) GetNotificationByID(notifID uint, userID uint) (*Notification, error) {
	var notification Notification
	if err := r.db.Where("id = ? AND user_id = ?", notifID, userID).First(&notification).Error; err != nil {
		return nil, err
	}
	return &notification, nil
}

func (r *Repository) MarkNotificationRead(notifID uint) error {
	return r.db.Model(&Notification{}).Where("id = ?", notifID).Update("read", true).Error
}

func (r *Repository) MarkAllNotificationsRead(userID uint) error {
	return r.db.Model(&Notification{}).Where("user_id = ? AND read = ?", userID, false).Update("read", true).Error
}

func (r *Repository) CountAdmissions(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&admission.Admission{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *Repository) CountSavedColleges(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&Bookmark{}).Where("user_id = ? AND item_type = ?", userID, "college").Count(&count).Error
	return count, err
}

func (r *Repository) CountScholarshipApplications(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&scholarship.ScholarshipApplication{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func (r *Repository) GetUserByID(userID uint) (*auth.User, error) {
	var user auth.User
	err := r.db.First(&user, userID).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetRecentAdmissions(userID uint, limit int) ([]admission.Admission, error) {
	var admissions []admission.Admission
	if err := r.db.Where("user_id = ?", userID).
		Preload("College").
		Order("created_at desc").Limit(limit).Find(&admissions).Error; err != nil {
		return nil, err
	}
	return admissions, nil
}

func (r *Repository) CountBookmarksByType(userID uint, itemType string) (int64, error) {
	var count int64
	err := r.db.Model(&Bookmark{}).Where("user_id = ? AND item_type = ?", userID, itemType).Count(&count).Error
	return count, err
}

func (r *Repository) CountActiveInvites(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&SphereInvite{}).Where("user_id = ? AND status = ?", userID, "pending").Count(&count).Error
	return count, err
}

func (r *Repository) CountUnreadMessages(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&Message{}).Where("receiver_id = ? AND read = ?", userID, false).Count(&count).Error
	return count, err
}

func (r *Repository) CountUpcomingEvents(userID uint) (int64, error) {
	var count int64
	err := r.db.Model(&CalendarEvent{}).Where("user_id = ? AND start_date > ?", userID, time.Now()).Count(&count).Error
	return count, err
}

func (r *Repository) GetUserAdmissions(userID uint, page, limit int) ([]admission.Admission, int64, error) {
	var total int64
	if err := r.db.Model(&admission.Admission{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var admissions []admission.Admission
	offset := (page - 1) * limit
	if err := r.db.Where("user_id = ?", userID).
		Preload("College").
		Order("created_at desc").Offset(offset).Limit(limit).Find(&admissions).Error; err != nil {
		return nil, 0, err
	}

	return admissions, total, nil
}

type Contact struct {
	UserID      uint   `json:"user_id"`
	Name        string `json:"name"`
	LastMessage string `json:"last_message"`
	Unread      int    `json:"unread"`
}

type User struct {
	FirstName string
	LastName  string
}
