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
  const submittedActions = Array.isArray(body?.actions)
    ? body.actions.slice(0, 4).map((action) => [String(action?.playerId || ''), String(action?.text || '').trim().slice(0, 2000)])
    : Object.entries(body?.actions || {}).map(([playerId, text]) => [playerId, String(text || '').trim().slice(0, 2000)]);
  const actions = new Map(submittedActions);
  if (players.length < 1 || players.some((player) => !actions.get(player.id))) {
    const error = new Error('需要隊伍中每位玩家的行動才能進行裁定。');
    // @ts-ignore custom HTTP status for the local server
    error.statusCode = 400;
    throw error;
  }
  const recent = Array.isArray(body?.history) ? body.history.slice(-16).map((entry) => {
    const audience = entry?.audience && entry.audience !== 'public' ? `（僅 ${entry.audience} 可見）` : '';
    return `${entry.speaker}${audience}: ${String(entry.text).slice(0, 1400)}`;
  }).join('\n') : '';
  const playerBlocks = players.flatMap((player, index) => [`玩家 ${index + 1}「${player.name}」（${player.className}／${player.subclass || '未選子職業'}）`, player.summary, `本輪宣告：${actions.get(player.id)}`, '']);
  const combat = body?.combat?.active && Array.isArray(body.combat.combatants)
    ? `戰鬥第 ${Number(body.combat.round || 1)} 輪：${body.combat.combatants.map((entry) => `${String(entry.name).slice(0, 50)} HP ${Number(entry.hp)}/${Number(entry.maxHp)} AC ${Number(entry.ac)} 先攻 ${Number(entry.initiative)}`).join('；')}`
    : '目前沒有進行中的戰鬥。';
  const prompt = [
    '規則版本：2024 第五版／SRD 5.2.1。角色卡快照與戰鬥追蹤器是本輪裁定的事實來源。',
    `戰役：${String(body?.campaign?.title || '灰燼王冠').slice(0, 180)}`, `場景：${String(body?.campaign?.scene || '未知地點').slice(0, 240)}`, `回合：${Number(body?.campaign?.round || 1)}`, combat,
    '', '最近紀錄：', recent || '這是冒險的開始。', '', '角色狀態與本輪行動：', ...playerBlocks,
    `請公平處理全隊 ${players.length} 個行動並推進場景。`,
    '若宣告需要已耗盡的資源、未準備的法術或角色不具備的能力，請指出限制並讓玩家改選；不可自行補回資源。',
    '若行動合理，讓每位角色的選擇產生可見回應。只有結果具有風險且不確定時才要求檢定。',
  ].join('\n');
  return { prompt, players };
}
