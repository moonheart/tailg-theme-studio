package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *app) currentMenuPage() *menuPage {
	if a.proj == nil || a.proj.Scene == nil || len(a.proj.Scene.MenuPages) == 0 {
		return nil
	}
	for i := range a.proj.Scene.MenuPages {
		if a.proj.Scene.MenuPages[i].Key == a.menuPageKey {
			return &a.proj.Scene.MenuPages[i]
		}
	}
	return &a.proj.Scene.MenuPages[0]
}

func (a *app) menuPageIndex() int {
	if a.proj == nil || a.proj.Scene == nil {
		return 0
	}
	for i := range a.proj.Scene.MenuPages {
		if a.proj.Scene.MenuPages[i].Key == a.menuPageKey {
			return i
		}
	}
	return 0
}

func (a *app) setMenuPage(key string) {
	a.menuPageKey = key
	a.sceneEpoch = time.Now()
	a.scenePhase = 0
	a.invalidatePreview()
}

func (a *app) menuPageStep(delta int) {
	if a.proj == nil || a.proj.Scene == nil || len(a.proj.Scene.MenuPages) == 0 {
		return
	}
	i := a.menuPageIndex() + delta
	n := len(a.proj.Scene.MenuPages)
	i = (i%n + n) % n
	a.setMenuPage(a.proj.Scene.MenuPages[i].Key)
}

func (a *app) popupStep(delta int) {
	if a.proj == nil || a.proj.Scene == nil || len(a.proj.Scene.Popups) == 0 {
		return
	}
	idx := 0
	for i := range a.proj.Scene.Popups {
		if a.proj.Scene.Popups[i].Key == a.popupType {
			idx = i
		}
	}
	n := len(a.proj.Scene.Popups)
	idx = (idx + delta%n + n) % n
	a.popupType = a.proj.Scene.Popups[idx].Key
	a.sceneEpoch = time.Now()
	a.scenePhase = 0
	a.invalidatePreview()
}

func (a *app) currentPopup() *popupDef {
	if a.proj == nil || a.proj.Scene == nil {
		return nil
	}
	for i := range a.proj.Scene.Popups {
		if a.proj.Scene.Popups[i].Key == a.popupType {
			return &a.proj.Scene.Popups[i]
		}
	}
	if len(a.proj.Scene.Popups) > 0 {
		return &a.proj.Scene.Popups[0]
	}
	return nil
}

func (a *app) drawBitmapTint(cp *ui.Painter, rel string, r ui.Rect, tint ui.Color) bool {
	b := a.bitmap(rel)
	if b == nil {
		return false
	}
	w, h := b.Size()
	cp.ImageRegion(b, ui.Rect{W: float32(w), H: float32(h)}, r, ui.ImageOptions{Tint: tint})
	return true
}

// ---- Menu pages ------------------------------------------------------------

