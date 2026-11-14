package main

import (
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var (
	windowTypeRe = regexp.MustCompile(`(?m)^[ \t]*type[ \t]*=[ \t]*"window"`)
	styleBlockRe = regexp.MustCompile(`(?m)^([A-Za-z_]+)\.([A-Za-z0-9_]+)[ \t]*\{`)
	bgRe         = regexp.MustCompile(`bg[ \t]*=[ \t]*"([^"]+)"`)
)

func loadMenuStyles(p *ThemeProject) map[string]string {
	return loadStylesUnder(p, "menu/style")
}

func loadDashStyles(p *ThemeProject) map[string]string {
	return loadStylesUnder(p, "dashboard/style")
}

func loadStylesUnder(p *ThemeProject, root string) map[string]string {
	out := map[string]string{}
	dir := p.Abs(root)
	filepath.WalkDir(dir, func(fp string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(fp), ".rc") {
			return nil
		}
		rel, err := filepath.Rel(p.ThemeRoot, fp)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		data := readRel(p, rel)
		if data == "" {
			return nil
		}
		base := path.Dir(rel)
		for _, loc := range styleBlockRe.FindAllStringSubmatchIndex(data, -1) {
			typ := data[loc[2]:loc[3]]
			name := data[loc[4]:loc[5]]
			open := strings.IndexByte(data[loc[0]:loc[1]], '{') + loc[0]
			closeAt := matchingBrace(data, open)
			if closeAt < 0 {
				continue
			}
			block := data[open+1 : closeAt]
			m := bgRe.FindStringSubmatch(block)
			if m == nil {
				continue
			}
			resolved := resolveMenuAsset(base, m[1])
			if resolved != "" {
				out[typ+"."+name] = resolved
			}
		}
		return nil
	})
	return out
}

func resolveMenuAsset(base, p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	if p == "" || p == "none" || p == "ParentRelative" ||
		strings.HasPrefix(p, "rgb(") || strings.HasPrefix(p, "rgba(") || strings.HasPrefix(p, "#") {
		return ""
	}
	for _, root := range []string{"menu/", "dashboard/", "switch_theme/", "bot/", "tdSuspendMgr/"} {
		if strings.HasPrefix(p, root) {
			return path.Clean(p)
		}
	}
	return path.Clean(path.Join(base, p))
}

func styleBG(p *ThemeProject, typ, style string) string {
	if p == nil || p.Scene == nil || style == "" || style == "none" {
		return ""
	}
	key := typ + "." + style
	if v := p.Scene.DashStyleBg[key]; v != "" {
		return v
	}
	return p.Scene.StyleBg[key]
}

func loadMenuPages(p *ThemeProject, zh map[string]string) []menuPage {
	root := p.Abs("menu/etc/layout")
	byKey := map[string]menuPage{}
	var keys []string
	filepath.WalkDir(root, func(fp string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(fp), ".ini") {
			return nil
		}
		rel, err := filepath.Rel(root, fp)
		if err != nil {
			return nil
		}
		key := strings.TrimSuffix(filepath.ToSlash(rel), ".ini")
		if strings.HasPrefix(key, "msg_box/") || key == "home" {
			return nil
		}
		text := readRel(p, "menu/etc/layout/"+key+".ini")
		win, style, widgets, ok := parseMenuLayout(text)
		if !ok {
			return nil
		}
		byKey[key] = menuPage{
			Key: key, Title: menuPageTitle(key, zh), Group: menuPageGroup(key),
			Window: win, Style: style, Widgets: widgets,
			List: menuPageList(key, zh),
		}
		keys = append(keys, key)
		return nil
	})
	for _, key := range keys {
		if parentMenuKey(key) != "" {
			continue
		}
		pg := byKey[key]
		pg.Window = positionMenuRoot(pg.Window)
		byKey[key] = pg
	}
	// Some layout files describe a child page of another window: their
	// 0,0 is the parent's content origin, not the screen origin.
	for _, key := range keys {
		parent := parentMenuKey(key)
		if parent == "" {
			continue
		}
		pg := byKey[key]
		pp, ok := byKey[parent]
		if ok && pg.Window.X == 0 && pg.Window.Y == 0 {
			pg.Window.X, pg.Window.Y = pp.Window.X, pp.Window.Y
		} else {
			pg.Window = positionMenuRoot(pg.Window)
		}
		byKey[key] = pg
	}
	var pages []menuPage
	for _, key := range keys {
		if key == "car_set/CarSet" {
			continue
		}
		pages = append(pages, byKey[key])
	}
	order := map[string]int{"菜单": 0, "车辆设置": 1, "DVR": 2, "导航": 3, "照片": 4, "TPMS": 5}
	sort.Slice(pages, func(i, j int) bool {
		oi, oj := order[pages[i].Group], order[pages[j].Group]
		if oi != oj {
			return oi < oj
		}
		return pages[i].Title < pages[j].Title
	})
	return pages
}

