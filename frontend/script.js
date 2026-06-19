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

serverUrlInput.value = defaultServerUrl;
gameTypeInput.value = urlParams.get("game") || savedGameType || "rps";
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
    gameMode = undefined;
    playerCount = undefined;
    isQueued = false;
    lobbyGames = [];
    currentGameState = undefined;
    submittedRound = undefined;
    updateLabels();
    renderExistingGames();
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
    gameMode = message.gameMode;
    playerCount = message.playerCount;
    roomId = message.roomId;
    isQueued = false;
    updateLabels();
    setStatus("Room created. Choose a move for round one.");
    currentGameState = message.payload;
    submittedRound = undefined;
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
    gameMode = undefined;
    playerCount = undefined;
    isQueued = false;
    currentGameState = undefined;
    submittedRound = undefined;
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
  setGameFrame(
    gameToJoin ? "Joining Game" : "Creating Game",
    "Waiting for the backend to confirm your place."
  );
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

function setGameFrame(title, detail) {
  gameFrame.innerHTML = "";

  const heading = document.createElement("h2");
  heading.textContent = title;

  const paragraph = document.createElement("p");
  paragraph.textContent = detail;

  gameFrame.append(heading, paragraph);
}

function renderGameState(gameState) {
  gameFrame.innerHTML = "";

  if (!gameState) {
    setGameFrame("Room Ready", "Waiting for the first game state from the backend.");
    return;
  }

  const board = document.createElement("section");
  board.className = "game-board";

  const header = document.createElement("header");
  header.className = "game-board-header";

  const heading = document.createElement("h2");
  heading.textContent = gameState.phase === "finished"
    ? "Game finished"
    : `Round ${gameState.round}`;

  const summary = document.createElement("p");
  summary.textContent = gameState.phase === "finished"
    ? `Winner: ${formatPlayerIds(gameState.winners)}`
    : "Choose one move. The round resolves after every alive player has moved.";

  header.append(heading, summary);

  if (Array.isArray(gameState.lastResults) && gameState.lastResults.length > 0) {
    const results = document.createElement("ul");
    results.className = "round-results";
    for (const result of gameState.lastResults) {
      const item = document.createElement("li");
      item.textContent = result.targetId
        ? `${result.playerId} ${result.message} ${result.targetId}`
        : `${result.playerId} ${result.message}`;
      results.append(item);
    }
    header.append(results);
  }

  const content = document.createElement("div");
  content.className = "game-board-content";

  const playerPanel = document.createElement("section");
  playerPanel.className = "player-status-panel";
  const playerPanelTitle = document.createElement("h3");
  playerPanelTitle.textContent = "Players";

  const playerGrid = document.createElement("div");
  playerGrid.className = "game-player-grid";

  for (const player of Object.values(gameState.players || {})) {
    const card = document.createElement("div");
    card.className = `game-player-card${player.id === playerId ? " current-player" : ""}${
      player.alive ? "" : " eliminated"
    }`;
    card.innerHTML = `
      <strong>${player.id === playerId ? "You" : player.id}</strong>
      <span>Team: ${player.teamId}</span>
      <span>Health: ${player.health}</span>
      <span>Power: ${player.power}</span>
      <span>${player.alive ? "Alive" : "Eliminated"}</span>
    `;
    playerGrid.append(card);
  }

  playerPanel.append(playerPanelTitle, playerGrid);
  content.append(playerPanel, createActionPanel(gameState));
  board.append(header, content);
  gameFrame.append(board);
}

function createActionPanel(gameState) {
  const panel = document.createElement("aside");
  panel.className = "action-panel";

  const title = document.createElement("h3");
  title.textContent = "Actions";
  panel.append(title);

  const helperText = document.createElement("p");
  helperText.className = "action-helper";

  if (!canSubmitMove(gameState)) {
    if (gameState.phase === "finished") {
      helperText.textContent = "The game is over.";
    } else if (submittedRound === gameState.round) {
      helperText.textContent = "You already moved this round. Waiting for other players.";
    } else if (!currentPlayer(gameState)?.alive) {
      helperText.textContent = "You are eliminated and cannot move.";
    } else {
      helperText.textContent = "Waiting for the next available action.";
    }
    panel.append(helperText);
    return panel;
  }

  helperText.textContent = "Pick one move for this round.";
  panel.append(helperText, createMoveControls(gameState));
  return panel;
}

