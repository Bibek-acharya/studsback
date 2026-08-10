package websocket

import (
	"encoding/json"
	"io"
	"net"
	"sync"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

type Connection struct {
	userID   uint
	userType string
	conn     net.Conn
	writer   *wsutil.Writer
	mu       sync.Mutex
	closed   bool
}

func NewConnection(userID uint, userType string, conn net.Conn) *Connection {
	return &Connection{
		userID:   userID,
		userType: userType,
		conn:     conn,
		writer:   wsutil.NewWriter(conn, ws.StateServerSide, ws.OpText),
	}
}

func (c *Connection) WriteMessage(msg WSMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	c.writer.Reset(c.conn, ws.StateServerSide, ws.OpText)
	_, err = c.writer.Write(data)
	if err != nil {
		return err
	}
	return c.writer.Flush()
}

func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.conn.Close()
}

func (c *Connection) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *Connection) ReadPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.Close()
	}()

	reader := wsutil.NewReader(c.conn, ws.StateServerSide)
	for {
		_, err := reader.NextFrame()
		if err != nil {
			break
		}

		payload, err := io.ReadAll(reader)
		if err != nil {
			break
		}

		var wsMsg WSMessage
		if err := json.Unmarshal(payload, &wsMsg); err != nil {
			continue
		}

		hub.HandleMessage(c, wsMsg)
	}
}