func positionMenuRoot(win SceneRect) SceneRect {
	if win.X != 0 || win.Y != 0 {
		return win
	}
	// Small second-menu windows live in the right panel at 364,110.
	if win.W <= 320 && win.H <= 400 {
		win.X, win.Y = 364, 110
	} else if win.W <= 768 && win.H <= 300 {
		// 768×218 pages (dash_set, abs, TCS, time…) are local
		// coordinate spaces inside the menu host, not screen
		// coordinates. Center the host panel on the 1024×600 canvas.
		win.X, win.Y = (1024-win.W)/2, (600-win.H)/2
	}
	return win
}

func parentMenuKey(key string) string {
	switch {
	case strings.HasPrefix(key, "car_set/set/"):
		return "car_set/CarSet"
	case strings.HasPrefix(key, "dvr/set/"):
		return "dvr/set"
	}
	return ""
}

func parseMenuLayout(text string) (SceneRect, string, []menuWidget, bool) {
	var chosenName, chosenBody string
	pos := 0
	for _, loc := range blockStartRe.FindAllStringSubmatchIndex(text, -1) {
		if loc[0] < pos {
			continue
		}
		name := text[loc[2]:loc[3]]
		open := strings.IndexByte(text[loc[0]:loc[1]], '{') + loc[0]
		closeAt := matchingBrace(text, open)
		if closeAt < 0 {
			continue
		}
		pos = closeAt + 1
		body := text[open+1 : closeAt]
		if windowTypeRe.MatchString(menuHead(body)) {
			chosenName, chosenBody = name, body
			break
		}
	}
	if chosenBody == "" {
		return SceneRect{}, "", nil, false
	}
	w, ok := parseMenuWidget(chosenName, chosenBody)
	if !ok {
		return SceneRect{}, "", nil, false
	}
	return w.Rect, w.Style, w.Children, true
}

func parseMenuWidget(name, body string) (menuWidget, bool) {
	// A widget's own fields come before its first child block. Restricting
	// the search to that prefix keeps a nested child's x/y/w/h from being
	// mistaken for this widget's rect.
	head := menuHead(body)
	x, okX := intField(head, "x")
	y, okY := intField(head, "y")
	w, okW := intField(head, "w")
	h, okH := intField(head, "h")
	if m := xywhRe.FindStringSubmatchIndex(head); m != nil {
		x, y, w, h = atoi(head[m[2]:m[3]]), atoi(head[m[4]:m[5]]), atoi(head[m[6]:m[7]]), atoi(head[m[8]:m[9]])
		okX, okY, okW, okH = true, true, true, true
	}
	if !okX || !okY || !okW || !okH {
		return menuWidget{}, false
	}
	m := menuWidget{Name: name, Rect: SceneRect{X: x, Y: y, W: w, H: h}}
	if sm := typeRe.FindStringSubmatch(head); sm != nil {
		m.Type = sm[1]
	}
	if sm := styleRe.FindStringSubmatch(head); sm != nil {
		m.Style = sm[1]
	}
	if sm := captionRe.FindStringSubmatch(head); sm != nil {
		m.Caption = strings.ReplaceAll(sm[1], `\"`, `"`)
	}
	m.Hidden = containsFlag(menuFlags(body), "hide")
	m.Children = parseMenuChildren(body)
	return m, true
}

