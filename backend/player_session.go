package main

import (
	"regexp"
	"time"
)

const disconnectGracePeriod = 5 * time.Minute

var playerIDPattern = regexp.MustCompile(`^player_[0-9a-f]{8}$`)

func resolvePlayerID(requestedID string) string {
	if playerIDPattern.MatchString(requestedID) {
		return requestedID
	}
	return createID("player")
}

func (h *hub) registerPlayer(conn jsonWriter, requestedID string) (*player, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	playerID := resolvePlayerID(requestedID)
	if existing := h.players[playerID]; existing != nil {
		h.cancelPlayerCleanupLocked(existing)
		oldConn := existing.conn
		existing.conn = conn
		if oldConn != nil && oldConn != conn {
			go closeConn(oldConn)
		}
		return existing, true
	}

	currentPlayer := &player{
		id:   playerID,
		conn: conn,
	}
	h.players[playerID] = currentPlayer
	return currentPlayer, false
}

func (h *hub) detachPlayer(currentPlayer *player, conn jsonWriter) {
	h.mu.Lock()

	storedPlayer, exists := h.players[currentPlayer.id]
	if !exists || storedPlayer != currentPlayer {
		h.mu.Unlock()
		closeConn(conn)
		return
	}

	if currentPlayer.conn != conn {
		h.mu.Unlock()
		return
	}

	if !currentPlayer.refreshDisconnect {
		h.mu.Unlock()
		h.removePlayer(currentPlayer)
		return
	}

	currentPlayer.conn = nil
	var messages []outboundMessage

	if currentPlayer.queueKey == "" && currentPlayer.roomID == "" {
		delete(h.players, currentPlayer.id)
		h.mu.Unlock()
		closeConn(conn)
		return
	}

	if currentPlayer.roomID != "" {
		h.notifyPeerConnectionLocked(currentPlayer, "peer_disconnected", &messages)
	}

	h.schedulePlayerCleanupLocked(currentPlayer)
	h.mu.Unlock()

	flushMessages(messages)
	closeConn(conn)
}

func (h *hub) markRefreshPending(currentPlayer *player) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if storedPlayer, exists := h.players[currentPlayer.id]; exists && storedPlayer == currentPlayer {
		currentPlayer.refreshDisconnect = true
	}
}

func (h *hub) leaveSession(currentPlayer *player) {
	h.removePlayer(currentPlayer)
}

func (h *hub) sendSessionRestore(currentPlayer *player) {
	h.mu.Lock()
	var messages []outboundMessage
	h.appendSessionRestoreLocked(currentPlayer, &messages)

	if currentPlayer.roomID != "" {
		h.notifyPeerConnectionLocked(currentPlayer, "peer_reconnected", &messages)
	}

	h.mu.Unlock()
	flushMessages(messages)
}

func (h *hub) appendSessionRestoreLocked(currentPlayer *player, messages *[]outboundMessage) {
	currentRoom := h.rooms[currentPlayer.roomID]
	if currentRoom == nil {
		currentPlayer.roomID = ""
		currentPlayer.gameType = ""
		currentPlayer.gameMode = ""
		currentPlayer.teamCount = 0
		currentPlayer.playerCount = 0
		return
	}

	players := append([]string(nil), currentRoom.playerIDs...)
	if currentRoom.game == nil {
		*messages = append(*messages, outboundMessage{
			player: currentPlayer,
			body: serverMessage{
				Type:        "room_waiting",
				PlayerID:    currentPlayer.id,
				RoomID:      currentRoom.id,
				GameType:    currentRoom.gameType,
				GameMode:    currentRoom.gameMode,
				TeamCount:   currentRoom.teamCount,
				PlayerCount: currentRoom.playerCount,
				Players:     players,
				Restored:    true,
			},
		})
		return
	}

	*messages = append(*messages, outboundMessage{
		player: currentPlayer,
		body: serverMessage{
			Type:          "room_created",
			PlayerID:      currentPlayer.id,
			RoomID:        currentRoom.id,
			GameType:      currentRoom.gameType,
			GameMode:      currentRoom.gameMode,
			TeamCount:     currentRoom.teamCount,
			PlayerCount:   currentRoom.playerCount,
			Players:       players,
			Restored:      true,
			SubmittedMove: pendingMoveForPlayer(currentRoom.game, currentPlayer.id),
			Payload:       marshalPayload(currentRoom.game),
		},
	})
}

func (h *hub) notifyPeerConnectionLocked(currentPlayer *player, messageType string, messages *[]outboundMessage) {
	currentRoom := h.rooms[currentPlayer.roomID]
	if currentRoom == nil {
		return
	}

	for _, playerID := range currentRoom.playerIDs {
		if playerID == currentPlayer.id {
			continue
		}
		if recipient := h.players[playerID]; recipient != nil && recipient.conn != nil {
			*messages = append(*messages, outboundMessage{
				player: recipient,
				body: serverMessage{
					Type:        messageType,
					PlayerID:    currentPlayer.id,
					RoomID:      currentRoom.id,
					GameType:    currentRoom.gameType,
					GameMode:    currentRoom.gameMode,
					TeamCount:   currentRoom.teamCount,
					PlayerCount: currentRoom.playerCount,
				},
			})
		}
	}
}

func (h *hub) schedulePlayerCleanupLocked(currentPlayer *player) {
	h.cancelPlayerCleanupLocked(currentPlayer)
	currentPlayer.disconnectTimer = time.AfterFunc(disconnectGracePeriod, func() {
		h.expirePlayerSession(currentPlayer.id)
	})
}

func (h *hub) cancelPlayerCleanupLocked(currentPlayer *player) {
	if currentPlayer.disconnectTimer != nil {
		currentPlayer.disconnectTimer.Stop()
		currentPlayer.disconnectTimer = nil
	}
}

func (h *hub) expirePlayerSession(playerID string) {
	h.mu.Lock()
	var messages []outboundMessage

	currentPlayer := h.players[playerID]
	if currentPlayer == nil || currentPlayer.conn != nil {
		h.mu.Unlock()
		return
	}

	h.cancelPlayerCleanupLocked(currentPlayer)

	if currentPlayer.queueKey != "" {
		currentPlayer.queueKey = ""
	}
	h.leaveRoomLocked(currentPlayer, "session_expired", &messages)
	currentPlayer.gameType = ""
	currentPlayer.gameMode = ""
	currentPlayer.teamCount = 0
	currentPlayer.playerCount = 0
	currentPlayer.queueKey = ""
	currentPlayer.roomID = ""
	delete(h.players, playerID)
	h.prependLobbyBroadcastLocked(&messages)

	h.mu.Unlock()
	flushMessages(messages)
}

func closeConn(conn jsonWriter) {
	if closer, ok := conn.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}