func (a *app) drawMenuPageScene(cp *ui.Painter) {
	page := a.currentMenuPage()
	if page == nil {
		cp.Text(360, 280, "菜单布局缺失", 18, ui.Hex("#ffffff"))
		return
	}
	win := ui.Rect{X: float32(page.Window.X), Y: float32(page.Window.Y),
		W: float32(page.Window.W), H: float32(page.Window.H)}
	full := win.W > 900 && win.H > 500
	if full {
		cp.FillGradient(ui.Rect{W: themeW, H: themeH}, ui.LinearGradient{
			From: ui.Hex("#130c0c"), To: ui.Hex("#040405"), Angle: 135,
		}, 0)
	} else {
		a.drawDashboardScene(cp)
		a.drawBotAt(cp, 0, "normal", false)
		cp.Fill(ui.Rect{W: themeW, H: themeH}, ui.RGBA(0, 0, 0, 0.68), 0)
	}
	if r, ok := optionPageRect(page); ok {
		a.drawOptionPage(cp, page, win, r)
		if page.Key == "time" {
			a.drawTimeEditor(cp, page, win)
		}
		return
	}
	bg := styleBG(a.proj, "window", page.Style)
	bgDrawn := false
	if page.Key == "BTConnect" || page.Key == "WIFIConnect" {
		lang := "zh_CN"
		if a.menuLang == "en_US" {
			lang = "en_US"
		}
		name := strings.TrimPrefix(page.Key, "navi/")
		rel := fmt.Sprintf("menu/style/Night/connect/%s_%s.png", name, lang)
		if a.drawBitmapAt(cp, rel, win, ui.FillBox) {
			bgDrawn = true
		}
	}
	if page.Style == "main_win" || strings.Contains(bg, "home_slide.png") {
		// main_win is the 82×282 home dock; stretched over an 768×218
		// page it would cover the whole page with a beige sheet.
		bg = ""
	}
	if !bgDrawn && bg != "" {
		if strings.Contains(bg, "second_menu") {
			a.drawBitmapTint(cp, bg, win, ui.RGBA(28, 14, 14, 1))
		} else {
			a.drawBitmapAt(cp, bg, win, ui.FillBox)
		}
	} else if !bgDrawn {
		cp.Fill(win, ui.RGBA(22, 12, 12, 0.96), 12)
	}
	if !full {
		cp.Stroke(win, ui.RGBA(160, 70, 50, 0.5), 12, 1)
	}
	if !bgDrawn && page.Key != "tpms/tire_disp" && page.Key != "navi/BTNaviLayout" {
		a.drawMenuPageTitle(cp, page, win, full)
	}
	if page.Key == "tpms/tire_disp" {
		a.drawTPMSPage(cp, page, win)
		return
	}
	if page.Key == "navi/BTNaviLayout" {
		a.drawNaviPage(cp, page, win)
		return
	}
	a.drawMenuWidgets(cp, page.Widgets, win.X, win.Y, page)
	if full {
		note := ""
		switch page.Group {
		case "DVR":
			note = "摄像头实时画面 / 录像缩略图由车机运行时绘制"
		case "照片":
			note = "照片缩略图由车机相册提供"
		case "导航":
			note = "导航地图由车机或手机导航绘制"
		}
		if note != "" {
			cp.Text(40, themeH-34, note, 11, ui.RGBA(255, 255, 255, 0.38))
		}
	}
}

func findMenuWidget(widgets []menuWidget, name string) *menuWidget {
	for i := range widgets {
		if widgets[i].Name == name {
			return &widgets[i]
		}
		if w := findMenuWidget(widgets[i].Children, name); w != nil {
			return w
		}
	}
	return nil
}

func (a *app) drawNaviPage(cp *ui.Painter, page *menuPage, win ui.Rect) {
	laneBg := a.naviLaneBg
	if laneBg < 1 || laneBg > 8 {
		laneBg = 1
	}
	a.drawBitmapAt(cp, fmt.Sprintf("menu/style/navi/BTNavi/%d.png", laneBg),
		ui.Rect{X: win.X + 4, Y: win.Y + 13, W: 287, H: 71}, ui.FillBox)
	arrowDir := fmt.Sprintf("menu/style/navi/BTNavi/guideArrows/%d", a.naviArrow)
	arrows := countPNGs(a.proj, arrowDir)
	if arrows > 0 {
		frame := 0
		if a.sceneAnim {
			frame = int(a.scenePhase*8) % arrows
		}
		lanes := a.naviLanes
		if lanes < 1 {
			lanes = 1
		}
		if lanes > 8 {
			lanes = 8
		}
		for i := 0; i < lanes; i++ {
			a.drawBitmapAt(cp, fmt.Sprintf("%s/%d.png", arrowDir, frame),
				ui.Rect{X: win.X + 3 + float32(i)*36, Y: win.Y + 20, W: 35, H: 57}, ui.FillBox)
		}
	}
	iconDir := fmt.Sprintf("menu/style/navi/BTNavi/dynamicIcon/%d", a.naviIcon)
	frames := countPNGs(a.proj, iconDir)
	if frames > 0 {
		frame := 0
		if a.sceneAnim {
			frame = int(a.scenePhase*12) % frames
		}
		a.drawBitmapAt(cp, fmt.Sprintf("%s/%d.png", iconDir, frame),
			ui.Rect{X: win.X + 72, Y: win.Y + 88, W: 143, H: 143}, ui.Contain)
	}
	a.drawDigitsRight(cp, "dashboard/style/num", "500.5",
		ui.Rect{X: win.X, Y: win.Y + 243, W: 100, H: 45}, 0)
	cp.Text(win.X+110, win.Y+258, "km", 13, ui.RGBA(255, 255, 255, 0.7))
	cp.Text(win.X+168, win.Y+258, "右转进入", 13, ui.Hex("#fff1e6"))
	cp.Text(win.X+10, win.Y+292, "南极路", 15, ui.Hex("#fff1e6"))
	cp.Text(win.X+10, win.Y+330, fmt.Sprintf("图标 %d / 箭头 %d / %d 车道", a.naviIcon, a.naviArrow, a.naviLanes),
		8, ui.RGBA(255, 255, 255, 0.4))
}

