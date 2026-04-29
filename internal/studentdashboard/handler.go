package studentdashboard

import (
	"net/http"
	"strconv"

	"studsphere/backend/internal/shared/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetMessages(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	messages, total, err := h.service.GetMessages(id, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve messages")
		return
	}

	response.Success(c, http.StatusOK, "Messages retrieved successfully", gin.H{
		"messages": toMessageResponses(messages),
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetMessageByID(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	msgID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid message ID")
		return
	}

	message, err := h.service.GetMessageByID(uint(msgID), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Message not found")
		return
	}

	response.Success(c, http.StatusOK, "Message retrieved successfully", toMessageResponse(message))
}

func (h *Handler) CreateMessage(c *gin.Context) {
	userID, _ := c.Get("user_id")
	senderID := userID.(uint)

	var req MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	message, err := h.service.CreateMessage(senderID, req.ReceiverID, req.Subject, req.Content)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to send message")
		return
	}

	response.Success(c, http.StatusCreated, "Message sent successfully", toMessageResponse(message))
}

func (h *Handler) ReplyToMessage(c *gin.Context) {
	userID, _ := c.Get("user_id")
	senderID := userID.(uint)
	msgID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid message ID")
		return
	}

	var req MessageReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	reply, err := h.service.ReplyToMessage(uint(msgID), senderID, req.Content)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Message not found")
		return
	}

	response.Success(c, http.StatusCreated, "Reply sent successfully", toMessageResponse(reply))
}

func (h *Handler) GetMessageContacts(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	contacts, err := h.service.GetMessageContacts(id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve contacts")
		return
	}

	response.Success(c, http.StatusOK, "Contacts retrieved successfully", contacts)
}

func (h *Handler) GetCalendarEvents(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	events, err := h.service.GetCalendarEvents(id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve calendar events")
		return
	}

	response.Success(c, http.StatusOK, "Calendar events retrieved successfully", toCalendarEventResponses(events))
}

func (h *Handler) GetCalendarEventByID(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	eventID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event ID")
		return
	}

	event, err := h.service.GetCalendarEventByID(uint(eventID), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Event not found")
		return
	}

	response.Success(c, http.StatusOK, "Event retrieved successfully", toCalendarEventResponse(event))
}

func (h *Handler) CreateCalendarEvent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	var req CalendarEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	event, err := h.service.CreateCalendarEvent(id, req)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to create event")
		return
	}

	response.Success(c, http.StatusCreated, "Event created successfully", toCalendarEventResponse(event))
}

func (h *Handler) UpdateCalendarEvent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	eventID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event ID")
		return
	}

	var req CalendarEventUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	event, err := h.service.UpdateCalendarEvent(uint(eventID), id, req)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Event not found")
		return
	}

	response.Success(c, http.StatusOK, "Event updated successfully", toCalendarEventResponse(event))
}

func (h *Handler) DeleteCalendarEvent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	eventID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid event ID")
		return
	}

	if err := h.service.DeleteCalendarEvent(uint(eventID), id); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Event not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete event")
		return
	}

	response.Success(c, http.StatusOK, "Event deleted successfully", nil)
}

func (h *Handler) GetInvites(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	invites, total, err := h.service.GetInvites(id, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve invites")
		return
	}

	response.Success(c, http.StatusOK, "Invites retrieved successfully", gin.H{
		"invites": invites,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) GetInviteByID(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	inviteID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid invite ID")
		return
	}

	invite, err := h.service.GetInviteByID(uint(inviteID), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Invite not found")
		return
	}

	response.Success(c, http.StatusOK, "Invite retrieved successfully", invite)
}

func (h *Handler) AcceptInvite(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	inviteID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid invite ID")
		return
	}

	invite, err := h.service.AcceptInvite(uint(inviteID), id)
	if err != nil {
		if err.Error() == "invite already processed" {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(c, http.StatusNotFound, "Invite not found")
		return
	}

	response.Success(c, http.StatusOK, "Invite accepted successfully", invite)
}

func (h *Handler) DeclineInvite(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	inviteID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid invite ID")
		return
	}

	invite, err := h.service.DeclineInvite(uint(inviteID), id)
	if err != nil {
		if err.Error() == "invite already processed" {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(c, http.StatusNotFound, "Invite not found")
		return
	}

	response.Success(c, http.StatusOK, "Invite declined successfully", invite)
}

func (h *Handler) SaveInvite(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	inviteID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid invite ID")
		return
	}

	invite, err := h.service.SaveInvite(uint(inviteID), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Invite not found")
		return
	}

	response.Success(c, http.StatusOK, "Invite saved successfully", invite)
}

