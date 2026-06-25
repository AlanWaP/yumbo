(function () {
  const DEFAULT_GAME_RULES = {
    gainPowerAmount: 1,
    waveCost: 1,
    superBlastCost: 3,
  };

  function defaultRulesForGameType() {
    return { ...DEFAULT_GAME_RULES };
  }

  function buildGameRuleSections(gameType, rules, t) {
    if (gameType === "power_defense_wave") {
      return [
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
    }

    if (gameType === "chaos_of_the_baby_city") {
      return [
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
          title: t("move.power"),
          items: [
            t("rules.babyCity.power.gain", { amount: rules.gainPowerAmount }),
            t("rules.babyCity.power.noTarget"),
            t("rules.babyCity.power.vulnerable"),
          ],
        },
        {
          title: t("move.defense"),
          items: [
            t("rules.babyCity.defense.free"),
            t("rules.babyCity.defense.blocks"),
            t("rules.babyCity.defense.limit"),
          ],
        },
        {
          title: t("move.cover_ear"),
          items: [
            t("rules.babyCity.coverEar.free"),
            t("rules.babyCity.coverEar.blocks"),
            t("rules.babyCity.coverEar.limit"),
          ],
        },
        {
          title: t("move.v"),
          items: [
            t("rules.babyCity.v.free"),
            t("rules.babyCity.v.blocks"),
            t("rules.babyCity.v.limit"),
          ],
        },
        {
          title: t("move.absorb"),
          items: [
            t("rules.babyCity.absorb.free"),
            t("rules.babyCity.absorb.blocks"),
            t("rules.babyCity.absorb.gain"),
            t("rules.babyCity.absorb.weakness"),
          ],
        },
        {
          title: t("move.knife"),
          items: [
            t("rules.babyCity.knife.free"),
            t("rules.babyCity.knife.range"),
            t("rules.babyCity.knife.beats"),
            t("rules.babyCity.knife.weakness"),
          ],
        },
        {
          title: t("move.seal"),
          items: [
            t("rules.babyCity.seal.free"),
            t("rules.babyCity.seal.nullify"),
            t("rules.babyCity.seal.invulnerable"),
            t("rules.babyCity.seal.power"),
            t("rules.babyCity.seal.detonation"),
          ],
        },
        {
          title: t("move.prick"),
          items: [t("rules.babyCity.prick.cost"), t("rules.babyCity.prick.mutual")],
        },
        {
          title: t("move.clang_clang"),
          items: [t("rules.babyCity.clangClang.cost"), t("rules.babyCity.clangClang.beats")],
        },
        {
          title: t("move.visa_ray"),
          items: [t("rules.babyCity.visaRay.cost"), t("rules.babyCity.visaRay.beats")],
        },
        {
          title: t("move.detonation"),
          items: [
            t("rules.babyCity.detonation.cost"),
            t("rules.babyCity.detonation.beats"),
            t("rules.babyCity.detonation.safe"),
          ],
        },
      ];
    }

    return [
      {
        title: t("rules.basic"),
        items: [
          t("rules.allAliveChoose"),
          t("rules.attackCost"),
          t("rules.defendBlocks"),
          t("rules.gainPowerBasic"),
          t("rules.ends"),
        ],
      },
    ];
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

  function renderGameRules(container, { gameType, rules, t }) {
    container.innerHTML = "";
    for (const section of buildGameRuleSections(gameType, rules, t)) {
      container.append(createRuleSection(section.title, section.items));
    }
  }

  function createGameRuleFrame({ gameType, rules, t }) {
    const ruleFrame = document.createElement("div");
    ruleFrame.className = "game-rule-frame";
    renderGameRules(ruleFrame, { gameType, rules, t });
    return ruleFrame;
  }

  window.yumboGameRules = {
    buildGameRuleSections,
    createGameRuleFrame,
    createRuleSection,
    defaultRulesForGameType,
    renderGameRules,
  };
})();
