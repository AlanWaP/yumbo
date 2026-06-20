(function () {
  function createGameScreen({
    frame,
    getPlayerId,
    getSubmittedRound,
    getSubmittedMove,
    getSelectedTargetId,
    setSelectedTargetId,
    getCurrentGameState,
    sendGameMove,
    formatPlayerIds,
  }) {
    let currentGameId;
    let activeInfoTab = "log";
    let pendingTargetMoveType;
    const gameLogEntries = [];
    const loggedRoundKeys = new Set();
    const loggedResultKeys = new Set();
    const loggedFinishKeys = new Set();

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
      if (!canSubmitMove(gameState)) {
        pendingTargetMoveType = undefined;
      }

      const board = document.createElement("section");
      board.className = "game-board";

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
      board.append(createGameInfoPanel(gameState), content);
      frame.append(board);
      scrollLogToBottom();
    }

    function syncGameLog(gameState) {
      if (gameState.id !== currentGameId) {
        currentGameId = gameState.id;
        pendingTargetMoveType = undefined;
        gameLogEntries.length = 0;
        loggedRoundKeys.clear();
        loggedResultKeys.clear();
        loggedFinishKeys.clear();
      }

      if (Array.isArray(gameState.lastResults) && gameState.lastResults.length > 0) {
        const resultRound = gameState.phase === "finished"
          ? gameState.round
          : Math.max(1, gameState.round - 1);
        const resultKey = `${resultRound}:${JSON.stringify(gameState.lastResults)}`;
        if (!loggedResultKeys.has(resultKey)) {
          loggedResultKeys.add(resultKey);
          appendRoundMarker(resultRound);
          for (const result of gameState.lastResults) {
            gameLogEntries.push({ type: "result", text: formatResultLine(result) });
          }
        }
      }

      if (gameState.phase !== "finished") {
        appendRoundMarker(gameState.round);
      } else {
        appendFinishedMarker(gameState);
      }
    }

    function appendRoundMarker(round) {
      const roundKey = String(round);
      if (loggedRoundKeys.has(roundKey)) {
        return;
      }

      loggedRoundKeys.add(roundKey);
      gameLogEntries.push({ type: "round", text: `Round ${round}` });
    }

    function appendFinishedMarker(gameState) {
      const finishKey = String(gameState.id);
      if (loggedFinishKeys.has(finishKey)) {
        return;
      }

      loggedFinishKeys.add(finishKey);
      gameLogEntries.push({ type: "round", text: "Game finished" });
      gameLogEntries.push({ type: "result", text: `Winner: ${formatPlayerIds(gameState.winners)}` });
    }

    function createGameInfoPanel(gameState) {
      const panel = document.createElement("section");
      panel.className = "game-info-panel";

      const tabs = document.createElement("div");
      tabs.className = "game-info-tabs";
      tabs.role = "tablist";

      const logTab = createInfoTab("log", "Game log");
      const ruleTab = createInfoTab("rule", "Game rule");
      tabs.append(logTab, ruleTab);

      const content = activeInfoTab === "rule"
        ? createGameRuleFrame(gameState)
        : createGameLogFrame();

      panel.append(tabs, content);
      return panel;
    }

    function createInfoTab(tabId, label) {
      const tab = document.createElement("button");
      tab.type = "button";
      tab.className = `game-info-tab${activeInfoTab === tabId ? " active" : ""}`;
      tab.role = "tab";
      tab.ariaSelected = String(activeInfoTab === tabId);
      tab.textContent = label;
      tab.addEventListener("click", () => {
        activeInfoTab = tabId;
        renderGameState(getCurrentGameState());
      });
      return tab;
    }

    function createGameLogFrame() {
      const logFrame = document.createElement("div");
      logFrame.className = "game-log-frame";

      if (gameLogEntries.length === 0) {
        const empty = document.createElement("p");
        empty.className = "game-log-empty";
        empty.textContent = "Round results will appear here.";
        logFrame.append(empty);
      } else {
        for (const entry of gameLogEntries) {
          const line = document.createElement("p");
          line.className = entry.type === "round" ? "game-log-round" : "game-log-line";
          line.textContent = entry.text;
          logFrame.append(line);
        }
      }

      return logFrame;
    }

    function createGameRuleFrame(gameState) {
      const ruleFrame = document.createElement("div");
      ruleFrame.className = "game-rule-frame";

      if (gameState.gameType === "power_defense_wave") {
        const rules = gameState.rules;
        const sections = [
          {
            title: "Round flow",
            items: [
              "All alive players choose one move each round.",
              "Moves resolve simultaneously after every alive player has moved.",
              "A move's survival is determined by the move chosen and incoming attacks, not by who dies later in the same round.",
              "The game ends when only one player, or one team, remains alive.",
            ],
          },
          {
            title: "Power",
            items: [
              `Gain ${rules.gainPowerAmount} power.`,
              "Power does not target another player.",
              "A player using Power is eliminated if attacked by Wave or Super Blast in the same round.",
            ],
          },
          {
            title: "Defense",
            items: [
              "Defense does not target another player.",
              "Defense survives an incoming Wave or one Super Blast.",
              "Multiple Super Blasts break Defense.",
              "A player cannot use Defense three rounds in a row.",
            ],
          },
          {
            title: "Wave",
            items: [
              `Costs ${rules.waveCost} power and targets one enemy.`,
              "Mutual Waves offset only when both players target each other.",
              "A Wave aimed somewhere else does not protect against an incoming Wave.",
              "Wave loses to Super Blast.",
            ],
          },
          {
            title: "Super Blast",
            items: [
              `Costs ${rules.superBlastCost} power.`,
              "Targets every enemy player.",
              "Super Blast users are not eliminated by other Super Blasts.",
              "Air Cannon is the direct counter to Super Blast.",
            ],
          },
          {
            title: "Air Cannon",
            items: [
              "Targets one enemy player.",
              "If the target used Super Blast, Air Cannon eliminates that target.",
              "Air Cannon survives the targeted Super Blast.",
              "Air Cannon does not block unrelated incoming attacks from other players.",
            ],
          },
        ];

        for (const section of sections) {
          ruleFrame.append(createRuleSection(section.title, section.items));
        }
      } else {
        ruleFrame.append(
          createRuleSection("Basic rules", [
            "All alive players choose one move each round.",
            "Attack costs power and targets one enemy.",
            "Defend blocks incoming attacks for the round.",
            "Gain power increases your available power for future attacks.",
            "The game ends when only one player, or one team, remains alive.",
          ])
        );
      }

      return ruleFrame;
    }

    function createRuleSection(title, items) {
      const section = document.createElement("section");
      section.className = "game-rule-section";

      const heading = document.createElement("h4");
      heading.textContent = title;

      const list = document.createElement("ul");
      for (const itemText of items) {
        const item = document.createElement("li");
        item.textContent = itemText;
        list.append(item);
      }

      section.append(heading, list);
      return section;
    }

    function scrollLogToBottom() {
      const logFrame = frame.querySelector(".game-log-frame");
      if (!logFrame) {
        return;
      }

      requestAnimationFrame(() => {
        logFrame.scrollTop = logFrame.scrollHeight;
      });
    }

    function formatResultLine(result) {
      return result.targetId
        ? `${result.playerId} ${result.message} to ${result.targetId}`
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
          const submittedMove = getSubmittedMove();
          helperText.textContent = submittedMove
            ? `You chose ${formatSubmittedMove(submittedMove)}. Waiting for other players.`
            : "You already moved this round. Waiting for other players.";
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

    function formatSubmittedMove(move) {
      const targetSuffix = move.targetId ? ` targeting ${move.targetId}` : "";
      return `${formatMoveName(move.moveType)}${targetSuffix}`;
    }

    function formatMoveName(moveType) {
      const moveNames = {
        air_cannon: "Air Cannon",
        attack: "Attack",
        defense: "Defense",
        defend: "Defend",
        gain_power: "Gain power",
        power: "Power",
        super_blast: "Super Blast",
        wave: "Wave",
      };

      return moveNames[moveType] || moveType;
    }

    function createPowerDefenseWaveControls(gameState) {
      const controls = document.createElement("div");
      controls.className = "game-controls";
      const player = currentPlayer(gameState);
      const hasTargets = attackTargets(gameState).length > 0;

      const targetHint = document.createElement("p");
      targetHint.className = "action-helper";
      targetHint.textContent = pendingTargetMoveType
        ? `Choose a target for ${formatMoveName(pendingTargetMoveType)}.`
        : "Choose Wave or Air Cannon first, then click a target player.";

      const powerButton = document.createElement("button");
      powerButton.type = "button";
      powerButton.textContent = `Power (+${gameState.rules.gainPowerAmount})`;
      powerButton.addEventListener("click", () => sendImmediateMove("power"));

      const defenseButton = document.createElement("button");
      defenseButton.type = "button";
      defenseButton.textContent = "Defense";
      defenseButton.disabled = (player?.defenseStreak || 0) >= 2;
      defenseButton.addEventListener("click", () => sendImmediateMove("defense"));

      const waveButton = document.createElement("button");
      waveButton.type = "button";
      waveButton.textContent = `Wave (${gameState.rules.waveCost} power)`;
      waveButton.disabled = !hasTargets || player.power < gameState.rules.waveCost;
      waveButton.addEventListener("click", () => startTargetedMove("wave"));

      const superBlastButton = document.createElement("button");
      superBlastButton.type = "button";
      superBlastButton.textContent = `Super Blast (${gameState.rules.superBlastCost} power)`;
      superBlastButton.disabled = player.power < gameState.rules.superBlastCost;
      superBlastButton.addEventListener("click", () => sendImmediateMove("super_blast"));

      const airCannonButton = document.createElement("button");
      airCannonButton.type = "button";
      airCannonButton.textContent = "Air Cannon";
      airCannonButton.disabled = !hasTargets;
      airCannonButton.addEventListener("click", () => startTargetedMove("air_cannon"));

      const basicActions = document.createElement("div");
      basicActions.className = "action-group";
      basicActions.append(powerButton, defenseButton, superBlastButton);

      const targetedActions = document.createElement("div");
      targetedActions.className = "action-group";
      targetedActions.append(targetHint, waveButton, airCannonButton);

      controls.append(basicActions, targetedActions);
      return controls;
    }

    function startTargetedMove(moveType) {
      pendingTargetMoveType = moveType;
      setSelectedTargetId(undefined);
      renderGameState(getCurrentGameState());
    }

    function sendImmediateMove(moveType) {
      pendingTargetMoveType = undefined;
      setSelectedTargetId(undefined);
      sendGameMove(moveType);
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
      return Boolean(
        pendingTargetMoveType &&
          canSubmitMove(gameState) &&
          attackTargets(gameState).some((target) => target.id === player.id)
      );
    }

    function selectPlayerTarget(targetId) {
      if (!pendingTargetMoveType) {
        return;
      }
      const moveType = pendingTargetMoveType;
      pendingTargetMoveType = undefined;
      setSelectedTargetId(targetId);
      sendGameMove(moveType, targetId);
    }

    return {
      hide,
      renderGameState,
      show,
    };
  }

  window.createGameScreen = createGameScreen;
})();
