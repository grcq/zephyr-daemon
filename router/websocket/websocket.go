package websocket

import (
	"bufio"
	"context"
	"daemon/env"
	"daemon/events"
	"daemon/server"
	"daemon/templates"
	"encoding/json"
	"github.com/apex/log"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Handler struct {
	sync.RWMutex
	Conn   *websocket.Conn
	server *server.Server
	uuid   uuid.UUID
}

const (
	ServerLogEvent             = "send console log"
	ServerCommand              = "send command"
	ServerStatsEvent           = "send server stats"
	ServerInstallStartedEvent  = "server install started"
	ServerInstallFinishedEvent = "server install finished"
	ServerPowerEvent           = "server power event"
	ServerCreatedEvent         = "server created"
	ErrorEvent                 = "error"
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
	d, err := env.GetDocker()
	if err != nil {
		log.WithError(err).Error("failed to get docker client")
		return err
	}

	s := h.server
	switch msg.Event {
	case ServerStatsEvent:
		stats, err := s.GetStats()
		if err != nil {
			h.SendError()
			return err
		}

		if err := h.Write(Message{Event: ServerStatsEvent, Data: stats}); err != nil {
			log.WithError(err).Error("failed to send server stats")
		}
	case ServerCommand:
		command, ok := msg.Data.(string)
		if !ok {
			log.Error("invalid command received")
			h.SendError()
			return nil
		}

		if command == "" {
			return nil
		}

		if err := s.Command(command); err != nil {
			log.WithError(err).Error("failed to execute command on server")
			h.SendError()
			return err
		}
	case ServerPowerEvent:
		action, ok := msg.Data.(string)
		if !ok {
			log.Error("invalid power action received")
			h.SendError()
			return nil
		}

		switch action {
		case server.PowerStart.String():
			err = s.Power(server.PowerStart)
		case server.PowerStop.String():
			err = s.Power(server.PowerStop)
		case server.PowerRestart.String():
			err = s.Power(server.PowerRestart)
		case server.PowerKill.String():
			err = s.Power(server.PowerKill)
		default:
			log.WithField("action", action).Error("unknown power action")
		}

		if err != nil {
			log.WithError(err).Error("failed to power on server")
			h.SendError()
			return err
		}
	case ServerLogEvent:
		go func() {
			log.Debugf("starting to listen to logs for server %s", s.Uuid)

			previousLogs, err := s.GetLogsSinceStart()
			if err != nil {
				log.WithError(err).Error("failed to get previous logs, is the server running?")
			}

			payload := map[string]interface{}{
				"lines":    previousLogs,
				"daemon":   false,
				"previous": true,
			}
			if err := h.Write(Message{Event: ServerLogEvent, Data: payload}); err != nil {
				log.WithError(err).Error("failed to send previous logs")
				return
			}

			reader, err := s.Logs(true)
			if err != nil {
				log.WithError(err).Error("failed to get server logs")
				h.SendError()
				return
			}
			defer func(reader io.ReadCloser) {
				err := reader.Close()
				if err != nil {
					log.WithError(err).Error("failed to close log reader")
				}
			}(reader)

			t, err := templates.GetTemplate(s.Template)
			if err != nil {
				log.WithError(err).Error("failed to get server template")
				h.SendError()
				return
			}

			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				line := scanner.Text()
				var startConfig struct {
					Started *string `json:"started"`
				}
				if err := json.Unmarshal([]byte(t.Docker.StartConfig), &startConfig); err != nil {
					log.WithError(err).Error("failed to unmarshal start config")
					h.SendError()
					break
				}

				if s.State == server.Starting && startConfig.Started != nil && *startConfig.Started != "" {
					if strings.Contains(line, *startConfig.Started) {
						log.Info("server started successfully")
						s.State = server.Running
						if err := s.Save(); err != nil {
							log.WithError(err).Error("failed to save server state after starting")
						}
						events.New(events.ServerLog, map[string]interface{}{
							"daemon":  true,
							"message": "Server is now running",
						}).Publish()
					}
				}

				payload := map[string]interface{}{
					"message": line,
					"daemon":  false,
				}

				if err := h.Write(Message{Event: ServerLogEvent, Data: payload}); err != nil {
					log.WithError(err).Error("failed to send log message")
				}
			}

			if err := scanner.Err(); err != nil {
				log.WithError(err).Error("error reading server logs")
				h.SendError()
				return
			}

			if s.State == server.Installing || s.State == server.Stopped {
				return
			}

			time.Sleep(500 * time.Millisecond) // wait a bit before sending close message

			log.Info("server stopped, closing log stream")
			events.New(events.PowerEvent, map[string]interface{}{
				"action": server.PowerStop.String(),
				"status": server.Stopped.String(),
			}).Publish()

			inspect, err := d.ContainerInspect(ctx, s.DockerId)
			if err != nil {
				log.WithError(err).Error("failed to inspect container")
				return
			}

			events.New(events.ServerLog, map[string]interface{}{
				"daemon":  true,
				"message": "Server is no longer running",
			}).Publish()

			if inspect.State.ExitCode != 0 {
				events.New(events.ServerLog, map[string]interface{}{
					"daemon":  true,
					"message": "Server crashed with exit code " + strconv.Itoa(inspect.State.ExitCode),
				}).Publish()
				log.WithField("exit_code", inspect.State.ExitCode).Error("server crashed")
			}

			s.Stdin.Close()
			s.State = server.Stopped
			if err := s.Save(); err != nil {
				log.WithError(err).Error("failed to save server state after stopping")
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