// optionPageRect recognizes the 768×218 horizontal option pages. The
// listview must be short; a tall listview (DVR settings, for example) is a
// normal vertical list, not a row of options.
func optionPageRect(page *menuPage) (ui.Rect, bool) {
	if page.Style != "parent_relative" || page.Window.W < 700 || page.Window.H > 300 || len(page.List) == 0 {
		return ui.Rect{}, false
	}
	for i := range page.Widgets {
		w := &page.Widgets[i]
		if w.Type == "listview" && w.Rect.W >= 400 && w.Rect.H >= 24 && w.Rect.H <= 70 {
			return ui.Rect{X: float32(page.Window.X + w.Rect.X), Y: float32(page.Window.Y + w.Rect.Y),
				W: float32(w.Rect.W), H: float32(w.Rect.H)}, true
		}
	}
	return ui.Rect{}, false
}

func (a *app) drawOptionPage(cp *ui.Painter, page *menuPage, win ui.Rect, r ui.Rect) {
	title := page.Title
	if len([]rune(title)) > 0 {
		tw := float32(len([]rune(title))) * 20 * 0.58
		cp.Text(win.X+(win.W-tw)/2, win.Y+20, title, 20, ui.Hex("#fff1e6"))
	}
	n := len(page.List)
	gap := float32(8)
	itemW := (r.W - gap*float32(n-1)) / float32(n)
	sel := 0
	if a.sceneAnim {
		sel = int(a.scenePhase/1.7) % n
	}
	for i, label := range page.List {
		ri := ui.Rect{X: r.X + float32(i)*(itemW+gap), Y: r.Y, W: itemW, H: r.H}
		if i == sel {
			if !a.drawBitmapAt(cp, "menu/style/Night/list/second_select.png", ri, ui.FillBox) {
				cp.Fill(ri, ui.Hex("#c42424"), 8)
			}
		} else {
			cp.Fill(ri, ui.RGBA(0, 0, 0, 0.38), 8)
			cp.Stroke(ri, ui.RGBA(255, 255, 255, 0.12), 8, 1)
		}
		size := float32(15)
		tw := float32(len([]rune(label))) * size * 0.58
		col := ui.RGBA(255, 255, 255, 0.72)
		if i == sel {
			col = ui.Hex("#ffffff")
		}
		cp.Text(ri.X+(ri.W-tw)/2, ri.Y+(ri.H-size)/2-1, label, size, col)
	}
	// Red tip strip used by TCS and similar pages.
	for _, w := range page.Widgets {
		if w.Style == "red_tips" {
			rel := styleBG(a.proj, "button", "red_tips")
			rr := ui.Rect{X: win.X + float32(w.Rect.X), Y: win.Y + float32(w.Rect.Y),
				W: float32(w.Rect.W), H: float32(w.Rect.H)}
			if !a.drawBitmapAt(cp, rel, rr, ui.FillBox) {
				cp.Fill(rr, ui.RGBA(160, 30, 30, 0.75), 6)
			}
			if w.Caption != "" && w.Caption != " " {
				cp.Text(rr.X+12, rr.Y+6, w.Caption, 11, ui.Hex("#ffffff"))
			}
		}
	}
}

