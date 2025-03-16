package websocket

import (
	"daemon/server"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
)

type Handler struct {
	Conn   *websocket.Conn
	server *server.Server
}

func Configure(s *server.Server, w http.ResponseWriter, r *http.Request, c *gin.Context) (*Handler, error) {
}
