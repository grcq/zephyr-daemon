package websocket

import (
	"context"
	"daemon/server"
	"github.com/apex/log"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"net/http"
	"sync"
)

type Handler struct {
	sync.RWMutex
	Conn   *websocket.Conn
	server *server.Server
	uuid   uuid.UUID
}

const (
	ServerLogEvent = "send console log"
	ServerCommand  = "send command"
	ErrorEvent     = "error"
)

type Message struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

func NewHandler(s *server.Server, w http.ResponseWriter, r *http.Request, c *gin.Context) (*Handler, error) {
	conn, err := websocket.Upgrade(w, r, nil, 1024, 1024)
	if err != nil {
		return nil, err
	}

	u, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	return &Handler{
		Conn:   conn,
		server: s,
		uuid:   u,
	}, nil
}

func (h *Handler) UUID() uuid.UUID {
	return h.uuid
}

func (h *Handler) Close() {
	err := h.Conn.Close()
	if err != nil {
		log.WithError(err).Error("failed to close websocket connection")
	}
}

func (h *Handler) Write(data Message) error {
	h.Lock()
	defer h.Unlock()

	return h.Conn.WriteJSON(data)
}

func (h *Handler) HandleIncoming(ctx context.Context, msg Message) error {
	s := h.server
	switch msg.Event {
	case ServerCommand:
		// handle command
	case ServerLogEvent:
		go func() {
			reader, err := s.Logs(true)
			if err != nil {
				h.SendError()
				return
			}
			defer reader.Close()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					line := make([]byte, 1024)
					_, err := reader.Read(line)
					if err != nil {
						h.SendError()
						return
					}

					if err := h.Write(Message{Event: ServerLogEvent, Data: line}); err != nil {
						return
					}
				}
			}
		}()
	}

	return nil
}

func (h *Handler) SendError() {
	if err := h.Write(Message{Event: ErrorEvent}); err != nil {
		log.WithError(err).Error("failed to send error message")
	}
}
