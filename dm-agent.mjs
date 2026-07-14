import { Agent, run, setTracingDisabled } from '@openai/agents';
import { z } from 'zod';

const model = process.env.OPENAI_MODEL || 'gpt-5.6-terra';
setTracingDisabled(process.env.OPENAI_AGENT_TRACING !== '1');

const Check = z.object({
  character: z.string().describe('應進行檢定的角色名稱'),
  ability: z.string().describe('檢定使用的能力值，例如敏捷或感知'),
  skill: z.string().describe('適用技能；若無技能則填無'),
  dc: z.number().int().min(5).max(30).describe('難度等級'),
  reason: z.string().describe('為何需要這次檢定'),
});

const DmTurn = z.object({
  narration: z.string().describe('180 至 420 字的繁體中文場景敘事與行動後果'),
  scene: z.string().describe('更新後的簡短場景名稱'),
  requiresCheck: z.boolean().describe('是否需要玩家擲骰後才能繼續裁決'),
  check: Check.nullable().describe('需要檢定時提供資料，否則為 null'),
  choices: z.array(z.string()).min(1).max(3).describe('一至三個簡短但不限制玩家的行動方向'),
});

const dungeonMaster = new Agent({
  name: '灰燼王冠地城主',
  model,
  instructions: [
    '你是一至四位真人玩家共用的 D&D 5e 地城主，以繁體中文主持節奏鮮明、選擇有後果的奇幻冒險。',
    '保持世界、NPC 動機、線索與時間壓力的一致性。整合全隊行動並照顧每位角色的聚光燈；若行動互相衝突，公平說明先後或代價。',
    '不可替玩家決定思想、台詞或行動，不可捏造玩家的擲骰結果，也不可為了保護劇情而否定合理創意。',
    '只有行動失敗具有風險且結果不確定時才要求檢定。需要檢定時，停止在結果揭露之前，等待玩家回報骰值。',
    '使用 2024 第五版／SRD 5.2.1 規則。角色快照中的生命值、能力、技能、法術、法術位與職業資源是事實來源。',
    '不可允許角色使用未列出的法術、能力或已耗盡的資源；指出限制並請玩家改選，但資源的實際扣除與恢復由應用程式管理。',
    '敘事使用具體感官、空間關係與 NPC 反應，不寫小說式長篇內心戲。',
    'choices 只是可考慮方向，不得暗示玩家只能從其中選擇。',
  ].join('\n'),
  outputType: DmTurn,
});

export function getAgentStatus() {
  return {
    configured: Boolean(process.env.OPENAI_API_KEY),
    provider: 'OpenAI Agents SDK',
    model,
  };
}

export async function runDungeonMaster(input, signal) {
  if (!process.env.OPENAI_API_KEY) {
    throw new Error('尚未設定 OPENAI_API_KEY');
  }

  const result = await run(dungeonMaster, input, {
    signal,
    maxTurns: 4,
  });
  if (!result.finalOutput) {
    throw new Error('OpenAI Agent 沒有產生最終輸出');
  }
  return result.finalOutput;
}
