package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"

	"github.com/gorilla/websocket"
)

const (
	defaultPlayerCount = 2
	maxPlayerCount     = 16
)

type jsonWriter interface {
	WriteJSON(v any) error
}

type player struct {
	id          string
	conn        jsonWriter
	roomID      string
	gameType    string
	playerCount int
	queueKey    string
	writeMu     sync.Mutex
}

type room struct {
	id          string
	gameType    string
	playerCount int
	playerIDs   []string
}

type clientMessage struct {
	Type        string          `json:"type"`
	GameType    string          `json:"gameType,omitempty"`
	PlayerCount int             `json:"playerCount,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

type serverMessage struct {
	Type        string          `json:"type"`
	PlayerID    string          `json:"playerId,omitempty"`
	RoomID      string          `json:"roomId,omitempty"`
	GameType    string          `json:"gameType,omitempty"`
	PlayerCount int             `json:"playerCount,omitempty"`
	Players     []string        `json:"players,omitempty"`
	Games       []gameSummary   `json:"games,omitempty"`
	Message     string          `json:"message,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

type gameSummary struct {
	ID                string   `json:"id"`
	Status            string   `json:"status"`
	GameType          string   `json:"gameType"`
	PlayerCount       int      `json:"playerCount"`
	JoinedPlayerCount int      `json:"joinedPlayerCount"`
	Players           []string `json:"players,omitempty"`
}

type outboundMessage struct {
	player *player
	body   serverMessage
}

type hub struct {
	mu      sync.Mutex
	players map[string]*player
	rooms   map[string]*room
	queues  map[string][]string
}

var upgrader = websocket.Upgrader{
	// GitHub Pages and local tunnels are different origins from the backend.
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	gameHub := newHub()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleRequest(gameHub, w, r)
	})

	log.Printf("Yumbo multiplayer backend listening on ws://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func newHub() *hub {
	return &hub{
		players: map[string]*player{},
		rooms:   map[string]*room{},
		queues:  map[string][]string{},
	}
}

func handleRequest(gameHub *hub, w http.ResponseWriter, r *http.Request) {
	if websocket.IsWebSocketUpgrade(r) {
		handleWebSocket(gameHub, w, r)
		return
	}

	w.Header().Set("content-type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "Yumbo multiplayer backend is running.")
}

func handleWebSocket(gameHub *hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}

	currentPlayer := gameHub.addPlayer(conn)
	send(currentPlayer, serverMessage{
		Type:     "connected",
		PlayerID: currentPlayer.id,
	})
	gameHub.sendLobby(currentPlayer)

	defer gameHub.removePlayer(currentPlayer)

	for {
		var message clientMessage
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		gameHub.handleMessage(currentPlayer, message)
	}
}

func (h *hub) addPlayer(conn jsonWriter) *player {
	h.mu.Lock()
	defer h.mu.Unlock()

	currentPlayer := &player{
		id:   createID("player"),
		conn: conn,
	}
	h.players[currentPlayer.id] = currentPlayer
	return currentPlayer
}

func (h *hub) handleMessage(currentPlayer *player, message clientMessage) {
	switch message.Type {
	case "join_queue":
		h.joinQueue(currentPlayer, message.GameType, message.PlayerCount)
	case "leave_queue":
		h.leaveQueue(currentPlayer)
	case "leave_room":
		h.leaveRoom(currentPlayer, "left_room")
	case "request_lobby":
		h.sendLobby(currentPlayer)
	case "room_message":
		h.relayRoomMessage(currentPlayer, message.Payload)
	default:
		flushMessages([]outboundMessage{errorMessage(currentPlayer, "Unknown message type: "+message.Type)})
	}
}

