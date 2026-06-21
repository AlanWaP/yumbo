package main

import (
	"encoding/json"
	"testing"
)

type recordingConn struct {
	messages []serverMessage
	closed   bool
}

func (c *recordingConn) WriteJSON(v any) error {
	bytes, err := json.Marshal(v)
	if err != nil {
		return err
	}

	var message serverMessage
	if err := json.Unmarshal(bytes, &message); err != nil {
		return err
	}

	c.messages = append(c.messages, message)
	return nil
}

func (c *recordingConn) Close() error {
	c.closed = true
	return nil
}

func addTestPlayer(h *hub, id string) (*player, *recordingConn) {
	conn := &recordingConn{}
	currentPlayer := &player{
		id:   id,
		conn: conn,
	}
	h.players[id] = currentPlayer
	return currentPlayer, conn
}

func lastMessage(t *testing.T, conn *recordingConn) serverMessage {
	t.Helper()

	for i := len(conn.messages) - 1; i >= 0; i-- {
		if conn.messages[i].Type != "lobby_update" {
			return conn.messages[i]
		}
	}
	t.Fatal("expected at least one non-lobby message")
	return serverMessage{}
}

func lastLobbyMessage(t *testing.T, conn *recordingConn) serverMessage {
	t.Helper()

	for i := len(conn.messages) - 1; i >= 0; i-- {
		if conn.messages[i].Type == "lobby_update" {
			return conn.messages[i]
		}
	}
	t.Fatal("expected at least one lobby message")
	return serverMessage{}
}

func messageOfType(t *testing.T, conn *recordingConn, messageType string) serverMessage {
	t.Helper()

	for i := len(conn.messages) - 1; i >= 0; i-- {
		if conn.messages[i].Type == messageType {
			return conn.messages[i]
		}
	}
	t.Fatalf("expected message type %q, got %#v", messageType, conn.messages)
	return serverMessage{}
}

func decodePayload[T any](t *testing.T, payload json.RawMessage) T {
	t.Helper()

	var decoded T
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	return decoded
}

func TestJoinQueueRequiresGameType(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue"})

	message := lastMessage(t, playerOneConn)
	if message.Type != "error" {
		t.Fatalf("expected error, got %q", message.Type)
	}
	if playerOne.gameType != "" {
		t.Fatalf("expected player to remain outside queues, got game type %q", playerOne.gameType)
	}
}

func TestJoinQueueRejectsInvalidPlayerCount(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps", PlayerCount: 1})

	message := lastMessage(t, playerOneConn)
	if message.Type != "error" {
		t.Fatalf("expected error, got %q", message.Type)
	}
	if playerOne.queueKey != "" {
		t.Fatalf("expected player to remain outside queues, got queue key %q", playerOne.queueKey)
	}
}

func TestSameGameTypePlayersMatchIntoRoom(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})

	if len(gameHub.rooms) != 1 {
		t.Fatalf("expected one room, got %d", len(gameHub.rooms))
	}
	if len(gameHub.queues["rps:2"]) != 0 {
		t.Fatalf("expected rps two-player queue to be empty, got %v", gameHub.queues["rps:2"])
	}
	if playerOne.roomID == "" || playerTwo.roomID == "" || playerOne.roomID != playerTwo.roomID {
		t.Fatalf("expected players to share a room, got %q and %q", playerOne.roomID, playerTwo.roomID)
	}

	playerOneMessage := lastMessage(t, playerOneConn)
	playerTwoMessage := lastMessage(t, playerTwoConn)
	if playerOneMessage.Type != "room_created" || playerTwoMessage.Type != "room_created" {
		t.Fatalf("expected room_created messages, got %q and %q", playerOneMessage.Type, playerTwoMessage.Type)
	}
	if playerOneMessage.GameType != "rps" || playerTwoMessage.GameType != "rps" {
		t.Fatalf("expected rps game type, got %q and %q", playerOneMessage.GameType, playerTwoMessage.GameType)
	}
	if playerOneMessage.PlayerCount != 2 || playerTwoMessage.PlayerCount != 2 {
		t.Fatalf("expected default player count 2, got %d and %d", playerOneMessage.PlayerCount, playerTwoMessage.PlayerCount)
	}
}

