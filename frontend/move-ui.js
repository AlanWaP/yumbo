(function () {
  const TARGET_SCOPE_SINGLE_ENEMY = "single_enemy";
  const TARGETED_BUTTON_CLASS = "targeted-move";

  function requiresTargetPlayer(entry) {
    return Boolean(
      entry?.requiresTargetPlayer || entry?.targetScope === TARGET_SCOPE_SINGLE_ENEMY
    );
  }

  function hasTargetSelectionMoves(catalog) {
    return Array.isArray(catalog) && catalog.some(requiresTargetPlayer);
  }

  function formatMoveLabel(entry, gameState, t, formatMoveName) {
    if (entry.id === "power") {
      return t("game.powerButton", { amount: gameState.rules.gainPowerAmount });
    }
    if (entry.id === "gain_power") {
      return t("game.gainPowerButton", { amount: gameState.rules.gainPowerAmount });
    }
    return formatMoveName(entry.id);
  }

  function isMoveDisabled(entry, gameState, player, hasTargets) {
    if (!player) {
      return true;
    }
    if (entry.energyCost > 0 && player.power < entry.energyCost) {
      return true;
    }
    if (requiresTargetPlayer(entry) && !hasTargets) {
      return true;
    }
    if (entry.id === "defense" && (player.defenseStreak || 0) >= 2) {
      return true;
    }
    return false;
  }

  function buildTableMoves(catalog, gameState, player, hasTargets, t, formatMoveName) {
    if (!Array.isArray(catalog)) {
      return [];
    }

    return catalog.map((entry) => ({
      entry,
      energy: entry.energyCost,
      moveType: entry.id,
      label: formatMoveLabel(entry, gameState, t, formatMoveName),
      disabled: isMoveDisabled(entry, gameState, player, hasTargets),
    }));
  }

  window.yumboMoveUi = {
    TARGET_SCOPE_SINGLE_ENEMY,
    TARGETED_BUTTON_CLASS,
    requiresTargetPlayer,
    hasTargetSelectionMoves,
    buildTableMoves,
  };
})();
