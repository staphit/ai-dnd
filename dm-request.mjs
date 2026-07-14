const abilityLabels = { str: '力量', dex: '敏捷', con: '體質', int: '智力', wis: '感知', cha: '魅力' };

/** @param {any} player @param {number} index */
function sanitizePlayer(player, index) {
  const abilities = Object.entries(abilityLabels).map(([key, label]) => `${label}${Number(player?.abilities?.[key] || 10)}`).join('、');
  const skills = Array.isArray(player?.skills) ? player.skills.filter((skill) => skill?.proficient).map((skill) => `${String(skill.name).slice(0, 40)}${Number(skill.bonus) >= 0 ? '+' : ''}${Number(skill.bonus)}`).join('、') : '';
  const attacks = Array.isArray(player?.attacks) ? player.attacks.slice(0, 8).map((attack) => `${String(attack.name).slice(0, 60)} 命中${Number(attack.attackBonus) >= 0 ? '+' : ''}${Number(attack.attackBonus)}／${String(attack.damage).slice(0, 30)}${String(attack.damageType || '').slice(0, 20)}`).join('；') : '';
  const resources = Array.isArray(player?.resources) ? player.resources.slice(0, 10).map((resource) => `${String(resource.name).slice(0, 60)} ${Number(resource.current)}/${Number(resource.max)}`).join('、') : '';
  const slots = Array.isArray(player?.spellcasting?.slots) ? player.spellcasting.slots.map((slot) => `${Number(slot.level)}環 ${Number(slot.current)}/${Number(slot.max)}`).join('、') : '';
  const spells = Array.isArray(player?.spellcasting?.spells) ? player.spellcasting.spells.filter((spell) => Number(spell?.level) === 0 || spell?.prepared || spell?.alwaysPrepared).slice(0, 30).map((spell) => `${String(spell.name).slice(0, 60)}${Number(spell.level) === 0 ? '(戲法)' : `(${Number(spell.level)}環)`}`).join('、') : '';
  const features = Array.isArray(player?.features) ? player.features.slice(0, 12).map((entry) => String(entry.name).slice(0, 60)).join('、') : '';
  const classLevels = Array.isArray(player?.classLevels) ? player.classLevels.map((entry) => `${String(entry.className).slice(0, 40)}${Number(entry.level)}`).join('／') : '';
  return {
    id: `player${index + 1}`,
    name: String(player?.name || `玩家 ${index + 1}`).trim().slice(0, 100),
    className: String(player?.className || '冒險者').trim().slice(0, 100),
    subclass: String(player?.subclass || '').trim().slice(0, 100),
    summary: [
      `等級 ${Number(player?.level || 3)}${classLevels ? `（${classLevels}）` : ''}；種族 ${String(player?.species || '未設定').slice(0, 60)}；背景 ${String(player?.background || '未設定').slice(0, 60)}；${abilities}`,
      `HP ${Number(player?.hp || 0)}/${Number(player?.maxHp || 0)}；AC ${Number(player?.ac || 10)}；速度 ${Number(player?.speed || 30)}；熟練 +${Number(player?.proficiencyBonus || 2)}`,
      skills && `熟練技能：${skills}`, attacks && `攻擊：${attacks}`, resources && `職業資源：${resources}`, features && `職業能力：${features}`, slots && `法術位：${slots}`, spells && `可施放法術：${spells}`,
      player?.concentration && `目前專注：${String(player.concentration).slice(0, 80)}`,
    ].filter(Boolean).join('\n'),
  };
}

