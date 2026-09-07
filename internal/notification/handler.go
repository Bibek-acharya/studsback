// internal/notification/handler.go
package notification

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const timeRFC3339 = "2006-01-02T15:04:05Z07:00"

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// identity resolves the caller's inbox identity from JWT claims (doc 03 §4).
func identity(c *gin.Context) (string, uint, bool) {
	role, _ := c.Get("user_role")
	uid, _ := c.Get("user_id")
	pid, _ := c.Get("provider_id")
	userID, _ := uid.(uint)
	providerID, _ := pid.(uint)
	roleStr, _ := role.(string)
	return ResolveAccount(roleStr, userID, providerID)
}

func (h *Handler) list(c *gin.Context) {
	at, aid, ok := identity(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown role"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	rows, total, unread, err := h.svc.repo.ListInbox(at, aid, page, limit,
		c.Query("category"), c.Query("unread_only") == "true", c.Query("archived") == "true")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]NotificationItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, toItem(r, at, aid))
	}
	resp := InboxResponse{Notifications: items, UnreadCount: unread,
		Meta: InboxMeta{Total: total, Page: page, Limit: limit}}
	c.JSON(http.StatusOK, gin.H{"data": resp, "message": "ok"})
}

func (h *Handler) unreadCount(c *gin.Context) {
	at, aid, ok := identity(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown role"})
		return
	}
	n, err := h.svc.repo.UnreadCount(at, aid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": UnreadResponse{UnreadCount: n}, "message": "ok"})
}

func (h *Handler) markRead(c *gin.Context) {
	at, aid, ok := identity(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown role"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	n, err := h.svc.repo.MarkRead(at, aid, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found in your inbox"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "marked read"})
}

func (h *Handler) markAllRead(c *gin.Context) {
	at, aid, ok := identity(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown role"})
		return
	}
	n, err := h.svc.repo.MarkAllRead(at, aid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": BulkReadResponse{Updated: n}, "message": "ok"})
}

func (h *Handler) archive(archived bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		at, aid, ok := identity(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown role"})
			return
		}
		id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
		n, err := h.svc.repo.SetArchived(at, aid, uint(id), archived)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if n == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found in your inbox"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	}
}

func (h *Handler) remove(c *gin.Context) {
	at, aid, ok := identity(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unknown role"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	n, err := h.svc.repo.SoftDelete(at, aid, uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found in your inbox"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// toItem builds the response shape with the transition envelope synthesized.
func toItem(r AccountNotification, accountType string, accountID uint) NotificationItem {
	item := NotificationItem{
		ID: r.ID, EventKey: r.EventKey, Category: r.Category, Priority: r.Priority,
		Title: r.Title, Body: r.Body, Link: r.Link,
		CreatedAt: r.CreatedAt.Format(timeRFC3339), UpdatedAt: r.UpdatedAt.Format(timeRFC3339),
		Type: r.Category, Read: r.ReadAt != nil, Message: r.Body,
	}
	if r.ReadAt != nil {
		s := r.ReadAt.Format(timeRFC3339)
		item.ReadAt = &s
	}
	if r.ArchivedAt != nil {
		s := r.ArchivedAt.Format(timeRFC3339)
		item.ArchivedAt = &s
	}
	if accountType == "user" {
		item.UserID = accountID
	}
	if accountType == "provider" {
		item.ProviderID = accountID
	}
	return item
}

func (h *Handler) providerList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	rows, total, unread, err := h.svc.repo.ListInbox("provider", c.GetUint("provider_id"), page, limit, "", false, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := make([]ProviderNotificationItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, ProviderNotificationItem{
			ID: r.ID, ProviderID: r.AccountID, Title: r.Title, Message: r.Body,
			Type: r.Category, Read: r.ReadAt != nil, Link: r.Link,
			CreatedAt: r.CreatedAt.Format(timeRFC3339),
		})
	}
	resp := ProviderListResponse{Notifications: items, UnreadCount: unread,
		Meta: InboxMeta{Total: total, Page: page, Limit: limit}}
	c.JSON(http.StatusOK, gin.H{"data": resp, "message": "ok"})
}

func (h *Handler) providerMarkRead(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	n, err := h.svc.repo.MarkRead("provider", c.GetUint("provider_id"), uint(id))
	if err != nil || n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "marked read"})
}