// menuHead returns the part of a widget body before its first real child
// block. The widget's own flags block is skipped, so fields written after
// it (some layouts put caption below flags) are still part of the head.
// Braces inside quoted strings and comments are ignored.
func menuHead(body string) string {
	inString, comment := false, false
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if comment {
			if ch == '\n' {
				comment = false
			}
			continue
		}
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '#':
			comment = true
		case '"':
			inString = true
		case '{':
			closeAt := matchingBrace(body, i)
			if closeAt < 0 {
				return body[:i]
			}
			if menuBlockName(body, i) == "flags" {
				i = closeAt
				continue
			}
			return body[:i]
		}
	}
	return body
}

// menuFlags returns the immediate flags block of one widget, without
// descending into nested widgets.
func menuFlags(body string) string {
	inString, comment := false, false
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if comment {
			if ch == '\n' {
				comment = false
			}
			continue
		}
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '#':
			comment = true
		case '"':
			inString = true
		case '{':
			closeAt := matchingBrace(body, i)
			if closeAt < 0 {
				return ""
			}
			if menuBlockName(body, i) == "flags" {
				return body[i+1 : closeAt]
			}
			i = closeAt
		}
	}
	return ""
}

func containsFlag(flags, want string) bool {
	for _, line := range strings.Split(flags, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		for _, f := range strings.Fields(line) {
			if f == want {
				return true
			}
		}
	}
	return false
}

func parseMenuChildren(body string) []menuWidget {
	var out []menuWidget
	inString := false
	comment := false
	for i := 0; i < len(body); i++ {
		ch := body[i]
		if comment {
			if ch == '\n' {
				comment = false
			}
			continue
		}
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '#':
			comment = true
		case '"':
			inString = true
		case '{':
			closeAt := matchingBrace(body, i)
			if closeAt < 0 {
				return out
			}
			name := menuBlockName(body, i)
			if name != "flags" {
				if w, ok := parseMenuWidget(name, body[i+1:closeAt]); ok {
					out = append(out, w)
				}
			}
			i = closeAt
		}
	}
	return out
}

func menuBlockName(body string, open int) string {
	j := open - 1
	for j >= 0 && (body[j] == ' ' || body[j] == '\t') {
		j--
	}
	if j < 0 || body[j] != '=' {
		return ""
	}
	j--
	for j >= 0 && (body[j] == ' ' || body[j] == '\t') {
		j--
	}
	end := j + 1
	for j >= 0 && (body[j] == '_' || body[j] == '-' || body[j] >= '0' && body[j] <= '9' ||
		body[j] >= 'A' && body[j] <= 'Z' || body[j] >= 'a' && body[j] <= 'z') {
		j--
	}
	return body[j+1 : end]
}

func menuPageGroup(key string) string {
	switch {
	case strings.HasPrefix(key, "car_set/"):
		return "车辆设置"
	case strings.HasPrefix(key, "dvr/"):
		return "DVR"
	case strings.HasPrefix(key, "navi/"):
		return "导航"
	case strings.HasPrefix(key, "photo/"):
		return "照片"
	case strings.HasPrefix(key, "tpms/"):
		return "TPMS"
	default:
		return "菜单"
	}
}

func menuPageTitle(key string, zh map[string]string) string {
	names := map[string]string{
		"home": "主菜单", "dash_set": "仪表设置", "music": "音乐播放",
		"connect": "手机连接", "BTConnect": "蓝牙连接", "WIFIConnect": "Wi-Fi 连接",
		"ResetConnect": "重置连接", "brightness": "亮度设置", "language": "语言设置",
		"MAIUnit": "单位设置", "wallpaper": "壁纸", "ui_mode": "皮肤切换",
		"time": "时间设置", "DayOrNight": "日/夜模式", "abs": "ABS 设置",
		"TcsSet": "TCS 设置", "ride_mode": "骑行模式", "maintain": "维护",
		"reset": "恢复出厂设置", "DeviceInfo": "设备信息",
		"car_set/CarSet": "车辆设置", "dvr/dvr": "录像", "dvr/list": "录像列表",
		"dvr/play": "录像回放", "dvr/set": "录像设置", "tpms/tire_disp": "胎压显示",
		"navi/BTNaviLayout": "蓝牙导航", "navi/naviEndLayout": "导航结束",
		"navi/screenLinkNaviLayout": "投屏导航", "photo/photo_list": "照片列表",
		"photo/photo_play":   "照片查看",
		"dvr/set/CodeFormat": "编码格式", "dvr/set/LoopVideo": "循环录像",
		"dvr/set/MicSet": "麦克风", "dvr/set/camera": "摄像头",
		"dvr/set/display": "显示设置", "dvr/set/format": "格式化", "dvr/set/quality": "录像画质",
	}
	if v := names[key]; v != "" {
		return v
	}
	if strings.HasPrefix(key, "car_set/set/") {
		k := strings.TrimPrefix(key, "car_set/set/")
		if v := zh[k]; v != "" {
			return v
		}
		return k
	}
	return strings.ReplaceAll(path.Base(key), "_", " ")
}

