package handlers

import (
	"net/http"
	"strconv"

	"studsphere/backend/config"
	"studsphere/backend/models"
	"studsphere/backend/utils"

	"github.com/gin-gonic/gin"
)

func UpdateProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))

	var user models.User
	if err := config.GetDB().First(&user, id).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "User not found")
		return
	}

	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	updates := map[string]interface{}{}
	if req.FirstName != "" {
		updates["first_name"] = req.FirstName
	}
	if req.LastName != "" {
		updates["last_name"] = req.LastName
	}

	config.GetDB().Model(&user).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "Profile updated successfully", gin.H{
		"id":         user.ID,
		"email":      user.Email,
		"first_name": user.FirstName,
		"last_name":  user.LastName,
		"role":       user.Role,
	})
}

func GetMessages(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	config.GetDB().Model(&models.Message{}).
		Where("sender_id = ? OR receiver_id = ?", id, id).Count(&total)

	var messages []models.Message
	config.GetDB().Where("sender_id = ? OR receiver_id = ?", id, id).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&messages)

	utils.SuccessResponse(c, http.StatusOK, "Messages retrieved successfully", gin.H{
		"messages": messages,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetMessageByID(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))
	msgID := c.Param("id")

	var message models.Message
	if err := config.GetDB().Where("id = ? AND (sender_id = ? OR receiver_id = ?)", msgID, id, id).
		First(&message).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Message not found")
		return
	}

	if !message.Read && message.ReceiverID == id {
		config.GetDB().Model(&message).Update("read", true)
	}

	utils.SuccessResponse(c, http.StatusOK, "Message retrieved successfully", message)
}

func CreateMessage(c *gin.Context) {
	userID, _ := c.Get("user_id")
	senderID := uint(userID.(float64))

	var req struct {
		ReceiverID uint   `json:"receiver_id" binding:"required"`
		Subject    string `json:"subject" binding:"required"`
		Content    string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	message := models.Message{
		SenderID:   senderID,
		ReceiverID: req.ReceiverID,
		Subject:    req.Subject,
		Content:    req.Content,
		Direction:  "sent",
	}

	if err := config.GetDB().Create(&message).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to send message")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Message sent successfully", message)
}

func ReplyToMessage(c *gin.Context) {
	userID, _ := c.Get("user_id")
	senderID := uint(userID.(float64))
	msgID := c.Param("id")

	var original models.Message
	if err := config.GetDB().Where("id = ?", msgID).First(&original).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Message not found")
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	receiverID := original.SenderID
	if original.SenderID == senderID {
		receiverID = original.ReceiverID
	}

	reply := models.Message{
		SenderID:   senderID,
		ReceiverID: receiverID,
		Subject:    "Re: " + original.Subject,
		Content:    req.Content,
		Direction:  "sent",
	}

	if err := config.GetDB().Create(&reply).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to send reply")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Reply sent successfully", reply)
}

func GetMessageContacts(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))

	type ContactInfo struct {
		UserID  uint   `json:"user_id"`
		Name    string `json:"name"`
		LastMsg string `json:"last_message"`
		Unread  int    `json:"unread"`
	}

	var messages []models.Message
	config.GetDB().Where("sender_id = ? OR receiver_id = ?", id, id).
		Order("created_at desc").Find(&messages)

	contactMap := map[uint]*ContactInfo{}
	for _, msg := range messages {
		var contactID uint
		if msg.SenderID == id {
			contactID = msg.ReceiverID
		} else {
			contactID = msg.SenderID
		}

		if _, exists := contactMap[contactID]; !exists {
			var contactUser models.User
			config.GetDB().First(&contactUser, contactID)
			contactMap[contactID] = &ContactInfo{
				UserID:  contactID,
				Name:    contactUser.FirstName + " " + contactUser.LastName,
				LastMsg: msg.Content,
			}
		}

		if msg.ReceiverID == id && !msg.Read {
			contactMap[contactID].Unread++
		}
	}

	contacts := make([]ContactInfo, 0, len(contactMap))
	for _, c := range contactMap {
		contacts = append(contacts, *c)
	}

	utils.SuccessResponse(c, http.StatusOK, "Contacts retrieved successfully", contacts)
}

