const statusEl = document.querySelector("#status");
const serverUrlInput = document.querySelector("#server-url");
const connectButton = document.querySelector("#connect-button");
const connectionPanel = document.querySelector("#connection-panel");
const lobbyPanel = document.querySelector("#lobby-panel");
const gameTypeInput = document.querySelector("#game-type");
const playerCountInput = document.querySelector("#player-count");
const playerLabel = document.querySelector("#player-label");
const gameLabel = document.querySelector("#game-label");
const playerCountLabel = document.querySelector("#player-count-label");
const roomLabel = document.querySelector("#room-label");
const joinQueueButton = document.querySelector("#join-queue-button");
const leaveQueueButton = document.querySelector("#leave-queue-button");
const leaveRoomButton = document.querySelector("#leave-room-button");
const gameFrame = document.querySelector("#game-frame");

const urlParams = new URLSearchParams(window.location.search);
const savedServerUrl = localStorage.getItem("yumboServerUrl");
const savedGameType = localStorage.getItem("yumboGameType");
const savedPlayerCount = localStorage.getItem("yumboPlayerCount");
const defaultServerUrl =
  urlParams.get("server") ||
  savedServerUrl ||
  (window.location.hostname === "localhost" ||
  window.location.hostname === "127.0.0.1"
    ? "ws://localhost:3000"
    : "");

let socket;
let playerId;
let roomId;
let gameType;
let playerCount;
let isQueued = false;

serverUrlInput.value = defaultServerUrl;
gameTypeInput.value = urlParams.get("game") || savedGameType || "";
playerCountInput.value = urlParams.get("players") || savedPlayerCount || "2";
updateLabels();

if (defaultServerUrl) {
  connect(defaultServerUrl);
}

connectButton.addEventListener("click", () => {
  connect(serverUrlInput.value.trim());
});

serverUrlInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    connect(serverUrlInput.value.trim());
  }
});

gameTypeInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    joinQueue();
  }
});

playerCountInput.addEventListener("keydown", (event) => {
  if (event.key === "Enter") {
    joinQueue();
  }
});

joinQueueButton.addEventListener("click", () => {
  joinQueue();
});

leaveQueueButton.addEventListener("click", () => {
  send({ type: "leave_queue" });
});

leaveRoomButton.addEventListener("click", () => {
  send({ type: "leave_room" });
});

function connect(rawUrl) {
  if (!rawUrl) {
    setStatus("Enter your backend WebSocket URL to start.");
    return;
  }

  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.close();
  }

  setStatus("Connecting to multiplayer backend...");
  connectButton.disabled = true;
  localStorage.setItem("yumboServerUrl", rawUrl);

  socket = new WebSocket(rawUrl);

  socket.addEventListener("open", () => {
    connectionPanel.hidden = true;
    lobbyPanel.hidden = false;
    connectButton.disabled = false;
    setStatus("Connected. Choose a game type and enter the queue.");
    setGameFrame("Game Frame", "Waiting for a room. Future game UI will appear here.");
  });

  socket.addEventListener("message", (event) => {
    handleServerMessage(event.data);
  });

  socket.addEventListener("close", () => {
    connectionPanel.hidden = false;
    lobbyPanel.hidden = true;
    connectButton.disabled = false;
    joinQueueButton.hidden = false;
    leaveQueueButton.hidden = true;
    leaveRoomButton.hidden = true;
    playerId = undefined;
    roomId = undefined;
    gameType = undefined;
    playerCount = undefined;
    isQueued = false;
    updateLabels();
    setStatus("Disconnected from multiplayer backend.");
    setGameFrame("Connection closed", "Reconnect when your backend server is available.");
  });

  socket.addEventListener("error", () => {
    setStatus("Could not connect. Check the backend URL and server status.");
    connectButton.disabled = false;
  });
}

