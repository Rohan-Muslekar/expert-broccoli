package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"cheat-detection/game-server/telemetry"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type ClientConn struct {
	conn   *websocket.Conn
	player *Player
	mu     sync.Mutex
}

type Server struct {
	game    *Game
	clients map[string]*ClientConn
	mu      sync.RWMutex
}

func NewServer(game *Game) *Server {
	return &Server{
		game:    game,
		clients: make(map[string]*ClientConn),
	}
}

func (s *Server) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	player := s.game.AddHumanPlayer()
	if player == nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"server full"}`))
		conn.Close()
		return
	}

	cc := &ClientConn{conn: conn, player: player}
	s.mu.Lock()
	s.clients[player.ID] = cc
	s.mu.Unlock()

	log.Printf("player %s connected", player.ID)

	wallMsg, _ := json.Marshal(telemetry.GameStateMsg{
		Walls: s.game.Walls,
	})
	conn.WriteMessage(websocket.TextMessage, wallMsg)

	go s.readLoop(cc)
}

func (s *Server) readLoop(cc *ClientConn) {
	defer func() {
		s.mu.Lock()
		delete(s.clients, cc.player.ID)
		s.mu.Unlock()
		s.game.RemoveHumanPlayer(cc.player.ID)
		cc.conn.Close()
		log.Printf("player %s disconnected", cc.player.ID)
	}()

	for {
		_, msg, err := cc.conn.ReadMessage()
		if err != nil {
			return
		}
		var input telemetry.InputState
		if err := json.Unmarshal(msg, &input); err != nil {
			continue
		}
		cc.player.LatestInput = &input
	}
}

func (s *Server) HandleDashboardWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("dashboard ws upgrade error: %v", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		state := s.game.GetState()
		data, err := json.Marshal(state)
		if err != nil {
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}
}

func (s *Server) BroadcastState() {
	state := s.game.GetState()
	data, err := json.Marshal(state)
	if err != nil {
		log.Printf("marshal state error: %v", err)
		return
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, cc := range s.clients {
		cc.mu.Lock()
		err := cc.conn.WriteMessage(websocket.TextMessage, data)
		cc.mu.Unlock()
		if err != nil {
			log.Printf("write to %s error: %v", cc.player.ID, err)
		}
	}
}