func menuPageList(key string, zh map[string]string) []string {
	base := path.Base(key)
	if key == "dvr/set" {
		return []string{"录像画质", "循环录像", "编码格式", "麦克风", "摄像头", "显示设置", "格式化"}
	}
	switch {
	case base == "brightness":
		return []string{"10%", "20%", "30%", "40%", "50%", "60%", "70%", "80%", "90%", "100%"}
	case base == "language":
		return []string{"简体中文", "English"}
	case base == "MAIUnit":
		return []string{"公制 km", "英制 mile", "摄氏度 °C", "华氏度 °F"}
	case base == "ui_mode":
		return []string{"New Ride Joy", "Fire Eye"}
	case base == "wallpaper":
		return []string{"默认壁纸"}
	case base == "connect":
		return []string{"蓝牙连接", "Wi-Fi 连接", "重置连接"}
	case base == "abs":
		return []string{"打开前后轮", "关闭后轮", "关闭前后轮"}
	case base == "TcsSet":
		return []string{"开启", "关闭"}
	case base == "DayOrNight":
		return []string{"自动", "日间", "夜间"}
	case base == "time":
		return []string{"24 小时制", "12 小时制"}
	case base == "ride_mode":
		return []string{"经济", "标准", "运动"}
	case base == "DeviceInfo":
		return []string{"车型：台铃 K075", "固件：1.1.4", "主题：岁新骑悦", "电池：48V / 78%"}
	case base == "dvr":
		return []string{"实时预览", "录像", "回放", "设置"}
	case base == "list":
		return []string{"录像 2026-11-14 08:20", "录像 2026-11-14 08:35", "录像 2026-11-13 18:10"}
	case base == "tire_disp":
		return []string{"前轮：2.2 bar / 25°C", "后轮：2.3 bar / 26°C", "胎压状态：正常"}
	case base == "photo_list":
		return []string{"照片 1", "照片 2", "照片 3", "照片 4"}
	case base == "reset":
		return []string{"恢复出厂设置"}
	case base == "EnergyRecovery":
		return []string{"关闭", "弱", "强"}
	case base == "AutoShutdown":
		return []string{"关闭", "3 分钟", "5 分钟", "10 分钟", "15 分钟", "30 分钟"}
	case base == "SoftHardBoot":
		return []string{"软启动", "硬启动"}
	}
	if strings.HasPrefix(key, "car_set/set/") {
		return []string{"开启", "关闭"}
	}
	if strings.HasPrefix(key, "dvr/set/") {
		return []string{"默认", "选项 2", "选项 3"}
	}
	if v := zh[base]; v != "" {
		return []string{v}
	}
	return nil
}

func defaultPopups() []popupDef {
	return []popupDef{
		{"bt_msg", "蓝牙消息", "menu/style/Night/bg/main_msg_win.png", "手机已连接，可播放音乐和来电"},
		{"call", "来电", "menu/style/Night/bg/main_call_win.png", "176 **** 9417"},
		{"dtc", "车辆故障", "menu/style/Night/bg/main_dtc_win.png", "检测到车辆故障，请检查故障码"},
		{"asr", "语音助手", "menu/style/Night/bg/main_asr_win.png", "我在，请说"},
		{"wp", "壁纸提示", "menu/style/Night/bg/main_msg_win.png", "壁纸安装异常，请重试"},
		{"warn", "胎压警告", "menu/style/Night/bg/main_dtc_win.png", "前轮胎压异常，请检查"},
		{"clear", "清除小计里程", "menu/style/Night/bg/second_menu.png", "确认清除本次小计里程？"},
	}
}