// drawTimeEditor renders the nested date/time editor of time.ini. It is a
// child window (time_operate_win) containing five child windows, so every
// position is resolved relative to its parent instead of hardcoded.
func (a *app) drawTimeEditor(cp *ui.Painter, page *menuPage, win ui.Rect) {
	op := findMenuWidget(page.Widgets, "time_operate_win")
	if op == nil {
		return
	}
	ox := win.X + float32(op.Rect.X)
	oy := win.Y + float32(op.Rect.Y)
	for _, group := range op.Children {
		if group.Type != "window" {
			continue
		}
		gx := ox + float32(group.Rect.X)
		gy := oy + float32(group.Rect.Y)
		for _, w := range group.Children {
			r := ui.Rect{X: gx + float32(w.Rect.X), Y: gy + float32(w.Rect.Y),
				W: float32(w.Rect.W), H: float32(w.Rect.H)}
			switch {
			case w.Style == "time_win_bg":
				rel := styleBG(a.proj, "button", w.Style)
				if rel == "" || !a.drawBitmapAt(cp, rel, r, ui.FillBox) {
					cp.Fill(r, ui.Hex("#efe7d8"), 3)
				}
			case w.Style == "time_style_num":
				size := float32(16)
				tw := float32(len([]rune(w.Caption))) * size * 0.6
				cp.Text(r.X+(r.W-tw)/2, r.Y+(r.H-size)/2-1, w.Caption, size, ui.Hex("#241512"))
			case strings.Contains(w.Style, "middle_18") && w.Caption != "":
				label := a.tr(w.Caption)
				size := float32(12)
				tw := float32(len([]rune(label))) * size * 0.58
				cp.Text(r.X+(r.W-tw)/2, r.Y+4, label, size, ui.RGBA(255, 255, 255, 0.55))
			}
		}
	}
}

func (a *app) drawTPMSPage(cp *ui.Painter, page *menuPage, win ui.Rect) {
	cp.Text(win.X+92, win.Y+20, "胎压显示", 16, ui.Hex("#fff1e6"))
	digitDir := "menu/style/Night/tpms/number/normal"
	if a.tpmsAbnormal {
		digitDir = "menu/style/Night/tpms/number/abnormal"
	}
	draw := func(name, style, fallback string) {
		w := findMenuWidget(page.Widgets, name)
		if w == nil {
			return
		}
		rel := styleBG(a.proj, "button", style)
		if rel == "" {
			rel = styleBG(a.proj, "breath_button", style)
		}
		if rel == "" {
			rel = fallback
		}
		a.drawBitmapAt(cp, rel, ui.Rect{X: win.X + float32(w.Rect.X), Y: win.Y + float32(w.Rect.Y),
			W: float32(w.Rect.W), H: float32(w.Rect.H)}, ui.FillBox)
	}
	draw("btn_motor_icon", "motor_icon", "menu/style/Night/tpms/motor.png")
	draw("breathBtn_front_wheel", "front_wheel", "menu/style/Night/tpms/front_wheel_abnormal.png")
	draw("breathBtn_rear_wheel", "rear_wheel", "menu/style/Night/tpms/rear_wheel_abnormal.png")
	draw("front_guide_line_unit", "guide_line_unit", "menu/style/Night/tpms/unit_bar.png")
	draw("rear_guide_line_unit", "guide_line_unit", "menu/style/Night/tpms/unit_bar.png")
	cp.Text(win.X+12, win.Y+204, "前轮", 14, ui.RGBA(255, 255, 255, 0.75))
	cp.Text(win.X+12, win.Y+244, "后轮", 14, ui.RGBA(255, 255, 255, 0.75))
	num := func(name string, text string) {
		w := findMenuWidget(page.Widgets, name)
		if w == nil {
			return
		}
		a.drawDigitsRight(cp, digitDir, text,
			ui.Rect{X: win.X + float32(w.Rect.X), Y: win.Y + float32(w.Rect.Y),
				W: float32(w.Rect.W), H: float32(w.Rect.H)}, 0)
	}
	frontP, rearP := "2.2", "2.3"
	frontT, rearT := "25", "26"
	if a.tpmsAbnormal {
		frontP, rearP = "1.4", "1.3"
		frontT, rearT = "48", "51"
	}
	num("numBtn_front_wheel_tpms", frontP)
	num("numBtn_front_wheel_temp", frontT)
	num("numbtn_rear_wheel_tpms", rearP)
	num("numbtn_rear_wheel_temp", rearT)
}

