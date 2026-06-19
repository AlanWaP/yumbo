package main

import "fmt"

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
