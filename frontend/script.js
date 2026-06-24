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
const roomLabel = document.querySelector("#room-label");
const sessionStatusLabel = document.querySelector("#session-status-label");
const languageToggleButton = document.querySelector("#language-toggle-button");
const joinQueueButton = document.querySelector("#join-queue-button");
const leaveQueueButton = document.querySelector("#leave-queue-button");
const leaveRoomButton = document.querySelector("#leave-room-button");
const cancelMoveButton = document.querySelector("#cancel-move-button");
const refreshLobbyButton = document.querySelector("#refresh-lobby-button");
const existingGamesList = document.querySelector("#existing-games-list");
const gameFrame = document.querySelector("#game-frame");
const { t } = window.yumboI18n;

const gameTypes = new Set(["power_defense_wave", "chaos_of_the_baby_city"]);
const PLAYER_ID_STORAGE_KEY = "yumboPlayerId";
const playerIdPattern = /^player_[0-9a-f]{8}$/;

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
let lastMoveReceipt;
let moveCancelAvailable = false;
let currentStatus = { key: "status.enterBackend" };
let activeServerUrl;
let intentionalClose = false;
let reconnectTimer;
let reconnectAttempt = 0;
let pendingPageReload = false;
let pageUnloading = false;

if (window.navigation) {
  navigation.addEventListener("navigate", (event) => {
    if (event.navigationType === "reload") {
      pendingPageReload = true;
    }
  });
}

window.addEventListener("keydown", (event) => {
  if (event.key === "F5") {
    pendingPageReload = true;
  }
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "r") {
    pendingPageReload = true;
  }
});

window.addEventListener("pagehide", () => {
  pageUnloading = true;

  if (pendingPageReload) {
    sessionStorage.setItem("yumboReloadIntent", "1");
    sendIfConnected({ type: "refresh_pending" });
    return;
  }

  if (socket && socket.readyState === WebSocket.OPEN) {
    intentionalClose = true;
    sendIfConnected({ type: "leave_session" });
  }
});

const reloadIntent = sessionStorage.getItem("yumboReloadIntent") === "1";
const navigationEntry = performance.getEntriesByType("navigation")[0];
if (reloadIntent && navigationEntry?.type === "reload") {
  sessionStorage.removeItem("yumboReloadIntent");
}

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
  getMoveCancelAvailable: () => moveCancelAvailable,
  sendGameMove,
  cancelGameMove,
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

cancelMoveButton.addEventListener("click", () => {
  cancelGameMove();
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

  clearReconnectTimer();

  if (socket && socket.readyState === WebSocket.OPEN) {
    intentionalClose = true;
    socket.close();
  }

  intentionalClose = false;
  activeServerUrl = rawUrl;
  setStatus("status.connecting");
  connectButton.disabled = true;
  localStorage.setItem("yumboServerUrl", rawUrl);

  socket = new WebSocket(buildWebSocketUrl(rawUrl));

  socket.addEventListener("open", () => {
    reconnectAttempt = 0;
    connectionPanel.hidden = true;
    showWaitingRoom();
    connectButton.disabled = false;
    setStatus("status.connected");
  });

  socket.addEventListener("message", (event) => {
    handleServerMessage(event.data);
  });

  socket.addEventListener("close", () => {
    connectButton.disabled = false;

    if (pageUnloading) {
      return;
    }

    if (intentionalClose) {
      resetSessionState();
      setStatus("status.disconnected");
      return;
    }

    if (activeServerUrl) {
      scheduleReconnect();
      return;
    }

    resetSessionState();
    setStatus("status.disconnected");
  });

  socket.addEventListener("error", () => {
    setStatus("status.connectionFailed");
    connectButton.disabled = false;
  });
}

function buildWebSocketUrl(rawUrl) {
  const url = new URL(rawUrl);
  url.searchParams.set("playerId", getOrCreatePlayerId());
  return url.toString();
}

function getOrCreatePlayerId() {
  let storedId = sessionStorage.getItem(PLAYER_ID_STORAGE_KEY);
  if (!playerIdPattern.test(storedId)) {
    const bytes = crypto.getRandomValues(new Uint8Array(4));
    storedId = `player_${Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("")}`;
    sessionStorage.setItem(PLAYER_ID_STORAGE_KEY, storedId);
  }
  return storedId;
}

function storePlayerId(nextPlayerId) {
  if (playerIdPattern.test(nextPlayerId)) {
    sessionStorage.setItem(PLAYER_ID_STORAGE_KEY, nextPlayerId);
  }
}

function clearReconnectTimer() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = undefined;
  }
}

function scheduleReconnect() {
  if (!activeServerUrl || intentionalClose) {
    return;
  }

  reconnectAttempt += 1;
  const delay = Math.min(1000 * 2 ** (reconnectAttempt - 1), 30000);
  setStatus("status.reconnecting", { attempt: reconnectAttempt, seconds: Math.ceil(delay / 1000) });
  reconnectTimer = setTimeout(() => {
    reconnectTimer = undefined;
    connect(activeServerUrl);
  }, delay);
}

