package game

import (
	"fmt"
	"strings"

	"dndduet/internal/apperr"
	"dndduet/internal/rules"
)

// ShopItem is one entry in the fixed equipment-merchant catalog (SRD-style
// prices in gp). Buying appends the item to the character's equipment list;
// mechanical benefits are narrated by the DM.
type ShopItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Kind  string `json:"kind"` // weapon | armor | gear | potion
	Price int    `json:"price"`
	Note  string `json:"note"`
}

// ShopCatalog is the equipment merchant's stock, available whenever the party
// is out of combat (the DM narrates who the merchant is).
var ShopCatalog = []ShopItem{
	{ID: "dagger", Name: "匕首", Kind: "weapon", Price: 2, Note: "輕型、靈巧，可投擲（1d4 穿刺）"},
	{ID: "shortsword", Name: "短劍", Kind: "weapon", Price: 10, Note: "輕型、靈巧（1d6 穿刺）"},
	{ID: "longsword", Name: "長劍", Kind: "weapon", Price: 15, Note: "多用途（1d8／1d10 揮砍）"},
	{ID: "warhammer", Name: "戰鎚", Kind: "weapon", Price: 15, Note: "多用途（1d8／1d10 鈍擊）"},
	{ID: "rapier", Name: "刺劍", Kind: "weapon", Price: 25, Note: "靈巧（1d8 穿刺）"},
	{ID: "greatsword", Name: "巨劍", Kind: "weapon", Price: 50, Note: "雙手（2d6 揮砍）"},
	{ID: "longbow", Name: "長弓", Kind: "weapon", Price: 50, Note: "遠程 150/600 呎（1d8 穿刺）"},
	{ID: "arrows", Name: "箭矢（20 支）", Kind: "gear", Price: 1, Note: "長弓與短弓通用"},
	{ID: "leather-armor", Name: "皮甲", Kind: "armor", Price: 10, Note: "AC 11＋敏捷調整值"},
	{ID: "chain-shirt", Name: "鎖子衫", Kind: "armor", Price: 50, Note: "AC 13＋敏捷調整值（上限 +2）"},
	{ID: "chain-mail", Name: "鎖子甲", Kind: "armor", Price: 75, Note: "AC 16，需力量 13"},
	{ID: "shield", Name: "盾牌", Kind: "armor", Price: 10, Note: "AC +2"},
	{ID: "healing-potion", Name: "治療藥水", Kind: "potion", Price: 50, Note: "飲用回復 2d4+2 HP（由 DM 結算）"},
	{ID: "antitoxin", Name: "解毒劑", Kind: "potion", Price: 50, Note: "1 小時內對毒素豁免具優勢"},
	{ID: "rope", Name: "麻繩（50 呎）", Kind: "gear", Price: 1, Note: "攀爬與捆綁"},
	{ID: "torch", Name: "火把（10 支）", Kind: "gear", Price: 1, Note: "照明 20 呎，燃燒 1 小時"},
	{ID: "rations", Name: "口糧（5 日）", Kind: "gear", Price: 2, Note: "野外行進補給"},
	{ID: "thieves-tools", Name: "盜賊工具", Kind: "gear", Price: 25, Note: "開鎖與解除陷阱檢定所需"},
}

func shopItem(itemID string) *ShopItem {
	for i := range ShopCatalog {
		if ShopCatalog[i].ID == itemID {
			return &ShopCatalog[i]
		}
	}
	return nil
}

// BuyItem purchases one catalog item for a character (out of combat only).
func (s *Service) BuyItem(id, playerID, itemID string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	player, err := st.player(playerID)
	if err != nil {
		return View{}, err
	}
	if st.combat != nil && st.combat.Active {
		return View{}, apperr.New(400, "戰鬥進行中無法購買裝備。")
	}
	item := shopItem(itemID)
	if item == nil {
		return View{}, apperr.New(404, "商店沒有這個品項。")
	}
	if player.Gold < item.Price {
		return View{}, apperr.New(400, fmt.Sprintf("%s的金幣不足：%s 需要 %d gp，目前只有 %d gp。", player.Name, item.Name, item.Price, player.Gold))
	}
	player.Gold -= item.Price
	player.Equipment = append(player.Equipment, item.Name)
	return s.persist(st, []string{fmt.Sprintf("%s向裝備商購買「%s」（%d gp），剩餘 %d gp。", player.Name, item.Name, item.Price, player.Gold)})
}

// forgeUpgradeCap bounds blacksmith enhancement per weapon / armor set.
const forgeUpgradeCap = 3

// ForgeWeaponCost is the gp price for the NEXT weapon upgrade level.
func ForgeWeaponCost(nextLevel int) int { return nextLevel * 100 }

// ForgeArmorCost is the gp price for the NEXT armor upgrade level.
func ForgeArmorCost(nextLevel int) int { return nextLevel * 150 }

