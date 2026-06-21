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
	"time"

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
	id                string
	conn              jsonWriter
	roomID            string
	gameType          string
	gameMode          string
	teamCount         int
	playerCount       int
	queueKey          string
	writeMu           sync.Mutex
	disconnectTimer   *time.Timer
	refreshDisconnect bool
}

type room struct {
	id          string
	gameType    string
	gameMode    string
	teamCount   int
	playerCount int
	playerIDs   []string
	game        *gameSession
}

type clientMessage struct {
	Type        string          `json:"type"`
	GameType    string          `json:"gameType,omitempty"`
	GameMode    string          `json:"gameMode,omitempty"`
	TeamCount   int             `json:"teamCount,omitempty"`
	PlayerCount int             `json:"playerCount,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

type serverMessage struct {
	Type        string          `json:"type"`
	PlayerID    string          `json:"playerId,omitempty"`
	RoomID      string          `json:"roomId,omitempty"`
	GameType    string          `json:"gameType,omitempty"`
	GameMode    string          `json:"gameMode,omitempty"`
	TeamCount   int             `json:"teamCount,omitempty"`
	PlayerCount int             `json:"playerCount,omitempty"`
	Players     []string        `json:"players,omitempty"`
	Games       []gameSummary   `json:"games,omitempty"`
	Message     string          `json:"message,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Restored    bool            `json:"restored,omitempty"`
	Payload     json.RawMessage `json:"payload,omitempty"`
}

type gameSummary struct {
	ID                string   `json:"id"`
	Status            string   `json:"status"`
	GameType          string   `json:"gameType"`
	GameMode          string   `json:"gameMode"`
	TeamCount         int      `json:"teamCount,omitempty"`
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

	currentPlayer, reconnected := gameHub.registerPlayer(conn, r.URL.Query().Get("playerId"))
	send(currentPlayer, serverMessage{
		Type:     "connected",
		PlayerID: currentPlayer.id,
	})
	if reconnected {
		gameHub.sendSessionRestore(currentPlayer)
	}
	gameHub.sendLobby(currentPlayer)

	defer gameHub.detachPlayer(currentPlayer, conn)

	for {
		var message clientMessage
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		gameHub.handleMessage(currentPlayer, message)
	}
}

func (h *hub) handleMessage(currentPlayer *player, message clientMessage) {
	switch message.Type {
	case "join_queue":
		h.joinQueue(currentPlayer, message.GameType, message.GameMode, message.TeamCount, message.PlayerCount)
	case "leave_queue":
		h.leaveQueue(currentPlayer)
	case "leave_room":
		h.leaveRoom(currentPlayer, "left_room")
	case "request_lobby":
		h.sendLobby(currentPlayer)
	case "game_move":
		h.handleGameMove(currentPlayer, message.Payload)
	case "cancel_move":
		h.handleCancelMove(currentPlayer)
	case "refresh_pending":
		h.markRefreshPending(currentPlayer)
	case "leave_session":
		h.leaveSession(currentPlayer)
	case "room_message":
		h.relayRoomMessage(currentPlayer, message.Payload)
	default:
		flushMessages([]outboundMessage{errorMessage(currentPlayer, "Unknown message type: "+message.Type)})
	}
}

func (h *hub) leaveRoom(currentPlayer *player, reason string) {
	h.mu.Lock()
	var messages []outboundMessage
	h.leaveRoomLocked(currentPlayer, reason, &messages)
	h.prependLobbyBroadcastLocked(&messages)
	h.mu.Unlock()
	flushMessages(messages)
}

func (h *hub) handleGameMove(currentPlayer *player, payload json.RawMessage) {
	h.mu.Lock()
	var messages []outboundMessage

	currentRoom := h.rooms[currentPlayer.roomID]
	if currentRoom == nil {
		messages = append(messages, errorMessage(currentPlayer, "You are not in a room."))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}

	receipt, shouldResolve, err := currentRoom.game.submitMove(currentPlayer.id, payload)
	if err != nil {
		messages = append(messages, errorMessage(currentPlayer, err.Error()))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}

	messages = append(messages, outboundMessage{
		player: currentPlayer,
		body: serverMessage{
			Type:        "game_move_accepted",
			PlayerID:    currentPlayer.id,
			RoomID:      currentRoom.id,
			GameType:    currentRoom.gameType,
			GameMode:    currentRoom.gameMode,
			TeamCount:   currentRoom.teamCount,
			PlayerCount: currentRoom.playerCount,
			Payload:     marshalPayload(receipt),
		},
	})

	if shouldResolve {
		currentRoom.game.resolveRound()
		messageType := "round_resolved"
		if currentRoom.game.Phase == gamePhaseFinished {
			messageType = "game_finished"
		}
		h.appendGameBroadcastLocked(currentRoom, messageType, &messages)
	} else {
		h.appendGameBroadcastLocked(currentRoom, "game_state", &messages)
	}

	h.mu.Unlock()
	flushMessages(messages)
}

