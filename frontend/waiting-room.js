(function () {
  function createWaitingRoom({
    panel,
    existingGamesList,
    lobbyRulesContent,
    getLobbyGames,
    getSelectedGameType,
    isConnected,
    joinGame,
    formatGameType,
    formatGameMode,
    formatPlayerIds,
    t,
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
          ? t("lobby.emptyConnected")
          : t("lobby.emptyDisconnected");
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
        title.textContent = t("lobby.cardTitle", {
          gameType: formatGameType(game.gameType),
          playerCount: game.playerCount,
          gameMode: formatGameMode(game.gameMode),
        });

        const detail = document.createElement("p");
        detail.className = "game-card-detail";
        detail.textContent = t("lobby.cardDetail", {
          joinedPlayerCount: game.joinedPlayerCount,
          playerCount: game.playerCount,
          status: isWaiting ? t("lobby.waiting") : t("lobby.inRoom"),
          action: isWaiting ? t("lobby.clickToJoin") : t("lobby.alreadyStarted"),
        });

        const players = document.createElement("p");
        players.className = "game-card-players";
        players.textContent = t("lobby.players", { players: formatPlayerIds(game.players) });

        const status = document.createElement("span");
        status.className = `game-status ${game.status}`;
        status.textContent = isWaiting ? t("lobby.statusWaiting") : t("lobby.statusStarted");

        content.append(title, detail, players);
        card.append(content, status);
        existingGamesList.append(card);
      }
    }

    function renderLobbyRules() {
      if (!lobbyRulesContent) {
        return;
      }

      const gameType = getSelectedGameType();
      window.yumboGameRules.renderGameRules(lobbyRulesContent, {
        gameType,
        rules: window.yumboGameRules.defaultRulesForGameType(gameType),
        t,
      });
    }

    return {
      hide,
      renderExistingGames,
      renderLobbyRules,
      show,
    };
  }

  window.createWaitingRoom = createWaitingRoom;
})();
