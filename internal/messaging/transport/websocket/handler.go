package websocket

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gobwas/ws"
)

type WSHandler struct {
	hub *Hub
}

func NewWSHandler(hub *Hub) *WSHandler {
	return &WSHandler{hub: hub}
}

func (h *WSHandler) HandleWS(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 64)
	userType := c.Query("user_type")

	if userID == 0 || userType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing user_id or user_type"})
		return
	}

	conn, _, _, err := ws.UpgradeHTTP(c.Request, c.Writer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upgrade"})
		return
	}

	connection := NewConnection(uint(userID), userType, conn)
	h.hub.register <- connection
	go connection.ReadPump(h.hub)
}
