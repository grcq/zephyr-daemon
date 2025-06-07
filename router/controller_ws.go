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
	s := c.MustGet("server").(*server.Server)

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	h, err := websocket.NewHandler(s, c.Writer, c.Request, c)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to upgrade connection"})
		return
	}

	defer h.Conn.Close()

	log.WithField("server", s.Uuid).Info("web socket connection established")
	unlisten := events.Listen(h.UUID().String(), func(event events.Event) {
		var e string
		switch event.Name {
		case events.ServerLog:
			e = websocket.ServerLogEvent
		case events.ServerStats:
			e = websocket.ServerStatsEvent
		case events.ServerInstallStarted:
			e = websocket.ServerInstallStartedEvent
		case events.ServerInstallFinished:
			e = websocket.ServerInstallFinishedEvent
		case events.PowerEvent:
			e = websocket.ServerPowerEvent
		}

		if e != "" {
			msg := websocket.Message{
				Event: e,
				Data:  event.Payload,
			}
			if err := h.Write(msg); err != nil {
				log.WithError(err).Error("failed to write message")
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

func getGlobalWs(c *gin.Context) {
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	h, err := websocket.NewHandler(nil, c.Writer, c.Request, c)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to upgrade connection"})
		return
	}

	defer h.Conn.Close()

	log.Info("global web socket connection established")
	unlisten := events.Listen(h.UUID().String(), func(event events.Event) {
		var e string
		switch event.Name {
		case events.ServerCreated:
			e = websocket.ServerCreatedEvent
		}

		if e != "" {
			msg := websocket.Message{
				Event: e,
				Data:  event.Payload,
			}
			if err := h.Write(msg); err != nil {
				log.WithError(err).Error("failed to write message")
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