function resetSessionState() {
  connectionPanel.hidden = false;
  lobbyPanel.hidden = true;
  joinQueueButton.hidden = false;
  leaveQueueButton.hidden = true;
  leaveRoomButton.hidden = true;
  cancelMoveButton.hidden = true;
  playerId = undefined;
  roomId = undefined;
  gameType = undefined;
  gameMode = undefined;
  playerCount = undefined;
  isQueued = false;
  lobbyGames = [];
  currentGameState = undefined;
  clearSubmittedMoveState();
  updateLabels();
  renderExistingGames();
  hideGameSurfaces();
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
    storePlayerId(playerId);
    updateLabels();
    return;
  }

  if (message.type === "lobby_update") {
    lobbyGames = Array.isArray(message.games) ? message.games : [];
    renderExistingGames();
    return;
  }

  if (message.type === "queued" || message.type === "already_queued") {
    applyQueuedState(message);
    return;
  }

  if (message.type === "room_created") {
    applyRoomState(message);
    return;
  }

  if (message.type === "queue_left" || message.type === "not_queued") {
    roomId = undefined;
    gameType = undefined;
    gameMode = undefined;
    playerCount = undefined;
    isQueued = false;
    currentGameState = undefined;
    clearSubmittedMoveState();
    updateLabels();
    setStatus("status.notInQueue");
    showWaitingRoom();
    joinQueueButton.hidden = false;
    leaveQueueButton.hidden = true;
    leaveRoomButton.hidden = true;
    return;
  }

  if (message.type === "room_left") {
    roomId = undefined;
    gameType = undefined;
    gameMode = undefined;
    playerCount = undefined;
    isQueued = false;
    currentGameState = undefined;
    clearSubmittedMoveState();
    updateLabels();
    setStatus("status.leftRoom");
    showWaitingRoom();
    joinQueueButton.hidden = false;
    leaveQueueButton.hidden = true;
    leaveRoomButton.hidden = true;
    return;
  }

  if (message.type === "peer_left") {
    clearSubmittedMoveState();
    if (message.payload) {
      currentGameState = message.payload;
      updateLabels();
      if (currentGameState.phase === "finished") {
        setStatus("status.peerLeftGameEnded", {
          player: message.playerId || t("labels.playerUnassigned"),
          winners: formatPlayerIds(currentGameState.winners),
        });
      } else {
        setStatus("status.peerLeft", { player: message.playerId || t("labels.playerUnassigned") });
      }
      showGameFrame();
      renderGameState(currentGameState);
    } else {
      setStatus("status.peerLeft", { player: message.playerId || t("labels.playerUnassigned") });
      showWaitingRoom();
    }
    joinQueueButton.hidden = false;
    leaveQueueButton.hidden = true;
    leaveRoomButton.hidden = false;
    return;
  }

  if (message.type === "peer_disconnected") {
    setStatus("status.peerDisconnected", { player: message.playerId || t("labels.playerUnassigned") });
    return;
  }

  if (message.type === "peer_reconnected") {
    setStatus("status.peerReconnected", { player: message.playerId || t("labels.playerUnassigned") });
    return;
  }

  if (message.type === "room_message") {
    setStatus("status.roomMessage");
    return;
  }

  if (message.type === "game_move_accepted") {
    lastMoveReceipt = message.payload;
    submittedRound = roundNumber(message.payload?.round);
    applySubmittedMoveFromMessage(message);
    markMoveCancelAvailable();
    if (currentGameState && message.payload?.submittedPlayers) {
      currentGameState = {
        ...currentGameState,
        submittedPlayers: message.payload.submittedPlayers,
      };
    }
    setStatus("status.moveSubmitted");
    renderGameState(currentGameState);
    return;
  }

  if (message.type === "game_move_cancelled") {
    clearSubmittedMoveState();
    setStatus("status.moveCancelled");
    renderGameState(currentGameState);
    return;
  }

  if (
    message.type === "game_state" ||
    message.type === "round_resolved" ||
    message.type === "game_finished"
  ) {
    currentGameState = message.payload;
    const currentRound = roundNumber(currentGameState?.round);
    const trackedRound = roundNumber(submittedRound);
    if (message.type !== "game_state" || currentGameState?.phase === "finished") {
      clearSubmittedMoveState();
    } else if (trackedRound !== undefined && currentRound !== trackedRound) {
      clearSubmittedMoveState();
    } else {
      syncSubmissionFromGameState(currentGameState);
      if (lastMoveReceipt && Array.isArray(currentGameState?.submittedPlayers)) {
        lastMoveReceipt = {
          ...lastMoveReceipt,
          submittedPlayers: currentGameState.submittedPlayers,
        };
      }
    }
    setGameStatus(message.type, currentGameState);
    updateLabels();
    renderGameState(currentGameState);
    return;
  }

  if (message.type === "error") {
    setRawStatus(message.message || t("status.serverError"));
  }
}

