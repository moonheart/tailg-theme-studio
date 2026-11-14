package main

import (
	"archive/zip"
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

var sampleTheme = func() string {
	if p := os.Getenv("THEME_STUDIO_SAMPLE"); p != "" {
		return p
	}
	return "testdata/sample_theme.zip"
}()

func TestConfigRoundTrip(t *testing.T) {
	in := []byte("# a comment\nThemeName=\"台铃K075\"\nPackSize=28203230\n[section]\nThemeType=\"S19\"\n")
	c := parseThemeConfig(in)
	if c.Get("ThemeName") != "台铃K075" || c.Get("ThemeType") != "S19" {
		t.Fatalf("parsed %q / %q", c.Get("ThemeName"), c.Get("ThemeType"))
	}
	c.Set("ThemeVer", "1.2.3")
	c.Set("PackSize", "42")
	c.Set("CustomAuthor", `a "quoted" name`)
	out := string(c.Bytes())
	for _, want := range []string{"# a comment", "[section]", `ThemeVer="1.2.3"`, "PackSize=42", `CustomAuthor="a "quoted" name"`} {
		if !strings.Contains(out, want) {
			t.Errorf("%q missing from\n%s", want, out)
		}
	}
}

func TestWidgetParseAndMove(t *testing.T) {
	text := `main_win={
  type="window"
  x=0;y=0;w=1024;h=600
  speedmeter={
    type="num_button"
    x=350;y=142;w=324;h=143
  }
}`
	widgets := parseWidgets(text)
	w := widgets["speedmeter"]
	if w == nil || w.X != 350 || w.Y != 142 || w.W != 324 || w.H != 143 {
		t.Fatalf("got %+v", w)
	}
	moved, ok := moveWidgetInText(text, "speedmeter", 100, 200)
	if !ok {
		t.Fatal("move failed")
	}
	if !strings.Contains(moved, "x=100;y=200;w=324;h=143") {
		t.Fatalf("moved text:\n%s", moved)
	}
	if widgets := parseWidgets(moved); widgets["speedmeter"].X != 100 {
		t.Fatalf("reparse got %+v", widgets["speedmeter"])
	}
}

func TestBumpPatch(t *testing.T) {
	cases := map[string]string{"0.0.9": "0.0.10", "1.2": "1.3", "": "0.0.1", "x": "x.1"}
	for in, want := range cases {
		if got := bumpPatch(in); got != want {
			t.Errorf("bumpPatch(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExportTheme(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("official sample not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)
	if p.Config.Get("ThemeType") != "S19" || p.Config.Get("StartVersion") != "1.1.4" {
		t.Fatalf("config = %+v", p.Config)
	}
	if len(p.Assets) < 1000 {
		t.Fatalf("only %d assets", len(p.Assets))
	}

	src := filepath.Join(t.TempDir(), "replacement.png")
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 30), G: 40, B: 200, A: 255})
		}
	}
	f, _ := os.Create(src)
	png.Encode(f, img)
	f.Close()
	if err := p.ReplaceAsset("dashboard/style/speedmeter/0.png", src, false); err != nil {
		t.Fatal(err)
	}
	if !p.Dirty["dashboard/style/speedmeter/0.png"] {
		t.Fatal("asset not marked dirty")
	}
	if !p.MoveWidget("speedmeter", 360, 150) {
		t.Fatal("move failed")
	}

	meta := p.meta()
	meta.DisplayName = "测试主题"
	out := filepath.Join(t.TempDir(), "out.zip")
	res, err := p.Export(meta, ExportOptions{Path: out, BumpVersion: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != "0.0.10" {
		t.Errorf("version %q", res.Version)
	}
	if res.Size == 0 || len(res.MD5) != 32 || res.FileCount < 1000 {
		t.Errorf("result %+v", res)
	}

	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
	}
	zr.Close()
	if !names["config.ini"] || !names["package.zip"] {
		t.Fatalf("outer names %v", names)
	}

	back, err := openProject(out)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(back.TempDir)
	if back.Config.Get("Md5") != res.MD5 {
		t.Errorf("md5 config %q != %q", back.Config.Get("Md5"), res.MD5)
	}
	if back.Config.Get("CustomName") != "测试主题" {
		t.Errorf("CustomName = %q", back.Config.Get("CustomName"))
	}
	if back.Config.Get("ThemeVer") != "0.0.10" {
		t.Errorf("ThemeVer = %q", back.Config.Get("ThemeVer"))
	}
	if got := back.Widget("speedmeter"); got.X != 360 || got.Y != 150 {
		t.Errorf("layout not saved: %+v", got)
	}
	orig, _ := os.ReadFile(p.Abs("dashboard/style/speedmeter/1.png"))
	replaced, _ := os.ReadFile(back.Abs("dashboard/style/speedmeter/0.png"))
	if bytes.Equal(orig, replaced) {
		t.Error("replacement did not change the file")
	}
	// Symlinked agent files must be real files in the exported package.
	p1, err := os.ReadFile(back.Abs("dashboard/style/gearmeter/gear_p/P_1.png"))
	if err != nil || len(p1) < 8 || string(p1[:8]) != string([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		t.Errorf("exported P_1.png is not a PNG: %v %q", err, p1)
	}
	if st, err := os.Stat(back.Abs("lib/libspecial.so")); err != nil || st.Size() < 1000 {
		t.Errorf("exported libspecial.so is not the copied library: %v", err)
	}
}

func TestView(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("official sample not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)
	a := &app{proj: p, page: pageDashboard, speed: 68, soc: 78, gear: "3", clock: "0941", showGrid: true, sceneAnim: false}
	a.meta = p.meta()
	a.metaSnapshot = a.meta
	tt := ui.NewTester(a.view, 1440, 920)
	if !tt.HasText("实时预览") {
		t.Fatalf("texts: %v", tt.Texts())
	}
	if !tt.HasText("可见元素") {
		t.Errorf("inspector not built")
	}
	if err := tt.Click("灯光"); err != nil {
		t.Errorf("click 灯光 tab: %v", err)
	} else if !tt.HasText("自检 / 告警灯") || !tt.HasText("在预览中显示全部") {
		t.Errorf("lamp gallery missing")
	}
	// Switch to the asset browser through the sidebar row.
	if err := tt.Click("图片资源"); err != nil {
		t.Logf("click nav: %v", err)
	} else if a.page != pageAssets {
		t.Errorf("page = %v, want assets", a.page)
	}
	if err := tt.Click("界面预览"); err != nil {
		t.Logf("click scenes: %v", err)
	} else if a.page != pageScenes {
		t.Errorf("page = %v, want scenes", a.page)
	} else if !tt.HasText("动画") || !tt.HasText("仪表模式") {
		t.Errorf("scene inspector missing: %v", tt.Texts())
	}
}

func TestFormatBytes(t *testing.T) {
	if got := formatBytes(28203230); got != "26.9 MB" {
		t.Errorf("formatBytes = %q", got)
	}
}

func TestNaturalOrder(t *testing.T) {
	if !naturalLess("2.png", "10.png") || naturalLess("10.png", "2.png") {
		t.Fatal("2.png should sort before 10.png")
	}
	if !naturalLess("P_2.png", "P_10.png") {
		t.Fatal("P_2 should sort before P_10")
	}
}

func TestSceneData(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("official sample not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)
	d := p.Scene
	if d == nil {
		t.Fatal("no scene data")
	}
	if len(d.MenuItems) != 10 || len(d.Page0Lamps) != 18 || len(d.Page0Signal) != 4 {
		t.Errorf("menu %d lamps %d signals %d", len(d.MenuItems), len(d.Page0Lamps), len(d.Page0Signal))
	}
	for _, sig := range d.Page0Signal {
		if d.SignalPaths[sig.ID] == "" || !p.Has(d.SignalPaths[sig.ID]) {
			t.Errorf("signal asset missing for %d: %q", sig.ID, d.SignalPaths[sig.ID])
		}
	}
	if d.LampDir != "dashboard/style/lamps/nightime" || d.SignalDir != "dashboard/style/signals/nightime" ||
		d.GPSDir != "dashboard/style/lamps/GPS" {
		t.Errorf("light dirs lamp=%q signal=%q gps=%q", d.LampDir, d.SignalDir, d.GPSDir)
	}
	if !strings.HasSuffix(d.SignalPaths[1], "1(1).png") || !strings.HasSuffix(d.LampPaths[1], "1.png") {
		t.Errorf("light variants signal=%q lamp=%q", d.SignalPaths[1], d.LampPaths[1])
	}
	if !d.HasLock || len(d.LockSlots) != 5 || len(d.CarSet) != 13 {
		t.Errorf("lock %v slots %d carset %d", d.HasLock, len(d.LockSlots), len(d.CarSet))
	}
	if got := d.MenuItems[0].Zh; got == "" || !strings.HasSuffix(got, "DeviceInfo_zh.png") {
		t.Errorf("first menu image %q", got)
	}
	if d.Bot == nil {
		t.Fatal("no bot rules")
	}
	if r := d.Bot.Normal[0]; r.X != 202 || r.Y != 510 || r.Path != "bot/style/seq_pic/normal" {
		t.Errorf("bot page0 = %+v", r)
	}
	if r := d.Bot.Normal[3]; r.X != 892 || r.Y != 448 {
		t.Errorf("bot page3 = %+v", r)
	}
	if r := d.Bot.Msg[0]; r.Path != "bot/style/seq_pic/msg" {
		t.Errorf("bot msg = %+v", r)
	}
	if len(d.MenuPages) != 49 {
		t.Errorf("menu pages = %d, want 49", len(d.MenuPages))
	}
	if len(d.Popups) != 7 || len(d.StyleBg) < 20 {
		t.Errorf("popups=%d styles=%d", len(d.Popups), len(d.StyleBg))
	}
	if d.NitroIcon.W != 41 || d.NitroIcon.X != 748 || d.NitroSeq.W != 120 || d.NitroFrames != 30 {
		t.Errorf("nitro icon=%+v seq=%+v frames=%d", d.NitroIcon, d.NitroSeq, d.NitroFrames)
	}
	if d.NitroLinkIcon.X != 846 || d.NitroLinkSeq.X != 806 {
		t.Errorf("link nitro icon=%+v seq=%+v", d.NitroLinkIcon, d.NitroLinkSeq)
	}
	if d.EcuIcon.X != 361 || d.EcuTemp.X != 408 || d.MotorIcon.X != 527 || d.MotorTemp.X != 574 {
		t.Errorf("temps ecu=%+v/%+v motor=%+v/%+v", d.EcuIcon, d.EcuTemp, d.MotorIcon, d.MotorTemp)
	}
	if got := styleBG(p, "button", "btn_ecu_icon"); got != "dashboard/style/regular/Temp/ecu.png" {
		t.Errorf("ecu icon style = %q", got)
	}
	if d.WeatherBG.X != 806 || d.WeatherIcon.X != 926 || d.CompassIcon.X != 807 ||
		d.AltitudeValue.X != 910 || d.PowerNum.X != 90 || d.PowerTitle.X != 17 || d.RideData.W != 225 {
		t.Errorf("hud rects weather=%+v icon=%+v compass=%+v alt=%+v power=%+v/%+v ride=%+v",
			d.WeatherBG, d.WeatherIcon, d.CompassIcon, d.AltitudeValue, d.PowerTitle, d.PowerNum, d.RideData)
	}
	for _, pg := range d.MenuPages {
		if pg.Key == "brightness" && len(pg.List) != 10 {
			t.Errorf("brightness list = %v", pg.List)
		}
		if pg.Key == "dvr/set" && pg.Window.W < 600 {
			t.Errorf("dvr/set window = %+v", pg.Window)
		}
	}
}

func TestAssetCategories(t *testing.T) {
	cases := map[string][2]string{
		"dashboard/style/nightime/bg/bg_1.png":    {"仪表", "背景"},
		"dashboard/style/speedmeter/0.png":        {"仪表", "速度与档位"},
		"dashboard/style/soc/soc_total/num/1.png": {"仪表", "SOC 与电池"},
		"dashboard/style/ridedata/topic/ODO.png":  {"仪表", "里程数据"},
		"dashboard/style/lamps/nightime/10.png":   {"仪表", "灯光与告警"},
		"menu/style/navi/1.png":                   {"菜单", "导航"},
		"menu/style/charge/bg.png":                {"菜单", "充电"},
		"menu/style/home/Music_zh.png":            {"菜单", "主菜单卡片"},
		"tdSuspendMgr/style/lock/number/1.png":    {"锁屏", "锁屏数字"},
		"switch_theme/style/theme/0.png":          {"主题换肤", "全部"},
		"bot/style/seq_pic/normal/1.png":          {"其他", "bot 动画"},
	}
	for rel, want := range cases {
		top, sub := assetCategoryOf(rel)
		if top != want[0] || sub != want[1] {
			t.Errorf("%s = %s / %s, want %s / %s", rel, top, sub, want[0], want[1])
		}
	}
	if _, err := os.Stat(sampleTheme); err != nil {
		return
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)
	counts := map[string]int{}
	for _, a := range p.Assets {
		top, sub := assetCategoryOf(a.Rel)
		if top == "" || sub == "" {
			t.Errorf("%s has empty category", a.Rel)
		}
		counts[top]++
	}
	total := 0
	for _, n := range counts {
		total += n
	}
	if total != len(p.Assets) {
		t.Errorf("category sum %d != assets %d", total, len(p.Assets))
	}
}

func TestSymlinkEntries(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("official sample not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)
	for i := 1; i <= 3; i++ {
		rel := "dashboard/style/gearmeter/gear_p/P_" + string(rune('0'+i)) + ".png"
		data, err := os.ReadFile(p.Abs(rel))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
			t.Errorf("%s is not a PNG after symlink resolution: %q", rel, data)
		}
		if w, h := imageSize(p.Abs(rel)); w != 111 || h != 143 {
			t.Errorf("%s size %dx%d", rel, w, h)
		}
	}
	link := p.Abs("lib/libspecial.so")
	target := p.Abs("lib/libspecial.so.1.0.1")
	ls, err := os.Stat(link)
	if err != nil {
		t.Fatal(err)
	}
	ts, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if ls.Size() != ts.Size() || ls.Size() < 1000 {
		t.Errorf("libspecial.so size %d, target %d", ls.Size(), ts.Size())
	}
}

func TestAllMenuPagesRender(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("official sample not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)
	a := &app{proj: p, page: pageScenes, scene: sceneMenuPage, sceneAnim: false, menuLang: "zh_CN", botState: "normal"}
	a.meta = p.meta()
	tt := ui.NewTester(a.view, 1440, 920)
	for _, pg := range p.Scene.MenuPages {
		a.menuPageKey = pg.Key
		tt.Frame()
	}
	a.scene = scenePopup
	for _, pop := range p.Scene.Popups {
		a.popupType = pop.Key
		tt.Frame()
	}
}

func TestNitroState(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("official sample not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)
	a := &app{proj: p, page: pageScenes, scene: sceneDash, sceneAnim: true, nitroMode: "auto"}
	a.meta = p.meta()
	a.scenePhase = 1
	if got := a.nitroState(); got != "ready" {
		t.Errorf("phase 1 = %q", got)
	}
	a.scenePhase = 3.5
	if got := a.nitroState(); got != "boost" {
		t.Errorf("phase 3.5 = %q", got)
	}
	a.scenePhase = 5
	if got := a.nitroState(); got != "idle" {
		t.Errorf("phase 5 = %q", got)
	}
	a.nitroMode = "boost"
	a.scenePhase = 0
	tt := ui.NewTester(a.view, 1440, 920)
	if !tt.HasText("氮气加速状态") || !tt.HasText("当前：加速中 · 红色火焰 + 30 帧火焰环") {
		t.Errorf("nitro inspector missing: %v", tt.Texts())
	}
	if !tt.HasText("仪表温度") || !tt.HasText("ECU °C") || !tt.HasText("电机 °C") {
		t.Errorf("temp inspector missing: %v", tt.Texts())
	}
	if !tt.HasText("仪表附加") || !tt.HasText("天气") || !tt.HasText("骑行数据") || !tt.HasText("ADAS 警告") {
		t.Errorf("hud inspector missing: %v", tt.Texts())
	}
}

func TestDashboardOverlays(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("official sample not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)
	a := &app{proj: p, page: pageScenes, scene: sceneDash, sceneAnim: false,
		nitroMode: "boost", ridePage: "time", showCharge: true, showGPS: true,
		hintID: 44, adasMode: "sides", weatherID: 3, compassDeg: 225, altitude: 1280,
		showAllLamps: true}
	a.meta = p.meta()
	tt := ui.NewTester(a.view, 1440, 920)
	tt.Frame()
	a.scene = sceneLink
	tt.Frame()
}

func TestMultilineWidgetMoveAndComments(t *testing.T) {
	src := `
btn_test = {
	# comment with { and } braces
	type = "button"
	caption = "hello \"world\" }"
	x = 12
	y = 34
	w = 100
	h = 40
}
`
	ws := parseWidgets(src)
	w := ws["btn_test"]
	if w == nil || w.X != 12 || w.Y != 34 || w.W != 100 || w.H != 40 {
		t.Fatalf("parsed widget = %+v", w)
	}
	updated, ok := moveWidgetInText(src, "btn_test", 55, 77)
	if !ok {
		t.Fatal("moveWidgetInText failed on multiline block")
	}
	ws2 := parseWidgets(updated)
	w2 := ws2["btn_test"]
	if w2 == nil || w2.X != 55 || w2.Y != 77 || w2.W != 100 || w2.H != 40 {
		t.Fatalf("moved widget = %+v", w2)
	}
}

func TestResolveMenuAssetColors(t *testing.T) {
	for _, val := range []string{"", "none", "ParentRelative", "rgb(1,2,3)", "rgba(1,2,3,0.5)", "#112233"} {
		if got := resolveMenuAsset("menu/style/Night", val); got != "" {
			t.Errorf("resolveMenuAsset(%q) = %q, want empty", val, got)
		}
	}
}

func TestLiveHudMoveAndLinkWidgets(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("official sample not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)

	if len(p.Scene.LinkWidgets) == 0 {
		t.Fatal("expected LinkWidgets to be populated from layout_link.ini")
	}
	for _, name := range []string{"link_low_soc_icon", "link_speedstop", "link_gearP"} {
		if p.Scene.LinkWidgets[name] == nil {
			t.Errorf("missing %s in LinkWidgets", name)
		}
	}

	a := &app{proj: p, page: pageDashboard, gear: "3", speed: 60, soc: 80}
	if !p.MoveWidget("btn_ecu_temp", 412, 570) {
		t.Fatal("MoveWidget(btn_ecu_temp) returned false")
	}
	got := a.liveWidgetRect("btn_ecu_temp", p.Scene.EcuTemp)
	if got.X != 412 || got.Y != 570 {
		t.Errorf("liveWidgetRect(btn_ecu_temp) = (%v,%v), want (412,570)", got.X, got.Y)
	}
}

func TestLayoutUndoRedoAndDirtyFilter(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("official sample not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)

	w := p.Widget("btn_ecu_temp")
	if w == nil {
		t.Fatal("missing btn_ecu_temp")
	}
	origX, origY := w.X, w.Y
	a := &app{proj: p, page: pageDashboard, sel: "btn_ecu_temp"}
	a.nudgeSelection(5, -3)
	w = p.Widget("btn_ecu_temp")
	if w.X != origX+5 || w.Y != origY-3 {
		t.Fatalf("after nudge = (%d,%d), want (%d,%d)", w.X, w.Y, origX+5, origY-3)
	}
	a.undoLayout()
	w = p.Widget("btn_ecu_temp")
	if w.X != origX || w.Y != origY {
		t.Fatalf("after undo = (%d,%d), want (%d,%d)", w.X, w.Y, origX, origY)
	}
	a.redoLayout()
	w = p.Widget("btn_ecu_temp")
	if w.X != origX+5 || w.Y != origY-3 {
		t.Fatalf("after redo = (%d,%d), want (%d,%d)", w.X, w.Y, origX+5, origY-3)
	}

	if len(p.Assets) == 0 {
		t.Fatal("expected assets")
	}
	a.assetTop = "已修改"
	if got := a.countCategory("已修改", "已修改"); got != 0 {
		t.Fatalf("dirty count before edit = %d, want 0", got)
	}
	if len(a.filteredAssets()) != 0 {
		t.Fatalf("filtered dirty assets before edit = %d, want 0", len(a.filteredAssets()))
	}
	p.Dirty[p.Assets[0].Rel] = true
	if got := a.countCategory("已修改", "已修改"); got != 1 {
		t.Fatalf("dirty count after edit = %d, want 1", got)
	}
	if got := a.filteredAssets(); len(got) != 1 || got[0].Rel != p.Assets[0].Rel {
		t.Fatalf("filtered dirty assets = %+v, want [%s]", got, p.Assets[0].Rel)
	}
}