func GetCalendarEvents(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))

	var events []models.CalendarEvent
	config.GetDB().Where("user_id = ?", id).Order("start_date asc").Find(&events)

	utils.SuccessResponse(c, http.StatusOK, "Calendar events retrieved successfully", events)
}

func GetCalendarEventByID(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))
	eventID := c.Param("id")

	var event models.CalendarEvent
	if err := config.GetDB().Where("id = ? AND user_id = ?", eventID, id).First(&event).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Event not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Event retrieved successfully", event)
}

func CreateCalendarEvent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))

	var req struct {
		Title       string `json:"title" binding:"required"`
		Description string `json:"description"`
		StartDate   string `json:"start_date" binding:"required"`
		EndDate     string `json:"end_date"`
		Location    string `json:"location"`
		Link        string `json:"link"`
		Color       string `json:"color"`
		Reminder    bool   `json:"reminder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	startDate, _ := parseTime(req.StartDate)
	endDate, _ := parseTime(req.EndDate)

	event := models.CalendarEvent{
		UserID:      id,
		Title:       req.Title,
		Description: req.Description,
		StartDate:   startDate,
		EndDate:     endDate,
		Location:    req.Location,
		Link:        req.Link,
		Color:       req.Color,
		Reminder:    req.Reminder,
	}

	if err := config.GetDB().Create(&event).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create event")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Event created successfully", event)
}

func UpdateCalendarEvent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))
	eventID := c.Param("id")

	var event models.CalendarEvent
	if err := config.GetDB().Where("id = ? AND user_id = ?", eventID, id).First(&event).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Event not found")
		return
	}

	var req struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		StartDate   string `json:"start_date"`
		EndDate     string `json:"end_date"`
		Location    string `json:"location"`
		Link        string `json:"link"`
		Color       string `json:"color"`
		Reminder    *bool  `json:"reminder"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
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

	config.GetDB().Model(&event).Updates(updates)

	utils.SuccessResponse(c, http.StatusOK, "Event updated successfully", event)
}

func DeleteCalendarEvent(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))
	eventID := c.Param("id")

	result := config.GetDB().Where("id = ? AND user_id = ?", eventID, id).Delete(&models.CalendarEvent{})
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete event")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Event not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Event deleted successfully", nil)
}

func GetInvites(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	var total int64
	config.GetDB().Model(&models.SphereInvite{}).Where("user_id = ?", id).Count(&total)

	var invites []models.SphereInvite
	config.GetDB().Where("user_id = ?", id).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&invites)

	utils.SuccessResponse(c, http.StatusOK, "Invites retrieved successfully", gin.H{
		"invites": invites,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func GetInviteByID(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))
	inviteID := c.Param("id")

	var invite models.SphereInvite
	if err := config.GetDB().Where("id = ? AND user_id = ?", inviteID, id).First(&invite).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Invite not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Invite retrieved successfully", invite)
}

func AcceptInvite(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))
	inviteID := c.Param("id")

	var invite models.SphereInvite
	if err := config.GetDB().Where("id = ? AND user_id = ?", inviteID, id).First(&invite).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Invite not found")
		return
	}

	if invite.Status != "pending" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invite already processed")
		return
	}

	config.GetDB().Model(&invite).Update("status", "accepted")

	utils.SuccessResponse(c, http.StatusOK, "Invite accepted successfully", invite)
}

func DeclineInvite(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))
	inviteID := c.Param("id")

	var invite models.SphereInvite
	if err := config.GetDB().Where("id = ? AND user_id = ?", inviteID, id).First(&invite).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Invite not found")
		return
	}

	if invite.Status != "pending" {
		utils.ErrorResponse(c, http.StatusBadRequest, "Invite already processed")
		return
	}

	config.GetDB().Model(&invite).Update("status", "declined")

	utils.SuccessResponse(c, http.StatusOK, "Invite declined successfully", invite)
}