// drawMenuPageTitle uses the theme's short topical button as the title bar
// when there is one. The firmware centers the page caption there; drawing it
// at the window corner would put it over the first list row.
func (a *app) drawMenuPageTitle(cp *ui.Painter, page *menuPage, win ui.Rect, full bool) {
	size := float32(17)
	for _, w := range page.Widgets {
		if !strings.HasSuffix(w.Name, "_topical") || w.Rect.H > 60 || w.Rect.W < 100 {
			continue
		}
		if n := styleTrailingNumber(w.Style); n >= 18 {
			size = float32(n) * 0.72
			if size > 22 {
				size = 22
			}
		}
		r := ui.Rect{X: win.X + float32(w.Rect.X), Y: win.Y + float32(w.Rect.Y),
			W: float32(w.Rect.W), H: float32(w.Rect.H)}
		tw := float32(len([]rune(page.Title))) * size * 0.58
		cp.Text(r.X+(r.W-tw)/2, r.Y+(r.H-size)/2, page.Title, size, ui.Hex("#fff1e6"))
		return
	}
	tx, ty := win.X+16, win.Y+11
	if full {
		tx, ty = 28, 22
	}
	cp.Text(tx, ty, page.Title, size, ui.Hex("#fff1e6"))
}

func styleTrailingNumber(style string) int {
	i := len(style)
	for i > 0 && style[i-1] >= '0' && style[i-1] <= '9' {
		i--
	}
	if i == len(style) {
		return 0
	}
	n, _ := strconv.Atoi(style[i:])
	return n
}

func (a *app) drawMenuWidgets(cp *ui.Painter, widgets []menuWidget, ox, oy float32, page *menuPage) {
	for _, w := range widgets {
		a.drawMenuWidget(cp, w, ox, oy, page)
	}
}

// menuWindowTransparent reports container styles that inherit their parent
// instead of painting their own card. Filling these with the dark fallback
// would stack a second black box over the parent window.
func menuWindowTransparent(style string) bool {
	switch style {
	case "", "none", "alpha", "parent_relative", "navi_win_parent_relative":
		return true
	}
	return false
}

func (a *app) drawMenuWidget(cp *ui.Painter, w menuWidget, ox, oy float32, page *menuPage) {
	if w.Hidden && w.Type == "window" {
		// Alternate-state overlays (save confirmation, loading page, …)
		// are hidden until the firmware raises them. Drawing every state
		// at once would stack dialogs on top of the normal page.
		return
	}
	r := ui.Rect{X: ox + float32(w.Rect.X), Y: oy + float32(w.Rect.Y),
		W: float32(w.Rect.W), H: float32(w.Rect.H)}
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if w.Type == "window" || len(w.Children) > 0 && w.Type == "" {
		bg := styleBG(a.proj, "window", w.Style)
		if bg != "" {
			if strings.Contains(bg, "second_menu") {
				a.drawBitmapTint(cp, bg, r, ui.RGBA(26, 13, 13, 1))
			} else {
				a.drawBitmapAt(cp, bg, r, ui.FillBox)
			}
		} else if !menuWindowTransparent(w.Style) {
			cp.Fill(r, ui.RGBA(22, 12, 12, 0.95), 10)
		}
		a.drawMenuWidgets(cp, w.Children, r.X, r.Y, page)
		return
	}
	bg := styleBG(a.proj, w.Type, w.Style)
	if bg != "" {
		a.drawBitmapAt(cp, bg, r, ui.FillBox)
	}
	switch w.Type {
	case "listview":
		a.drawMenuList(cp, w, r, page)
	case "list_slide":
		a.drawMenuSlide(cp, w, r, page)
	case "qrcode":
		a.drawQRPlaceholder(cp, r)
	case "processbar":
		cp.Fill(r, ui.RGBA(0, 0, 0, 0.5), 4)
		if r.W > r.H {
			cp.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * 0.62, H: r.H}, ui.Hex("#d9d9d9"), 3)
		} else {
			cp.Fill(ui.Rect{X: r.X, Y: r.Y + r.H*0.38, W: r.W, H: r.H * 0.62}, ui.Hex("#d9d9d9"), 3)
		}
	case "roll_text":
		cp.Text(r.X+8, r.Y+8, "正在播放：一路向北", 13, ui.Hex("#ffffff"))
	}
	if len(w.Children) > 0 {
		a.drawMenuWidgets(cp, w.Children, r.X, r.Y, page)
	}
	txt := a.menuWidgetText(w)
	if txt == "" {
		return
	}
	size := float32(13)
	if w.Style == "middle_24" || strings.Contains(w.Style, "24") {
		size = 17
	}
	center := w.Type == "button" || w.Type == "num_button" || strings.Contains(w.Style, "middle")
	x := r.X + 8
	if center {
		tw := float32(len([]rune(txt))) * size * 0.55
		x = r.X + (r.W-tw)/2
	}
	y := r.Y + (r.H-size)/2
	if w.Rect.H > 60 {
		y = r.Y + 10
	}
	cp.Text(x, y, txt, size, ui.Hex("#f8e9df"))
}