func TestThreePlayersMatchIntoRoom(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")
	playerThree, playerThreeConn := addTestPlayer(gameHub, "player_three")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "cards", PlayerCount: 3})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "cards", PlayerCount: 3})

	if len(gameHub.rooms) != 0 {
		t.Fatalf("expected no room until third player joins, got %d", len(gameHub.rooms))
	}

	gameHub.handleMessage(playerThree, clientMessage{Type: "join_queue", GameType: "cards", PlayerCount: 3})

	if len(gameHub.rooms) != 1 {
		t.Fatalf("expected one room, got %d", len(gameHub.rooms))
	}
	if playerOne.roomID == "" || playerOne.roomID != playerTwo.roomID || playerTwo.roomID != playerThree.roomID {
		t.Fatalf("expected all players to share one room, got %q, %q, %q", playerOne.roomID, playerTwo.roomID, playerThree.roomID)
	}
	for _, conn := range []*recordingConn{playerOneConn, playerTwoConn, playerThreeConn} {
		message := lastMessage(t, conn)
		if message.Type != "room_created" {
			t.Fatalf("expected room_created, got %q", message.Type)
		}
		if message.PlayerCount != 3 {
			t.Fatalf("expected player count 3, got %d", message.PlayerCount)
		}
		if len(message.Players) != 3 {
			t.Fatalf("expected three players in message, got %v", message.Players)
		}
	}
}

func TestSameGameTypeDifferentPlayerCountsDoNotMatch(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "cards", PlayerCount: 3})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "cards", PlayerCount: 2})

	if len(gameHub.rooms) != 0 {
		t.Fatalf("expected no rooms, got %d", len(gameHub.rooms))
	}
	if got := lastMessage(t, playerOneConn).Type; got != "queued" {
		t.Fatalf("expected first player to remain queued, got %q", got)
	}
	if got := lastMessage(t, playerTwoConn).Type; got != "queued" {
		t.Fatalf("expected second player to remain queued, got %q", got)
	}
}

func TestDifferentGameTypesDoNotMatch(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "cards"})

	if len(gameHub.rooms) != 0 {
		t.Fatalf("expected no rooms, got %d", len(gameHub.rooms))
	}
	if got := lastMessage(t, playerOneConn).Type; got != "queued" {
		t.Fatalf("expected first player to remain queued, got %q", got)
	}
	if got := lastMessage(t, playerTwoConn).Type; got != "queued" {
		t.Fatalf("expected second player to remain queued, got %q", got)
	}
}

func TestDifferentGameModesDoNotMatch(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")
	playerThree, playerThreeConn := addTestPlayer(gameHub, "player_three")
	playerFour, playerFourConn := addTestPlayer(gameHub, "player_four")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps", GameMode: gameModeFreeForAll, PlayerCount: 4})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps", GameMode: gameModeTeam, PlayerCount: 4})
	gameHub.handleMessage(playerThree, clientMessage{Type: "join_queue", GameType: "rps", GameMode: gameModeFreeForAll, PlayerCount: 4})
	gameHub.handleMessage(playerFour, clientMessage{Type: "join_queue", GameType: "rps", GameMode: gameModeTeam, PlayerCount: 4})

	if len(gameHub.rooms) != 0 {
		t.Fatalf("expected no rooms for mixed game modes, got %d", len(gameHub.rooms))
	}
	for _, conn := range []*recordingConn{playerOneConn, playerTwoConn, playerThreeConn, playerFourConn} {
		if got := lastMessage(t, conn).Type; got != "queued" {
			t.Fatalf("expected player to remain queued, got %q", got)
		}
	}
}

func TestTeamQueueRequiresEvenPlayerCount(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps", GameMode: gameModeTeam, PlayerCount: 3})

	message := lastMessage(t, playerOneConn)
	if message.Type != "error" {
		t.Fatalf("expected error, got %q", message.Type)
	}
	if playerOne.queueKey != "" {
		t.Fatalf("expected player to remain outside queues, got queue key %q", playerOne.queueKey)
	}
}

func TestLeaveQueueRemovesPlayer(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, _ := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerOne, clientMessage{Type: "leave_queue"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})

	if len(gameHub.rooms) != 0 {
		t.Fatalf("expected no room after first player left queue, got %d", len(gameHub.rooms))
	}
	if playerOne.gameType != "" {
		t.Fatalf("expected first player game type to clear, got %q", playerOne.gameType)
	}
	if playerOne.queueKey != "" {
		t.Fatalf("expected first player queue key to clear, got %q", playerOne.queueKey)
	}

	message := lastMessage(t, playerOneConn)
	if message.Type != "queue_left" {
		t.Fatalf("expected queue_left, got %q", message.Type)
	}
}