function createMoveControls(gameState) {
  const controls = document.createElement("div");
  controls.className = "game-controls";

  const attackGroup = document.createElement("div");
  attackGroup.className = "action-group";

  const attackLabel = document.createElement("label");
  attackLabel.textContent = "Attack target";

  const targetSelect = document.createElement("select");
  const targets = attackTargets(gameState);
  for (const target of targets) {
    const option = document.createElement("option");
    option.value = target.id;
    option.textContent = target.id;
    targetSelect.append(option);
  }

  const attackButton = document.createElement("button");
  attackButton.type = "button";
  attackButton.textContent = `Attack (${gameState.rules.attackCost} power)`;
  attackButton.disabled = targets.length === 0 || currentPlayer(gameState).power < gameState.rules.attackCost;
  attackButton.addEventListener("click", () => {
    sendGameMove("attack", targetSelect.value);
  });
  attackGroup.append(attackLabel, targetSelect, attackButton);

  const basicActions = document.createElement("div");
  basicActions.className = "action-group";

  const defendButton = document.createElement("button");
  defendButton.type = "button";
  defendButton.textContent = "Defend";
  defendButton.addEventListener("click", () => {
    sendGameMove("defend");
  });

  const gainPowerButton = document.createElement("button");
  gainPowerButton.type = "button";
  gainPowerButton.textContent = `Gain power (+${gameState.rules.gainPowerAmount})`;
  gainPowerButton.addEventListener("click", () => {
    sendGameMove("gain_power");
  });
  basicActions.append(defendButton, gainPowerButton);

  controls.append(attackGroup, basicActions);
  return controls;
}

function sendGameMove(moveType, targetId) {
  send({
    type: "game_move",
    payload: {
      moveType,
      targetId,
    },
  });
}

function canSubmitMove(gameState) {
  const player = currentPlayer(gameState);
  return Boolean(
    player &&
      player.alive &&
      gameState.phase === "waiting_for_moves" &&
      submittedRound !== gameState.round
  );
}

function currentPlayer(gameState) {
  return gameState.players?.[playerId];
}

function attackTargets(gameState) {
  const player = currentPlayer(gameState);
  if (!player) {
    return [];
  }
  return Object.values(gameState.players || {}).filter((target) => {
    return target.alive && target.id !== playerId && target.teamId !== player.teamId;
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
  existingGamesList.innerHTML = "";

  if (lobbyGames.length === 0) {
    const emptyMessage = document.createElement("p");
    emptyMessage.className = "empty-list";
    emptyMessage.textContent = socket && socket.readyState === WebSocket.OPEN
      ? "No games yet. Create one to open the lobby."
      : "Connect to see waiting and started games.";
    existingGamesList.append(emptyMessage);
    return;
  }

  for (const game of lobbyGames) {
    const isWaiting = game.status === "waiting";
    const card = document.createElement(isWaiting ? "button" : "div");
    card.className = "game-card";

    if (isWaiting) {
      card.type = "button";
      card.addEventListener("click", () => joinQueue(game));
    }

    const content = document.createElement("div");
    const title = document.createElement("p");
    title.className = "game-card-title";
    title.textContent = `${formatGameType(game.gameType)} (${game.playerCount} players, ${formatGameMode(game.gameMode)})`;

    const detail = document.createElement("p");
    detail.className = "game-card-detail";
    detail.textContent = `${game.joinedPlayerCount}/${game.playerCount} players ${
      isWaiting ? "waiting" : "in room"
    }. ${isWaiting ? "Click to join." : "Already started."}`;

    const players = document.createElement("p");
    players.className = "game-card-players";
    players.textContent = `Players: ${formatPlayerIds(game.players)}`;

    const status = document.createElement("span");
    status.className = `game-status ${game.status}`;
    status.textContent = isWaiting ? "Waiting" : "Started";

    content.append(title, detail, players);
    card.append(content, status);
    existingGamesList.append(card);
  }
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