func (h *Handler) GetBookmarks(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	bookmarks, total, err := h.service.GetBookmarks(id, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve bookmarks")
		return
	}

	response.Success(c, http.StatusOK, "Bookmarks retrieved successfully", gin.H{
		"bookmarks": toBookmarkResponses(bookmarks),
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) CreateBookmark(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	var req BookmarkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	bookmark, err := h.service.CreateBookmark(id, req)
	if err != nil {
		if err.Error() == "already bookmarked" {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to create bookmark")
		return
	}

	response.Success(c, http.StatusCreated, "Bookmark created successfully", toBookmarkResponse(bookmark))
}

func (h *Handler) DeleteBookmark(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	bookmarkID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid bookmark ID")
		return
	}

	if err := h.service.DeleteBookmark(uint(bookmarkID), id); err != nil {
		if err.Error() == "record not found" {
			response.Error(c, http.StatusNotFound, "Bookmark not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "Failed to delete bookmark")
		return
	}

	response.Success(c, http.StatusOK, "Bookmark deleted successfully", nil)
}

func (h *Handler) GetBookmarksByType(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	itemType := c.Param("type")

	bookmarks, err := h.service.GetBookmarksByType(id, itemType)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve bookmarks")
		return
	}

	response.Success(c, http.StatusOK, "Bookmarks retrieved successfully", toBookmarkResponses(bookmarks))
}

func (h *Handler) GetNotifications(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	notifications, total, unreadCount, err := h.service.GetNotifications(id, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve notifications")
		return
	}

	response.Success(c, http.StatusOK, "Notifications retrieved successfully", gin.H{
		"notifications": toNotificationResponses(notifications),
		"unread_count":  unreadCount,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) MarkNotificationRead(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)
	notifID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid notification ID")
		return
	}

	if err := h.service.MarkNotificationRead(uint(notifID), id); err != nil {
		response.Error(c, http.StatusNotFound, "Notification not found")
		return
	}

	response.Success(c, http.StatusOK, "Notification marked as read", nil)
}

func (h *Handler) GetDashboardStats(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	stats, err := h.service.GetDashboardStats(id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve dashboard stats")
		return
	}

	response.Success(c, http.StatusOK, "Dashboard stats retrieved successfully", stats)
}

func (h *Handler) GetRecentApplications(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	apps, err := h.service.GetRecentApplications(id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve recent applications")
		return
	}

	response.Success(c, http.StatusOK, "Recent applications retrieved successfully", gin.H{
		"applications": apps,
	})
}

func (h *Handler) GetMyApplications(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	apps, total, err := h.service.GetMyApplications(id, page, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to retrieve applications")
		return
	}

	response.Success(c, http.StatusOK, "Applications retrieved successfully", gin.H{
		"applications": apps,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) MarkAllNotificationsRead(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := userID.(uint)

	if err := h.service.MarkAllNotificationsRead(id); err != nil {
		response.Error(c, http.StatusInternalServerError, "Failed to mark notifications as read")
		return
	}

	response.Success(c, http.StatusOK, "All notifications marked as read", nil)
}

func toMessageResponse(m *Message) MessageResponse {
	return MessageResponse{
		ID:         m.ID,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
		SenderID:   m.SenderID,
		ReceiverID: m.ReceiverID,
		Subject:    m.Subject,
		Content:    m.Content,
		Read:       m.Read,
		Direction:  m.Direction,
	}
}

func toMessageResponses(messages []Message) []MessageResponse {
	responses := make([]MessageResponse, len(messages))
	for i, m := range messages {
		responses[i] = toMessageResponse(&m)
	}
	return responses
}

func toCalendarEventResponse(e *CalendarEvent) CalendarEventResponse {
	return CalendarEventResponse{
		ID:          e.ID,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
		UserID:      e.UserID,
		Title:       e.Title,
		Description: e.Description,
		StartDate:   e.StartDate,
		EndDate:     e.EndDate,
		Location:    e.Location,
		Link:        e.Link,
		Color:       e.Color,
		Reminder:    e.Reminder,
		Type:        e.Type,
	}
}

func toCalendarEventResponses(events []CalendarEvent) []CalendarEventResponse {
	responses := make([]CalendarEventResponse, len(events))
	for i, e := range events {
		responses[i] = toCalendarEventResponse(&e)
	}
	return responses
}

func toBookmarkResponse(b *Bookmark) BookmarkResponse {
	return BookmarkResponse{
		ID:        b.ID,
		CreatedAt: b.CreatedAt,
		UserID:    b.UserID,
		ItemID:    b.ItemID,
		ItemType:  b.ItemType,
	}
}

func toBookmarkResponses(bookmarks []Bookmark) []BookmarkResponse {
	responses := make([]BookmarkResponse, len(bookmarks))
	for i, b := range bookmarks {
		responses[i] = toBookmarkResponse(&b)
	}
	return responses
}

func toNotificationResponse(n *Notification) NotificationResponse {
	return NotificationResponse{
		ID:        n.ID,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
		UserID:    n.UserID,
		Title:     n.Title,
		Message:   n.Message,
		Type:      n.Type,
		Read:      n.Read,
		Link:      n.Link,
	}
}

func toNotificationResponses(notifications []Notification) []NotificationResponse {
	responses := make([]NotificationResponse, len(notifications))
	for i, n := range notifications {
		responses[i] = toNotificationResponse(&n)
	}
	return responses
}