func TestLobbyListsWaitingQueuesAndStartedRooms(t *testing.T) {
	gameHub := newHub()
	waitingPlayer, waitingConn := addTestPlayer(gameHub, "waiting_player")
	playerOne, _ := addTestPlayer(gameHub, "player_one")
	playerTwo, _ := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(waitingPlayer, clientMessage{Type: "join_queue", GameType: "cards", PlayerCount: 3})
	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})

	message := lastLobbyMessage(t, waitingConn)
	if len(message.Games) != 2 {
		t.Fatalf("expected waiting queue and started room, got %#v", message.Games)
	}

	var waitingGame, startedGame *gameSummary
	for i := range message.Games {
		switch message.Games[i].Status {
		case "waiting":
			waitingGame = &message.Games[i]
		case "started":
			startedGame = &message.Games[i]
		}
	}

	if waitingGame == nil {
		t.Fatal("expected waiting game in lobby")
	}
	if waitingGame.GameType != "cards" || waitingGame.PlayerCount != 3 || waitingGame.JoinedPlayerCount != 1 {
		t.Fatalf("unexpected waiting game summary: %#v", *waitingGame)
	}

	if startedGame == nil {
		t.Fatal("expected started game in lobby")
	}
	if startedGame.GameType != "rps" || startedGame.PlayerCount != 2 || startedGame.JoinedPlayerCount != 2 {
		t.Fatalf("unexpected started game summary: %#v", *startedGame)
	}
}

func TestLeaveRoomNotifiesPeerAndClearsRoom(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerOne, clientMessage{Type: "leave_room"})

	if len(gameHub.rooms) != 0 {
		t.Fatalf("expected room to be removed, got %d rooms", len(gameHub.rooms))
	}
	if playerOne.roomID != "" || playerTwo.roomID != "" {
		t.Fatalf("expected room IDs to clear, got %q and %q", playerOne.roomID, playerTwo.roomID)
	}
	if got := lastMessage(t, playerOneConn).Type; got != "room_left" {
		t.Fatalf("expected leaving player to get room_left, got %q", got)
	}
	if got := lastMessage(t, playerTwoConn).Type; got != "peer_left" {
		t.Fatalf("expected peer to get peer_left, got %q", got)
	}
}

func TestRoomMessageRelaysPayloadToPeer(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})
	playerOneConn.messages = nil
	playerTwoConn.messages = nil

	payload := json.RawMessage(`{"action":"future-game-action"}`)
	gameHub.handleMessage(playerOne, clientMessage{Type: "room_message", Payload: payload})

	if len(playerOneConn.messages) != 0 {
		t.Fatalf("expected sender not to receive its own relay, got %d messages", len(playerOneConn.messages))
	}
	message := lastMessage(t, playerTwoConn)
	if message.Type != "room_message" {
		t.Fatalf("expected room_message, got %q", message.Type)
	}
	if string(message.Payload) != string(payload) {
		t.Fatalf("expected payload %s, got %s", payload, message.Payload)
	}
}

func TestRoomCreatedIncludesInitialGameState(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})

	for _, conn := range []*recordingConn{playerOneConn, playerTwoConn} {
		message := lastMessage(t, conn)
		if message.Type != "room_created" {
			t.Fatalf("expected room_created, got %q", message.Type)
		}
		gameState := decodePayload[gameSession](t, message.Payload)
		if gameState.Round != 1 || gameState.Phase != gamePhaseWaitingForMoves {
			t.Fatalf("unexpected game state: round=%d phase=%q", gameState.Round, gameState.Phase)
		}
		if len(gameState.Players) != 2 {
			t.Fatalf("expected two game players, got %d", len(gameState.Players))
		}
	}
}

