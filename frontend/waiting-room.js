(function () {
  function createWaitingRoom({
    panel,
    existingGamesList,
    getLobbyGames,
    isConnected,
    joinGame,
    formatGameType,
    formatGameMode,
    formatPlayerIds,
  }) {
    function show() {
      panel.hidden = false;
    }

    function hide() {
      panel.hidden = true;
    }

    function renderExistingGames() {
      existingGamesList.innerHTML = "";

      const lobbyGames = getLobbyGames();
      if (lobbyGames.length === 0) {
        const emptyMessage = document.createElement("p");
        emptyMessage.className = "empty-list";
        emptyMessage.textContent = isConnected()
          ? "No games yet. Create one to open the waiting room."
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
          card.addEventListener("click", () => joinGame(game));
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

    return {
      hide,
      renderExistingGames,
      show,
    };
  }

  window.createWaitingRoom = createWaitingRoom;
})();