func (h *hub) joinQueue(currentPlayer *player, gameType string, requestedPlayerCount int) {
	h.mu.Lock()
	var messages []outboundMessage

	if gameType == "" {
		messages = append(messages, errorMessage(currentPlayer, "gameType is required to join a queue."))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}

	playerCount := normalizePlayerCount(requestedPlayerCount)
	if playerCount == 0 {
		messages = append(messages, errorMessage(currentPlayer, fmt.Sprintf("playerCount must be between %d and %d.", defaultPlayerCount, maxPlayerCount)))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}
	queueKey := createQueueKey(gameType, playerCount)

	if currentPlayer.roomID != "" {
		h.leaveRoomLocked(currentPlayer, "joined_queue", &messages)
	}

	if currentPlayer.queueKey != "" {
		if currentPlayer.queueKey == queueKey && h.isQueuedLocked(currentPlayer.id, queueKey) {
			messages = append(messages, outboundMessage{
				player: currentPlayer,
				body: serverMessage{
					Type:        "already_queued",
					PlayerID:    currentPlayer.id,
					GameType:    gameType,
					PlayerCount: playerCount,
				},
			})
			h.mu.Unlock()
			flushMessages(messages)
			return
		}
		h.removeFromQueueLocked(currentPlayer.id, currentPlayer.queueKey)
	}

	currentPlayer.gameType = gameType
	currentPlayer.playerCount = playerCount
	currentPlayer.queueKey = queueKey
	h.queues[queueKey] = append(h.queues[queueKey], currentPlayer.id)
	messages = append(messages, outboundMessage{
		player: currentPlayer,
		body: serverMessage{
			Type:        "queued",
			PlayerID:    currentPlayer.id,
			GameType:    gameType,
			PlayerCount: playerCount,
		},
	})

	h.matchQueuedPlayersLocked(queueKey, &messages)

	h.prependLobbyBroadcastLocked(&messages)
	h.mu.Unlock()
	flushMessages(messages)
}

func (h *hub) leaveQueue(currentPlayer *player) {
	h.mu.Lock()
	var messages []outboundMessage

	if currentPlayer.queueKey == "" || !h.removeFromQueueLocked(currentPlayer.id, currentPlayer.queueKey) {
		messages = append(messages, outboundMessage{
			player: currentPlayer,
			body:   serverMessage{Type: "not_queued"},
		})
		h.mu.Unlock()
		flushMessages(messages)
		return
	}

	gameType := currentPlayer.gameType
	playerCount := currentPlayer.playerCount
	currentPlayer.gameType = ""
	currentPlayer.playerCount = 0
	currentPlayer.queueKey = ""
	messages = append(messages, outboundMessage{
		player: currentPlayer,
		body: serverMessage{
			Type:        "queue_left",
			PlayerID:    currentPlayer.id,
			GameType:    gameType,
			PlayerCount: playerCount,
		},
	})

	h.prependLobbyBroadcastLocked(&messages)
	h.mu.Unlock()
	flushMessages(messages)
}

func (h *hub) leaveRoom(currentPlayer *player, reason string) {
	h.mu.Lock()
	var messages []outboundMessage
	h.leaveRoomLocked(currentPlayer, reason, &messages)
	h.prependLobbyBroadcastLocked(&messages)
	h.mu.Unlock()
	flushMessages(messages)
}

func (h *hub) relayRoomMessage(currentPlayer *player, payload json.RawMessage) {
	h.mu.Lock()
	var messages []outboundMessage

	currentRoom := h.rooms[currentPlayer.roomID]
	if currentRoom == nil {
		messages = append(messages, errorMessage(currentPlayer, "You are not in a room."))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}

	for _, playerID := range currentRoom.playerIDs {
		if playerID == currentPlayer.id {
			continue
		}
		if recipient := h.players[playerID]; recipient != nil {
			messages = append(messages, outboundMessage{
				player: recipient,
				body: serverMessage{
					Type:        "room_message",
					PlayerID:    currentPlayer.id,
					RoomID:      currentRoom.id,
					GameType:    currentRoom.gameType,
					PlayerCount: currentRoom.playerCount,
					Payload:     payload,
				},
			})
		}
	}

	h.mu.Unlock()
	flushMessages(messages)
}