func (a *app) menuWidgetText(w menuWidget) string {
	txt := strings.TrimSpace(w.Caption)
	if txt == "" {
		return ""
	}
	if strings.Contains(txt, "/") || strings.HasSuffix(strings.ToLower(txt), ".png") {
		return ""
	}
	lower := strings.ToLower(txt)
	if strings.Contains(lower, "asdf") || lower == "bt_roll_name" || lower == "wifi_roll_name" {
		return "TAILG-K075"
	}
	if strings.HasPrefix(txt, "HINT_") {
		return "提示"
	}
	if tr := a.tr(txt); tr != txt {
		return tr
	}
	if strings.Contains(txt, "_") && strings.ToUpper(txt) == txt {
		return ""
	}
	return txt
}

func (a *app) drawMenuSlide(cp *ui.Painter, w menuWidget, r ui.Rect, page *menuPage) {
	items := page.List
	if len(items) == 0 {
		items = []string{"仪表信息", "骑行数据", "胎压", "主题", "单位"}
	}
	n := len(items)
	gap := float32(6)
	visible := n
	start := 0
	cardW := float32(182)
	if page.Key == "dash_set" && n > 5 {
		visible = 5
		cardW = (r.W - 24 - gap*float32(visible-1)) / float32(visible)
		if cardW > 182 {
			cardW = 182
		}
		if a.sceneAnim {
			start = int(a.scenePhase/2.2) % n
		}
	} else if w.Rect.W > 0 && n > 0 && w.Rect.W < int(cardW)*2 {
		cardW = float32(w.Rect.W) / 2
	}
	total := cardW*float32(visible) + gap*float32(visible-1)
	x := r.X + (r.W-total)/2
	if x < r.X+8 {
		x = r.X + 8
	}
	sel := 0
	if page.Key == "dash_set" && n > 5 {
		sel = (start + 2) % n
	} else if a.sceneAnim {
		sel = int(a.scenePhase/1.6) % n
	}
	for j := 0; j < visible; j++ {
		idx := (start + j) % n
		label := items[idx]
		if page.Key == "dash_set" {
			label = a.menuText(label)
		}
		ri := ui.Rect{X: x + float32(j)*(cardW+gap), Y: r.Y + 30, W: cardW, H: r.H - 34}
		cp.Fill(ri, ui.RGBA(20, 11, 11, 0.9), 12)
		if idx == sel {
			cp.Stroke(ri, ui.Hex("#d83434"), 12, 3)
		} else {
			cp.Stroke(ri, ui.RGBA(255, 255, 255, 0.12), 12, 1)
		}
		iconDrawn := false
		selectedArt := false
		if idx < len(page.SlideIcons) {
			ic := page.SlideIcons[idx]
			if idx == sel {
				// The active artwork is 68×100 and already contains the
				// localized label; draw it as one piece, don't double up.
				rel := ic.Zh
				if a.menuLang == "en_US" && ic.En != "" {
					rel = ic.En
				}
				if rel != "" {
					rr := ui.Rect{X: ri.X + 8, Y: ri.Y + 6, W: ri.W - 16, H: ri.H - 12}
					if a.drawBitmapAt(cp, rel, rr, ui.Contain) {
						iconDrawn = true
						selectedArt = true
					}
				}
			}
			if !selectedArt && ic.Normal != "" {
				size := float32(62)
				rr := ui.Rect{X: ri.X + (ri.W-size)/2, Y: ri.Y + 18, W: size, H: size}
				iconDrawn = a.drawBitmapAt(cp, ic.Normal, rr, ui.Contain)
			}
		}
		if !iconDrawn {
			cp.Text(ri.X+14, ri.Y+28, fmt.Sprintf("%02d", idx+1), 28, ui.RGBA(255, 255, 255, 0.22))
		}
		if selectedArt {
			continue
		}
		size := float32(15)
		if len([]rune(label)) > 8 {
			size = 13
		}
		cp.Text(ri.X+10, ri.Y+ri.H-32, label, size, ui.Hex("#fff1e6"))
	}
}

