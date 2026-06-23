(function () {
  function createGameScreen({
    frame,
    getPlayerId,
    getSubmittedRound,
    getSubmittedMove,
    getSelectedTargetId,
    setSelectedTargetId,
    getCurrentGameState,
    getMoveCancelAvailable,
    sendGameMove,
    cancelGameMove,
    formatPlayerIds,
    t,
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
        setPlaceholder(t("game.readyTitle"), t("game.readyDetail"));
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
      playerPanelTitle.textContent = t("game.players");

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
          <strong>${player.id === getPlayerId() ? t("game.you") : player.id}</strong>
          <span>${t("game.team", { team: player.teamId })}</span>
          <span>${t("game.health", { health: player.health })}</span>
          <span>${t("game.powerStat", { power: player.power })}</span>
          <span>${t("game.defenseStreak", { streak: player.defenseStreak || 0 })}</span>
          <span>${player.alive ? t("game.alive") : t("game.eliminated")}</span>
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
            gameLogEntries.push({ type: "result", result });
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
      gameLogEntries.push({ type: "round", key: "game.round", values: { round } });
    }

    function appendFinishedMarker(gameState) {
      const finishKey = String(gameState.id);
      if (loggedFinishKeys.has(finishKey)) {
        return;
      }

      loggedFinishKeys.add(finishKey);
      gameLogEntries.push({ type: "round", key: "game.finished" });
      gameLogEntries.push({
        type: "round",
        key: "game.winner",
        values: { winners: formatPlayerIds(gameState.winners) },
      });
    }

    function createGameInfoPanel(gameState) {
      const panel = document.createElement("section");
      panel.className = "game-info-panel";

      const tabs = document.createElement("div");
      tabs.className = "game-info-tabs";
      tabs.role = "tablist";

      const logTab = createInfoTab("log", t("game.logTab"));
      const ruleTab = createInfoTab("rule", t("game.ruleTab"));
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
        empty.textContent = t("game.logEmpty");
        logFrame.append(empty);
      } else {
        for (const entry of gameLogEntries) {
          const line = document.createElement("p");
          line.className = entry.type === "round" ? "game-log-round" : "game-log-line";
          line.textContent = formatGameLogEntry(entry);
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
            title: t("rules.roundFlow"),
            items: [
              t("rules.allAliveChoose"),
              t("rules.simultaneous"),
              t("rules.survival"),
              t("rules.ends"),
            ],
          },
          {
            title: t("rules.power"),
            items: [
              t("rules.gainPower", { amount: rules.gainPowerAmount }),
              t("rules.powerNoTarget"),
              t("rules.powerEliminated"),
            ],
          },
          {
            title: t("rules.defense"),
            items: [
              t("rules.defenseNoTarget"),
              t("rules.defenseSurvives"),
              t("rules.multipleBreak"),
              t("rules.noThreeDefense"),
            ],
          },
          {
            title: t("rules.wave"),
            items: [
              t("rules.waveCost", { cost: rules.waveCost }),
              t("rules.mutualWave"),
              t("rules.waveElsewhere"),
              t("rules.waveLoses"),
            ],
          },
          {
            title: t("rules.superBlast"),
            items: [
              t("rules.superBlastCost", { cost: rules.superBlastCost }),
              t("rules.targetsEveryEnemy"),
              t("rules.superBlastSafe"),
              t("rules.airCounter"),
            ],
          },
          {
            title: t("rules.airCannon"),
            items: [
              t("rules.airTargets"),
              t("rules.airEliminates"),
              t("rules.airSurvives"),
              t("rules.airDoesNotBlock"),
            ],
          },
        ];

        for (const section of sections) {
          ruleFrame.append(createRuleSection(section.title, section.items));
        }
      } else {
        ruleFrame.append(
          createRuleSection(t("rules.basic"), [
            t("rules.allAliveChoose"),
            t("rules.attackCost"),
            t("rules.defendBlocks"),
            t("rules.gainPowerBasic"),
            t("rules.ends"),
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

    function formatGameLogEntry(entry) {
      if (entry.result) {
        return formatResultLine(entry.result);
      }

      return t(entry.key, entry.values);
    }

    function formatResultLine(result) {
      const message = translateResultMessage(result.message);
      return result.targetId
        ? t("game.resultTo", { player: result.playerId, message, target: result.targetId })
        : t("game.result", { player: result.playerId, message });
    }

    function translateResultMessage(message) {
      const gainedPowerMatch = /^gained (\d+) power$/.exec(message);
      if (gainedPowerMatch) {
        return t("result.gainedPower", { amount: gainedPowerMatch[1] });
      }

      const eliminatedMatch = /^eliminated: (.+)$/.exec(message);
      if (eliminatedMatch) {
        return t("result.eliminated", { reason: translateEliminationReason(eliminatedMatch[1]) });
      }

      const messages = {
        "aimed air cannon": "result.aimedAirCannon",
        attacked: "result.attacked",
        "attack blocked": "result.attackBlocked",
        "attack eliminated target": "result.attackEliminatedTarget",
        defended: "result.defended",
        "sent a wave": "result.sentWave",
        "used super blast": "result.usedSuperBlast",
      };

      return messages[message] ? t(messages[message]) : message;
    }

    function translateEliminationReason(reason) {
      const airCannonMatch = /^hit by (.+)'s air cannon$/.exec(reason);
      if (airCannonMatch) {
        return t("reason.hitByAirCannon", { player: airCannonMatch[1] });
      }

      const reasons = {
        "attacked while using air cannon": "reason.attackedWithAirCannon",
        "defense was broken by multiple super blasts": "reason.defenseBroken",
        "powered up while attacked": "reason.poweredUpWhileAttacked",
        "wave did not offset incoming wave": "reason.waveDidNotOffset",
        "wave was overpowered by super blast": "reason.waveOverpowered",
      };

      return reasons[reason] ? t(reasons[reason]) : reason;
    }

    function createActionPanel(gameState) {
      const panel = document.createElement("aside");
      panel.className = "action-panel";

      const title = document.createElement("h3");
      title.textContent = t("game.actions");
      panel.append(title);

      const helperText = document.createElement("p");
      helperText.className = "action-helper";

      if (!canSubmitMove(gameState)) {
        if (gameState.phase === "finished") {
          helperText.textContent = t("game.over");
        } else if (isWaitingAfterSubmit(gameState)) {
          const submittedMove = getSubmittedMove();
          helperText.textContent = submittedMove
            ? t("game.choseMove", { move: formatSubmittedMove(submittedMove) })
            : t("game.alreadyMoved");
          panel.append(helperText);
          const actions = document.createElement("div");
          actions.className = "action-buttons";
          actions.append(createCancelButton());
          panel.append(actions);
        } else if (!currentPlayer(gameState)?.alive) {
          helperText.textContent = t("game.cannotMoveEliminated");
          panel.append(helperText);
        } else {
          helperText.textContent = t("game.waitingAction");
          panel.append(helperText);
        }
        return panel;
      }

      helperText.textContent = formatActionHelper(
        gameState,
        gameState.gameType === "power_defense_wave"
      );
      panel.append(
        helperText,
        gameState.gameType === "power_defense_wave"
          ? createPowerDefenseWaveControls(gameState)
          : createMoveControls(gameState)
      );
      return panel;
    }

    function formatActionHelper(gameState, isPowerDefenseWave) {
      if (pendingTargetMoveType) {
        return t("game.chooseTargetFor", { move: formatMoveName(pendingTargetMoveType) });
      }
      if (isPowerDefenseWave) {
        return t("game.pickMovePdw");
      }
      return t("game.pickMove");
    }

    function isWaitingAfterSubmit(gameState) {
      if (getMoveCancelAvailable()) {
        return gameState?.phase !== "finished";
      }
      return hasSubmittedThisRound(gameState) || hasOptimisticSubmission(gameState);
    }

    function hasOptimisticSubmission(gameState) {
      return Boolean(
        getSubmittedMove() &&
          gameState?.phase === "waiting_for_moves" &&
          roundNumber(getSubmittedRound()) === roundNumber(gameState?.round)
      );
    }

    function roundNumber(value) {
      const parsed = Number(value);
      return Number.isFinite(parsed) ? parsed : undefined;
    }

    function hasSubmittedThisRound(gameState) {
      const submittedRound = roundNumber(getSubmittedRound());
      const currentRound = roundNumber(gameState?.round);
      return submittedRound !== undefined && submittedRound === currentRound;
    }

    function createCancelButton() {
      const cancelButton = document.createElement("button");
      cancelButton.type = "button";
      cancelButton.className = "action-cancel-button";
      cancelButton.textContent = t("game.cancelMove");
      cancelButton.addEventListener("click", () => cancelSelection());
      return cancelButton;
    }

    function cancelSelection() {
      const gameState = getCurrentGameState();
      if (isWaitingAfterSubmit(gameState)) {
        cancelGameMove();
        return;
      }

      clearLocalStaging();
      renderGameState(getCurrentGameState());
    }

    function clearLocalStaging() {
      pendingTargetMoveType = undefined;
      setSelectedTargetId(undefined);
    }

    function armTargetedMove(moveType) {
      pendingTargetMoveType = moveType;
      setSelectedTargetId(undefined);
      renderGameState(getCurrentGameState());
    }

    function isArmedMove(moveType) {
      return pendingTargetMoveType === moveType;
    }

    function createMoveButton(label, moveType, options = {}) {
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = label;
      button.disabled = Boolean(options.disabled);
      if (isArmedMove(moveType)) {
        button.classList.add("selected-move");
      }
      button.addEventListener("click", () => {
        if (options.targeted) {
          armTargetedMove(moveType);
        } else {
          clearLocalStaging();
          sendGameMove(moveType);
        }
      });
      return button;
    }

    function formatSubmittedMove(move) {
      return move.targetId
        ? t("game.targeting", { move: formatMoveName(move.moveType), target: move.targetId })
        : formatMoveName(move.moveType);
    }

    function formatMoveName(moveType) {
      const translatedMove = t(`move.${moveType}`);
      return translatedMove === `move.${moveType}` ? moveType : translatedMove;
    }

    function createMoveTable(moves) {
      const table = document.createElement("table");
      table.className = "move-table";

      const thead = document.createElement("thead");
      const headerRow = document.createElement("tr");

      const energyHeader = document.createElement("th");
      energyHeader.scope = "col";
      energyHeader.textContent = t("game.moveTableEnergy");

      const movesHeader = document.createElement("th");
      movesHeader.scope = "col";
      movesHeader.textContent = t("game.moveTableMoves");

      headerRow.append(energyHeader, movesHeader);
      thead.append(headerRow);

      const tbody = document.createElement("tbody");
      const groups = groupAdjacentMovesByEnergy(moves);

      for (const group of groups) {
        const tableRow = document.createElement("tr");
        tableRow.classList.add("move-table-energy-end");

        const energyCell = document.createElement("th");
        energyCell.scope = "row";
        energyCell.textContent = String(group.energy);

        const movesCell = document.createElement("td");
        const moveButtons = document.createElement("div");
        moveButtons.className = "move-table-buttons";
        for (const move of group.moves) {
          moveButtons.append(
            createMoveButton(move.label, move.moveType, {
              disabled: move.disabled,
              targeted: move.targeted,
            })
          );
        }
        movesCell.append(moveButtons);
        tableRow.append(energyCell, movesCell);
        tbody.append(tableRow);
      }

      table.append(thead, tbody);
      return table;
    }

    function groupAdjacentMovesByEnergy(moves) {
      const groups = [];
      for (const move of moves) {
        const lastGroup = groups[groups.length - 1];
        if (lastGroup && lastGroup.energy === move.energy) {
          lastGroup.moves.push(move);
          continue;
        }
        groups.push({ energy: move.energy, moves: [move] });
      }
      return groups;
    }

    function createPowerDefenseWaveControls(gameState) {
      const controls = document.createElement("div");
      controls.className = "game-controls";
      const player = currentPlayer(gameState);
      const hasTargets = attackTargets(gameState).length > 0;
      const rules = gameState.rules;

      const moves = [
        {
          energy: 0,
          moveType: "power",
          label: t("game.powerButton", { amount: rules.gainPowerAmount }),
        },
        {
          energy: 0,
          moveType: "defense",
          label: formatMoveName("defense"),
          disabled: (player?.defenseStreak || 0) >= 2,
        },
        {
          energy: 0,
          moveType: "air_cannon",
          label: formatMoveName("air_cannon"),
          targeted: true,
          disabled: !hasTargets,
        },
        {
          energy: rules.waveCost,
          moveType: "wave",
          label: formatMoveName("wave"),
          targeted: true,
          disabled: !hasTargets || player.power < rules.waveCost,
        },
        {
          energy: rules.superBlastCost,
          moveType: "super_blast",
          label: formatMoveName("super_blast"),
          disabled: player.power < rules.superBlastCost,
        },
      ];

      controls.append(createMoveTable(moves));
      return controls;
    }

    function selectPlayerTarget(targetId) {
      if (!pendingTargetMoveType) {
        return;
      }
      const moveType = pendingTargetMoveType;
      clearLocalStaging();
      sendGameMove(moveType, targetId);
    }

    function createMoveControls(gameState) {
      const controls = document.createElement("div");
      controls.className = "game-controls";

      const attackGroup = document.createElement("div");
      attackGroup.className = "action-group";

      const attackLabel = document.createElement("label");
      attackLabel.textContent = t("game.attackTarget");

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
      attackButton.textContent = t("game.costPowerButton", {
        move: formatMoveName("attack"),
        cost: gameState.rules.attackCost,
      });
      attackButton.disabled =
        targets.length === 0 || currentPlayer(gameState).power < gameState.rules.attackCost;
      attackButton.addEventListener("click", () => {
        sendGameMove("attack", targetSelect.value);
      });
      attackGroup.append(attackLabel, targetSelect, attackButton);

      const basicActions = document.createElement("div");
      basicActions.className = "action-group";
      basicActions.append(
        createMoveButton(formatMoveName("defend"), "defend"),
        createMoveButton(
          t("game.gainPowerButton", { amount: gameState.rules.gainPowerAmount }),
          "gain_power"
        )
      );

      controls.append(attackGroup, basicActions);
      return controls;
    }

    function canSubmitMove(gameState) {
      const player = currentPlayer(gameState);
      return Boolean(
        player &&
          player.alive &&
          gameState.phase === "waiting_for_moves" &&
          !isWaitingAfterSubmit(gameState)
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

    return {
      hide,
      renderGameState,
      show,
    };
  }

  window.createGameScreen = createGameScreen;
})();
