(function () {
  function createGameScreen({
    frame,
    getPlayerId,
    getSubmittedRound,
    getSelectedTargetId,
    setSelectedTargetId,
    getCurrentGameState,
    sendGameMove,
    formatPlayerIds,
  }) {
    let currentGameId;
    const gameLogEntries = [];
    const loggedResultKeys = new Set();

    function show() {
      frame.hidden = false;
    }

    function hide() {
      frame.hidden = true;
    }

    function setPlaceholder(title, detail) {
      frame.innerHTML = "";

      const heading = document.createElement("h2");
      heading.textContent = title;

      const paragraph = document.createElement("p");
      paragraph.textContent = detail;

      frame.append(heading, paragraph);
    }

    function renderGameState(gameState) {
      show();
      frame.innerHTML = "";

      if (!gameState) {
        setPlaceholder("Room Ready", "Waiting for the first game state from the backend.");
        return;
      }
      syncGameLog(gameState);

      let selectedTargetId = getSelectedTargetId();
      if (selectedTargetId && !attackTargets(gameState).some((target) => target.id === selectedTargetId)) {
        selectedTargetId = undefined;
        setSelectedTargetId(undefined);
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
        card.className = `game-player-card${player.id === getPlayerId() ? " current-player" : ""}${
          player.alive ? "" : " eliminated"
        }${player.id === selectedTargetId ? " selected-target" : ""}${
          isTargetablePlayer(gameState, player) ? " targetable-player" : ""
        }`;
        if (isTargetablePlayer(gameState, player)) {
          card.tabIndex = 0;
          card.role = "button";
          card.addEventListener("click", () => selectPlayerTarget(player.id));
          card.addEventListener("keydown", (event) => {
            if (event.key === "Enter" || event.key === " ") {
              event.preventDefault();
              selectPlayerTarget(player.id);
            }
          });
        }
        card.innerHTML = `
          <strong>${player.id === getPlayerId() ? "You" : player.id}</strong>
          <span>Team: ${player.teamId}</span>
          <span>Health: ${player.health}</span>
          <span>Power: ${player.power}</span>
          <span>Defense streak: ${player.defenseStreak || 0}</span>
          <span>${player.alive ? "Alive" : "Eliminated"}</span>
        `;
        playerGrid.append(card);
      }

      playerPanel.append(playerPanelTitle, playerGrid);
      content.append(playerPanel, createActionPanel(gameState));
      board.append(header, createGameLogPanel(), content);
      frame.append(board);
    }

    function syncGameLog(gameState) {
      if (gameState.id !== currentGameId) {
        currentGameId = gameState.id;
        gameLogEntries.length = 0;
        loggedResultKeys.clear();
      }

      if (!Array.isArray(gameState.lastResults) || gameState.lastResults.length === 0) {
        return;
      }

      const resultRound = gameState.phase === "finished"
        ? gameState.round
        : Math.max(1, gameState.round - 1);
      const resultKey = `${resultRound}:${JSON.stringify(gameState.lastResults)}`;
      if (loggedResultKeys.has(resultKey)) {
        return;
      }

      loggedResultKeys.add(resultKey);
      gameLogEntries.push({ type: "round", text: `Round ${resultRound}` });
      for (const result of gameState.lastResults) {
        gameLogEntries.push({ type: "result", text: formatResultLine(result) });
      }
    }

    function createGameLogPanel() {
      const panel = document.createElement("section");
      panel.className = "game-log-panel";

      const title = document.createElement("h3");
      title.textContent = "Game log";

      const frame = document.createElement("div");
      frame.className = "game-log-frame";

      if (gameLogEntries.length === 0) {
        const empty = document.createElement("p");
        empty.className = "game-log-empty";
        empty.textContent = "Round results will appear here.";
        frame.append(empty);
      } else {
        for (const entry of gameLogEntries) {
          const line = document.createElement("p");
          line.className = entry.type === "round" ? "game-log-round" : "game-log-line";
          line.textContent = entry.text;
          frame.append(line);
        }
      }

      panel.append(title, frame);
      frame.scrollTop = frame.scrollHeight;
      return panel;
    }

    function formatResultLine(result) {
      return result.targetId
        ? `${result.playerId} ${result.message} ${result.targetId}`
        : `${result.playerId} ${result.message}`;
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
        } else if (getSubmittedRound() === gameState.round) {
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
      panel.append(
        helperText,
        gameState.gameType === "power_defense_wave"
          ? createPowerDefenseWaveControls(gameState)
          : createMoveControls(gameState)
      );
      return panel;
    }

    function createPowerDefenseWaveControls(gameState) {
      const controls = document.createElement("div");
      controls.className = "game-controls";
      const player = currentPlayer(gameState);
      const selectedTargetId = getSelectedTargetId();
      const hasTarget = Boolean(selectedTargetId);

      const targetHint = document.createElement("p");
      targetHint.className = "action-helper";
      targetHint.textContent = hasTarget
        ? `Selected target: ${selectedTargetId}`
        : "Click a player card on the left to choose a target for Wave or Air Cannon.";

      const powerButton = document.createElement("button");
      powerButton.type = "button";
      powerButton.textContent = `Power (+${gameState.rules.gainPowerAmount})`;
      powerButton.addEventListener("click", () => sendGameMove("power"));

      const defenseButton = document.createElement("button");
      defenseButton.type = "button";
      defenseButton.textContent = "Defense";
      defenseButton.disabled = (player?.defenseStreak || 0) >= 2;
      defenseButton.addEventListener("click", () => sendGameMove("defense"));

      const waveButton = document.createElement("button");
      waveButton.type = "button";
      waveButton.textContent = `Wave (${gameState.rules.waveCost} power)`;
      waveButton.disabled = !hasTarget || player.power < gameState.rules.waveCost;
      waveButton.addEventListener("click", () => sendGameMove("wave", selectedTargetId));

      const superBlastButton = document.createElement("button");
      superBlastButton.type = "button";
      superBlastButton.textContent = `Super Blast (${gameState.rules.superBlastCost} power)`;
      superBlastButton.disabled = player.power < gameState.rules.superBlastCost;
      superBlastButton.addEventListener("click", () => sendGameMove("super_blast"));

      const airCannonButton = document.createElement("button");
      airCannonButton.type = "button";
      airCannonButton.textContent = "Air Cannon";
      airCannonButton.disabled = !hasTarget;
      airCannonButton.addEventListener("click", () => sendGameMove("air_cannon", selectedTargetId));

      const basicActions = document.createElement("div");
      basicActions.className = "action-group";
      basicActions.append(powerButton, defenseButton, superBlastButton);

      const targetedActions = document.createElement("div");
      targetedActions.className = "action-group";
      targetedActions.append(targetHint, waveButton, airCannonButton);

      controls.append(basicActions, targetedActions);
      return controls;
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

    function canSubmitMove(gameState) {
      const player = currentPlayer(gameState);
      return Boolean(
        player &&
          player.alive &&
          gameState.phase === "waiting_for_moves" &&
          getSubmittedRound() !== gameState.round
      );
    }

    function currentPlayer(gameState) {
      return gameState.players?.[getPlayerId()];
    }

    function attackTargets(gameState) {
      const player = currentPlayer(gameState);
      if (!player) {
        return [];
      }
      return Object.values(gameState.players || {}).filter((target) => {
        return target.alive && target.id !== getPlayerId() && target.teamId !== player.teamId;
      });
    }

    function isTargetablePlayer(gameState, player) {
      return attackTargets(gameState).some((target) => target.id === player.id);
    }

    function selectPlayerTarget(targetId) {
      setSelectedTargetId(targetId);
      renderGameState(getCurrentGameState());
    }

    return {
      hide,
      renderGameState,
      show,
    };
  }

  window.createGameScreen = createGameScreen;
})();