/** @param {any} body */
export function buildDmRequest(body) {
  const players = Array.isArray(body?.players) ? body.players.slice(0, 4).map(sanitizePlayer) : [];
  const resolution = body?.resolution && typeof body.resolution === 'object' ? {
    character: String(body.resolution.character || '').slice(0, 100),
    ability: String(body.resolution.ability || '').slice(0, 100),
    skill: String(body.resolution.skill || '').slice(0, 100),
    reason: String(body.resolution.reason || '').slice(0, 500),
    dc: Math.max(5, Math.min(30, Number(body.resolution.dc || 10))),
    natural: Math.max(1, Math.min(20, Number(body.resolution.natural || 1))),
    modifier: Math.max(-20, Math.min(30, Number(body.resolution.modifier || 0))),
    total: Math.max(-19, Math.min(50, Number(body.resolution.total || 0))),
    success: body.resolution.success === true,
  } : null;
  const combatConclusion = body?.combatConclusion && typeof body.combatConclusion === 'object' ? {
    outcome: ['victory', 'defeat', 'withdrawal'].includes(body.combatConclusion.outcome) ? body.combatConclusion.outcome : 'withdrawal',
    summary: String(body.combatConclusion.summary || '').trim().slice(0, 3000),
  } : null;
  const isContinuation = Boolean(resolution || combatConclusion);
  const submittedActions = Array.isArray(body?.actions)
    ? body.actions.slice(0, 4).map((action) => [String(action?.playerId || ''), String(action?.text || '').trim().slice(0, 2000)])
    : Object.entries(body?.actions || {}).map(([playerId, text]) => [playerId, String(text || '').trim().slice(0, 2000)]);
  const actions = new Map(submittedActions);
  if (players.length < 1 || (!isContinuation && players.some((player) => !actions.get(player.id)))) {
    const error = new Error('需要隊伍中每位玩家的行動才能進行裁定。');
    // @ts-ignore custom HTTP status for the local server
    error.statusCode = 400;
    throw error;
  }
  const recent = Array.isArray(body?.history) ? body.history.slice(-16).map((entry) => {
    const audience = entry?.audience && entry.audience !== 'public' ? `（僅 ${entry.audience} 可見）` : '';
    return `${entry.speaker}${audience}: ${String(entry.text).slice(0, 1400)}`;
  }).join('\n') : '';
  const playerBlocks = players.flatMap((player, index) => [`玩家 ${index + 1}「${player.name}」（${player.className}／${player.subclass || '未選子職業'}）`, player.summary, ...(isContinuation ? [] : [`本輪宣告：${actions.get(player.id)}`]), '']);
  const combat = body?.combat?.active && Array.isArray(body.combat.combatants)
    ? `戰鬥第 ${Number(body.combat.round || 1)} 輪：${body.combat.combatants.map((entry) => `${String(entry.name).slice(0, 50)} HP ${Number(entry.hp)}/${Number(entry.maxHp)} AC ${Number(entry.ac)} 先攻 ${Number(entry.initiative)}`).join('；')}`
    : '目前沒有進行中的戰鬥。';
  const prompt = [
    '規則版本：2024 第五版／SRD 5.2.1。角色卡快照與戰鬥追蹤器是本輪裁定的事實來源。',
    `戰役：${String(body?.campaign?.title || '灰燼王冠').slice(0, 180)}`, `場景：${String(body?.campaign?.scene || '未知地點').slice(0, 240)}`, `目前目標：${String(body?.campaign?.objective || '尚未確定').slice(0, 240)}`, `任務背景：${String(body?.campaign?.objectiveContext || '尚未確定').slice(0, 600)}`, `風險：${String(body?.campaign?.stakes || '尚未確定').slice(0, 300)}`, `回合：${Number(body?.campaign?.round || 1)}`, combat,
    '', '最近紀錄：', recent || '這是冒險的開始。', '', isContinuation ? '角色狀態：' : '角色狀態與本輪行動：', ...playerBlocks,
    ...(resolution ? [
      '這是上一輪玩家行動所觸發的必要檢定結果，不是新的玩家行動：',
      `${resolution.character}進行${resolution.ability}（${resolution.skill}）檢定，原因：${resolution.reason}。d20 骰面 ${resolution.natural}${resolution.modifier >= 0 ? '+' : ''}${resolution.modifier}，總值 ${resolution.total}，DC ${resolution.dc}，結果為${resolution.success ? '成功' : '失敗'}。`,
      '請直接接續敘述此成功或失敗造成的具體後果並推進場景。不可插入、假設或要求任何新的玩家行動；actionIssues 必須為空陣列。若後果又立即產生另一個有風險且不確定的檢定，才可建立新的結構化 check。',
    ] : combatConclusion ? [
      '戰鬥追蹤器剛剛完成戰鬥，這不是新的玩家行動：',
      `戰鬥結果：${combatConclusion.outcome === 'victory' ? '隊伍勝利' : combatConclusion.outcome === 'defeat' ? '隊伍戰敗' : '戰鬥中止或撤退'}。${combatConclusion.summary}`,
      '請直接敘述戰鬥結束後的現場、存活者反應、立即後果與新局勢，並更新目標背景和風險。不可插入、假設或要求新的玩家行動；不可再次結算傷害或 XP；combat.starts 必須為 false、actionIssues 必須為空陣列。',
    ] : [`請公平處理全隊 ${players.length} 個行動並推進場景。`]),
    '若宣告需要已耗盡的資源、未準備的法術、缺少必要目標、超過行動次數，或角色不具備的能力，必須在 actionIssues 駁回並給出具體規則理由與修改方向；不可自行補回資源、改寫行動或讓故事先推進。',
    '若行動合理，讓每位角色的選擇產生可見回應。只有結果具有風險且不確定時才要求檢定。',
  ].join('\n');
  return { prompt, players };
}