func SaveInvite(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))
	inviteID := c.Param("id")

	var invite models.SphereInvite
	if err := config.GetDB().Where("id = ? AND user_id = ?", inviteID, id).First(&invite).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Invite not found")
		return
	}

	config.GetDB().Model(&invite).Update("status", "saved")

	utils.SuccessResponse(c, http.StatusOK, "Invite saved successfully", invite)
}

func GetBookmarks(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	config.GetDB().Model(&models.Bookmark{}).Where("user_id = ?", id).Count(&total)

	var bookmarks []models.Bookmark
	config.GetDB().Where("user_id = ?", id).
		Order("created_at desc").Offset(offset).Limit(limit).Find(&bookmarks)

	utils.SuccessResponse(c, http.StatusOK, "Bookmarks retrieved successfully", gin.H{
		"bookmarks": bookmarks,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func CreateBookmark(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))

	var req struct {
		ItemID   uint   `json:"item_id" binding:"required"`
		ItemType string `json:"type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ErrorResponse(c, http.StatusBadRequest, err.Error())
		return
	}

	var existing models.Bookmark
	if err := config.GetDB().Where("user_id = ? AND item_id = ? AND item_type = ?", id, req.ItemID, req.ItemType).
		First(&existing).Error; err == nil {
		utils.ErrorResponse(c, http.StatusConflict, "Already bookmarked")
		return
	}

	bookmark := models.Bookmark{
		UserID:   id,
		ItemID:   req.ItemID,
		ItemType: req.ItemType,
	}

	if err := config.GetDB().Create(&bookmark).Error; err != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to create bookmark")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Bookmark created successfully", bookmark)
}

func DeleteBookmark(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))
	bookmarkID := c.Param("id")

	result := config.GetDB().Where("id = ? AND user_id = ?", bookmarkID, id).Delete(&models.Bookmark{})
	if result.Error != nil {
		utils.ErrorResponse(c, http.StatusInternalServerError, "Failed to delete bookmark")
		return
	}
	if result.RowsAffected == 0 {
		utils.ErrorResponse(c, http.StatusNotFound, "Bookmark not found")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Bookmark deleted successfully", nil)
}

func GetBookmarksByType(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))
	itemType := c.Param("type")

	var bookmarks []models.Bookmark
	config.GetDB().Where("user_id = ? AND item_type = ?", id, itemType).
		Order("created_at desc").Find(&bookmarks)

	utils.SuccessResponse(c, http.StatusOK, "Bookmarks retrieved successfully", bookmarks)
}

func GetNotifications(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	config.GetDB().Model(&models.Notification{}).Where("user_id = ?", id).Count(&total)

	var notifications []models.Notification
	config.GetDB().Where("user_id = ?", id).
		Order("read asc, created_at desc").Offset(offset).Limit(limit).Find(&notifications)

	var unreadCount int64
	config.GetDB().Model(&models.Notification{}).Where("user_id = ? AND read = ?", id, false).Count(&unreadCount)

	utils.SuccessResponse(c, http.StatusOK, "Notifications retrieved successfully", gin.H{
		"notifications": notifications,
		"unread_count":  unreadCount,
		"meta": gin.H{
			"total": total,
			"page":  page,
			"limit": limit,
		},
	})
}

func MarkNotificationRead(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))
	notifID := c.Param("id")

	var notification models.Notification
	if err := config.GetDB().Where("id = ? AND user_id = ?", notifID, id).First(&notification).Error; err != nil {
		utils.ErrorResponse(c, http.StatusNotFound, "Notification not found")
		return
	}

	config.GetDB().Model(&notification).Update("read", true)

	utils.SuccessResponse(c, http.StatusOK, "Notification marked as read", nil)
}

func MarkAllNotificationsRead(c *gin.Context) {
	userID, _ := c.Get("user_id")
	id := uint(userID.(float64))

	config.GetDB().Model(&models.Notification{}).Where("user_id = ? AND read = ?", id, false).
		Update("read", true)

	utils.SuccessResponse(c, http.StatusOK, "All notifications marked as read", nil)
}
