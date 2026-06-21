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
const languageToggleButton = document.querySelector("#language-toggle-button");
const joinQueueButton = document.querySelector("#join-queue-button");
const leaveQueueButton = document.querySelector("#leave-queue-button");
const leaveRoomButton = document.querySelector("#leave-room-button");
const refreshLobbyButton = document.querySelector("#refresh-lobby-button");
const existingGamesList = document.querySelector("#existing-games-list");
const gameFrame = document.querySelector("#game-frame");
const { t } = window.yumboI18n;

const gameTypes = new Set(["power_defense_wave"]);

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
let currentStatus = { key: "status.enterBackend" };

const waitingRoom = window.createWaitingRoom({
  panel: lobbyPanel,
  existingGamesList,
  getLobbyGames: () => lobbyGames,
  isConnected: () => socket && socket.readyState === WebSocket.OPEN,
  joinGame: joinQueue,
  formatGameType,
  formatGameMode,
  formatPlayerIds,
  t,
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
  t,
});

serverUrlInput.value = defaultServerUrl;
gameTypeInput.value = urlParams.get("game") || savedGameType || "power_defense_wave";
if (!gameTypes.has(gameTypeInput.value)) {
  gameTypeInput.value = "power_defense_wave";
}
gameModeInput.value = urlParams.get("mode") || savedGameMode || "free_for_all";
if (!["free_for_all", "team"].includes(gameModeInput.value)) {
  gameModeInput.value = "free_for_all";
}
playerCountInput.value = urlParams.get("players") || savedPlayerCount || "2";
if (!playerCountInput.value) {
  playerCountInput.value = "2";
}
window.yumboI18n.applyStaticTranslations();
updateLanguageToggle();
updatePlayerCountOptions();
refreshStatus();
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

languageToggleButton.addEventListener("click", () => {
  window.yumboI18n.toggleLanguage();
});

window.yumboI18n.onLanguageChange(() => {
  updateLanguageToggle();
  updatePlayerCountOptions();
  refreshStatus();
  updateLabels();
  renderExistingGames();
  renderGameState(currentGameState);
});

function connect(rawUrl) {
  if (!rawUrl) {
    setStatus("status.enterBackendToStart");
    return;
  }

  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.close();
  }

  setStatus("status.connecting");
  connectButton.disabled = true;
  localStorage.setItem("yumboServerUrl", rawUrl);

  socket = new WebSocket(rawUrl);

  socket.addEventListener("open", () => {
    connectionPanel.hidden = true;
    showWaitingRoom();
    connectButton.disabled = false;
    setStatus("status.connected");
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
    setStatus("status.disconnected");
    hideGameSurfaces();
  });

  socket.addEventListener("error", () => {
    setStatus("status.connectionFailed");
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
    setStatus("status.waitingPlayers");
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
    setStatus("status.roomCreated");
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
    setStatus("status.notInQueue");
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
    setStatus(message.type === "peer_left" ? "status.peerLeft" : "status.leftRoom");
    showWaitingRoom();
    joinQueueButton.hidden = false;
    leaveQueueButton.hidden = true;
    leaveRoomButton.hidden = true;
    return;
  }

  if (message.type === "room_message") {
    setStatus("status.roomMessage");
    return;
  }

  if (message.type === "game_move_accepted") {
    submittedRound = message.payload?.round;
    setStatus("status.moveSubmitted");
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
    setGameStatus(message.type, currentGameState);
    renderGameState(currentGameState);
    return;
  }

  if (message.type === "error") {
    setRawStatus(message.message || t("status.serverError"));
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
    setStatus("status.connectFirst");
    return;
  }

  if (!requestedGameType) {
    setStatus("status.enterGameType");
    return;
  }

  if (
    !Number.isInteger(requestedPlayerCount) ||
    requestedPlayerCount < 2 ||
    requestedPlayerCount > 16
  ) {
    setStatus("status.invalidPlayerCount");
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
  setStatus(gameToJoin ? "status.joiningSelected" : "status.creatingGame");
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

function setStatus(key, values = {}) {
  currentStatus = { key, values };
  refreshStatus();
}

function setRawStatus(text) {
  currentStatus = { text };
  refreshStatus();
}

function refreshStatus() {
  statusEl.textContent = currentStatus.text || t(currentStatus.key, currentStatus.values);
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

function setGameStatus(messageType, gameState) {
  if (messageType === "game_finished") {
    setStatus("status.gameFinishedWinner", { winners: formatPlayerIds(gameState?.winners) });
    return;
  }
  if (messageType === "round_resolved") {
    setStatus(gameState?.phase === "finished" ? "status.roundResolvedGameOver" : "status.roundResolvedNextMove");
    return;
  }
  setStatus("status.gameUpdated");
}

function renderExistingGames() {
  waitingRoom.renderExistingGames();
}

function updateLanguageToggle() {
  languageToggleButton.textContent = t("language.toggle");
  languageToggleButton.title = t("language.current");
  languageToggleButton.setAttribute("aria-label", t("language.switch"));
}

function updatePlayerCountOptions() {
  for (const option of playerCountInput.options) {
    option.textContent = t("lobby.playerCountOption", { count: option.value });
  }
}

function updateLabels() {
  playerLabel.textContent = t("labels.player", {
    player: playerId || t("labels.playerUnassigned"),
  });
  gameLabel.textContent = t("labels.game", {
    game: gameType ? formatGameType(gameType) : t("labels.gameNone"),
    mode: gameMode ? t("labels.gameModeSuffix", { mode: formatGameMode(gameMode) }) : "",
  });
  playerCountLabel.textContent = t("labels.playersNeeded", {
    count: playerCount || t("labels.gameNone"),
  });
  roomLabel.textContent = t("labels.room", { room: roomId || t("labels.gameNone") });
}

function formatGameType(value) {
  if (!value) {
    return t("gameType.unknown");
  }

  const translatedGameType = t(`gameType.${value}`);
  return translatedGameType === `gameType.${value}` ? value : translatedGameType;
}

function formatGameMode(value) {
  if (value === "team") {
    return t("gameMode.team");
  }
  return t("gameMode.free_for_all");
}

function formatPlayerIds(players) {
  return Array.isArray(players) && players.length > 0 ? players.join(", ") : t("labels.gameNone");
}
