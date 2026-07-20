package game

import (
	"fmt"
	"strings"

	"dndduet/internal/apperr"
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
