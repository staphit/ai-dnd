import { useId, type ReactNode } from 'react';
import { Info } from '@phosphor-icons/react';

export type StatHintKey =
  | 'str' | 'dex' | 'con' | 'int' | 'wis' | 'cha'
  | 'hp' | 'ac' | 'initiative' | 'speed' | 'passive'
  | 'proficiency' | 'experience' | 'spellAbility' | 'spellAttack'
  | 'spellSaveDc' | 'spellSlots';

const descriptions: Record<StatHintKey, string> = {
  str: '力量代表肌力與爆發力，常用於近戰攻擊、運動、搬運重物與力量豁免。屬性值會換算成檢定修正值。',
  dex: '敏捷代表反應、平衡與手部精細度，影響先攻、護甲、防具外的閃避，以及隱匿、巧手等檢定。',
  con: '體質代表耐力與生命力，影響最大生命值、抵抗毒素，以及維持法術專注的豁免。',
  int: '智力代表記憶、推理與學識，常用於奧秘、歷史、調查、自然與宗教檢定。',
  wis: '感知代表直覺、觀察與意志，常用於察覺、洞悉、求生、醫藥與感知豁免。',
  cha: '魅力代表自信、氣勢與影響力，常用於說服、欺瞞、威嚇、表演及部分職業施法。',
  hp: '生命值歸零時角色會倒下。暫時生命會先承受傷害，但不能和一般生命相加治療。',
  ac: '護甲等級（AC）是攻擊要命中的門檻；攻擊總值等於或高於 AC 才會命中。',
  initiative: '先攻加值會加到戰鬥開始時的 d20；總值越高，通常越早行動。',
  speed: '速度是角色一回合通常可移動的距離，單位為呎；困難地形通常會加倍消耗移動距離。',
  passive: '被動察覺用於角色沒有主動搜索時發現威脅，通常是 10 加上察覺技能加值。',
  proficiency: '熟練加值會加入角色熟練的攻擊、技能、豁免與法術 DC，並隨總等級提升。',
  experience: '完成探索、里程碑與戰鬥可獲得 XP；達到下一級門檻後，才能在成長頁升級。',
  spellAbility: '施法屬性決定這個職業法術的攻擊加值與豁免 DC，不同職業可能使用不同屬性。',
  spellAttack: '施法攻擊時擲 d20 並加上此數值；總值等於或高於目標 AC 才命中。',
  spellSaveDc: '法術豁免 DC 是目標抵抗法術時必須達到的門檻；由施法屬性、熟練加值與固定基數計算。',
  spellSlots: '法術位是施放一環以上法術的資源。高環法術位可施放低環法術，多數法術位在長休後恢復。',
};

interface StatHintProps {
  hint: StatHintKey;
  enabled?: boolean;
  children: ReactNode;
}

export function StatHint({ hint, enabled = true, children }: StatHintProps) {
  const tooltipId = useId();
  if (!enabled) return <>{children}</>;

  return (
    <span className="stat-hint" tabIndex={0} aria-describedby={tooltipId}>
      {children}<Info className="stat-hint-icon" size={12} weight="bold" aria-hidden="true" />
      <span id={tooltipId} className="stat-tooltip" role="tooltip">{descriptions[hint]}</span>
    </span>
  );
}
