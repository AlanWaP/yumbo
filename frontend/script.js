const statusEl = document.querySelector("#status");
const serverUrlInput = document.querySelector("#server-url");
const connectButton = document.querySelector("#connect-button");
const connectionPanel = document.querySelector("#connection-panel");
const lobbyPanel = document.querySelector("#lobby-panel");
const gameTypeInput = document.querySelector("#game-type");
const gameModeInput = document.querySelector("#game-mode");
const playerCountInput = document.querySelector("#player-count");
const playerLabel = document.querySelector("#player-label");
const gameLabel = document.querySelector("#game-label");
const playerCountLabel = document.querySelector("#player-count-label");
const roomLabel = document.querySelector("#room-label");
const joinQueueButton = document.querySelector("#join-queue-button");
const leaveQueueButton = document.querySelector("#leave-queue-button");
const leaveRoomButton = document.querySelector("#leave-room-button");
const refreshLobbyButton = document.querySelector("#refresh-lobby-button");
const existingGamesList = document.querySelector("#existing-games-list");
const gameFrame = document.querySelector("#game-frame");

const gameTypeNames = {
  power_defense_wave: "Power, Defense and Wave",
  rps: "Rock Paper Scissors",
  cards: "Cards",
  trivia: "Trivia",
};

const urlParams = new URLSearchParams(window.location.search);
const savedServerUrl = localStorage.getItem("yumboServerUrl");
const savedGameType = localStorage.getItem("yumboGameType");
const savedGameMode = localStorage.getItem("yumboGameMode");
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
let gameMode;
let playerCount;
let isQueued = false;
let lobbyGames = [];
let currentGameState;
let submittedRound;
let submittedMove;
let selectedTargetId;

const waitingRoom = window.createWaitingRoom({
  panel: lobbyPanel,
  existingGamesList,
  getLobbyGames: () => lobbyGames,
  isConnected: () => socket && socket.readyState === WebSocket.OPEN,
  joinGame: joinQueue,
  formatGameType,
  formatGameMode,
  formatPlayerIds,
});

const gameScreen = window.createGameScreen({
  frame: gameFrame,
  getPlayerId: () => playerId,
  getSubmittedRound: () => submittedRound,
  getSubmittedMove: () => submittedMove,
  getSelectedTargetId: () => selectedTargetId,
  setSelectedTargetId: (targetId) => {
    selectedTargetId = targetId;
  },
  getCurrentGameState: () => currentGameState,
  sendGameMove,
  formatPlayerIds,
});

serverUrlInput.value = defaultServerUrl;
gameTypeInput.value = urlParams.get("game") || savedGameType || "power_defense_wave";
if (!gameTypeNames[gameTypeInput.value]) {
  gameTypeInput.value = "rps";
}
gameModeInput.value = urlParams.get("mode") || savedGameMode || "free_for_all";
if (!["free_for_all", "team"].includes(gameModeInput.value)) {
  gameModeInput.value = "free_for_all";
}
playerCountInput.value = urlParams.get("players") || savedPlayerCount || "2";
if (!playerCountInput.value) {
  playerCountInput.value = "2";
}
updateLabels();
renderExistingGames();

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