func TestGameMoveAcceptedAndRoundResolved(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})
	playerOneConn.messages = nil
	playerTwoConn.messages = nil

	playerOneMove := json.RawMessage(`{"moveType":"attack","targetId":"player_two"}`)
	playerTwoMove := json.RawMessage(`{"moveType":"gain_power"}`)
	gameHub.handleMessage(playerOne, clientMessage{Type: "game_move", Payload: playerOneMove})

	if got := messageOfType(t, playerOneConn, "game_move_accepted").Type; got != "game_move_accepted" {
		t.Fatalf("expected move acceptance for first player, got %q", got)
	}
	if got := lastMessage(t, playerTwoConn).Type; got != "game_state" {
		t.Fatalf("expected game state broadcast while waiting, got %q", got)
	}

	gameHub.handleMessage(playerTwo, clientMessage{Type: "game_move", Payload: playerTwoMove})

	if got := messageOfType(t, playerTwoConn, "game_move_accepted").Type; got != "game_move_accepted" {
		t.Fatalf("expected move acceptance for second player, got %q", got)
	}
	resolved := messageOfType(t, playerOneConn, "round_resolved")
	gameState := decodePayload[gameSession](t, resolved.Payload)
	if gameState.Round != 2 {
		t.Fatalf("expected round two after resolution, got %d", gameState.Round)
	}
	if got := gameState.Players["player_two"].Health; got != 8 {
		t.Fatalf("expected player_two to take attack damage, got health %d", got)
	}
}

func TestRemovePlayerCleansQueueAndNotifiesRoomPeer(t *testing.T) {
	gameHub := newHub()
	queuedPlayer, _ := addTestPlayer(gameHub, "queued_player")
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(queuedPlayer, clientMessage{Type: "join_queue", GameType: "cards", PlayerCount: 3})
	gameHub.removePlayer(queuedPlayer)
	if len(gameHub.queues["cards:3"]) != 0 {
		t.Fatalf("expected disconnected queued player to be removed, got %v", gameHub.queues["cards:3"])
	}

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})
	playerOneConn.messages = nil
	playerTwoConn.messages = nil

	gameHub.removePlayer(playerOne)

	if len(gameHub.rooms) != 0 {
		t.Fatalf("expected room to be removed, got %d rooms", len(gameHub.rooms))
	}
	if got := lastMessage(t, playerTwoConn).Type; got != "peer_left" {
		t.Fatalf("expected remaining peer to get peer_left, got %q", got)
	}
	if playerTwo.roomID != "" || playerTwo.gameType != "" {
		t.Fatalf("expected remaining peer room state to clear, got room %q game %q", playerTwo.roomID, playerTwo.gameType)
	}
}

func TestDetachPlayerPreservesQueueUntilExpiry(t *testing.T) {
	gameHub := newHub()
	queuedPlayer, queuedConn := addTestPlayer(gameHub, "queued_player")

	gameHub.handleMessage(queuedPlayer, clientMessage{Type: "join_queue", GameType: "cards", PlayerCount: 3})
	queuedPlayer.refreshDisconnect = true
	gameHub.detachPlayer(queuedPlayer, queuedConn)

	if len(gameHub.queues["cards:3"]) != 1 {
		t.Fatalf("expected queued player to remain reserved, got %v", gameHub.queues["cards:3"])
	}
	if gameHub.players["queued_player"] == nil {
		t.Fatal("expected disconnected player record to remain")
	}
	if gameHub.players["queued_player"].conn != nil {
		t.Fatal("expected player connection to be detached")
	}
}

func TestDetachPlayerPreservesRoomUntilExpiry(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_11111111")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_22222222")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})
	playerOneConn.messages = nil
	playerTwoConn.messages = nil

	playerOne.refreshDisconnect = true
	gameHub.detachPlayer(playerOne, playerOneConn)

	if len(gameHub.rooms) != 1 {
		t.Fatalf("expected room to remain while player reconnects, got %d rooms", len(gameHub.rooms))
	}
	if playerOne.roomID == "" {
		t.Fatal("expected disconnected player to remain assigned to room")
	}
	if got := lastMessage(t, playerTwoConn).Type; got != "peer_disconnected" {
		t.Fatalf("expected peer_disconnected, got %q", got)
	}
	if playerTwo.roomID == "" {
		t.Fatal("expected remaining peer to stay in room")
	}
}

