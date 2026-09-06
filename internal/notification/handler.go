// internal/notification/handler.go
package notification

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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