func (h *hub) removePlayer(currentPlayer *player) {
	h.mu.Lock()
	var messages []outboundMessage

	if currentPlayer.queueKey != "" {
		h.removeFromQueueLocked(currentPlayer.id, currentPlayer.queueKey)
	}
	h.leaveRoomLocked(currentPlayer, "peer_disconnected", &messages)
	delete(h.players, currentPlayer.id)
	h.prependLobbyBroadcastLocked(&messages)

	h.mu.Unlock()
	flushMessages(messages)

	if closer, ok := currentPlayer.conn.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

func (h *hub) matchQueuedPlayersLocked(queueKey string, messages *[]outboundMessage) {
	h.queues[queueKey] = h.compactQueueLocked(queueKey)

	for len(h.queues[queueKey]) > 0 {
		first := h.players[h.queues[queueKey][0]]
		if first == nil {
			h.queues[queueKey] = h.queues[queueKey][1:]
			continue
		}

		playerCount := first.playerCount
		if playerCount <= 0 || len(h.queues[queueKey]) < playerCount {
			return
		}

		roomPlayers := make([]*player, 0, playerCount)
		playerIDs := h.queues[queueKey][:playerCount]
		h.queues[queueKey] = h.queues[queueKey][playerCount:]
		for _, playerID := range playerIDs {
			if currentPlayer := h.players[playerID]; currentPlayer != nil {
				roomPlayers = append(roomPlayers, currentPlayer)
			}
		}

		if len(roomPlayers) != playerCount {
			h.queues[queueKey] = h.compactQueueLocked(queueKey)
			continue
		}

		h.createRoomLocked(first.gameType, playerCount, roomPlayers, messages)
	}
}

func (h *hub) createRoomLocked(gameType string, playerCount int, roomPlayers []*player, messages *[]outboundMessage) {
	roomID := createID("room")
	playerIDs := make([]string, 0, len(roomPlayers))
	for _, currentPlayer := range roomPlayers {
		playerIDs = append(playerIDs, currentPlayer.id)
	}

	currentRoom := &room{
		id:          roomID,
		gameType:    gameType,
		playerCount: playerCount,
		playerIDs:   playerIDs,
	}

	h.rooms[roomID] = currentRoom
	for _, currentPlayer := range roomPlayers {
		currentPlayer.roomID = roomID
		currentPlayer.gameType = gameType
		currentPlayer.playerCount = playerCount
		currentPlayer.queueKey = ""
	}

	players := append([]string(nil), currentRoom.playerIDs...)
	for _, currentPlayer := range roomPlayers {
		*messages = append(*messages, outboundMessage{
			player: currentPlayer,
			body: serverMessage{
				Type:        "room_created",
				PlayerID:    currentPlayer.id,
				RoomID:      roomID,
				GameType:    gameType,
				PlayerCount: playerCount,
				Players:     players,
			},
		})
	}
}

func (h *hub) leaveRoomLocked(currentPlayer *player, reason string, messages *[]outboundMessage) {
	currentRoom := h.rooms[currentPlayer.roomID]
	if currentRoom == nil {
		currentPlayer.roomID = ""
		return
	}

	delete(h.rooms, currentRoom.id)
	for _, playerID := range currentRoom.playerIDs {
		roomPlayer := h.players[playerID]
		if roomPlayer == nil {
			continue
		}

		roomPlayer.roomID = ""
		roomPlayer.gameType = ""
		roomPlayer.playerCount = 0
		roomPlayer.queueKey = ""
		messageType := "room_left"
		if roomPlayer.id != currentPlayer.id {
			messageType = "peer_left"
		}

		*messages = append(*messages, outboundMessage{
			player: roomPlayer,
			body: serverMessage{
				Type:        messageType,
				PlayerID:    currentPlayer.id,
				RoomID:      currentRoom.id,
				GameType:    currentRoom.gameType,
				PlayerCount: currentRoom.playerCount,
				Reason:      reason,
			},
		})
	}
}

func (h *hub) compactQueueLocked(queueKey string) []string {
	active := make([]string, 0, len(h.queues[queueKey]))
	for _, playerID := range h.queues[queueKey] {
		currentPlayer := h.players[playerID]
		if currentPlayer != nil && currentPlayer.roomID == "" && currentPlayer.queueKey == queueKey {
			active = append(active, playerID)
		}
	}
	return active
}

func (h *hub) removeFromQueueLocked(playerID string, queueKey string) bool {
	removed := false
	queue := h.queues[queueKey]
	filtered := queue[:0]
	for _, queuedPlayerID := range queue {
		if queuedPlayerID == playerID {
			removed = true
			continue
		}
		filtered = append(filtered, queuedPlayerID)
	}
	h.queues[queueKey] = filtered
	return removed
}

func (h *hub) isQueuedLocked(playerID string, queueKey string) bool {
	for _, queuedPlayerID := range h.queues[queueKey] {
		if queuedPlayerID == playerID {
			return true
		}
	}
	return false
}

func (h *hub) sendLobby(currentPlayer *player) {
	h.mu.Lock()
	message := outboundMessage{
		player: currentPlayer,
		body: serverMessage{
			Type:  "lobby_update",
			Games: h.gameSummariesLocked(),
		},
	}
	h.mu.Unlock()
	flushMessages([]outboundMessage{message})
}

func (h *hub) prependLobbyBroadcastLocked(messages *[]outboundMessage) {
	games := h.gameSummariesLocked()
	lobbyMessages := make([]outboundMessage, 0, len(h.players))
	for _, currentPlayer := range h.players {
		lobbyMessages = append(lobbyMessages, outboundMessage{
			player: currentPlayer,
			body: serverMessage{
				Type:  "lobby_update",
				Games: games,
			},
		})
	}
	*messages = append(lobbyMessages, *messages...)
}

func (h *hub) gameSummariesLocked() []gameSummary {
	games := make([]gameSummary, 0, len(h.queues)+len(h.rooms))

	for queueKey := range h.queues {
		h.queues[queueKey] = h.compactQueueLocked(queueKey)
		queue := h.queues[queueKey]
		if len(queue) == 0 {
			continue
		}

		firstPlayer := h.players[queue[0]]
		if firstPlayer == nil {
			continue
		}

		players := append([]string(nil), queue...)
		games = append(games, gameSummary{
			ID:                queueKey,
			Status:            "waiting",
			GameType:          firstPlayer.gameType,
			PlayerCount:       firstPlayer.playerCount,
			JoinedPlayerCount: len(players),
			Players:           players,
		})
	}

	for _, currentRoom := range h.rooms {
		players := append([]string(nil), currentRoom.playerIDs...)
		games = append(games, gameSummary{
			ID:                currentRoom.id,
			Status:            "started",
			GameType:          currentRoom.gameType,
			PlayerCount:       currentRoom.playerCount,
			JoinedPlayerCount: len(players),
			Players:           players,
		})
	}

	sort.Slice(games, func(i, j int) bool {
		if games[i].Status != games[j].Status {
			return games[i].Status > games[j].Status
		}
		if games[i].GameType != games[j].GameType {
			return games[i].GameType < games[j].GameType
		}
		if games[i].PlayerCount != games[j].PlayerCount {
			return games[i].PlayerCount < games[j].PlayerCount
		}
		return games[i].ID < games[j].ID
	})

	return games
}

func normalizePlayerCount(requestedPlayerCount int) int {
	if requestedPlayerCount == 0 {
		return defaultPlayerCount
	}
	if requestedPlayerCount < defaultPlayerCount || requestedPlayerCount > maxPlayerCount {
		return 0
	}
	return requestedPlayerCount
}

func createQueueKey(gameType string, playerCount int) string {
	return fmt.Sprintf("%s:%d", gameType, playerCount)
}

func errorMessage(currentPlayer *player, message string) outboundMessage {
	return outboundMessage{
		player: currentPlayer,
		body: serverMessage{
			Type:    "error",
			Message: message,
		},
	}
}

func flushMessages(messages []outboundMessage) {
	for _, message := range messages {
		send(message.player, message.body)
	}
}

func send(currentPlayer *player, message serverMessage) {
	if currentPlayer == nil {
		return
	}

	currentPlayer.writeMu.Lock()
	defer currentPlayer.writeMu.Unlock()

	if err := currentPlayer.conn.WriteJSON(message); err != nil {
		log.Printf("send to %s failed: %v", currentPlayer.id, err)
	}
}

func createID(prefix string) string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(bytes)
}
