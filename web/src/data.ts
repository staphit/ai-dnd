import type { Campaign } from './types';
import { createLevel3Character } from './rules/characters';

export const initialCampaign: Campaign = {
  setupComplete: false,
  title: '灰燼王冠',
  chapter: '第一章／沉鐘之夜',
  scene: '下城區・無燈禮拜堂',
  round: 1,
  objective: '在午夜鐘響前找到失蹤的製圖師伊薩克',
  players: [
    createLevel3Character('player1', '賽勒恩・瓦爾', '遊俠'),
    createLevel3Character('player2', '米芮・鐵歌', '牧師'),
  ],
  story: [
    {
      id: 'opening-1',
      speaker: 'dm',
      time: '23:41',
      text: '禮拜堂的門在你們身後闔上，沒有風，燭火卻同時朝祭壇偏斜。石板中央留著一圈新鮮的泥痕，像有沉重的東西剛被拖進地下。祭壇後方傳來三下緩慢的敲擊聲。',
    },
    {
      id: 'opening-2',
      speaker: 'system',
      time: '23:42',
      text: '目前目標：在午夜鐘響前找到失蹤的製圖師伊薩克。兩位玩家都提交行動後，DM 才會推進場景。',
    },
  ],
  pending: {},
};

export const demoResponses = [
  '隊伍靠近泥痕時，聞到河底淤泥與燈油混合的氣味；這不是從街上帶進來的。祭壇側面的石縫浮出一行被刮除的銘文，就在字跡即將顯現時，地下的敲擊忽然停了。若要移開祭壇，需要一名角色進行 DC 13 的運動檢定；也可以先尋找不必用蠻力的機關。你們接下來怎麼做？',
  '隊伍的行動迫使藏在壁龕後的人影放棄偷襲。灰袍人把一卷沾血的羊皮紙丟進火盆，轉身奔向鐘塔。火焰尚未吞沒紙上的路線，而樓梯間已傳來鎖鏈拖地的聲音。救下地圖或追趕灰袍人，只剩一次快速選擇。',
  '銅製暗門向內滑開，露出只容一人通過的階梯。第一級階梯上放著伊薩克的羅盤，指針不是指向北方，而是牢牢對準隊伍中央。下方有潮水聲，也有刻意壓低的呼吸。誰先下去？其餘成員準備什麼掩護？',
];