function handleServerMessage(rawMessage) {
  let message;

  try {
    message = JSON.parse(rawMessage);
  } catch {
    return;
  }

  if (message.type === "connected") {
    playerId = message.playerId;
    updateLabels();
    return;
  }

  if (message.type === "queued" || message.type === "already_queued") {
    playerId = message.playerId || playerId;
    gameType = message.gameType;
    playerCount = message.playerCount;
    roomId = undefined;
    isQueued = true;
    updateLabels();
    setStatus("Waiting for players...");
    setGameFrame(
      "Waiting Room",
      `Keep this page open while the backend finds ${message.playerCount || "enough"} players.`
    );
    joinQueueButton.hidden = true;
    leaveQueueButton.hidden = false;
    leaveRoomButton.hidden = true;
    return;
  }

  if (message.type === "room_created") {
    playerId = message.playerId || playerId;
    gameType = message.gameType;
    playerCount = message.playerCount;
    roomId = message.roomId;
    isQueued = false;
    updateLabels();
    setStatus("Room created. A game module can now take over the game frame.");
    setGameFrame(
      "Room Ready",
      `Room ${message.roomId} is ready for ${message.gameType} with ${
        message.playerCount || message.players?.length || "multiple"
      } players. Players: ${
        message.players?.join(", ") || "unknown"
      }.`
    );
    joinQueueButton.hidden = false;
    leaveQueueButton.hidden = true;
    leaveRoomButton.hidden = false;
    return;
  }

  if (message.type === "queue_left" || message.type === "not_queued") {
    roomId = undefined;
    playerCount = undefined;
    isQueued = false;
    updateLabels();
    setStatus("You are not in the queue.");
    setGameFrame("Game Frame", "Choose a game type and enter the queue when ready.");
    joinQueueButton.hidden = false;
    leaveQueueButton.hidden = true;
    leaveRoomButton.hidden = true;
    return;
  }

  if (message.type === "room_left" || message.type === "peer_left") {
    roomId = undefined;
    gameType = undefined;
    playerCount = undefined;
    isQueued = false;
    updateLabels();
    setStatus(message.type === "peer_left" ? "The other player left." : "You left the room.");
    setGameFrame("Room Closed", "Return to the lobby to queue for another game.");
    joinQueueButton.hidden = false;
    leaveQueueButton.hidden = true;
    leaveRoomButton.hidden = true;
    return;
  }

  if (message.type === "room_message") {
    setStatus("Received a room message for the active game module.");
    return;
  }

  if (message.type === "error") {
    setStatus(message.message || "The server reported an error.");
  }
}

function joinQueue() {
  const requestedGameType = gameTypeInput.value.trim();
  const requestedPlayerCount = Number.parseInt(playerCountInput.value, 10);

  if (!socket || socket.readyState !== WebSocket.OPEN) {
    setStatus("Connect to the multiplayer backend first.");
    return;
  }

  if (!requestedGameType) {
    setStatus("Enter a game type before joining the queue.");
    return;
  }

  if (
    !Number.isInteger(requestedPlayerCount) ||
    requestedPlayerCount < 2 ||
    requestedPlayerCount > 16
  ) {
    setStatus("Players needed must be a number between 2 and 16.");
    return;
  }

  localStorage.setItem("yumboGameType", requestedGameType);
  localStorage.setItem("yumboPlayerCount", String(requestedPlayerCount));
  gameType = requestedGameType;
  playerCount = requestedPlayerCount;
  roomId = undefined;
  isQueued = true;
  updateLabels();
  send({
    type: "join_queue",
    gameType: requestedGameType,
    playerCount: requestedPlayerCount,
  });
  setStatus("Joining the waiting queue...");
  setGameFrame("Joining Queue", "Waiting for the backend to confirm your place.");
  joinQueueButton.hidden = true;
  leaveQueueButton.hidden = false;
  leaveRoomButton.hidden = true;
}

function send(message) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(message));
  }
}

function setStatus(text) {
  statusEl.textContent = text;
}

function setGameFrame(title, detail) {
  gameFrame.innerHTML = "";

  const heading = document.createElement("h2");
  heading.textContent = title;

  const paragraph = document.createElement("p");
  paragraph.textContent = detail;

  gameFrame.append(heading, paragraph);
}

function updateLabels() {
  playerLabel.textContent = `Player: ${playerId || "not assigned"}`;
  gameLabel.textContent = `Game type: ${gameType || "none"}`;
  playerCountLabel.textContent = `Players needed: ${playerCount || "none"}`;
  roomLabel.textContent = `Room: ${roomId || "none"}`;
}
