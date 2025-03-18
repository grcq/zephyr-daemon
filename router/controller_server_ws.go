package router

import (
	"context"
	"daemon/events"
	"daemon/router/websocket"
	"daemon/server"
	"encoding/json"
	"github.com/apex/log"
	"github.com/gin-gonic/gin"
)

func getServerWs(c *gin.Context) {
	id := c.Param("server")
	s, err := server.GetServer(id)
	if err != nil || s == nil {
		c.JSON(404, gin.H{"error": "Server not found"})
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	h, err := websocket.NewHandler(s, c.Writer, c.Request, c)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to upgrade connection"})
		return
	}
	defer h.Conn.Close()

	unlisten := events.Listen(h.UUID().String(), func(event events.Event) {
		switch event.Name {
		case events.ServerLog:
			msg := websocket.Message{
				Event: websocket.ServerLogEvent,
				Data:  event.Payload,
			}
			err := h.Write(msg)
			if err != nil {
				return
			}
		}
	})

	go func() {
		<-ctx.Done()
		unlisten()
	}()

	for {
		msg := websocket.Message{}
		_, p, err := h.Conn.ReadMessage()
		if err != nil {
			h.SendError()

			log.WithError(err).Error("failed to read message")
			break
		}

		if err := json.Unmarshal(p, &msg); err != nil {
			h.SendError()
			continue
		}

		go func(msg websocket.Message) {
			if err := h.HandleIncoming(ctx, msg); err != nil {
				h.SendError()
				log.WithError(err).Error("failed to handle incoming message")
			}
		}(msg)
	}
}
