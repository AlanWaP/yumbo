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

	if len(conn.messages) == 0 {
		t.Fatal("expected at least one message")
	}
	return conn.messages[len(conn.messages)-1]
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

func TestSameGameTypePlayersMatchIntoRoom(t *testing.T) {
	gameHub := newHub()
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(playerOne, clientMessage{Type: "join_queue", GameType: "rps"})
	gameHub.handleMessage(playerTwo, clientMessage{Type: "join_queue", GameType: "rps"})

	if len(gameHub.rooms) != 1 {
		t.Fatalf("expected one room, got %d", len(gameHub.rooms))
	}
	if len(gameHub.queues["rps"]) != 0 {
		t.Fatalf("expected rps queue to be empty, got %v", gameHub.queues["rps"])
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

	message := playerOneConn.messages[1]
	if message.Type != "queue_left" {
		t.Fatalf("expected queue_left, got %q", message.Type)
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

func TestRemovePlayerCleansQueueAndNotifiesRoomPeer(t *testing.T) {
	gameHub := newHub()
	queuedPlayer, queuedConn := addTestPlayer(gameHub, "queued_player")
	playerOne, playerOneConn := addTestPlayer(gameHub, "player_one")
	playerTwo, playerTwoConn := addTestPlayer(gameHub, "player_two")

	gameHub.handleMessage(queuedPlayer, clientMessage{Type: "join_queue", GameType: "cards"})
	gameHub.removePlayer(queuedPlayer)
	if len(gameHub.queues["cards"]) != 0 {
		t.Fatalf("expected disconnected queued player to be removed, got %v", gameHub.queues["cards"])
	}
	if !queuedConn.closed {
		t.Fatal("expected disconnected queued player's connection to close")
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