function applyQueuedState(message) {
  playerId = message.playerId || playerId;
  storePlayerId(playerId);
  gameType = message.gameType;
  gameMode = message.gameMode;
  playerCount = message.playerCount;
  roomId = undefined;
  isQueued = true;
  currentGameState = undefined;
  clearSubmittedMoveState();
  updateLabels();
  setStatus(message.type === "already_queued" && message.restored ? "status.sessionRestoredQueue" : "status.waitingPlayers");
  showWaitingRoom();
  joinQueueButton.hidden = true;
  leaveQueueButton.hidden = false;
  leaveRoomButton.hidden = true;
}

function applyRoomState(message) {
  playerId = message.playerId || playerId;
  storePlayerId(playerId);
  gameType = message.gameType;
  gameMode = message.gameMode;
  playerCount = message.playerCount;
  roomId = message.roomId;
  isQueued = false;
  updateLabels();
  setStatus(message.restored ? "status.sessionRestoredRoom" : "status.roomCreated");
  currentGameState = message.payload;
  clearSubmittedMoveState();
  applySubmittedMoveFromMessage(message);
  syncSubmissionFromGameState(currentGameState);
  if (hasActiveSubmittedMove()) {
    markMoveCancelAvailable();
  }
  showGameFrame();
  renderGameState(currentGameState);
  joinQueueButton.hidden = false;
  leaveQueueButton.hidden = true;
  leaveRoomButton.hidden = false;
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

function applySubmittedMoveFromMessage(message) {
  let move = message?.submittedMove;
  if (!move) {
    return;
  }
  if (typeof move === "string") {
    try {
      move = JSON.parse(move);
    } catch {
      return;
    }
  }
  if (move?.moveType) {
    submittedMove = {
      moveType: move.moveType,
      targetId: move.targetId,
    };
  }
}

function syncSubmissionFromGameState(gameState) {
  if (!gameState || gameState.phase !== "waiting_for_moves" || !playerId) {
    return;
  }

  const submittedPlayers = gameState.submittedPlayers;
  if (!Array.isArray(submittedPlayers) || !submittedPlayers.includes(playerId)) {
    return;
  }

  submittedRound = roundNumber(gameState.round);
  markMoveCancelAvailable();
  if (!lastMoveReceipt) {
    const neededPlayers = Object.values(gameState.players || {})
      .filter((player) => player.alive)
      .map((player) => player.id);
    lastMoveReceipt = {
      round: submittedRound,
      submittedPlayers,
      neededPlayers,
    };
  }
}

function roundNumber(value) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function send(message) {
  if (socket && socket.readyState === WebSocket.OPEN) {
    socket.send(JSON.stringify(message));
  }
}

function sendIfConnected(message) {
  send(message);
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
  updateActionButtons();
}

function clearSubmittedMoveState() {
  lastMoveReceipt = undefined;
  submittedRound = undefined;
  submittedMove = undefined;
  selectedTargetId = undefined;
  moveCancelAvailable = false;
  updateActionButtons();
}

function markMoveCancelAvailable() {
  moveCancelAvailable = true;
  updateActionButtons();
}

function hasActiveSubmittedMove() {
  if (!currentGameState || currentGameState.phase === "finished") {
    return false;
  }
  if (moveCancelAvailable) {
    return true;
  }
  const round = roundNumber(currentGameState?.round);
  const tracked = roundNumber(submittedRound);
  return Boolean(submittedMove && tracked !== undefined && tracked === round);
}

function updateActionButtons() {
  cancelMoveButton.hidden = !(roomId && hasActiveSubmittedMove());
}

function sendGameMove(moveType, targetId) {
  submittedMove = {
    moveType,
    targetId,
  };
  submittedRound = roundNumber(currentGameState?.round);
  markMoveCancelAvailable();
  send({
    type: "game_move",
    payload: {
      moveType,
      targetId,
    },
  });
}

function cancelGameMove() {
  send({ type: "cancel_move" });
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

function formatSessionStatus() {
  const connected = Boolean(socket && socket.readyState === WebSocket.OPEN && playerId);

  if (!connected) {
    return t("session.notConnected");
  }
  if (isQueued) {
    return t("session.waiting");
  }
  if (roomId && currentGameState?.phase === "finished") {
    return t("session.finished");
  }
  if (roomId) {
    return t("session.inProgress");
  }
  return t("session.noGame");
}

function updateLabels() {
  playerLabel.textContent = t("labels.player", {
    player: playerId || t("labels.playerUnassigned"),
  });
  gameLabel.textContent = t("labels.game", {
    game: gameType ? formatGameType(gameType) : t("labels.gameNone"),
    mode: gameMode ? t("labels.gameModeSuffix", { mode: formatGameMode(gameMode) }) : "",
  });
  roomLabel.textContent = t("labels.room", { room: roomId || t("labels.gameNone") });
  sessionStatusLabel.textContent = t("labels.sessionStatus", {
    status: formatSessionStatus(),
  });
  updateActionButtons();
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