func (a *app) menuText(key string) string {
	if a.proj == nil || a.proj.Scene == nil {
		return key
	}
	m := a.proj.Scene.Zh
	if a.menuLang == "en_US" {
		m = a.proj.Scene.En
	}
	if s, ok := m[key]; ok && s != "" {
		return s
	}
	return key
}

func (a *app) drawMenuList(cp *ui.Painter, w menuWidget, r ui.Rect, page *menuPage) {
	list := page.List
	if len(list) == 0 {
		return
	}
	rowH := float32(44)
	if r.W < 240 {
		rowH = 30
	}
	// Keep at least three rows visible in short lists instead of
	// compressing every card to 30 px.
	if r.H < rowH*3 {
		rowH = r.H / 3
	}
	if rowH < 22 {
		rowH = 22
	}
	visible := int(r.H / rowH)
	if visible < 1 {
		visible = 1
	}
	sel := 0
	if a.sceneAnim && len(list) > 0 {
		sel = int(a.scenePhase/1.3) % len(list)
	}
	off := sel - visible/2
	if off < 0 {
		off = 0
	}
	if off > len(list)-visible {
		off = len(list) - visible
	}
	if off < 0 {
		off = 0
	}
	for i := 0; i < visible && i+off < len(list); i++ {
		idx := i + off
		y := r.Y + float32(i)*rowH
		row := ui.Rect{X: r.X + 4, Y: y, W: r.W - 8, H: rowH - 2}
		if idx == sel {
			if !a.drawBitmapAt(cp, "menu/style/Night/list/second_select.png", row, ui.FillBox) {
				cp.Fill(row, ui.RGBA(196, 36, 36, 0.9), 6)
			}
		}
		col := ui.Hex("#ffffff")
		if idx != sel {
			col = ui.RGBA(255, 255, 255, 0.68)
		}
		size := float32(13)
		if rowH < 36 {
			size = 11
		}
		cp.Text(row.X+12, row.Y+(rowH-size)/2-1, list[idx], size, col)
	}
}

func (a *app) drawQRPlaceholder(cp *ui.Painter, r ui.Rect) {
	cp.Fill(ui.Rect{X: r.X - 4, Y: r.Y - 4, W: r.W + 8, H: r.H + 8}, ui.Hex("#ffffff"), 6)
	cell := r.W / 21
	for y := 0; y < 21; y++ {
		for x := 0; x < 21; x++ {
			if (x*7+y*11+x*y)%5 < 2 {
				cp.Fill(ui.Rect{X: r.X + float32(x)*cell, Y: r.Y + float32(y)*cell,
					W: cell, H: cell}, ui.Hex("#171717"), 0)
			}
		}
	}
	for _, p := range [][2]int{{0, 0}, {14, 0}, {0, 14}} {
		outer := ui.Rect{X: r.X + float32(p[0])*cell, Y: r.Y + float32(p[1])*cell, W: cell * 7, H: cell * 7}
		inner := ui.Rect{X: outer.X + cell*1.5, Y: outer.Y + cell*1.5, W: cell * 4, H: cell * 4}
		dot := ui.Rect{X: outer.X + cell*2.5, Y: outer.Y + cell*2.5, W: cell * 2, H: cell * 2}
		cp.Fill(outer, ui.Hex("#171717"), 1)
		cp.Fill(inner, ui.Hex("#ffffff"), 1)
		cp.Fill(dot, ui.Hex("#171717"), 1)
	}
}

// ---- Popups ----------------------------------------------------------------