func (h *hub) handleCancelMove(currentPlayer *player) {
	h.mu.Lock()
	var messages []outboundMessage

	currentRoom := h.rooms[currentPlayer.roomID]
	if currentRoom == nil {
		messages = append(messages, errorMessage(currentPlayer, "You are not in a room."))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}

	receipt, err := currentRoom.game.cancelMove(currentPlayer.id)
	if err != nil {
		messages = append(messages, errorMessage(currentPlayer, err.Error()))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}

	messages = append(messages, outboundMessage{
		player: currentPlayer,
		body: serverMessage{
			Type:        "game_move_cancelled",
			PlayerID:    currentPlayer.id,
			RoomID:      currentRoom.id,
			GameType:    currentRoom.gameType,
			GameMode:    currentRoom.gameMode,
			TeamCount:   currentRoom.teamCount,
			PlayerCount: currentRoom.playerCount,
			Payload:     marshalPayload(receipt),
		},
	})
	h.appendGameBroadcastLocked(currentRoom, "game_state", &messages)

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

	h.cancelPlayerCleanupLocked(currentPlayer)

	if currentPlayer.queueKey != "" {
		h.removeFromQueueLocked(currentPlayer.id, currentPlayer.queueKey)
	}
	h.leaveRoomLocked(currentPlayer, "peer_disconnected", &messages)
	currentPlayer.gameType = ""
	currentPlayer.gameMode = ""
	currentPlayer.teamCount = 0
	currentPlayer.playerCount = 0
	currentPlayer.queueKey = ""
	currentPlayer.roomID = ""
	delete(h.players, currentPlayer.id)
	h.prependLobbyBroadcastLocked(&messages)

	h.mu.Unlock()
	flushMessages(messages)
	closeConn(currentPlayer.conn)
}

func (h *hub) createRoomLocked(gameType string, gameMode string, teamCount int, playerCount int, roomPlayers []*player, messages *[]outboundMessage) {
	roomID := createID("room")
	playerIDs := make([]string, 0, len(roomPlayers))
	for _, currentPlayer := range roomPlayers {
		playerIDs = append(playerIDs, currentPlayer.id)
	}

	currentRoom := &room{
		id:          roomID,
		gameType:    gameType,
		gameMode:    gameMode,
		teamCount:   teamCount,
		playerCount: playerCount,
		playerIDs:   playerIDs,
		game:        newGameSession(roomID, gameType, playerIDs, gameMode, teamCount),
	}

	h.rooms[roomID] = currentRoom
	for _, currentPlayer := range roomPlayers {
		currentPlayer.roomID = roomID
		currentPlayer.gameType = gameType
		currentPlayer.gameMode = gameMode
		currentPlayer.teamCount = teamCount
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
				GameMode:    gameMode,
				TeamCount:   teamCount,
				PlayerCount: playerCount,
				Players:     players,
				Payload:     marshalPayload(currentRoom.game),
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
		roomPlayer.gameMode = ""
		roomPlayer.teamCount = 0
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
				GameMode:    currentRoom.gameMode,
				TeamCount:   currentRoom.teamCount,
				PlayerCount: currentRoom.playerCount,
				Reason:      reason,
			},
		})
	}
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
			GameMode:          firstPlayer.gameMode,
			TeamCount:         firstPlayer.teamCount,
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
			GameMode:          currentRoom.gameMode,
			TeamCount:         currentRoom.teamCount,
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

func (h *hub) appendGameBroadcastLocked(currentRoom *room, messageType string, messages *[]outboundMessage) {
	for _, playerID := range currentRoom.playerIDs {
		if recipient := h.players[playerID]; recipient != nil {
			*messages = append(*messages, outboundMessage{
				player: recipient,
				body: serverMessage{
					Type:        messageType,
					RoomID:      currentRoom.id,
					GameType:    currentRoom.gameType,
					GameMode:    currentRoom.gameMode,
					TeamCount:   currentRoom.teamCount,
					PlayerCount: currentRoom.playerCount,
					Payload:     marshalPayload(currentRoom.game),
				},
			})
		}
	}
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
	if currentPlayer == nil || currentPlayer.conn == nil {
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
