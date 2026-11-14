//go:build audit

package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPreviewAssetAudit(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("official sample not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)
	a := &app{
		proj: p, page: pageScenes, scene: sceneDash, sceneAnim: false,
		menuLang: "zh_CN", botState: "normal", menuPageKey: "brightness",
		popupType: "bt_msg", nitroMode: "auto", ridePage: "auto",
		showAllLamps: true, showCharge: true, showGPS: true,
		hintID: 44, adasMode: "sides", weatherID: 1, compassDeg: 45, altitude: 128,
		usedAssets: map[string]bool{},
	}
	a.meta = p.meta()
	tt := ui.NewTester(a.view, 1440, 920)
	render := func() {
		a.invalidatePreview()
		a.drawnVer = -1
		tt.Frame()
	}

	for mode := 0; mode < 4; mode++ {
		a.page = pageDashboard
		a.mode = mode
		render()
	}
	for s := 0; s < int(sceneKindCount); s++ {
		a.page = pageScenes
		a.scene = sceneKind(s)
		a.sceneAnim = false
		render()
	}
	a.page = pageScenes
	a.scene = sceneMenuPage
	for _, pg := range p.Scene.MenuPages {
		a.menuPageKey = pg.Key
		render()
	}
	a.scene = scenePopup
	for _, pop := range p.Scene.Popups {
		a.popupType = pop.Key
		render()
	}
	a.scene = sceneBot
	for _, st := range []string{"init", "normal", "menu", "msg"} {
		a.botState = st
		render()
	}
	a.scene = sceneDash
	for _, id := range []int{1, 2, 3, 4, 6, 7, 8, 9} {
		a.weatherID = id
		render()
	}
	for _, deg := range []int{0, 45, 90, 135, 180, 225, 270, 315} {
		a.compassDeg = deg
		render()
	}
	for _, lang := range []string{"zh_CN", "en_US"} {
		a.menuLang = lang
		a.page = pageScenes
		a.scene = sceneMenuPage
		for _, pg := range []string{"brightness", "BTConnect", "WIFIConnect", "music", "DeviceInfo", "dvr/dvr", "photo/photo_list", "tpms/tire_disp"} {
			a.menuPageKey = pg
			render()
		}
		a.scene = scenePopup
		for _, pop := range p.Scene.Popups {
			a.popupType = pop.Key
			render()
		}
	}
	a.menuLang = "zh_CN"

	// Animation sequences the preview can show over time.
	mark := func(dir string, n int) {
		for i := 0; i < n; i++ {
			a.bitmap(fmt.Sprintf("%s/%d.png", dir, i))
		}
	}
	mark("menu/style/charge/battery", 48)
	mark("menu/style/charge/line", 48)
	mark("menu/style/charge/icon", 48)
	a.bitmap("menu/style/charge/bg.png")
	a.bitmap("menu/style/charge/icon.png")
	mark("switch_theme/style/theme", 48)
	for _, st := range []string{"init", "normal", "menu", "msg"} {
		mark("bot/style/seq_pic/"+st, 50)
	}
	for i := 1; i <= 3; i++ {
		a.bitmap(fmt.Sprintf("menu/style/count_down/num/%d.png", i))
	}
	for i := 0; i <= 9; i++ {
		a.bitmap(fmt.Sprintf("tdSuspendMgr/style/lock/number/%d.png", i))
		a.bitmap(fmt.Sprintf("tdSuspendMgr/style/lock/select_num/%d.png", i))
	}
	for _, l := range p.Scene.Page0Lamps {
		if rel := p.Scene.LampPaths[l.ID]; rel != "" {
			a.bitmap(rel)
		}
	}
	for _, s := range p.Scene.Page0Signal {
		if rel := p.Scene.SignalPaths[s.ID]; rel != "" {
			a.bitmap(rel)
		}
	}
	for _, dir := range []string{
		"dashboard/style/nightime/weather/name_zh", "dashboard/style/nightime/weather/name_en",
	} {
		entries, _ := os.ReadDir(p.Abs(dir))
		for _, e := range entries {
			if e.IsDir() {
				sub, _ := os.ReadDir(filepath.Join(p.Abs(dir), e.Name()))
				for _, f := range sub {
					a.bitmap(dir + "/" + e.Name() + "/" + f.Name())
				}
			}
		}
	}
	for _, lang := range []string{"zh_CN", "en_US"} {
		for _, d := range []string{"north", "northeast", "east", "southeast", "south", "southwest", "west", "northwest"} {
			a.bitmap("dashboard/style/compass/text/" + lang + "/" + d + ".png")
		}
	}

	all := map[string]bool{}
	filepath.WalkDir(p.ThemeRoot, func(fp string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(fp), ".png") {
			rel, _ := filepath.Rel(p.ThemeRoot, fp)
			all[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	var unused []string
	for rel := range all {
		if !a.usedAssets[rel] {
			unused = append(unused, rel)
		}
	}
	sort.Strings(unused)
	counts := map[string]int{}
	for _, rel := range unused {
		counts[path.Dir(rel)]++
	}
	type kv struct {
		k string
		n int
	}
	var groups []kv
	for k, n := range counts {
		groups = append(groups, kv{k, n})
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].n != groups[j].n {
			return groups[i].n > groups[j].n
		}
		return groups[i].k < groups[j].k
	})
	var b strings.Builder
	fmt.Fprintf(&b, "# Theme Studio 预览未使用图片\n\n")
	fmt.Fprintf(&b, "- 主题图片总数：%d\n- 预览加载：%d\n- 预览未使用：%d\n\n", len(all), len(a.usedAssets), len(unused))
	fmt.Fprintf(&b, "## 按目录\n\n")
	for _, g := range groups {
		fmt.Fprintf(&b, "- %d：`%s`\n", g.n, g.k)
	}
	fmt.Fprintf(&b, "\n## 完整列表\n\n")
	for _, rel := range unused {
		fmt.Fprintf(&b, "- `%s`\n", rel)
	}
	outPath := filepath.Join(os.TempDir(), "theme_unused_preview.md")
	if err := os.WriteFile(outPath, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("all=%d used=%d unused=%d report=%s", len(all), len(a.usedAssets), len(unused), outPath)
}
