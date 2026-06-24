package main

import "fmt"

func (h *hub) createGame(currentPlayer *player, gameType string, requestedGameMode string, requestedTeamCount int, requestedPlayerCount int) {
	h.mu.Lock()
	var messages []outboundMessage

	if gameType == "" {
		messages = append(messages, errorMessage(currentPlayer, "gameType is required to create a game."))
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
	gameMode, teamCount, err := normalizeGameMode(requestedGameMode, playerCount)
	if err != nil {
		messages = append(messages, errorMessage(currentPlayer, err.Error()))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}
	if requestedTeamCount != 0 && requestedTeamCount != teamCount {
		messages = append(messages, errorMessage(currentPlayer, fmt.Sprintf("teamCount must be %d for %s games.", teamCount, gameMode)))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}

	if currentPlayer.roomID != "" {
		h.leaveRoomLocked(currentPlayer, "created_new_room", &messages)
	}

	h.createWaitingRoomLocked(currentPlayer, gameType, gameMode, teamCount, playerCount, &messages)
	h.prependLobbyBroadcastLocked(&messages)
	h.mu.Unlock()
	flushMessages(messages)
}

func (h *hub) joinRoom(currentPlayer *player, roomID string) {
	h.mu.Lock()
	var messages []outboundMessage

	if roomID == "" {
		messages = append(messages, errorMessage(currentPlayer, "roomId is required to join a game."))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}

	currentRoom := h.rooms[roomID]
	if currentRoom == nil {
		messages = append(messages, errorMessage(currentPlayer, "That waiting game no longer exists."))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}
	if currentRoom.game != nil {
		messages = append(messages, errorMessage(currentPlayer, "That game has already started."))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}
	if len(currentRoom.playerIDs) >= currentRoom.playerCount {
		messages = append(messages, errorMessage(currentPlayer, "That waiting game is already full."))
		h.mu.Unlock()
		flushMessages(messages)
		return
	}
	if containsPlayerID(currentRoom.playerIDs, currentPlayer.id) {
		messages = append(messages, outboundMessage{
			player: currentPlayer,
			body: serverMessage{
				Type:        "room_waiting",
				PlayerID:    currentPlayer.id,
				RoomID:      currentRoom.id,
				GameType:    currentRoom.gameType,
				GameMode:    currentRoom.gameMode,
				TeamCount:   currentRoom.teamCount,
				PlayerCount: currentRoom.playerCount,
				Players:     append([]string(nil), currentRoom.playerIDs...),
			},
		})
		h.mu.Unlock()
		flushMessages(messages)
		return
	}

	if currentPlayer.roomID != "" {
		h.leaveRoomLocked(currentPlayer, "joined_room", &messages)
	}

	currentRoom.playerIDs = append(currentRoom.playerIDs, currentPlayer.id)
	currentPlayer.roomID = currentRoom.id
	currentPlayer.gameType = currentRoom.gameType
	currentPlayer.gameMode = currentRoom.gameMode
	currentPlayer.teamCount = currentRoom.teamCount
	currentPlayer.playerCount = currentRoom.playerCount

	if len(currentRoom.playerIDs) == currentRoom.playerCount {
		h.startRoomLocked(currentRoom, &messages)
	} else {
		h.appendRoomWaitingMessagesLocked(currentRoom, &messages)
	}

	h.prependLobbyBroadcastLocked(&messages)
	h.mu.Unlock()
	flushMessages(messages)
}

func (h *hub) leaveQueue(currentPlayer *player) {
	h.mu.Lock()
	var messages []outboundMessage

	if currentPlayer.roomID != "" {
		currentRoom := h.rooms[currentPlayer.roomID]
		if currentRoom != nil && currentRoom.game == nil {
			h.leaveRoomLocked(currentPlayer, "left_waiting_room", &messages)
			h.prependLobbyBroadcastLocked(&messages)
			h.mu.Unlock()
			flushMessages(messages)
			return
		}
	}

	messages = append(messages, outboundMessage{
		player: currentPlayer,
		body:   serverMessage{Type: "not_queued"},
	})
	h.mu.Unlock()
	flushMessages(messages)
}

func (h *hub) createWaitingRoomLocked(
	currentPlayer *player,
	gameType string,
	gameMode string,
	teamCount int,
	playerCount int,
	messages *[]outboundMessage,
) {
	roomID := createID("room")
	currentRoom := &room{
		id:          roomID,
		gameType:    gameType,
		gameMode:    gameMode,
		teamCount:   teamCount,
		playerCount: playerCount,
		playerIDs:   []string{currentPlayer.id},
		game:        nil,
	}

	h.rooms[roomID] = currentRoom
	currentPlayer.roomID = roomID
	currentPlayer.gameType = gameType
	currentPlayer.gameMode = gameMode
	currentPlayer.teamCount = teamCount
	currentPlayer.playerCount = playerCount

	h.appendRoomWaitingMessagesLocked(currentRoom, messages)
}

func (h *hub) appendRoomWaitingMessagesLocked(currentRoom *room, messages *[]outboundMessage) {
	players := append([]string(nil), currentRoom.playerIDs...)
	for _, playerID := range currentRoom.playerIDs {
		currentPlayer := h.players[playerID]
		if currentPlayer == nil || currentPlayer.conn == nil {
			continue
		}
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
			},
		})
	}
}

func (h *hub) startRoomLocked(currentRoom *room, messages *[]outboundMessage) {
	currentRoom.game = newGameSession(
		currentRoom.id,
		currentRoom.gameType,
		append([]string(nil), currentRoom.playerIDs...),
		currentRoom.gameMode,
		currentRoom.teamCount,
	)

	players := append([]string(nil), currentRoom.playerIDs...)
	for _, playerID := range currentRoom.playerIDs {
		currentPlayer := h.players[playerID]
		if currentPlayer == nil || currentPlayer.conn == nil {
			continue
		}
		*messages = append(*messages, outboundMessage{
			player: currentPlayer,
			body: serverMessage{
				Type:        "room_created",
				PlayerID:    currentPlayer.id,
				RoomID:      currentRoom.id,
				GameType:    currentRoom.gameType,
				GameMode:    currentRoom.gameMode,
				TeamCount:   currentRoom.teamCount,
				PlayerCount: currentRoom.playerCount,
				Players:     players,
				Payload:     marshalPayload(currentRoom.game),
			},
		})
	}
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

func containsPlayerID(playerIDs []string, playerID string) bool {
	for _, candidate := range playerIDs {
		if candidate == playerID {
			return true
		}
	}
	return false
}