func (a *app) drawPopupScene(cp *ui.Painter) {
	a.drawDashboardScene(cp)
	a.drawBotAt(cp, 0, "normal", false)
	cp.Fill(ui.Rect{W: themeW, H: themeH}, ui.RGBA(0, 0, 0, 0.62), 0)
	p := a.currentPopup()
	if p == nil {
		cp.Text(380, 280, "弹窗资源缺失", 18, ui.Hex("#ffffff"))
		return
	}
	off := float32(0)
	if a.sceneAnim {
		ph := math.Mod(a.scenePhase, 4)
		off = -60 * (1 - float32(smoothstep(math.Min(ph/0.45, 1))))
	}
	if p.Key == "clear" {
		r := ui.Rect{X: 364, Y: 110 + off, W: 296, H: 360}
		if a.drawBitmapTint(cp, p.BG, r, ui.RGBA(28, 14, 14, 1)) == false {
			cp.Fill(r, ui.RGBA(26, 13, 13, 0.98), 12)
		}
		cp.Stroke(r, ui.RGBA(180, 70, 50, 0.45), 12, 1)
		cp.Text(r.X+24, r.Y+40, p.Title, 19, ui.Hex("#fff1e6"))
		cp.Text(r.X+24, r.Y+84, p.Body, 13, ui.RGBA(255, 255, 255, 0.65))
		cp.Fill(ui.Rect{X: r.X + 24, Y: r.Y + 230, W: 248, H: 42}, ui.RGBA(255, 255, 255, 0.08), 8)
		cp.Text(r.X+92, r.Y+241, "取消", 14, ui.Hex("#ffffff"))
		cp.Fill(ui.Rect{X: r.X + 24, Y: r.Y + 288, W: 248, H: 42}, ui.Hex("#c42424"), 8)
		cp.Text(r.X+92, r.Y+299, "确认清除", 14, ui.Hex("#ffffff"))
	} else {
		r := ui.Rect{X: 294, Y: 32 + off, W: 436, H: 60}
		if !a.drawBitmapTint(cp, p.BG, r, ui.RGBA(34, 16, 16, 1)) {
			cp.Fill(r, ui.RGBA(26, 13, 13, 0.98), 10)
		}
		cp.Stroke(r, ui.RGBA(190, 80, 55, 0.35), 10, 1)
		icon := "menu/style/App/phone.png"
		switch p.Key {
		case "dtc":
			icon = fmt.Sprintf("menu/style/dtc/%d.png", a.dtcCode)
			if !a.proj.Has(icon) {
				icon = "menu/style/dtc/fault.png"
			}
		case "warn":
			icon = "menu/style/warn/tire.png"
		case "asr":
			icon = ""
		case "wp":
			icon = "menu/style/dash_set/WallPaperSwitch_zh.png"
		}
		if icon != "" {
			cp.Fill(ui.Rect{X: r.X + 12, Y: r.Y + 8, W: 44, H: 44}, ui.RGBA(255, 255, 255, 0.08), 22)
			a.drawBitmapAt(cp, icon, ui.Rect{X: r.X + 14, Y: r.Y + 10, W: 40, H: 40}, ui.Contain)
		}
		cp.Text(r.X+66, r.Y+10, p.Title, 15, ui.Hex("#ffffff"))
		cp.Text(r.X+66, r.Y+32, p.Body, 11, ui.RGBA(255, 255, 255, 0.62))
		if p.Key == "asr" {
			frame := 0
			if a.sceneAnim {
				frame = int(a.scenePhase*30) % 38
			}
			bg := ui.Rect{X: 372, Y: 100 + off, W: 280, H: 76}
			cp.Fill(bg, ui.RGBA(20, 10, 10, 0.82), 12)
			cp.Stroke(bg, ui.RGBA(190, 80, 55, 0.3), 12, 1)
			a.drawBitmapAt(cp, fmt.Sprintf("menu/style/asr/night/%d.png", frame),
				ui.Rect{X: 384, Y: 106 + off, W: 256, H: 64}, ui.Contain)
		}
		if p.Key == "dtc" || p.Key == "warn" {
			detail := ui.Rect{X: 318, Y: 104 + off, W: 388, H: 92}
			if a.drawBitmapTint(cp, "menu/style/Night/bg/msg_detail.png", detail, ui.RGBA(30, 14, 14, 1)) {
				cp.Text(detail.X+18, detail.Y+18, p.Title, 13, ui.Hex("#ffffff"))
				cp.Text(detail.X+18, detail.Y+42, p.Body, 11, ui.RGBA(255, 255, 255, 0.6))
			}
		}
	}
}