func TestReconnectRestoresQueueSession(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_a1b2c3d4")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.markRefreshPending(playerOne)
	gameHub.detachPlayer(playerOne, playerOneConn)

	reconnectConn := &recordingConn{}
	reconnectedPlayer, reconnected := gameHub.registerPlayer(reconnectConn, "player_a1b2c3d4")
	if !reconnected {
		t.Fatal("expected reconnecting player to reuse existing session")
	}
	if reconnectedPlayer.id != "player_a1b2c3d4" {
		t.Fatalf("expected same player id, got %q", reconnectedPlayer.id)
	}
	gameHub.sendSessionRestore(reconnectedPlayer)

	message := lastMessage(t, reconnectConn)
	if message.Type != "already_queued" || !message.Restored {
		t.Fatalf("expected restored queue session, got %#v", message)
	}
	if message.GameType != "rps" {
		t.Fatalf("expected rps queue to restore, got %q", message.GameType)
	}
}

func TestReconnectRestoresRoomSession(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_11111111")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_22222222")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})
	playerOneConn.messages = nil
	playerTwoConn.messages = nil

	roomID := playerOne.roomID
	gameHub.markRefreshPending(playerOne)
	gameHub.detachPlayer(playerOne, playerOneConn)

	reconnectConn := &recordingConn{}
	reconnectedPlayer, reconnected := gameHub.registerPlayer(reconnectConn, "player_11111111")
	if !reconnected {
		t.Fatal("expected reconnecting player to reuse existing session")
	}
	gameHub.sendSessionRestore(reconnectedPlayer)

	message := lastMessage(t, reconnectConn)
	if message.Type != "room_created" || !message.Restored {
		t.Fatalf("expected restored room session, got %#v", message)
	}
	if message.RoomID != roomID {
		t.Fatalf("expected room %q to restore, got %q", roomID, message.RoomID)
	}
	if got := lastMessage(t, playerTwoConn).Type; got != "peer_reconnected" {
		t.Fatalf("expected peer_reconnected, got %q", got)
	}
}

func TestDetachWithoutRefreshLeavesImmediately(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_11111111")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_22222222")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})
	playerOneConn.messages = nil
	playerTwoConn.messages = nil

	gameHub.detachPlayer(playerOne, playerOneConn)

	if len(gameHub.rooms) != 0 {
		t.Fatalf("expected room to be removed on window close, got %d rooms", len(gameHub.rooms))
	}
	if gameHub.players["player_11111111"] != nil {
		t.Fatal("expected disconnected player to be removed")
	}
	if got := lastMessage(t, playerTwoConn).Type; got != "peer_left" {
		t.Fatalf("expected peer_left, got %q", got)
	}
}

func TestLeaveSessionRemovesPlayerImmediately(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_11111111")
	playerTwo, _ := addTestPlayer(gameHub, "player_22222222")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "cards", PlayerCount: 3})
	gameHub.leaveSession(playerOne)

	if len(gameHub.queues["cards:3"]) != 0 {
		t.Fatalf("expected queue to clear after leave session, got %v", gameHub.queues["cards:3"])
	}
	if gameHub.players["player_11111111"] != nil {
		t.Fatal("expected player to be removed")
	}
	if !playerOneConn.closed {
		t.Fatal("expected player connection to close")
	}
}

func TestCancelMoveBroadcastsUpdatedGameState(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_11111111")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_22222222")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})
	playerOneConn.messages = nil
	playerTwoConn.messages = nil

	gameHub.handleMessage(playerOne, clientMessage{
		Type:    "game_move",
		Payload: json.RawMessage(`{"moveType":"gain_power"}`),
	})
	playerOneConn.messages = nil
	playerTwoConn.messages = nil

	gameHub.handleMessage(playerOne, clientMessage{Type: "cancel_move"})

	if got := messageOfType(t, playerOneConn, "game_move_cancelled").Type; got != "game_move_cancelled" {
		t.Fatalf("expected game_move_cancelled, got %q", got)
	}
	if got := lastMessage(t, playerTwoConn).Type; got != "game_state" {
		t.Fatalf("expected game_state broadcast, got %q", got)
	}
}

func TestResolvePlayerIDAcceptsClientFormat(t *testing.T) {
	if got := resolvePlayerID("player_a1b2c3d4"); got != "player_a1b2c3d4" {
		t.Fatalf("expected client id to be accepted, got %q", got)
	}
	generated := resolvePlayerID("invalid")
	if generated == "invalid" {
		t.Fatal("expected invalid id to be replaced")
	}
	if !playerIDPattern.MatchString(generated) {
		t.Fatalf("expected generated id to match pattern, got %q", generated)
	}
}
