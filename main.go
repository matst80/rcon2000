package main

import (
	"encoding/json"
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

var clients = make(map[*websocket.Conn]bool)
var broadcast = make(chan string)
var mutex = &sync.Mutex{}

const maxHistorySize = 50

var messageHistory []string

var rconClient *rcon.Conn
var rconMutex = &sync.Mutex{}

func getRconClient() (*rcon.Conn, error) {
	rconMutex.Lock()
	defer rconMutex.Unlock()

	if rconClient != nil {
		// A simple ping-like command could work. 'help' is a good candidate for many servers.
		if _, err := rconClient.Execute("help"); err == nil {
			return rconClient, nil
		}
		rconClient.Close()
		rconClient = nil
		log.Println("RCON connection lost, will attempt to reconnect on next command.")
	}

	log.Printf("Attempting to connect to RCON server")
	newRconClient, err := CurrentConfig.RCon.Conenct()
	if err != nil {
		return nil, err
	}

	log.Println("Successfully connected to RCON server!")
	rconClient = newRconClient
	return rconClient, nil
}

func main() {

	log.Printf("[rcon2000] Starting for game type: %s\n", CurrentConfig.RCon.Game)

	// Attempt initial connection, but don't fail if it doesn't work
	_, err := getRconClient()
	if err != nil {
		log.Printf("Initial RCON connection failed, will retry on command: %v", err)
	}

	go handleMessages()

	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.Dir("./public")))
	mux.HandleFunc("/ws", handleConnections)

	if CurrentConfig.K8s != nil {
		gw, err := NewGameWatcher(*CurrentConfig.K8s)
		if err != nil {
			log.Fatal(err)
		}
		gw.RegisterHandlers(mux)
	}

	log.Printf("HTTP server starting on 1337")
	err = http.ListenAndServe(":1337", mux)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ws.Close()

	mutex.Lock()
	clients[ws] = true
	mutex.Unlock()

	// Send meta information
	meta := map[string]string{"type": "meta", "game": CurrentConfig.RCon.Game}
	metaJSON, _ := json.Marshal(meta)
	ws.WriteMessage(websocket.TextMessage, metaJSON)
	ws.WriteMessage(websocket.TextMessage, []byte("Welcome to the multiplayer rcon!"))

	// Send message history
	mutex.Lock()
	for _, msg := range messageHistory {
		ws.WriteMessage(websocket.TextMessage, []byte(msg))
	}
	mutex.Unlock()

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			mutex.Lock()
			delete(clients, ws)
			mutex.Unlock()
			break
		}
		// Also broadcast the command to other clients
		broadcast <- string(msg)
		// Send message to RCON
		client, err := getRconClient()
		if err != nil {
			log.Printf("Failed to get RCON client: %v", err)
			broadcast <- fmt.Sprintf("Failed to connect to RCON: %v", err)
		} else {
			log.Printf("Sending to RCON: %s", string(msg))
			response, err := client.Execute(string(msg))
			if err != nil {
				log.Printf("RCON send error: %v", err)
				broadcast <- fmt.Sprintf("RCON command failed: %v", err)
			}
			if len(response) > 0 {
				broadcast <- response
			}
		}

	}
}

func handleMessages() {
	for {
		msg := <-broadcast

		mutex.Lock()
		// Add to history
		if len(messageHistory) >= maxHistorySize {
			messageHistory = messageHistory[1:]
		}
		messageHistory = append(messageHistory, msg)

		for client := range clients {
			err := client.WriteMessage(websocket.TextMessage, []byte(msg))
			if err != nil {
				log.Printf("error: %v", err)
				client.Close()
				delete(clients, client)
			}
		}
		mutex.Unlock()
	}
}
