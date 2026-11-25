package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type RawSkin struct {
	WeaponDefIndex int    `json:"weapon_defindex"`
	WeaponName     string `json:"weapon_name"`
	Paint          any    `json:"paint"`
	Image          string `json:"image"`
	PaintName      string `json:"paint_name"`
}

type SkinOption struct {
	PaintID   int    `json:"paint_id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name"`
	Image     string `json:"image"`
}

type WeaponGroup struct {
	DefIndex     int          `json:"defindex"`
	WeaponName   string       `json:"weapon_name"`
	DisplayName  string       `json:"display_name"`
	Category     string       `json:"category"`
	DefaultImage string       `json:"default_image"`
	Skins        []SkinOption `json:"skins"`
}

type KnifeType struct {
	DefIndex    int    `json:"defindex"`
	WeaponName  string `json:"weapon_name"`
	DisplayName string `json:"display_name"`
	Image       string `json:"image"`
}

type GloveItem struct {
	DefIndex int    `json:"weapon_defindex"`
	PaintID  int    `json:"paint"`
	Name     string `json:"name"`
	Image    string `json:"image"`
}

type AgentItem struct {
	Team  int    `json:"team"`
	Model string `json:"model"`
	Name  string `json:"name"`
	Image string `json:"image"`
}

type SimpleItem struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
}

type Catalog struct {
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

func rewriteImage(imgURL, imgDir string) string {
	if imgURL == "" {
		return ""
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

func Load(dataDir, imgDir string) (*Catalog, error) {
	cat := &Catalog{}

	// 1. Skins + Weapons + Knives
	skinBytes, err := os.ReadFile(filepath.Join(dataDir, "skins_zh-CN.json"))
	if err != nil {
		return nil, err
	}
	var rawSkins []RawSkin
	if err := json.Unmarshal(skinBytes, &rawSkins); err != nil {
		return nil, err
	}
	wMap := map[int]*WeaponGroup{}
	for _, s := range rawSkins {
		paintID := toInt(s.Paint)
		img := rewriteImage(s.Image, imgDir)
		parts := strings.SplitN(s.PaintName, "|", 2)
		disp := strings.TrimSpace(parts[0])
		short := s.PaintName
		if len(parts) == 2 {
			short = strings.TrimSpace(parts[1])
		}
		if paintID == 0 {
			short = "默认原版"
		}
		wg, ok := wMap[s.WeaponDefIndex]
		if !ok {
			wg = &WeaponGroup{
				DefIndex:     s.WeaponDefIndex,
				WeaponName:   s.WeaponName,
				DisplayName:  disp,
				Category:     weaponCategory(s.WeaponDefIndex),
				DefaultImage: img,
			}
			wMap[s.WeaponDefIndex] = wg
		}
		if paintID == 0 && img != "" {
			wg.DefaultImage = img
		}
		wg.Skins = append(wg.Skins, SkinOption{
			PaintID:   paintID,
			Name:      s.PaintName,
			ShortName: short,
			Image:     img,
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
				DefIndex:    wg.DefIndex,
				WeaponName:  wg.WeaponName,
				DisplayName: wg.DisplayName,
				Image:       wg.DefaultImage,
			})
		}
	}
	sort.Slice(cat.Weapons, func(i, j int) bool {
		return weaponOrder(cat.Weapons[i].DefIndex) < weaponOrder(cat.Weapons[j].DefIndex)
	})
	sort.Slice(cat.Knives, func(i, j int) bool {
		return cat.Knives[i].DefIndex < cat.Knives[j].DefIndex
	})

	// 2. Gloves
	if b, err := os.ReadFile(filepath.Join(dataDir, "gloves_zh-CN.json")); err == nil {
		var raw []struct {
			DefIndex  int    `json:"weapon_defindex"`
			Paint     any    `json:"paint"`
			Image     string `json:"image"`
			PaintName string `json:"paint_name"`
		}
		if json.Unmarshal(b, &raw) == nil {
			for _, g := range raw {
				name := g.PaintName
				if g.DefIndex == 0 {
					name = "默认原版手套"
				}
				cat.Gloves = append(cat.Gloves, GloveItem{
					DefIndex: g.DefIndex,
					PaintID:  toInt(g.Paint),
					Name:     name,
					Image:    rewriteImage(g.Image, imgDir),
				})
			}
		}
	}

	// 3. Agents
	if b, err := os.ReadFile(filepath.Join(dataDir, "agents_zh-CN.json")); err == nil {
		var raw []struct {
			Team      int    `json:"team"`
			Image     string `json:"image"`
			Model     string `json:"model"`
			AgentName string `json:"agent_name"`
		}
		if json.Unmarshal(b, &raw) == nil {
			for _, a := range raw {
				model := a.Model
				name := a.AgentName
				if model == "null" || model == "" {
					model = ""
					if a.Team == 2 {
						name = "默认 T 阵营探员"
					} else {
						name = "默认 CT 阵营探员"
					}
				}
				cat.Agents = append(cat.Agents, AgentItem{
					Team:  a.Team,
					Model: model,
					Name:  name,
					Image: rewriteImage(a.Image, imgDir),
				})
			}
		}
	}

	// 4. Music
	if b, err := os.ReadFile(filepath.Join(dataDir, "music_zh-CN.json")); err == nil {
		var raw []struct {
			ID    any    `json:"id"`
			Name  string `json:"name"`
			Image string `json:"image"`
		}
		if json.Unmarshal(b, &raw) == nil {
			cat.Music = append(cat.Music, SimpleItem{ID: 0, Name: "默认音乐盒"})
			for _, m := range raw {
				cat.Music = append(cat.Music, SimpleItem{
					ID:    toInt(m.ID),
					Name:  m.Name,
					Image: rewriteImage(m.Image, imgDir),
				})
			}
		}
	}

	// 5. Pins (Collectibles)
	if b, err := os.ReadFile(filepath.Join(dataDir, "collectibles_zh-CN.json")); err == nil {
		var raw []struct {
			ID    any    `json:"id"`
			Name  string `json:"name"`
			Image string `json:"image"`
		}
		if json.Unmarshal(b, &raw) == nil {
			cat.Pins = append(cat.Pins, SimpleItem{ID: 0, Name: "不佩戴徽章"})
			for _, p := range raw {
				cat.Pins = append(cat.Pins, SimpleItem{
					ID:    toInt(p.ID),
					Name:  p.Name,
					Image: rewriteImage(p.Image, imgDir),
				})
			}
		}
	}

	return cat, nil
}
