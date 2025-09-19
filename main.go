package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorcon/rcon"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const maxHistorySize = 10

type SocketHandler struct {
	RconConfig
	connectionMutex *sync.RWMutex
	historyMutex    *sync.RWMutex
	rconClient      *rcon.Conn
	messageHistory  []BaseMessage
	clients         map[*websocket.Conn]struct{}
}

func NewSocketHandler(rconConfig RconConfig) *SocketHandler {

	sh := &SocketHandler{
		RconConfig:      rconConfig,
		connectionMutex: &sync.RWMutex{},
		historyMutex:    &sync.RWMutex{},
		messageHistory:  make([]BaseMessage, 0, maxHistorySize),
		clients:         make(map[*websocket.Conn]struct{}),
	}
	return sh
}

type BaseMessage interface {
	Send(*websocket.Conn) error
}

type Message struct {
	Type string `json:"type"`
}

type TextMessage struct {
	Message
	Data string `json:"data"`
}

func (m *TextMessage) Send(conn *websocket.Conn) error {
	return conn.WriteJSON(m)
}

type ResponseMessage struct {
	Message
	Data   string `json:"data"`
	Origin string `json:"origin"`
	Ok     bool   `json:"ok"`
}

func (m *ResponseMessage) Send(conn *websocket.Conn) error {
	return conn.WriteJSON(m)
}

type MetaMessage struct {
	Message
	Game string `json:"game"`
}

func (m *MetaMessage) Send(conn *websocket.Conn) error {
	return conn.WriteJSON(m)
}

type ConnectionMessage struct {
	Message
	Connected bool `json:"connected"`
}

func (h *SocketHandler) getRconClient() (*rcon.Conn, error) {
	h.connectionMutex.Lock()
	defer h.connectionMutex.Unlock()

	if h.rconClient != nil {
		// A simple ping-like command could work. 'help' is a good candidate for many servers.
		if _, err := h.rconClient.Execute("help"); err == nil {
			return h.rconClient, nil
		}
		h.rconClient.Close()
		h.rconClient = nil
		log.Println("RCON connection lost, will attempt to reconnect on next command.")
	}

	newRconClient, err := h.RconConfig.Connect()
	if err != nil {
		log.Printf("Failed to connect to RCON server: %v", err)
		return nil, err
	}

	log.Println("Successfully connected to RCON server!")
	h.rconClient = newRconClient
	return h.rconClient, nil
}

func (h *SocketHandler) HandleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer ws.Close()
	h.connectionMutex.Lock()
	h.clients[ws] = struct{}{}
	h.connectionMutex.Unlock()

	// Send meta information
	meta := MetaMessage{
		Message: Message{Type: "meta"},
		Game:    CurrentConfig.RCon.Game,
	}
	meta.Send(ws)

	h.historyMutex.RLock()

	for _, msg := range h.messageHistory {
		msg.Send(ws)
	}
	h.historyMutex.RUnlock()

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			h.connectionMutex.Lock()
			delete(h.clients, ws)
			h.connectionMutex.Unlock()
			break
		}
		// Also broadcast the command to other clients
		h.BroadcastMessage(string(msg))
		// Send message to RCON
		client, err := h.getRconClient()
		if err != nil {
			log.Printf("Failed to get RCON client: %v", err)

			ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("Failed to connect to RCON: %v", err)))
		} else {
			response, err := client.Execute(string(msg))
			if err != nil {
				log.Printf("RCON send error: %v", err)
				h.BroadcastResponse(fmt.Sprintf("RCON command failed: %v", err), "rcon", false)

			}
			if len(response) > 0 {
				h.BroadcastResponse(response, "rcon", true)
			}
		}

	}
}

func (h *SocketHandler) BroadcastResponse(message string, origin string, ok bool) {
	resp := ResponseMessage{
		Message: Message{Type: "response"},
		Data:    message,
		Origin:  origin,
		Ok:      ok,
	}
	h.sendMessage(&resp)
}

func (h *SocketHandler) sendMessage(msg BaseMessage) {
	h.historyMutex.Lock()

	if len(h.messageHistory) >= maxHistorySize {
		h.messageHistory = h.messageHistory[1:]
	}
	h.messageHistory = append(h.messageHistory, msg)
	h.historyMutex.Unlock()

	h.connectionMutex.Lock()
	for client := range h.clients {
		err := msg.Send(client)
		if err != nil {
			log.Printf("WebSocket write error: %v", err)
			client.Close()
			delete(h.clients, client)
		}
	}
	h.connectionMutex.Unlock()
}

func (h *SocketHandler) BroadcastMessage(message string) {
	// Add to history
	msg := TextMessage{
		Message: Message{Type: "text"},
		Data:    message,
	}
	h.sendMessage(&msg)
}

func main() {

	log.Printf("[rcon2000] Starting for game type: %s\n", CurrentConfig.RCon.Game)

	sh := NewSocketHandler(CurrentConfig.RCon)

	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.Dir("./public")))
	mux.HandleFunc("/ws", sh.HandleConnections)

	if CurrentConfig.K8s != nil {
		gw, err := NewGameWatcher(*CurrentConfig.K8s)
		if err != nil {
			log.Fatal(err)
		}
		gw.RegisterHandlers(mux)
	}

	log.Printf("HTTP server starting on 1337")
	err := http.ListenAndServe(":1337", mux)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
