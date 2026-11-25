package catalog

import (
	"embed"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed embedded/*.json
var embeddedCatalogFS embed.FS

type RawSkin struct {
	WeaponDefIndex int    `json:"weapon_defindex"`
	WeaponName     string `json:"weapon_name"`
	Paint          any    `json:"paint"`
	Image          string `json:"image"`
	PaintName      string `json:"paint_name"`
}

type SkinOption struct {
	PaintID     int    `json:"paint_id"`
	Name        string `json:"name"`
	NameEN      string `json:"name_en"`
	ShortName   string `json:"short_name"`
	ShortNameEN string `json:"short_name_en"`
	Image       string `json:"image"`
}

type WeaponGroup struct {
	DefIndex      int          `json:"defindex"`
	WeaponName    string       `json:"weapon_name"`
	DisplayName   string       `json:"display_name"`
	DisplayNameEN string       `json:"display_name_en"`
	Category      string       `json:"category"`
	DefaultImage  string       `json:"default_image"`
	Skins         []SkinOption `json:"skins"`
}

type KnifeType struct {
	DefIndex      int    `json:"defindex"`
	WeaponName    string `json:"weapon_name"`
	DisplayName   string `json:"display_name"`
	DisplayNameEN string `json:"display_name_en"`
	Image         string `json:"image"`
}

type GloveItem struct {
	DefIndex int    `json:"weapon_defindex"`
	PaintID  int    `json:"paint"`
	Name     string `json:"name"`
	NameEN   string `json:"name_en"`
	Image    string `json:"image"`
}

type AgentItem struct {
	Team   int    `json:"team"`
	Model  string `json:"model"`
	Name   string `json:"name"`
	NameEN string `json:"name_en"`
	Image  string `json:"image"`
}

type SimpleItem struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	NameEN string `json:"name_en"`
	Image  string `json:"image"`
}

type Catalog struct {
	Source  string        `json:"source"`
	Weapons []WeaponGroup `json:"weapons"`
	Knives  []KnifeType   `json:"knives"`
	Gloves  []GloveItem   `json:"gloves"`
	Agents  []AgentItem   `json:"agents"`
	Music   []SimpleItem  `json:"music"`
	Pins    []SimpleItem  `json:"pins"`
}

func toInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	default:
		return 0
	}
}

func readCatalogFile(dataDir, filename string) ([]byte, bool, error) {
	if dataDir != "" {
		if b, err := os.ReadFile(filepath.Join(dataDir, filename)); err == nil && len(b) > 0 {
			return b, false, nil
		}
	}
	b, err := embeddedCatalogFS.ReadFile("embedded/" + filename)
	return b, true, err
}

func rewriteImage(imgURL, imgDir string) string {
	if imgURL == "" || imgDir == "" {
		return imgURL
	}
	const marker = "/website/img/"
	if idx := strings.Index(imgURL, marker); idx >= 0 {
		rel := imgURL[idx+len(marker):]
		if _, err := os.Stat(filepath.Join(imgDir, filepath.FromSlash(rel))); err == nil {
			return "/img/" + rel
		}
	}
	return imgURL
}

func weaponCategory(def int) string {
	if def >= 500 {
		return "knives"
	}
	switch def {
	case 7, 16, 60, 8, 39, 10, 13:
		return "rifles"
	case 9, 40, 11, 38:
		return "snipers"
	case 1, 61, 4, 32, 36, 3, 30, 63, 2, 64, 31:
		return "pistols"
	case 34, 17, 33, 23, 24, 19, 26:
		return "smgs"
	default:
		return "heavy"
	}
}

func weaponOrder(def int) int {
	order := map[int]int{
		7: 1, 16: 2, 60: 3, 9: 4, 1: 5, 61: 6, 4: 7, 40: 8,
		10: 9, 13: 10, 8: 11, 39: 12, 34: 13, 17: 14, 24: 15, 33: 16,
		23: 17, 19: 18, 26: 19, 36: 20, 3: 21, 30: 22, 63: 23, 32: 24,
		2: 25, 64: 26, 31: 27, 25: 28, 27: 29, 35: 30, 29: 31, 14: 32,
		28: 33, 11: 34, 38: 35,
	}
	if v, ok := order[def]; ok {
		return v
	}
	return 100 + def
}

func splitName(full string, isDefault bool, defaultLabel string) (string, string) {
	parts := strings.SplitN(full, "|", 2)
	disp := strings.TrimSpace(parts[0])
	short := full
	if len(parts) == 2 {
		short = strings.TrimSpace(parts[1])
	}
	if isDefault {
		short = defaultLabel
	}
	return disp, short
}

func Load(dataDir, imgDir string) (*Catalog, error) {
	cat := &Catalog{Source: "disk"}

	// 1. Skins + Weapons + Knives (zh-CN + en)
	skinZHBytes, fromEmbed, err := readCatalogFile(dataDir, "skins_zh-CN.json")
	if err != nil {
		return nil, err
	}
	if fromEmbed {
		cat.Source = "embedded"
	}
	var rawSkinsZH, rawSkinsEN []RawSkin
	if err := json.Unmarshal(skinZHBytes, &rawSkinsZH); err != nil {
		return nil, err
	}
	enSkinMap := map[string]RawSkin{}
	enWeaponDisp := map[int]string{}
	if bEN, _, err := readCatalogFile(dataDir, "skins_en.json"); err == nil && json.Unmarshal(bEN, &rawSkinsEN) == nil {
		for _, s := range rawSkinsEN {
			pid := toInt(s.Paint)
			k := strconv.Itoa(s.WeaponDefIndex) + ":" + strconv.Itoa(pid)
			enSkinMap[k] = s
			if _, ok := enWeaponDisp[s.WeaponDefIndex]; !ok {
				d, _ := splitName(s.PaintName, pid == 0, "Default")
				enWeaponDisp[s.WeaponDefIndex] = d
			}
		}
	}

	wMap := map[int]*WeaponGroup{}
	for _, s := range rawSkinsZH {
		paintID := toInt(s.Paint)
		img := rewriteImage(s.Image, imgDir)
		dispZH, shortZH := splitName(s.PaintName, paintID == 0, "默认原版")

		enKey := strconv.Itoa(s.WeaponDefIndex) + ":" + strconv.Itoa(paintID)
		nameEN := s.PaintName
		shortEN := shortZH
		if es, ok := enSkinMap[enKey]; ok {
			nameEN = es.PaintName
			_, shortEN = splitName(es.PaintName, paintID == 0, "Default")
		}
		dispEN := enWeaponDisp[s.WeaponDefIndex]
		if dispEN == "" {
			dispEN = dispZH
		}

		wg, ok := wMap[s.WeaponDefIndex]
		if !ok {
			wg = &WeaponGroup{
				DefIndex:      s.WeaponDefIndex,
				WeaponName:    s.WeaponName,
				DisplayName:   dispZH,
				DisplayNameEN: dispEN,
				Category:      weaponCategory(s.WeaponDefIndex),
				DefaultImage:  img,
			}
			wMap[s.WeaponDefIndex] = wg
		}
		if paintID == 0 && img != "" {
			wg.DefaultImage = img
		}
		wg.Skins = append(wg.Skins, SkinOption{
			PaintID:     paintID,
			Name:        s.PaintName,
			NameEN:      nameEN,
			ShortName:   shortZH,
			ShortNameEN: shortEN,
			Image:       img,
		})
	}
	for _, wg := range wMap {
		sort.Slice(wg.Skins, func(i, j int) bool {
			if wg.Skins[i].PaintID == 0 {
				return true
			}
			if wg.Skins[j].PaintID == 0 {
				return false
			}
			return wg.Skins[i].PaintID < wg.Skins[j].PaintID
		})
		cat.Weapons = append(cat.Weapons, *wg)
		if wg.DefIndex >= 500 {
			cat.Knives = append(cat.Knives, KnifeType{
				DefIndex:      wg.DefIndex,
				WeaponName:    wg.WeaponName,
				DisplayName:   wg.DisplayName,
				DisplayNameEN: wg.DisplayNameEN,
				Image:         wg.DefaultImage,
			})
		}
	}
	sort.Slice(cat.Weapons, func(i, j int) bool {
		return weaponOrder(cat.Weapons[i].DefIndex) < weaponOrder(cat.Weapons[j].DefIndex)
	})
	sort.Slice(cat.Knives, func(i, j int) bool {
		return cat.Knives[i].DefIndex < cat.Knives[j].DefIndex
	})

	// 2. Gloves (zh-CN + en)
	type rawGlove struct {
		DefIndex  int    `json:"weapon_defindex"`
		Paint     any    `json:"paint"`
		Image     string `json:"image"`
		PaintName string `json:"paint_name"`
	}
	enGloveMap := map[string]string{}
	if b, _, err := readCatalogFile(dataDir, "gloves_en.json"); err == nil {
		var raw []rawGlove
		if json.Unmarshal(b, &raw) == nil {
			for _, g := range raw {
				enGloveMap[strconv.Itoa(g.DefIndex)+":"+strconv.Itoa(toInt(g.Paint))] = g.PaintName
			}
		}
	}
	if b, _, err := readCatalogFile(dataDir, "gloves_zh-CN.json"); err == nil {
		var raw []rawGlove
		if json.Unmarshal(b, &raw) == nil {
			for _, g := range raw {
				pid := toInt(g.Paint)
				nameZH := g.PaintName
				nameEN := enGloveMap[strconv.Itoa(g.DefIndex)+":"+strconv.Itoa(pid)]
				if g.DefIndex == 0 {
					nameZH = "默认原版手套"
					nameEN = "Default Gloves"
				}
				if nameEN == "" {
					nameEN = nameZH
				}
				cat.Gloves = append(cat.Gloves, GloveItem{
					DefIndex: g.DefIndex,
					PaintID:  pid,
					Name:     nameZH,
					NameEN:   nameEN,
					Image:    rewriteImage(g.Image, imgDir),
				})
			}
		}
	}

	// 3. Agents (zh-CN + en)
	type rawAgent struct {
		Team      int    `json:"team"`
		Image     string `json:"image"`
		Model     string `json:"model"`
		AgentName string `json:"agent_name"`
	}
	enAgentMap := map[string]string{}
	if b, _, err := readCatalogFile(dataDir, "agents_en.json"); err == nil {
		var raw []rawAgent
		if json.Unmarshal(b, &raw) == nil {
			for _, a := range raw {
				enAgentMap[strconv.Itoa(a.Team)+":"+a.Model] = a.AgentName
			}
		}
	}
	if b, _, err := readCatalogFile(dataDir, "agents_zh-CN.json"); err == nil {
		var raw []rawAgent
		if json.Unmarshal(b, &raw) == nil {
			for _, a := range raw {
				model := a.Model
				nameZH := a.AgentName
				nameEN := enAgentMap[strconv.Itoa(a.Team)+":"+a.Model]
				if model == "null" || model == "" {
					model = ""
					if a.Team == 2 {
						nameZH = "默认 T 阵营探员"
						nameEN = "Default T Agent"
					} else {
						nameZH = "默认 CT 阵营探员"
						nameEN = "Default CT Agent"
					}
				}
				if nameEN == "" {
					nameEN = nameZH
				}
				cat.Agents = append(cat.Agents, AgentItem{
					Team:   a.Team,
					Model:  model,
					Name:   nameZH,
					NameEN: nameEN,
					Image:  rewriteImage(a.Image, imgDir),
				})
			}
		}
	}

	// 4. Music (zh-CN + en)
	type rawSimple struct {
		ID    any    `json:"id"`
		Name  string `json:"name"`
		Image string `json:"image"`
	}
	enMusicMap := map[int]string{}
	if b, _, err := readCatalogFile(dataDir, "music_en.json"); err == nil {
		var raw []rawSimple
		if json.Unmarshal(b, &raw) == nil {
			for _, m := range raw {
				enMusicMap[toInt(m.ID)] = m.Name
			}
		}
	}
	if b, _, err := readCatalogFile(dataDir, "music_zh-CN.json"); err == nil {
		var raw []rawSimple
		if json.Unmarshal(b, &raw) == nil {
			cat.Music = append(cat.Music, SimpleItem{ID: 0, Name: "默认音乐盒", NameEN: "Default Music Kit"})
			for _, m := range raw {
				id := toInt(m.ID)
				en := enMusicMap[id]
				if en == "" {
					en = m.Name
				}
				cat.Music = append(cat.Music, SimpleItem{ID: id, Name: m.Name, NameEN: en, Image: rewriteImage(m.Image, imgDir)})
			}
		}
	}

	// 5. Pins / Collectibles (zh-CN + en)
	enPinMap := map[int]string{}
	if b, _, err := readCatalogFile(dataDir, "collectibles_en.json"); err == nil {
		var raw []rawSimple
		if json.Unmarshal(b, &raw) == nil {
			for _, p := range raw {
				enPinMap[toInt(p.ID)] = p.Name
			}
		}
	}
	if b, _, err := readCatalogFile(dataDir, "collectibles_zh-CN.json"); err == nil {
		var raw []rawSimple
		if json.Unmarshal(b, &raw) == nil {
			cat.Pins = append(cat.Pins, SimpleItem{ID: 0, Name: "不佩戴徽章", NameEN: "No Pin"})
			for _, p := range raw {
				id := toInt(p.ID)
				en := enPinMap[id]
				if en == "" {
					en = p.Name
				}
				cat.Pins = append(cat.Pins, SimpleItem{ID: id, Name: p.Name, NameEN: en, Image: rewriteImage(p.Image, imgDir)})
			}
		}
	}

	return cat, nil
}
