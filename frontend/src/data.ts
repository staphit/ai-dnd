import type { Campaign } from './types';
import { createLevel3Character } from './rules/characters';

interface DemoBeat {
  text: string;
  choices: string[];
  objective: string;
  objectiveContext: string;
  check?: { ability: string; skill: string; dc: number; reason: string };
}

export interface StoryPreset {
  id: string;
  title: string;
  genre: string;
  summary: string;
  tags: string[];
  chapter: string;
  scene: string;
  objective: string;
  objectiveContext: string;
  stakes: string;
  opening: string;
  demoBeats: DemoBeat[];
}

export const storyPresets: StoryPreset[] = [
  {
    id: 'ashen-crown',
    title: '灰燼王冠',
    genre: '哥德懸疑',
    summary: '追查失蹤製圖師，揭開無燈禮拜堂與沉沒王冠的祕密。',
    tags: ['調查', '地下城', '城市陰謀'],
    chapter: '第一章／沉鐘之夜',
    scene: '下城區・無燈禮拜堂',
    objective: '在午夜鐘響前找到失蹤的製圖師伊薩克',
    objectiveContext: '製圖師伊薩克在調查下城區失蹤事件後失去音訊。他最後留下的地圖指向無燈禮拜堂，而祭壇下方傳來不自然的敲擊聲。',
    stakes: '午夜鐘響後，地下水道會漲潮，伊薩克留下的線索可能被淹沒，失蹤者也將更難救回。',
    opening: '禮拜堂的門在隊伍身後闔上。沒有風，燭火卻同時朝祭壇偏斜；石板上的泥痕一路延伸到祭壇，而後方傳來三下緩慢的敲擊聲。',
    demoBeats: [
      {
        text: '隊伍靠近泥痕時，聞到河底淤泥與燈油混合的氣味；這不是從街上帶進來的。祭壇側面的石縫浮出一行被刮除的銘文，就在字跡即將顯現時，地下的敲擊忽然停了。若要移開祭壇，需要一名角色進行 DC 13 的運動檢定；也可以先尋找不必用蠻力的機關。你們接下來怎麼做？',
        choices: ['尋找祭壇機關', '移動祭壇', '先觀察泥痕'],
        objective: '找出祭壇下方敲擊聲的來源',
        objectiveContext: '伊薩克最後的地圖指向這座禮拜堂；新鮮泥痕與河底淤泥的氣味顯示，有東西剛從祭壇附近被拖入地下。',
        check: { ability: '力量', skill: '運動', dc: 13, reason: '祭壇沉重且卡在石槽中，強行移動有失手風險。' },
      },
      {
        text: '隊伍的行動迫使藏在壁龕後的人影放棄偷襲。灰袍人把一卷沾血的羊皮紙丟進火盆，轉身奔向鐘塔。火焰尚未吞沒紙上的路線，而樓梯間已傳來鎖鏈拖地的聲音。救下地圖或追趕灰袍人，只剩一次快速選擇。',
        choices: ['搶救燃燒的地圖', '追趕灰袍人', '封鎖鐘塔出口'],
        objective: '在地圖燒毀前取得線索，或攔下逃往鐘塔的灰袍人',
        objectiveContext: '灰袍人攜帶伊薩克的染血地圖，暴露後焚毀證據並逃往鐘塔；樓梯間另有鎖鏈聲逼近。',
      },
      {
        text: '銅製暗門向內滑開，露出只容一人通過的階梯。第一級階梯上放著伊薩克的羅盤，指針不是指向北方，而是牢牢對準隊伍中央。下方有潮水聲，也有刻意壓低的呼吸。誰先下去？其餘成員準備什麼掩護？',
        choices: ['由斥候先下階梯', '檢查羅盤的魔法', '在入口設置防線'],
        objective: '沿暗門階梯尋找伊薩克並確認下方威脅',
        objectiveContext: '祭壇下的暗門已開啟，伊薩克的羅盤留在第一級階梯，指針卻異常指向隊伍中央。',
      },
    ],
  },
  {
    id: 'last-light-of-frostharbor',
    title: '霜港的最後燈火',
    genre: '極地求生',
    summary: '暴風雪封鎖孤港，熄滅的燈塔正把亡者船隊引回人間。',
    tags: ['求生', '航海', '超自然'],
    chapter: '第一章／白潮封港',
    scene: '霜港・碎冰碼頭',
    objective: '在幽靈艦隊靠岸前重新點燃北岬燈塔',
    objectiveContext: '霜港的燈塔在百年暴雪中熄滅，守塔人失蹤，海面卻出現逆風航行的黑帆。唯一通往北岬的冰橋正快速崩裂。',
    stakes: '黎明前若燈塔仍未點亮，亡者船隊將循著港內居民的體溫登岸，整座霜港會成為新的幽靈港。',
    opening: '暴雪吞沒碼頭盡頭，港鐘卻在無人拉動下連響七次。冰層下浮現一艘倒著航行的黑船影子，而遠方燈塔的最後一點火光恰在此刻熄滅。',
    demoBeats: [
      {
        text: '破冰船長把一枚結霜的燈塔鑰匙塞進你們手中，隨即被冰層下伸出的蒼白手掌拖倒。碼頭棧橋正在斷裂，船長腰間還綁著通往北岬的航路圖。救人、搶圖，或立刻奔向冰橋，每個選擇都會讓暴雪更近一步。',
        choices: ['拉住破冰船長', '割下航路圖', '立刻衝向冰橋'],
        objective: '取得前往北岬的安全路線並穿越崩裂冰橋',
        objectiveContext: '守塔鑰匙已到手，但通往北岬的航路圖仍在遇襲船長身上；碼頭與冰橋都撐不了多久。',
        check: { ability: '力量', skill: '運動', dc: 13, reason: '冰下的力量正在把船長拖走，濕滑棧橋也讓救援更加危險。' },
      },
      {
        text: '冰橋中央傳來幼童的哭聲。一名裹著紅披風的女孩站在風雪裡，腳下卻沒有影子；她指向安全路徑，要求你們答應在燈塔裡「不要叫醒父親」。同一時間，身後的黑帆已穿過第一道浮冰。',
        choices: ['相信紅披風女孩', '識破她的真身', '另找路繞過冰橋'],
        objective: '穿過冰橋並查明守塔人的命運',
        objectiveContext: '神祕女孩知道安全路徑，也似乎與失蹤守塔人有關；亡者船隊正在逼近港口。',
      },
      {
        text: '燈塔頂層的火盆裡不是燈油，而是一顆仍在跳動的冰藍心臟。守塔人跪在旁邊，懇求你們別點火：每次燈塔照亮海面，都會燒掉他女兒的一段記憶。塔外，第一艘幽靈船已撞上防波堤。',
        choices: ['點燃冰藍心臟', '尋找替代燃料', '逼問守塔人真相'],
        objective: '決定燈塔的代價並阻止幽靈艦隊登岸',
        objectiveContext: '燈塔以女孩的記憶為燃料，守塔人因此讓火焰熄滅；艦隊已抵達防波堤。',
      },
    ],
  },
  {
    id: 'emerald-slumber',
    title: '翡翠沉眠',
    genre: '荒野奇譚',
    summary: '整座村莊陷入共享夢境，森林則開始長出居民不願面對的記憶。',
    tags: ['探索', '夢境', '道德抉擇'],
    chapter: '第一章／不醒之村',
    scene: '翠眠森林・苔鐘村',
    objective: '在日落前喚醒苔鐘村，找到夢疫的源頭',
    objectiveContext: '商旅發現村民全在同一刻沉睡，藤蔓從屋內纏向森林深處。每個睡夢者都反覆念著一個已被村史抹去的名字。',
    stakes: '日落後夢境會永久覆寫現實；村民將忘記自己曾經醒著，而森林會把更多旅人納入同一場夢。',
    opening: '村中每扇門都敞開著，桌上的湯仍冒著熱氣，居民卻在原地沉睡。廣場古樹長滿一夜綻放的翡翠花，其中一朵正用隊伍成員的聲音低聲求救。',
    demoBeats: [
      {
        text: '當你們碰觸翡翠花，廣場瞬間變成十年前的慶典。夢中的村民看得見你們，卻堅稱今天將處決一名叫「瑟芙」的女巫。現實裡，藤蔓正沿著你們的手腕攀爬；若想留下調查，必須先抵抗夢境的安撫。',
        choices: ['混入十年前的慶典', '立刻斬斷藤蔓', '尋找夢中的瑟芙'],
        objective: '查明瑟芙與夢疫的關係',
        objectiveContext: '共享夢境重演十年前被抹去的處刑；瑟芙可能是受害者，也可能是操控夢境的人。',
        check: { ability: '感知', skill: '洞悉', dc: 12, reason: '夢境會放大最安心的記憶，使人忽略身體正被藤蔓纏住。' },
      },
      {
        text: '你們找到年幼的瑟芙時，她正把一枚種子藏進村長的影子。她說村民不是被詛咒，而是自願忘記當年的罪；只要喚醒他們，所有被壓下的記憶也會一起回來。遠處已響起處刑前的鼓聲。',
        choices: ['保護年幼的瑟芙', '奪走記憶種子', '質問夢中的村長'],
        objective: '取得記憶種子並決定是否揭露村莊舊罪',
        objectiveContext: '村民以遺忘換取平靜，夢疫則逼迫真相重演；記憶種子是喚醒眾人的關鍵。',
      },
      {
        text: '森林心臟裡，成年瑟芙已與巨樹融為一體。她願意釋放村民，但要求帶走每個人最珍貴的一段回憶，讓森林不再飢餓。若拒絕，就必須在日落前切斷遍布全村的夢根。',
        choices: ['接受記憶交換', '切斷森林夢根', '提出由隊伍支付代價'],
        objective: '在日落前終結夢疫並選擇誰承擔代價',
        objectiveContext: '瑟芙既是夢疫核心也是森林的囚徒；喚醒全村必須犧牲記憶、摧毀夢根，或找到第三條路。',
      },
    ],
  },
  {
    id: 'blood-moon-express',
    title: '血月特快車',
    genre: '魔導列車驚魂',
    summary: '密室命案、失竊聖物與一列不再停靠現世車站的魔導列車。',
    tags: ['推理', '追逐', '高節奏'],
    chapter: '第一章／第十三節車廂',
    scene: '橫越裂谷的血月特快車',
    objective: '在列車穿過血月隧道前找回失竊的星火聖匣',
    objectiveContext: '王都使節在上鎖包廂內遇害，護送的星火聖匣不翼而飛。列車長宣稱全車無人上下車，但乘客名單多出一位不存在的人。',
    stakes: '列車進入血月隧道後將脫離現世；聖匣若在彼端開啟，車上所有乘客都會被獻給等待接站的古老存在。',
    opening: '列車越過裂谷時猛烈一震，第十三節車廂的燈全數轉紅。上鎖包廂內，使節倒在沒有血跡的地板上；他手中緊握一張車票，乘客姓名正緩慢變成其中一名冒險者。',
    demoBeats: [
      {
        text: '包廂門沒有破壞痕跡，但窗外黏著一枚向內彎曲的銀鈕扣。驗票員趕來封鎖現場，口袋卻露出同款制服線頭。此時餐車方向傳來爆炸聲，一名蒙面人抱著發光箱子躍上車頂。',
        choices: ['盤問驗票員', '追上列車車頂', '徹查密室機關'],
        objective: '確認聖匣去向並阻止蒙面人逃離',
        objectiveContext: '驗票員可能涉案，蒙面人則帶著疑似聖匣的發光箱子逃上車頂；兩條線索同時出現。',
        check: { ability: '敏捷', skill: '特技', dc: 13, reason: '列車高速穿越裂谷，跳上濕滑車頂可能直接墜落。' },
      },
      {
        text: '蒙面人被逼到車廂接縫時主動摘下面具——她正是三小時前已在前站下車的王室法師。她說箱子是誘餌，真正聖匣藏在某位乘客的影子裡；話音未落，全車乘客的影子同時站了起來。',
        choices: ['相信王室法師', '制伏她奪取箱子', '用光源辨認異常影子'],
        objective: '從乘客的影子中找出真正聖匣',
        objectiveContext: '發光箱子可能是誘餌，聖匣被影魔藏在乘客影子中；王室法師的身分與動機仍有疑點。',
      },
      {
        text: '火車頭的鍋爐門自行打開，裡面沒有火，只有一輪縮小的血月。列車長承認第十三節車廂本來就是祭壇，而死去使節才是阻止儀式的人。前方隧道已張開如獸口，煞車與聖匣只能先處理一個。',
        choices: ['啟動緊急煞車', '用聖匣封印血月', '奪取火車頭控制權'],
        objective: '阻止列車進入血月隧道或完成獻祭',
        objectiveContext: '列車本身是移動祭壇，列車長參與儀式；隧道入口已近，煞車與封印都需要立即行動。',
      },
    ],
  },
];

const defaultStory = storyPresets[0];

export const initialCampaign: Campaign = {
  setupComplete: false,
  storyId: defaultStory.id,
  title: defaultStory.title,
  chapter: defaultStory.chapter,
  scene: defaultStory.scene,
  round: 1,
  objective: defaultStory.objective,
  objectiveContext: defaultStory.objectiveContext,
  stakes: defaultStory.stakes,
  showStatHints: true,
  players: [
    createLevel3Character('player1', '賽勒恩・瓦爾', '遊俠'),
    createLevel3Character('player2', '米芮・鐵歌', '牧師'),
  ],
  story: [
    { id: 'opening-1', speaker: 'dm', time: '23:41', text: defaultStory.opening },
    { id: 'opening-2', speaker: 'system', time: '23:42', text: `目前目標：${defaultStory.objective}。所有玩家都提交行動後，DM 才會推進場景。` },
  ],
  pending: {},
};