refreshLobbyButton.addEventListener("click", () => {
  requestLobby();
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
    showWaitingRoom();
    connectButton.disabled = false;
    setStatus("Connected. Choose a game type and enter the queue.");
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
    gameMode = undefined;
    playerCount = undefined;
    isQueued = false;
    lobbyGames = [];
    currentGameState = undefined;
    submittedRound = undefined;
    submittedMove = undefined;
    selectedTargetId = undefined;
    updateLabels();
    renderExistingGames();
    setStatus("Disconnected from multiplayer backend.");
    hideGameSurfaces();
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

  if (message.type === "lobby_update") {
    lobbyGames = Array.isArray(message.games) ? message.games : [];
    renderExistingGames();
    return;
  }

  if (message.type === "queued" || message.type === "already_queued") {
    playerId = message.playerId || playerId;
    gameType = message.gameType;
    gameMode = message.gameMode;
    playerCount = message.playerCount;
    roomId = undefined;
    isQueued = true;
    currentGameState = undefined;
    submittedRound = undefined;
    submittedMove = undefined;
    selectedTargetId = undefined;
    updateLabels();
    setStatus("Waiting for players...");
    showWaitingRoom();
    joinQueueButton.hidden = true;
    leaveQueueButton.hidden = false;
    leaveRoomButton.hidden = true;
    return;
  }

  if (message.type === "room_created") {
    playerId = message.playerId || playerId;
    gameType = message.gameType;
    gameMode = message.gameMode;
    playerCount = message.playerCount;
    roomId = message.roomId;
    isQueued = false;
    updateLabels();
    setStatus("Room created. Choose a move for round one.");
    currentGameState = message.payload;
    submittedRound = undefined;
    submittedMove = undefined;
    selectedTargetId = undefined;
    showGameFrame();
    renderGameState(currentGameState);
    joinQueueButton.hidden = false;
    leaveQueueButton.hidden = true;
    leaveRoomButton.hidden = false;
    return;
  }

  if (message.type === "queue_left" || message.type === "not_queued") {
    roomId = undefined;
    playerCount = undefined;
    isQueued = false;
    currentGameState = undefined;
    submittedRound = undefined;
    submittedMove = undefined;
    selectedTargetId = undefined;
    updateLabels();
    setStatus("You are not in the queue.");
    showWaitingRoom();
    joinQueueButton.hidden = false;
    leaveQueueButton.hidden = true;
    leaveRoomButton.hidden = true;
    return;
  }

  if (message.type === "room_left" || message.type === "peer_left") {
    roomId = undefined;
    gameType = undefined;
    gameMode = undefined;
    playerCount = undefined;
    isQueued = false;
    currentGameState = undefined;
    submittedRound = undefined;
    submittedMove = undefined;
    selectedTargetId = undefined;
    updateLabels();
    setStatus(message.type === "peer_left" ? "The other player left." : "You left the room.");
    showWaitingRoom();
    joinQueueButton.hidden = false;
    leaveQueueButton.hidden = true;
    leaveRoomButton.hidden = true;
    return;
  }

  if (message.type === "room_message") {
    setStatus("Received a room message for the active game module.");
    return;
  }

  if (message.type === "game_move_accepted") {
    submittedRound = message.payload?.round;
    setStatus("Move submitted. Waiting for the rest of the round.");
    renderGameState(currentGameState);
    return;
  }

  if (
    message.type === "game_state" ||
    message.type === "round_resolved" ||
    message.type === "game_finished"
  ) {
    currentGameState = message.payload;
    if (currentGameState?.round !== submittedRound) {
      submittedRound = undefined;
      submittedMove = undefined;
      selectedTargetId = undefined;
    }
    setStatus(formatGameStatus(message.type, currentGameState));
    renderGameState(currentGameState);
    return;
  }

  if (message.type === "error") {
    setStatus(message.message || "The server reported an error.");
  }
}

function joinQueue(gameToJoin) {
  const requestedGameType = gameToJoin?.gameType || gameTypeInput.value;
  const requestedGameMode = gameToJoin?.gameMode || gameModeInput.value;
  const requestedPlayerCount = Number.parseInt(
    gameToJoin?.playerCount || playerCountInput.value,
    10
  );

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
  localStorage.setItem("yumboGameMode", requestedGameMode);
  localStorage.setItem("yumboPlayerCount", String(requestedPlayerCount));
  gameTypeInput.value = requestedGameType;
  gameModeInput.value = requestedGameMode;
  playerCountInput.value = String(requestedPlayerCount);
  gameType = requestedGameType;
  gameMode = requestedGameMode;
  playerCount = requestedPlayerCount;
  roomId = undefined;
  isQueued = true;
  updateLabels();
  send({
    type: "join_queue",
    gameType: requestedGameType,
    gameMode: requestedGameMode,
    playerCount: requestedPlayerCount,
  });
  setStatus(gameToJoin ? "Joining the selected game..." : "Creating a waiting game...");
  showWaitingRoom();
  joinQueueButton.hidden = true;
  leaveQueueButton.hidden = false;
  leaveRoomButton.hidden = true;
}

function requestLobby() {
  send({ type: "request_lobby" });
}

function send(message) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(message));
  }
}

function setStatus(text) {
  statusEl.textContent = text;
}

function showWaitingRoom() {
  waitingRoom.show();
  gameScreen.hide();
}

function showGameFrame() {
  waitingRoom.hide();
  gameScreen.show();
}

function hideGameSurfaces() {
  waitingRoom.hide();
  gameScreen.hide();
}

function renderGameState(gameState) {
  gameScreen.renderGameState(gameState);
}

function sendGameMove(moveType, targetId) {
  submittedMove = {
    moveType,
    targetId,
  };
  send({
    type: "game_move",
    payload: {
      moveType,
      targetId,
    },
  });
}

function formatGameStatus(messageType, gameState) {
  if (messageType === "game_finished") {
    return `Game finished. Winner: ${formatPlayerIds(gameState?.winners)}.`;
  }
  if (messageType === "round_resolved") {
    return `Round resolved. ${gameState?.phase === "finished" ? "Game over." : "Choose your next move."}`;
  }
  return "Game state updated.";
}

function renderExistingGames() {
  waitingRoom.renderExistingGames();
}

function updateLabels() {
  playerLabel.textContent = `Player: ${playerId || "not assigned"}`;
  gameLabel.textContent = `Game: ${gameType ? formatGameType(gameType) : "none"}${
    gameMode ? `, ${formatGameMode(gameMode)}` : ""
  }`;
  playerCountLabel.textContent = `Players needed: ${playerCount || "none"}`;
  roomLabel.textContent = `Room: ${roomId || "none"}`;
}

function formatGameType(value) {
  return gameTypeNames[value] || value || "Unknown";
}

function formatGameMode(value) {
  if (value === "team") {
    return "Team vs team";
  }
  return "Free for all";
}

function formatPlayerIds(players) {
  return Array.isArray(players) && players.length > 0 ? players.join(", ") : "none";
}