func (h *Handler) providerMarkAllRead(c *gin.Context) {
	n, err := h.svc.repo.MarkAllRead("provider", c.GetUint("provider_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": BulkReadResponse{Updated: n}, "message": "ok"})
}

type broadcastRequest struct {
	Title          string   `json:"title" binding:"required"`
	Body           string   `json:"body" binding:"required"`
	Link           string   `json:"link"`
	Audience       []string `json:"audience" binding:"required"`
	Priority       string   `json:"priority"`
	IdempotencyKey string   `json:"idempotency_key"`
}

func (h *Handler) createBroadcast(c *gin.Context) {
	createdBy := c.GetUint("user_id")
	var req broadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Priority == "" {
		req.Priority = PriorityCritical
	}
	campaign, created, err := h.svc.CreateBroadcast(createdBy, req.Title, req.Body, req.Link, req.Priority, req.Audience, req.IdempotencyKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	code := http.StatusAccepted
	if !created {
		code = http.StatusOK // idempotent replay returns the original campaign
	}
	c.JSON(code, gin.H{"data": gin.H{"broadcast_id": campaign.ID, "status": campaign.Status}})
}

func (h *Handler) listBroadcasts(c *gin.Context) {
	var rows []NotificationBroadcast
	if err := h.svc.db.Order("created_at DESC").Limit(100).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "message": "ok"})
}

func (h *Handler) cancelBroadcast(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.db.Model(&NotificationBroadcast{}).
		Where("id = ? AND status = 'sending'", id).Update("status", "cancelled").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "cancelled"})
}

// ToPublicNotificationResponse maps a banner to the public shape (moved from
// the system module, Task 13 — the guest GET serves the identical JSON).
func ToPublicNotificationResponse(n *PublicNotification) PublicNotificationResponse {
	return PublicNotificationResponse{
		ID:        n.ID,
		CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z"),
		Title:     n.Title,
		Message:   n.Message,
		Type:      n.Type,
		Link:      n.Link,
		Icon:      n.Icon,
		Color:     n.Color,
		BgColor:   n.BgColor,
	}
}

type publicNotificationRequest struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Link    string `json:"link"`
	Icon    string `json:"icon"`
	Color   string `json:"color"`
	BgColor string `json:"bg_color"`
	Active  *bool  `json:"active"`
}

func (h *Handler) createPublicNotification(c *gin.Context) {
	var req publicNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	notifType := req.Type
	if notifType == "" {
		notifType = "info"
	}
	n := &PublicNotification{
		Title:   req.Title,
		Message: req.Message,
		Type:    notifType,
		Link:    req.Link,
		Active:  true,
		Icon:    req.Icon,
		Color:   req.Color,
		BgColor: req.BgColor,
	}
	if err := h.svc.repo.CreatePublicNotification(n); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": ToPublicNotificationResponse(n), "message": "created"})
}

func (h *Handler) updatePublicNotification(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req publicNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{}
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Message != "" {
		updates["message"] = req.Message
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Link != "" {
		updates["link"] = req.Link
	}
	if req.Icon != "" {
		updates["icon"] = req.Icon
	}
	if req.Color != "" {
		updates["color"] = req.Color
	}
	if req.BgColor != "" {
		updates["bg_color"] = req.BgColor
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}
	n, err := h.svc.repo.UpdatePublicNotification(uint(id), updates)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": ToPublicNotificationResponse(n), "message": "updated"})
}

func (h *Handler) listAllPublicNotifications(c *gin.Context) {
	rows, err := h.svc.repo.ListAllPublicNotifications()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]PublicNotificationResponse, 0, len(rows))
	for i := range rows {
		out = append(out, ToPublicNotificationResponse(&rows[i]))
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "message": "ok"})
}

func (h *Handler) deletePublicNotification(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	n, err := h.svc.repo.SoftDeletePublicNotification(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