// ForgeUpgrade is the blacksmith: kind "weapon" enhances one attack (+1 hit,
// +1 damage per level), kind "armor" enhances AC (+1 per level). Out of
// combat only; each item caps at +3.
func (s *Service) ForgeUpgrade(id, playerID, kind, attackID string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	player, err := st.player(playerID)
	if err != nil {
		return View{}, err
	}
	if st.combat != nil && st.combat.Active {
		return View{}, apperr.New(400, "戰鬥進行中無法鍛造裝備。")
	}
	switch kind {
	case "weapon":
		var attack *rules.Attack
		for i := range player.Attacks {
			if player.Attacks[i].ID == attackID {
				attack = &player.Attacks[i]
				break
			}
		}
		if attack == nil {
			return View{}, apperr.New(404, "找不到要鍛造的武器。")
		}
		if attack.UpgradeLevel >= forgeUpgradeCap {
			return View{}, apperr.New(400, attack.Name+"已達鍛造上限 +"+fmt.Sprint(forgeUpgradeCap)+"。")
		}
		cost := ForgeWeaponCost(attack.UpgradeLevel + 1)
		if player.Gold < cost {
			return View{}, apperr.New(400, fmt.Sprintf("鍛造%s需要 %d gp，%s目前只有 %d gp。", attack.Name, cost, player.Name, player.Gold))
		}
		player.Gold -= cost
		attack.UpgradeLevel++
		name := attack.Name
		level := attack.UpgradeLevel
		*player = rules.Recalculate(*player)
		return s.persist(st, []string{fmt.Sprintf("%s委託鍛造商強化「%s」至 +%d（%d gp）：命中與傷害各 +%d。", player.Name, name, level, cost, level)})
	case "armor":
		if player.ArmorUpgrade >= forgeUpgradeCap {
			return View{}, apperr.New(400, player.Name+"的護甲已達鍛造上限 +"+fmt.Sprint(forgeUpgradeCap)+"。")
		}
		cost := ForgeArmorCost(player.ArmorUpgrade + 1)
		if player.Gold < cost {
			return View{}, apperr.New(400, fmt.Sprintf("強化護甲需要 %d gp，%s目前只有 %d gp。", cost, player.Name, player.Gold))
		}
		player.Gold -= cost
		player.ArmorUpgrade++
		player.AC++
		return s.persist(st, []string{fmt.Sprintf("%s委託鍛造商強化護甲至 +%d（%d gp）：AC 現在 %d。", player.Name, player.ArmorUpgrade, cost, player.AC)})
	default:
		return View{}, apperr.New(400, "鍛造類型必須是 weapon 或 armor。")
	}
}

// consumableEffects maps carried item names the server can resolve in play.
var consumableEffects = map[string]string{
	"治療藥水": "2d4+2", // heal roll
	"解毒劑":  "",      // clears poison-style conditions
}

// UseItem consumes one carried consumable. 治療藥水 heals 2d4+2 (a bonus
// action in combat); 解毒劑 clears poison conditions. The item is removed
// from the equipment list.
func (s *Service) UseItem(id, playerID, itemName string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	player, err := st.player(playerID)
	if err != nil {
		return View{}, err
	}
	itemName = strings.TrimSpace(itemName)
	index := -1
	for i, owned := range player.Equipment {
		if owned == itemName {
			index = i
			break
		}
	}
	if index < 0 {
		return View{}, apperr.New(404, player.Name+"身上沒有「"+itemName+"」。")
	}
	healExpr, known := consumableEffects[itemName]
	if !known {
		return View{}, apperr.New(400, "「"+itemName+"」不是可直接使用的消耗品；把它交給敘事處理即可。")
	}
	combatActive := st.combat != nil && st.combat.Active
	if combatActive {
		// Drinking is a bonus action; dry-run before consuming the item.
		if _, err := rules.SpendCombatResource(*st.combat, playerID, "bonusAction"); err != nil {
			return View{}, apperr.New(400, err.Error())
		}
	}

	var logs []string
	if healExpr != "" {
		heal, err := rules.RollExpression(healExpr, s.dice, false)
		if err != nil {
			return View{}, err
		}
		healed := min(player.MaxHP-player.HP, heal)
		if healed < 0 {
			healed = 0
		}
		player.HP += healed
		logs = append(logs, fmt.Sprintf("%s飲用「%s」恢復 %d 生命，現在 HP %d/%d。", player.Name, itemName, healed, player.HP, player.MaxHP))
	} else {
		if strings.Contains(player.Condition, "中毒") {
			player.Condition = "正常"
		}
		logs = append(logs, fmt.Sprintf("%s使用「%s」，中毒類效果被中和。", player.Name, itemName))
	}
	player.Equipment = append(player.Equipment[:index], player.Equipment[index+1:]...)

	if combatActive {
		if spent, err := rules.SpendCombatResource(*st.combat, playerID, "bonusAction"); err == nil {
			*st.combat = spent
		}
		for i := range st.combat.Combatants {
			if st.combat.Combatants[i].PlayerID == playerID {
				st.combat.Combatants[i].HP = player.HP
				break
			}
		}
		logs[0] += "（已使用附贈動作）"
	}
	return s.persist(st, logs)
}

// SellItem sells one carried item back to the merchant. Catalog items refund
// half price; unknown items (story loot) sell for a flat 5 gp.
func (s *Service) SellItem(id, playerID, itemName string) (View, error) {
	unlock := s.Lock(id)
	defer unlock()
	st, err := s.loadState(id)
	if err != nil {
		return View{}, err
	}
	player, err := st.player(playerID)
	if err != nil {
		return View{}, err
	}
	if st.combat != nil && st.combat.Active {
		return View{}, apperr.New(400, "戰鬥進行中無法變賣裝備。")
	}
	itemName = strings.TrimSpace(itemName)
	index := -1
	for i, owned := range player.Equipment {
		if owned == itemName {
			index = i
			break
		}
	}
	if index < 0 {
		return View{}, apperr.New(404, player.Name+"身上沒有「"+itemName+"」。")
	}
	price := 5
	for _, item := range ShopCatalog {
		if item.Name == itemName {
			price = item.Price / 2
			if price < 1 {
				price = 1
			}
			break
		}
	}
	player.Equipment = append(player.Equipment[:index], player.Equipment[index+1:]...)
	player.Gold += price
	return s.persist(st, []string{fmt.Sprintf("%s將「%s」賣給裝備商（+%d gp），現有 %d gp。", player.Name, itemName, price, player.Gold)})
}
