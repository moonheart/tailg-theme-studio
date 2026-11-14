package main

import (
	"fmt"
	"image/png"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

type sceneKind int

const (
	sceneDash sceneKind = iota
	sceneMenu
	sceneMenuSub
	sceneMenuPage
	scenePopup
	sceneBot
	sceneCharge
	sceneLock
	sceneLink
	sceneLights
	sceneSwitch
	sceneCountdown
	sceneKindCount
)

var sceneNames = []string{
	"仪表驾驶", "主菜单", "车辆设置", "菜单页面", "弹窗", "bot 助手", "充电", "锁屏密码",
	"投屏导航", "自检 / 告警灯", "主题换肤", "关机倒计时",
}

var sceneDescriptions = []string{
	"仪表主界面：车速、档位、SOC、时间、骑行数据、功率、天气/日期、指南针/海拔、氮气、ECU / 电机温度；速度和电量会循环变化。",
	"主菜单纵向 dock：10 个灯笼菜单卡片自动滚动，背景是当前仪表。",
	"车辆设置二级菜单：红色选中条随选项移动，文字来自主题翻译文件。",
	"菜单页面：遍历主题里的全部菜单布局，用通用渲染器显示背景、按钮、文本和列表。",
	"弹窗：蓝牙消息、来电、DTC、ASR、壁纸提示、胎压警告和清除里程。",
	"bot 助手：init / normal / menu / msg 四套各 50 帧，116×90 序列动画。",
	"充电界面：百分比、电池、线缆和闪电图标都由 48 帧序列图循环播放。",
	"锁屏密码：5 位密码逐格输入，选中框跟随光标移动。",
	"投屏导航：保留主题的投屏背景，底部显示车速、SOC、档位和时间。",
	"开机自检 / 告警灯：18 盏警示灯 + 4 个状态灯 + GPS 信号，全部点亮后分组闪烁。",
	"主题 UI 换肤：switch_theme 的 48 帧 Loading 动画。",
	"关机倒计时：3–2–1 大数字和进度条。",
}

func (s sceneKind) label() string {
	if int(s) >= 0 && int(s) < len(sceneNames) {
		return sceneNames[s]
	}
	return "仪表驾驶"
}

func sceneGroup(s sceneKind) string {
	switch s {
	case sceneDash:
		return "仪表"
	case sceneMenu, sceneMenuSub, sceneMenuPage:
		return "菜单"
	case scenePopup:
		return "消息"
	case sceneBot:
		return "助手"
	case sceneSwitch:
		return "主题"
	default:
		return "车机"
	}
}

func sceneOptionLabels() []string {
	out := make([]string, int(sceneKindCount))
	for i := 0; i < int(sceneKindCount); i++ {
		s := sceneKind(i)
		label := sceneGroup(s) + " · " + s.label()
		switch s {
		case sceneMenuPage:
			label += " (49 页)"
		case scenePopup:
			label += " (7 类)"
		}
		out[i] = label
	}
	return out
}

func (s sceneKind) desc() string {
	if int(s) >= 0 && int(s) < len(sceneDescriptions) {
		return sceneDescriptions[s]
	}
	return ""
}

func (s sceneKind) shortDesc() string {
	switch s {
	case sceneMenu:
		return "10 个菜单卡片纵向轮播"
	case sceneMenuSub:
		return "车辆设置选项与红色选中条"
	case sceneMenuPage:
		return "全部菜单布局通用预览"
	case scenePopup:
		return "7 类消息/来电/故障弹窗"
	case sceneBot:
		return "四套 50 帧 bot 动画"
	case sceneCharge:
		return "百分比 + 48 帧充电序列"
	case sceneLock:
		return "5 位密码逐格输入"
	case sceneLink:
		return "投屏背景 + 底部行车信息"
	case sceneLights:
		return "18 灯 + 4 信号 + GPS 自检"
	case sceneSwitch:
		return "48 帧换肤 Loading 动画"
	case sceneCountdown:
		return "3–2–1 下电倒计时"
	default:
		return "车速 / SOC / 氮气 / ECU·电机温度"
	}
}

func (s sceneKind) folder() string {
	switch s {
	case sceneMenu:
		return "menu/style/home"
	case sceneMenuSub:
		return "menu/style/Night/list"
	case sceneMenuPage:
		return "menu/etc/layout"
	case scenePopup:
		return "menu/style/Night/bg"
	case sceneBot:
		return "bot/style/seq_pic"
	case sceneCharge:
		return "menu/style/charge"
	case sceneLock:
		return "tdSuspendMgr/style/lock"
	case sceneLink:
		return "dashboard/style/link"
	case sceneLights:
		return "dashboard/style/lamps"
	case sceneSwitch:
		return "switch_theme/style/theme"
	case sceneCountdown:
		return "menu/style/count_down"
	default:
		return "dashboard/style/nightime/bg"
	}
}

// ---- Scene data ------------------------------------------------------------

type SceneRect struct{ X, Y, W, H int }

type menuEntry struct {
	Title string
	N     string
	Zh    string
	En    string
}

func (m menuEntry) image(lang string) string {
	if lang == "en_US" && m.En != "" {
		return m.En
	}
	if m.Zh != "" {
		return m.Zh
	}
	return m.N
}

type lampEntry struct {
	ID, X, Y int
	Check    bool
}

type botRule struct {
	X, Y, W, H int
	Path       string
}

type botData struct {
	Normal map[int]botRule
	Msg    map[int]botRule
	Has    bool
}

type menuWidget struct {
	Name, Type, Style, Caption string
	Rect                       SceneRect
	Children                   []menuWidget
	Hidden                     bool
}

type menuIcon struct {
	Title  string
	Normal string
	Zh     string
	En     string
}

type menuPage struct {
	Key        string
	Title      string
	Group      string
	Window     SceneRect
	Style      string
	Widgets    []menuWidget
	List       []string
	SlideIcons []menuIcon
}

type popupDef struct {
	Key, Title, BG, Body string
}

type SceneData struct {
	MenuDock        SceneRect
	CardW           int
	CardH           int
	MenuItems       []menuEntry
	Zh              map[string]string
	En              map[string]string
	CarSet          []string
	Page0Lamps      []lampEntry
	Page0Signal     []lampEntry
	Page3Lamps      []lampEntry
	Page3Signal     []lampEntry
	LampDir         string
	SignalDir       string
	GPSDir          string
	LampPaths       map[int]string
	SignalPaths     map[int]string
	LinkLampDir     string
	LinkSignalDir   string
	LinkLampPaths   map[int]string
	LinkSignalPaths map[int]string
	DashSetIcons    []menuIcon
	Bot             *botData
	MenuPages       []menuPage
	StyleBg         map[string]string
	Popups          []popupDef
	NitroIcon       SceneRect
	NitroSeq        SceneRect
	NitroLinkIcon   SceneRect
	NitroLinkSeq    SceneRect
	NitroFrames     int
	NitroSeqDir     string
	EcuIcon         SceneRect
	EcuTemp         SceneRect
	EcuUnit         SceneRect
	MotorIcon       SceneRect
	MotorTemp       SceneRect
	MotorUnit       SceneRect
	DashStyleBg     map[string]string
	RideData        SceneRect
	WeatherBG       SceneRect
	WeatherIcon     SceneRect
	WeatherText     SceneRect
	AirTemp         SceneRect
	TempUnit        SceneRect
	DateText        SceneRect
	WeekText        SceneRect
	CompassIcon     SceneRect
	CompassText     SceneRect
	CompassValue    SceneRect
	CompassUnit     SceneRect
	AltitudeText    SceneRect
	AltitudeValue   SceneRect
	AltitudeUnit    SceneRect
	PowerTitle      SceneRect
	PowerNum        SceneRect
	PowerUnit       SceneRect
	ChargeWin       SceneRect
	ChargeTitle     SceneRect
	ChargeInfo      SceneRect
	ChargeHint      SceneRect
	ChargePower     SceneRect
	ChargeVoltage   SceneRect
	LockSelect      SceneRect
	LockSlots       []SceneRect
	LinkWidgets     map[string]*Widget
	HasLock         bool
	HasCharge       bool
	HasSwitch       bool
}

var (
	menuHomeTitleRe = regexp.MustCompile(`(?m)^\s*title\s*=\s*"([^"]*)"`)
	menuFieldRe     = regexp.MustCompile(`(?m)^\s*(n|en_US|zh_CN)\s*=\s*"([^"]*)"`)
	transLineRe     = regexp.MustCompile(`(?m)^\s*"([^"]+)"\s*=\s*"([^"]*)"`)
	lightXYRe       = regexp.MustCompile(`x\s*=\s*(-?\d+)\s*;\s*y\s*=\s*(-?\d+)`)
	lightIDRe       = regexp.MustCompile(`(?m)^\s*id\s*=\s*(\d+)`)
	cardSizeRe      = regexp.MustCompile(`home_node\s*\{[^}]*?w\s*=\s*(\d+)\s*;?\s*h\s*=\s*(\d+)`)
)

func readRel(p *ThemeProject, rel string) string {
	data, err := os.ReadFile(p.Abs(rel))
	if err != nil {
		return ""
	}
	return string(data)
}

func iniSection(text, name string) string {
	needle := "[" + name + "]"
	start := strings.Index(text, needle)
	if start < 0 {
		return ""
	}
	rest := text[start+len(needle):]
	if end := strings.Index(rest, "\n["); end >= 0 {
		return rest[:end]
	}
	return rest
}

func unquoteINI(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		s = s[1 : len(s)-1]
	}
	return strings.ReplaceAll(s, `\"`, `"`)
}

func loadSceneData(p *ThemeProject) *SceneData {
	d := &SceneData{
		MenuDock:        SceneRect{X: 10, Y: 129, W: 82, H: 282},
		CardW:           68,
		CardH:           100,
		Zh:              map[string]string{},
		En:              map[string]string{},
		LampPaths:       map[int]string{},
		SignalPaths:     map[int]string{},
		LinkLampPaths:   map[int]string{},
		LinkSignalPaths: map[int]string{},
	}
	if p == nil {
		return d
	}
	if widgets := parseWidgets(readRel(p, "menu/etc/layout/home.ini")); widgets["home_win"] != nil {
		w := widgets["home_win"]
		d.MenuDock = SceneRect{X: w.X, Y: w.Y, W: w.W, H: w.H}
	}
	if m := cardSizeRe.FindStringSubmatch(readRel(p, "menu/etc/list_layout.ini")); m != nil {
		d.CardW = atoi(m[1])
		d.CardH = atoi(m[2])
	}
	d.MenuItems = parseMenuHome(readRel(p, "menu/etc/menu-config.ini"))
	d.Zh = parseTranslations(readRel(p, "menu/translate/trans.zh_CN"))
	d.En = parseTranslations(readRel(p, "menu/translate/trans.en_US"))
	d.CarSet = parseSectionTitles(readRel(p, "menu/etc/menu-config.ini"), "CarSet")
	lights := readRel(p, "dashboard/etc/light.ini")
	d.Page0Lamps, d.Page0Signal = parseLightPage(lights, "page_0")
	d.Page3Lamps, d.Page3Signal = parseLightPage(lights, "page_3")
	loadLightPaths(p, d, lights)
	d.HasLock = regexp.MustCompile(`has_lock\s*=\s*1`).MatchString(readRel(p, "tdSuspendMgr/etc/config.ini"))
	d.Bot = loadBotData(p)
	d.StyleBg = loadMenuStyles(p)
	d.DashStyleBg = loadDashStyles(p)
	d.MenuPages = loadMenuPages(p, d.Zh)
	for i := range d.MenuPages {
		if d.MenuPages[i].Key == "dash_set" {
			d.MenuPages[i].SlideIcons = d.DashSetIcons
			d.MenuPages[i].List = d.MenuPages[i].List[:0]
			for _, ic := range d.DashSetIcons {
				d.MenuPages[i].List = append(d.MenuPages[i].List, ic.Title)
			}
		}
	}
	d.Popups = defaultPopups()
	loadNitroData(p, d)
	loadTempData(p, d)
	loadHudData(p, d)
	d.HasCharge = p.Has("menu/style/charge/bg.png")
	d.HasSwitch = p.Has("switch_theme/style/theme/0.png")
	if d.HasLock {
		lw := parseWidgets(readRel(p, "tdSuspendMgr/etc/lock/layout.ini"))
		if w := lw["select_bg"]; w != nil {
			d.LockSelect = SceneRect{X: w.X, Y: w.Y, W: w.W, H: w.H}
		}
		for i := 0; i < 5; i++ {
			name := fmt.Sprintf("pwd_btn_%d", i)
			if w := lw[name]; w != nil {
				d.LockSlots = append(d.LockSlots, SceneRect{X: w.X, Y: w.Y, W: w.W, H: w.H})
			}
		}
		if len(d.LockSlots) == 0 {
			d.LockSelect = SceneRect{X: 279, Y: 190, W: 85, H: 144}
			for i := 0; i < 5; i++ {
				d.LockSlots = append(d.LockSlots, SceneRect{X: 288 + i*97, Y: 223, W: 60, H: 74})
			}
		}
	}
	return d
}

func loadNitroData(p *ThemeProject, d *SceneData) {
	if p == nil || d == nil {
		return
	}
	if w := p.Widget("n2_icon_0"); w != nil {
		d.NitroIcon = SceneRect{X: w.X, Y: w.Y, W: w.W, H: w.H}
	}
	if w := p.Widget("n2_left_warn_0"); w != nil {
		d.NitroSeq = SceneRect{X: w.X, Y: w.Y, W: w.W, H: w.H}
		d.NitroSeqDir = strings.ReplaceAll(strings.TrimSpace(w.Style), "\\", "/")
	}
	link := parseWidgets(readRel(p, "dashboard/etc/layout_link.ini"))
	d.LinkWidgets = link
	if w := link["n2_icon_3"]; w != nil {
		d.NitroLinkIcon = SceneRect{X: w.X, Y: w.Y, W: w.W, H: w.H}
	}
	if w := link["n2_left_warn_3"]; w != nil {
		d.NitroLinkSeq = SceneRect{X: w.X, Y: w.Y, W: w.W, H: w.H}
		if d.NitroSeqDir == "" {
			d.NitroSeqDir = strings.ReplaceAll(strings.TrimSpace(w.Style), "\\", "/")
		}
	}
	seqDir := d.NitroSeqDir
	if seqDir == "" {
		seqDir = "dashboard/style/regular/Nitrogen/seq/left"
	}
	d.NitroFrames = countPNGs(p, seqDir)
}

func countPNGs(p *ThemeProject, rel string) int {
	entries, err := os.ReadDir(p.Abs(rel))
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".png") {
			n++
		}
	}
	return n
}

func loadLightPaths(p *ThemeProject, d *SceneData, lights string) {
	loadPageLightPaths(p, iniSection(lights, "page_0"), d.Page0Lamps, d.Page0Signal,
		&d.LampDir, &d.SignalDir, &d.LampPaths, &d.SignalPaths)
	loadPageLightPaths(p, iniSection(lights, "page_3"), d.Page3Lamps, d.Page3Signal,
		&d.LinkLampDir, &d.LinkSignalDir, &d.LinkLampPaths, &d.LinkSignalPaths)
	d.GPSDir = strings.TrimRight(lightDirField(iniSection(lights, "page_0"), "signalDir"), "/") + "/GPS"
	if st, err := os.Stat(p.Abs(d.GPSDir)); err != nil || !st.IsDir() {
		d.GPSDir = strings.TrimRight(d.LampDir, "/") + "/GPS"
	}
	d.DashSetIcons = loadDashSetIcons(p, d)
}

func loadDashSetIcons(p *ThemeProject, d *SceneData) []menuIcon {
	var out []menuIcon
	for _, m := range d.MenuItems {
		out = append(out, menuIcon{Title: m.Title, Normal: m.N, Zh: m.Zh, En: m.En})
	}
	return out
}

func loadPageLightPaths(p *ThemeProject, sec string, lamps, signals []lampEntry,
	lampDir, signalDir *string, lampPaths, signalPaths *map[int]string) {
	state := lightDirField(sec, "stateDir")
	lamp := lightDirField(sec, "signalDir")
	if state == "" {
		state = "dashboard/style/signals"
	}
	if lamp == "" {
		lamp = "dashboard/style/lamps"
	}
	*signalDir = resolveIconDir(p, state)
	*lampDir = resolveIconDir(p, lamp)
	if *lampPaths == nil {
		*lampPaths = map[int]string{}
	}
	if *signalPaths == nil {
		*signalPaths = map[int]string{}
	}
	for _, l := range lamps {
		(*lampPaths)[l.ID] = visibleIcon(p, *lampDir, l.ID)
	}
	for _, s := range signals {
		(*signalPaths)[s.ID] = visibleIcon(p, *signalDir, s.ID)
	}
}
func lightDirField(section, key string) string {
	re := regexp.MustCompile(`(?m)^\s*` + key + `\s*=\s*"([^"]*)"`)
	if m := re.FindStringSubmatch(section); m != nil {
		return strings.TrimSpace(strings.ReplaceAll(m[1], "\\", "/"))
	}
	return ""
}

func resolveIconDir(p *ThemeProject, base string) string {
	base = strings.TrimRight(strings.TrimSpace(strings.ReplaceAll(base, "\\", "/")), "/")
	for _, sub := range []string{"nightime", "daytime"} {
		if st, err := os.Stat(p.Abs(base + "/" + sub)); err == nil && st.IsDir() {
			return base + "/" + sub
		}
	}
	return base
}

// visibleIcon finds {id}.png and its variants (1).png, (2).png... and picks the
// first one that has any opaque pixel. Some themes use id.png as an empty "off"
// frame and id(1).png as the visible icon.
func visibleIcon(p *ThemeProject, dir string, id int) string {
	entries, err := os.ReadDir(p.Abs(dir))
	if err != nil {
		return ""
	}
	prefix := strconv.Itoa(id)
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".png") {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := strings.TrimPrefix(name, prefix)
		if rest != "" && rest[0] != '.' && rest[0] != '(' {
			continue
		}
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == prefix+".png" {
			return names[j] != prefix+".png"
		}
		if names[j] == prefix+".png" {
			return false
		}
		return names[i] < names[j]
	})
	for _, name := range names {
		rel := dir + "/" + name
		if pngHasContent(p.Abs(rel)) {
			return rel
		}
	}
	if len(names) > 0 {
		return dir + "/" + names[0]
	}
	return ""
}

func pngHasContent(file string) bool {
	f, err := os.Open(file)
	if err != nil {
		return false
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return false
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a > 0 {
				return true
			}
		}
	}
	return false
}

func loadTempData(p *ThemeProject, d *SceneData) {
	if p == nil || d == nil {
		return
	}
	d.EcuIcon = widgetRect(p, "btn_ecu_icon")
	d.EcuTemp = widgetRect(p, "btn_ecu_temp")
	d.EcuUnit = widgetRect(p, "btn_ecu_unit")
	d.MotorIcon = widgetRect(p, "btn_motor_icon")
	d.MotorTemp = widgetRect(p, "btn_motor_temp")
	d.MotorUnit = widgetRect(p, "btn_motor_unit")
}

func widgetRect(p *ThemeProject, name string) SceneRect {
	w := p.Widget(name)
	if w == nil {
		return SceneRect{}
	}
	return SceneRect{X: w.X, Y: w.Y, W: w.W, H: w.H}
}

// widgetRectByStyle finds an anonymous layout block such as the POWER title.
func widgetRectByStyle(p *ThemeProject, style string) SceneRect {
	text := readRel(p, "dashboard/etc/layout.ini")
	for i := 0; i < len(text); i++ {
		if text[i] != '{' {
			continue
		}
		closeAt := matchingBrace(text, i)
		if closeAt < 0 {
			continue
		}
		body := menuHead(text[i+1 : closeAt])
		sm := styleRe.FindStringSubmatch(body)
		if sm == nil || sm[1] != style {
			continue
		}
		if m := xywhRe.FindStringSubmatch(body); m != nil {
			return SceneRect{X: atoi(m[1]), Y: atoi(m[2]), W: atoi(m[3]), H: atoi(m[4])}
		}
		x, okX := intField(body, "x")
		y, okY := intField(body, "y")
		w, okW := intField(body, "w")
		h, okH := intField(body, "h")
		if okX && okY && okW && okH {
			return SceneRect{X: x, Y: y, W: w, H: h}
		}
	}
	return SceneRect{}
}

func loadHudData(p *ThemeProject, d *SceneData) {
	if p == nil || d == nil {
		return
	}
	d.RideData = widgetRect(p, "lv_ridedata")
	d.WeatherBG = widgetRect(p, "weather_bg")
	d.WeatherText = widgetRect(p, "btn_weather_text")
	d.AirTemp = widgetRect(p, "btn_airtemp")
	d.TempUnit = widgetRect(p, "btn_tempunit")
	d.DateText = widgetRect(p, "btn_date")
	d.WeekText = widgetRect(p, "btn_week_text")
	d.CompassIcon = widgetRect(p, "btn_compass_icon")
	d.CompassText = widgetRect(p, "btn_compass_text")
	d.CompassValue = widgetRect(p, "btn_compass_value")
	d.CompassUnit = widgetRect(p, "btn_compass_unit")
	d.AltitudeText = widgetRect(p, "btn_altitude_text")
	d.AltitudeValue = widgetRect(p, "btn_altitude_value")
	d.AltitudeUnit = widgetRect(p, "btn_altitude_unit")
	d.PowerTitle = widgetRect(p, "power_title")
	if d.PowerTitle.W <= 0 {
		d.PowerTitle = widgetRectByStyle(p, "power_title")
	}
	d.PowerNum = widgetRect(p, "power_num")
	d.PowerUnit = widgetRect(p, "power_unit")
	d.ChargeWin = widgetRect(p, "charge_win")
	d.ChargeTitle = widgetRect(p, "btn_charge_title")
	d.ChargeInfo = widgetRect(p, "btn_charge_info")
	d.ChargeHint = widgetRect(p, "btn_charge_hint")
	d.ChargePower = widgetRect(p, "btn_power")
	d.ChargeVoltage = widgetRect(p, "btn_voltage")
	// cluster_weather_x/y/w/h are not widgets; they live in config.ini.
	cfg := parseThemeConfig([]byte(readRel(p, "dashboard/etc/config.ini")))
	x, y := atoi(cfg.Get("cluster_weather_x")), atoi(cfg.Get("cluster_weather_y"))
	w, h := atoi(cfg.Get("cluster_weather_w")), atoi(cfg.Get("cluster_weather_h"))
	if w <= 0 || h <= 0 {
		x, y, w, h = 926, 103, 82, 82
	}
	d.WeatherIcon = SceneRect{X: x, Y: y, W: w, H: h}
}

func parseMenuHome(text string) []menuEntry {
	section := iniSection(text, "home")
	if section == "" {
		return nil
	}
	var out []menuEntry
	var cur *menuEntry
	flush := func() {
		if cur != nil && cur.Title != "" {
			out = append(out, *cur)
		}
		cur = nil
	}
	for _, line := range strings.Split(section, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "title=") {
			flush()
			cur = &menuEntry{Title: unquoteINI(strings.TrimPrefix(t, "title="))}
			continue
		}
		if cur == nil {
			continue
		}
		for _, m := range menuFieldRe.FindAllStringSubmatch(t, -1) {
			switch m[1] {
			case "n":
				cur.N = m[2]
			case "en_US":
				cur.En = m[2]
			case "zh_CN":
				cur.Zh = m[2]
			}
		}
	}
	flush()
	return out
}

func parseTranslations(text string) map[string]string {
	out := map[string]string{}
	for _, m := range transLineRe.FindAllStringSubmatch(text, -1) {
		out[m[1]] = strings.ReplaceAll(m[2], `\"`, `"`)
	}
	return out
}

func parseSectionTitles(text, section string) []string {
	part := iniSection(text, section)
	var out []string
	for _, m := range menuHomeTitleRe.FindAllStringSubmatch(part, -1) {
		out = append(out, m[1])
	}
	return out
}

func loadBotData(p *ThemeProject) *botData {
	d := &botData{Normal: map[int]botRule{}, Msg: map[int]botRule{}}
	sec := iniSection(readRel(p, "bot/etc/bot_rules.ini"), "bot_info")
	normalAt, msgAt := -1, -1
	if loc := regexp.MustCompile(`(?m)^\s*normal\s*=\s*\{`).FindStringIndex(sec); loc != nil {
		normalAt = loc[0]
	}
	if loc := regexp.MustCompile(`(?m)^\s*msg\s*=\s*\{`).FindStringIndex(sec); loc != nil {
		msgAt = loc[0]
	}
	pageRe := regexp.MustCompile(`(?s)page_(\d+)\s*=\s*\{(.*?)\}`)
	for _, m := range pageRe.FindAllStringSubmatchIndex(sec, -1) {
		page := atoi(sec[m[2]:m[3]])
		rule, ok := parseBotRule(sec[m[4]:m[5]])
		if !ok {
			continue
		}
		if normalAt >= 0 && m[0] >= normalAt && (msgAt < 0 || m[0] < msgAt) {
			d.Normal[page] = rule
		}
		if msgAt >= 0 && m[0] >= msgAt {
			d.Msg[page] = rule
		}
	}
	if len(d.Normal) == 0 {
		d.Normal[0] = botRule{202, 510, 116, 90, "bot/style/seq_pic/normal"}
		d.Normal[1] = botRule{202, 510, 116, 90, "bot/style/seq_pic/menu"}
		d.Normal[3] = botRule{892, 448, 116, 90, "bot/style/seq_pic/normal"}
	}
	if len(d.Msg) == 0 {
		for page, rule := range d.Normal {
			rule.Path = "bot/style/seq_pic/msg"
			d.Msg[page] = rule
		}
	}
	d.Has = p.Has("bot/style/seq_pic/normal/0.png") || len(d.Normal) > 0
	return d
}

func parseBotRule(block string) (botRule, bool) {
	m := xywhRe.FindStringSubmatch(block)
	if m == nil {
		return botRule{}, false
	}
	r := botRule{X: atoi(m[1]), Y: atoi(m[2]), W: atoi(m[3]), H: atoi(m[4])}
	if r.W == 0 {
		r.W = 116
	}
	if r.H == 0 {
		r.H = 90
	}
	pm := regexp.MustCompile(`pic_path\s*=\s*"([^"]+)"`).FindStringSubmatch(block)
	if pm == nil {
		return botRule{}, false
	}
	path := strings.TrimPrefix(pm[1], "/")
	if !strings.HasPrefix(path, "bot/") {
		path = "bot/style/" + path
	}
	r.Path = path
	return r, true
}

func parseLightPage(text, page string) (lamps, signals []lampEntry) {
	section := iniSection(text, page)
	if section == "" {
		return nil, nil
	}
	sigAt := -1
	if loc := regexp.MustCompile(`signals\s*=\s*\{`).FindStringIndex(section); loc != nil {
		sigAt = loc[0]
	}
	locs := lightIDRe.FindAllStringSubmatchIndex(section, -1)
	for i, loc := range locs {
		end := len(section)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		chunk := section[loc[0]:end]
		m := lightXYRe.FindStringSubmatch(chunk)
		if m == nil {
			continue
		}
		e := lampEntry{ID: atoi(section[loc[2]:loc[3]]), X: atoi(m[1]), Y: atoi(m[2])}
		e.Check = regexp.MustCompile(`if_check\s*=\s*1`).MatchString(chunk)
		if sigAt >= 0 && loc[0] > sigAt {
			signals = append(signals, e)
		} else {
			lamps = append(lamps, e)
		}
	}
	return lamps, signals
}

// ---- Scene timing ----------------------------------------------------------

func (a *app) prepareSceneRender() {
	if !a.sceneAnim {
		return
	}
	ph := a.scenePhase
	switch a.scene {
	case sceneDash, sceneLights, sceneMenu, sceneMenuSub, sceneLink:
		speed := 52 + 26*math.Sin(ph*0.55) + 8*math.Sin(ph*1.9)
		a.renderSpeed = clampInt(int(math.Round(speed)), 0, 299)
		a.renderSOC = clampInt(a.soc+int(3*math.Sin(ph*0.21)), 0, 100)
		gears := []string{"3", "3", "2", "1", "0", "P"}
		a.renderGear = gears[int(ph/2.7)%len(gears)]
		a.renderClock = time.Now().Format("1504")
		a.renderEcuTemp = clampInt(a.ecuTemp+int(math.Round(2*math.Sin(ph*0.13))), 0, 150)
		a.renderMotorTemp = clampInt(a.motorTemp+int(math.Round(3*math.Sin(ph*0.09+1.1))), 0, 180)
		a.renderPower = math.Max(0, a.powerKW+0.35*math.Sin(ph*0.37))
	}
}

func smoothstep(t float64) float64 {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t * t * (3 - 2*t)
}

// ---- Draw helpers ----------------------------------------------------------

func (a *app) drawBitmapAt(cp *ui.Painter, rel string, r ui.Rect, fit ui.Fit) bool {
	b := a.bitmap(rel)
	if b == nil {
		return false
	}
	cp.Image(b, r, fit)
	return true
}

func (a *app) drawBitmapAlpha(cp *ui.Painter, rel string, r ui.Rect, alpha float32) {
	b := a.bitmap(rel)
	if b == nil {
		return
	}
	w, h := b.Size()
	tint := ui.RGBA(255, 255, 255, alpha)
	cp.ImageRegion(b, ui.Rect{W: float32(w), H: float32(h)}, r, ui.ImageOptions{Tint: tint})
}

func (a *app) drawClockFolder(cp *ui.Painter, folder string, box ui.Rect, text string) {
	var digits []rune
	for _, r := range text {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	for len(digits) < 4 {
		digits = append(digits, '0')
	}
	if len(digits) > 4 {
		digits = digits[:4]
	}
	slots := []rune{digits[0], digits[1], ':', digits[2], digits[3]}
	slotW := box.W / float32(len(slots))
	col := a.assetColor(folder + "/0.png")
	for i, ch := range slots {
		sx := box.X + float32(i)*slotW
		if ch == ':' {
			dot := float32(2.2)
			cx := sx + slotW/2
			r := ui.Rect{X: cx - dot, Y: box.Y + box.H*0.30, W: dot * 2, H: dot * 2}
			cp.Fill(r, col, dot)
			r.Y = box.Y + box.H*0.64
			cp.Fill(r, col, dot)
			continue
		}
		b := a.bitmap(folder + "/" + string(ch) + ".png")
		if b == nil {
			continue
		}
		bw, bh := b.Size()
		ih := box.H
		iw := float32(bw) * ih / float32(maxInt(1, bh))
		cp.Image(b, ui.Rect{X: sx + (slotW-iw)/2, Y: box.Y, W: iw, H: ih}, ui.FillBox)
	}
}

func (a *app) tr(key string) string {
	if a.proj == nil || a.proj.Scene == nil {
		return key
	}
	var m map[string]string
	if a.menuLang == "en_US" {
		m = a.proj.Scene.En
	} else {
		m = a.proj.Scene.Zh
	}
	if v := m[key]; v != "" {
		return v
	}
	return key
}

// ---- Bot ------------------------------------------------------------------

func (a *app) drawBotAt(cp *ui.Painter, page int, state string, animate bool) {
	if a.proj == nil || a.proj.Scene == nil || a.proj.Scene.Bot == nil {
		return
	}
	d := a.proj.Scene.Bot
	var rule botRule
	var ok bool
	if state == "msg" {
		rule, ok = d.Msg[page]
	} else {
		rule, ok = d.Normal[page]
	}
	if !ok {
		return
	}
	if rule.W <= 0 || rule.H <= 0 {
		rule.W, rule.H = 116, 90
	}
	frame := 0
	if animate && a.sceneAnim {
		frame = int(a.scenePhase*30) % 50
	} else if state == "init" {
		frame = 25
	}
	a.drawBitmapAt(cp, fmt.Sprintf("%s/%d.png", rule.Path, frame),
		ui.Rect{X: float32(rule.X), Y: float32(rule.Y), W: float32(rule.W), H: float32(rule.H)}, ui.FillBox)
}

func (a *app) drawBotScene(cp *ui.Painter) {
	cp.FillGradient(ui.Rect{W: themeW, H: themeH}, ui.LinearGradient{
		From: ui.Hex("#1b1010"), To: ui.Hex("#050506"), Angle: 145,
	}, 0)
	// A quiet stage for the 116×90 sequences.
	for x := float32(16); x < themeW; x += 32 {
		for y := float32(16); y < themeH; y += 32 {
			cp.Fill(ui.Rect{X: x, Y: y, W: 1, H: 1}, ui.RGBA(255, 255, 255, 0.055), 0)
		}
	}
	cp.Fill(ui.Rect{X: 280, Y: 130, W: 464, H: 340}, ui.RGBA(255, 255, 255, 0.035), 24)
	state := a.botState
	switch state {
	case "init", "normal", "menu", "msg":
	default:
		state = "normal"
	}
	frame := 0
	if a.sceneAnim {
		frame = int(a.scenePhase*30) % 50
	} else if state == "init" {
		frame = 25
	}
	r := ui.Rect{X: 326, Y: 176, W: 372, H: 289}
	if !a.drawBitmapAt(cp, fmt.Sprintf("bot/style/seq_pic/%s/%d.png", state, frame), r, ui.Contain) {
		cp.Text(400, 300, "bot 序列帧缺失", 18, ui.Hex("#ffffff"))
	}

	bubbleText := map[string]string{
		"init":   "正在启动…",
		"normal": "我在，随时待命。",
		"menu":   "菜单已打开，需要什么？",
		"msg":    "收到一条新消息。",
	}[state]
	if a.menuLang == "en_US" {
		bubbleText = map[string]string{
			"init":   "Booting…",
			"normal": "Ready when you are.",
			"menu":   "The menu is open.",
			"msg":    "You have a new message.",
		}[state]
	}
	bubble := ui.Rect{X: 250, Y: 72, W: 524, H: 72}
	cp.Fill(bubble, ui.RGBA(32, 18, 18, 0.94), 16)
	cp.Stroke(bubble, ui.RGBA(255, 160, 120, 0.35), 16, 1)
	cp.Text(bubble.X+20, bubble.Y+14, "BOT / 台铃助手", 11, ui.Hex("#ffb98f"))
	cp.Text(bubble.X+20, bubble.Y+38, bubbleText, 18, ui.Hex("#fff1e6"))

	labels := map[string]string{
		"init": "INIT / 初始化", "normal": "NORMAL / 待机",
		"menu": "MENU / 菜单", "msg": "MSG / 消息",
	}
	cp.Text(326, 494, labels[state], 12, ui.Hex("#ffcf9b"))
	bar := ui.Rect{X: 326, Y: 522, W: 372, H: 4}
	cp.Fill(bar, ui.RGBA(255, 255, 255, 0.1), 2)
	cp.Fill(ui.Rect{X: bar.X, Y: bar.Y, W: bar.W * float32(frame+1) / 50, H: bar.H}, ui.Hex("#ff7a55"), 2)
	cp.Text(326, 536, "50 帧 · 33ms/帧 · 116×90", 10, ui.RGBA(255, 255, 255, 0.45))
}

// ---- Scenes ----------------------------------------------------------------

func (a *app) drawMenuScene(cp *ui.Painter) {
	a.drawDashboardScene(cp)
	cp.Fill(ui.Rect{W: themeW, H: themeH}, ui.RGBA(0, 0, 0, 0.55), 0)
	d := a.proj.Scene
	if d == nil || len(d.MenuItems) == 0 {
		cp.Text(40, 60, "菜单资源缺失", 16, ui.Hex("#ffffff"))
		return
	}
	dock := ui.Rect{X: float32(d.MenuDock.X), Y: float32(d.MenuDock.Y),
		W: float32(d.MenuDock.W), H: float32(d.MenuDock.H)}
	a.drawBitmapAt(cp, "menu/style/Night/bg/home_slide.png", dock, ui.FillBox)
	cp.Clip(dock, 41, func() {
		n := len(d.MenuItems)
		t := a.scenePhase / 1.7
		if !a.sceneAnim {
			t = 2
		}
		sel := int(t) % n
		frac := t - math.Floor(t)
		from := (sel - 1 + n) % n
		fromOff := float64(menuOffset(from, n, d))
		toOff := float64(menuOffset(sel, n, d))
		offset := fromOff + (toOff-fromOff)*smoothstep(frac)
		for i, item := range d.MenuItems {
			y := dock.Y + 3 + float32(i*d.CardH) - float32(offset)
			if y > dock.Y+dock.H || y+float32(d.CardH) < dock.Y {
				continue
			}
			r := ui.Rect{X: dock.X + 7, Y: y, W: float32(d.CardW), H: float32(d.CardH)}
			rel := item.image(a.menuLang)
			if !a.drawBitmapAt(cp, rel, r, ui.FillBox) {
				if !a.drawBitmapAt(cp, item.N, ui.Rect{X: r.X + 14, Y: r.Y + 14, W: 40, H: 40}, ui.Contain) {
					cp.Fill(r, ui.Hex("#5a1b1b"), 8)
				}
				cp.Text(r.X+6, r.Y+62, a.tr(item.Title), 11, ui.Hex("#ffffff"))
			}
			if i == sel {
				cp.Stroke(ui.Rect{X: r.X - 2, Y: r.Y - 2, W: r.W + 4, H: r.H + 4}, ui.Hex("#ffcf9b"), 8, 2)
			}
		}
	})
	a.drawBotAt(cp, 1, "menu", a.sceneAnim)
}

func menuOffset(i, n int, d *SceneData) int {
	if n <= 3 {
		return 0
	}
	off := i*d.CardH - d.CardH
	max := (n - 3) * d.CardH
	if off < 0 {
		return 0
	}
	if off > max {
		return max
	}
	return off
}

func (a *app) drawMenuSubScene(cp *ui.Painter) {
	a.drawDashboardScene(cp)
	cp.Fill(ui.Rect{W: themeW, H: themeH}, ui.RGBA(0, 0, 0, 0.72), 0)
	d := a.proj.Scene
	options := []string{"TCS", "倒车/助力推能", "定速巡航", "陡坡缓降", "防溜坡", "软硬启动", "氮气加速", "能量回收", "坡道驻车", "永久云动力", "边撑感应功能", "坐垫感应功能", "自动下电功能"}
	if d != nil && len(d.CarSet) > 0 {
		options = options[:0]
		for _, k := range d.CarSet {
			options = append(options, a.tr(k))
		}
	}
	panel := ui.Rect{X: 590, Y: 118, W: 316, H: 376}
	cp.Fill(panel, ui.RGBA(22, 10, 10, 0.96), 12)
	cp.Stroke(panel, ui.RGBA(120, 48, 38, 0.6), 12, 1)
	cp.Text(panel.X+16, panel.Y+14, a.tr("CarSet"), 17, ui.Hex("#fff0dc"))
	cp.Line(panel.X+16, panel.Y+42, panel.X+panel.W-16, panel.Y+42, 1, ui.RGBA(255, 255, 255, 0.12))
	sel := 0
	if len(options) > 0 {
		t := a.scenePhase / 1.4
		if !a.sceneAnim {
			t = 4
		}
		sel = int(t) % len(options)
	}
	visible := 7
	off := sel - 2
	if off < 0 {
		off = 0
	}
	if off > len(options)-visible {
		off = len(options) - visible
	}
	if off < 0 {
		off = 0
	}
	for i := 0; i < visible && i+off < len(options); i++ {
		idx := i + off
		y := panel.Y + 52 + float32(i)*44
		row := ui.Rect{X: panel.X + 8, Y: y, W: 300, H: 40}
		if idx == sel {
			a.drawBitmapAt(cp, "menu/style/Night/list/second_select.png", row, ui.FillBox)
		}
		col := ui.Hex("#ffffff")
		if idx != sel {
			col = ui.RGBA(255, 255, 255, 0.62)
		}
		cp.Text(row.X+14, y+12, options[idx], 13, col)
		if idx == sel {
			cp.Text(row.X+row.W-48, y+12, ">", 16, ui.Hex("#ffffff"))
		}
	}
	cp.Text(panel.X+16, panel.Y+panel.H-24, "长按确认 / 短按切换", 10, ui.RGBA(255, 255, 255, 0.4))
	a.drawBotAt(cp, 1, "menu", a.sceneAnim)
}

func (a *app) drawChargeScene(cp *ui.Painter) {
	a.drawBitmapAt(cp, "menu/style/charge/bg.png", ui.Rect{W: themeW, H: themeH}, ui.FillBox)
	pct := a.soc
	if a.sceneAnim {
		pct = int(a.scenePhase*7.5) % 101
	}
	a.drawDigitsRight(cp, "menu/style/charge/num", strconv.Itoa(pct),
		ui.Rect{X: 300, Y: 105, W: 282, H: 116}, 0)
	frame := 24
	if a.sceneAnim {
		frame = int(a.scenePhase*10) % 48
	}
	a.drawBitmapAt(cp, fmt.Sprintf("menu/style/charge/battery/%d.png", frame),
		ui.Rect{X: 302, Y: 455, W: 247, H: 50}, ui.FillBox)
	a.drawBitmapAt(cp, fmt.Sprintf("menu/style/charge/line/%d.png", frame),
		ui.Rect{X: 636, Y: 336, W: 229, H: 199}, ui.FillBox)
	a.drawBitmapAt(cp, fmt.Sprintf("menu/style/charge/icon/%d.png", frame),
		ui.Rect{X: 715, Y: 212, W: 124, H: 124}, ui.FillBox)
	a.drawBitmapAt(cp, "menu/style/charge/icon.png",
		ui.Rect{X: 679, Y: 100, W: 41, H: 41}, ui.FillBox)
	label := "充电中"
	if a.menuLang == "en_US" {
		label = "Charging"
	}
	cp.Text(730, 104, label, 25, ui.Hex("#ffffff"))
}

func (a *app) drawLockScene(cp *ui.Painter) {
	a.drawBitmapAt(cp, "tdSuspendMgr/style/lock/bg.png", ui.Rect{W: themeW, H: themeH}, ui.FillBox)
	d := a.proj.Scene
	if d == nil || len(d.LockSlots) < 5 {
		cp.Text(380, 280, "锁屏布局缺失", 16, ui.Hex("#ffffff"))
		return
	}
	step := int(a.scenePhase / 1.1)
	if !a.sceneAnim {
		step = 3
	}
	cycled := step % 7
	filled := cycled
	if filled > 5 {
		filled = 5
	}
	cursor := filled
	if cursor > 4 {
		cursor = 4
	}
	code := "12345"
	if filled < 5 {
		slot := d.LockSlots[cursor]
		selX := float32(slot.X) + float32(slot.W-d.LockSelect.W)/2
		sr := ui.Rect{X: selX, Y: float32(d.LockSelect.Y),
			W: float32(d.LockSelect.W), H: float32(d.LockSelect.H)}
		a.drawBitmapAt(cp, "tdSuspendMgr/style/lock/select.png", sr, ui.FillBox)
	}
	for i := 0; i < 5; i++ {
		if i >= filled {
			break
		}
		slot := d.LockSlots[i]
		rel := fmt.Sprintf("tdSuspendMgr/style/lock/number/%c.png", code[i])
		if i == cursor && filled < 5 {
			rel = fmt.Sprintf("tdSuspendMgr/style/lock/select_num/%c.png", code[i])
		}
		a.drawBitmapAt(cp, rel, ui.Rect{X: float32(slot.X), Y: float32(slot.Y),
			W: float32(slot.W), H: float32(slot.H)}, ui.FillBox)
	}
	title := "请输入密码"
	if filled == 5 {
		title = "验证中…"
	}
	if a.menuLang == "en_US" {
		title = "Enter password"
		if filled == 5 {
			title = "Checking…"
		}
	}
	cp.Text(430, 154, title, 18, ui.RGBA(255, 255, 255, 0.7))
}

func (a *app) drawLinkScene(cp *ui.Painter) {
	a.drawBitmapAt(cp, "dashboard/style/link/bg.png", ui.Rect{W: themeW, H: themeH}, ui.FillBox)
	area := ui.Rect{X: 48, Y: 64, W: 928, H: 418}
	cp.Fill(area, ui.RGBA(8, 20, 34, 0.62), 14)
	cp.Stroke(area, ui.RGBA(90, 150, 210, 0.16), 14, 1)
	for x := float32(64); x < area.X+area.W; x += 64 {
		cp.Line(x, area.Y+8, x, area.Y+area.H-8, 1, ui.RGBA(120, 170, 220, 0.09))
	}
	for y := float32(80); y < area.Y+area.H; y += 64 {
		cp.Line(area.X+8, y, area.X+area.W-8, y, 1, ui.RGBA(120, 170, 220, 0.09))
	}
	cp.Text(area.X+24, area.Y+20, "手机导航投屏区域 · 1024×600", 14, ui.Hex("#bcd8ff"))
	cp.Text(area.X+24, area.Y+42, "实际画面由手机端绘制，这里只预览车机主题边框", 10, ui.Hex("#6f8cab"))
	// A route placeholder drawn with plain lines, which the canvas always
	// rasterizes the same way as the grid above.
	pts := [][2]float32{
		{area.X + 120, area.Y + 330},
		{area.X + 300, area.Y + 240},
		{area.X + 470, area.Y + 280},
		{area.X + 650, area.Y + 150},
		{area.X + 810, area.Y + 190},
	}
	cp.StrokePolyline(pts, 6, ui.Hex("#46c8ff"))
	for i, pt := range pts {
		sz := float32(8)
		if i == 0 || i == len(pts)-1 {
			sz = 12
		}
		cp.Fill(ui.Rect{X: pt[0] - sz/2, Y: pt[1] - sz/2, W: sz, H: sz}, ui.Hex("#9beaff"), sz/2)
	}
	dotX := pts[1][0] + float32(math.Mod(a.scenePhase*90, 300))
	cp.Fill(ui.Rect{X: dotX, Y: pts[1][1] - 6, W: 12, H: 12}, ui.Hex("#ffffff"), 6)

	a.drawClockFolder(cp, "dashboard/style/link/timepiece",
		ui.Rect{X: 472, Y: 20, W: 80, H: 27}, a.renderClock)
	a.drawDigitsCenter(cp, "dashboard/style/link/speedmeter", strconv.Itoa(a.renderSpeed),
		ui.Rect{X: 458, Y: 532, W: 108, H: 51}, 0)
	// SOC bar and number.
	box := ui.Rect{X: 617, Y: 560, W: 70, H: 18}
	a.drawBitmapAt(cp, "dashboard/style/link/soc/process/bg.png", box, ui.FillBox)
	pct := float32(a.renderSOC) / 100
	if b := a.bitmap("dashboard/style/link/soc/process/process.png"); b != nil && pct > 0 {
		clip := box
		clip.W *= pct
		cp.Clip(clip, 0, func() { cp.Image(b, box, ui.FillBox) })
	}
	a.drawDigitsRight(cp, "dashboard/style/link/soc/num", strconv.Itoa(a.renderSOC),
		ui.Rect{X: 687, Y: 555, W: 50, H: 27}, 0)
	a.drawBitmapAt(cp, "dashboard/style/link/soc/unit.png",
		ui.Rect{X: 740, Y: 566, W: 17, H: 16}, ui.FillBox)
	if a.renderGear != "P" {
		a.drawBitmapAt(cp, "dashboard/style/link/gearmeter/region_0/nightime/"+a.renderGear+".png",
			ui.Rect{X: 970, Y: 548, W: 34, H: 44}, ui.FillBox)
	}
	if a.linkLoading {
		cp.Fill(ui.Rect{W: themeW, H: themeH}, ui.Hex("#000000"), 0)
		frame := 0
		if a.sceneAnim {
			frame = int(a.scenePhase*8) % 6
		}
		a.drawBitmapAt(cp, fmt.Sprintf("menu/style/navi/screenLinkNavi/loadAnimation/%d.png", frame),
			ui.Rect{X: 477, Y: 190, W: 70, H: 70}, ui.FillBox)
		cp.Text(212, 280, "正在连接手机导航…", 26, ui.Hex("#ffffff"))
		cp.Text(212, 320, "请保持手机导航在前台", 13, ui.RGBA(255, 255, 255, 0.45))
		return
	}
	a.drawBotAt(cp, 3, "normal", a.sceneAnim)
	a.drawNitro(cp, a.proj.Scene.NitroLinkIcon, a.proj.Scene.NitroLinkSeq)
	d := a.proj.Scene
	for _, l := range d.Page3Lamps {
		if rel := d.LinkLampPaths[l.ID]; rel != "" {
			a.drawLamp(cp, rel, l, 0.95)
		}
	}
	for _, s := range d.Page3Signal {
		if rel := d.LinkSignalPaths[s.ID]; rel != "" {
			a.drawSignalRel(cp, rel, s, 0.95)
		}
	}
	if a.renderSOC <= 15 {
		if w := d.LinkWidgets["link_low_soc_icon"]; w != nil {
			rel := styleBG(a.proj, "button", "low_soc_icon")
			if rel != "" {
				a.drawBitmapAt(cp, rel, ui.Rect{X: float32(w.X), Y: float32(w.Y), W: float32(w.W), H: float32(w.H)}, ui.FillBox)
			}
		}
	}
	if a.hillHold {
		if w := d.LinkWidgets["link_speedstop"]; w != nil {
			a.drawBitmapAt(cp, "dashboard/style/link/speedmeter/H.png",
				ui.Rect{X: float32(w.X), Y: float32(w.Y), W: float32(w.W), H: float32(w.H)}, ui.FillBox)
		}
	}
	if a.hintID > 0 {
		lang := "zh"
		if a.menuLang == "en_US" {
			lang = "en"
		}
		a.drawBitmapAt(cp, fmt.Sprintf("dashboard/style/link/hint_msg/img_%s/HINT_ID_%d.png", lang, a.hintID),
			ui.Rect{X: 382, Y: 62, W: 260, H: 41}, ui.FillBox)
	}
	if a.renderGear == "P" {
		if w := d.LinkWidgets["link_gearP"]; w != nil {
			a.drawBitmapAt(cp, "dashboard/style/link/gearmeter/gear_p/P_0.png",
				ui.Rect{X: float32(w.X), Y: float32(w.Y), W: float32(w.W), H: float32(w.H)}, ui.FillBox)
		}
	}
}

func (a *app) drawLightsScene(cp *ui.Painter) {
	d := a.proj.Scene
	if d == nil {
		return
	}
	ph := a.scenePhase
	if !a.sceneAnim {
		// A paused frame shows the whole status set: 18 warning lamps,
		// all four signal icons and the GPS strength bars.
		for _, l := range d.Page0Lamps {
			if l.Check {
				a.drawLamp(cp, a.proj.Scene.LampPaths[l.ID], l, 0.95)
			}
		}
		for _, sig := range d.Page0Signal {
			a.drawSignal(cp, sig, 1, 0.95)
		}
		a.drawGPSSignal(cp, 4, 0.95)
		return
	}
	alphaFor := func(on bool) float32 {
		if on {
			return 0.95
		}
		return 0
	}
	if ph < 2.4 {
		// Self-check: all lamps and signals on, blinking quickly.
		on := int(ph*6)%2 == 0
		alpha := alphaFor(on || ph < 1.6)
		for _, l := range d.Page0Lamps {
			if l.Check {
				a.drawLamp(cp, a.proj.Scene.LampPaths[l.ID], l, alpha)
			}
		}
		for _, sig := range d.Page0Signal {
			a.drawSignal(cp, sig, 1, alpha)
		}
		a.drawGPSSignal(cp, 4, alpha)
	} else if ph >= 3.2 {
		groups := [][]int{
			{1, 3, 4}, {10, 21, 27}, {38, 39, 40, 41}, {43, 50, 52}, {70, 71, 73, 75, 104},
		}
		gi := int((ph - 3.2) / 1.3)
		group := groups[gi%len(groups)]
		on := int(ph*4)%2 == 0
		for _, l := range d.Page0Lamps {
			for _, id := range group {
				if l.ID == id {
					a.drawLamp(cp, a.proj.Scene.LampPaths[l.ID], l, alphaFor(on))
				}
			}
		}
	}
	// Phone / status signals stay visible; the two turn arrows alternate
	// and the GPS bars sweep 2 → 3 → 4.
	for _, sig := range d.Page0Signal {
		switch sig.ID {
		case 1:
			if int(ph*2)%2 == 0 {
				a.drawSignal(cp, sig, 1, 0.95)
			}
		case 2:
			if int(ph*2)%2 == 1 {
				a.drawSignal(cp, sig, 1, 0.95)
			}
		default:
			a.drawSignal(cp, sig, 1, 0.9)
		}
	}
	a.drawGPSSignal(cp, 2+int(ph*2)%3, 0.95)
}

func (a *app) drawLamp(cp *ui.Painter, rel string, l lampEntry, alpha float32) {
	if alpha <= 0 {
		return
	}
	if rel == "" && a.proj != nil && a.proj.Scene != nil {
		rel = fmt.Sprintf("%s/%d.png", a.proj.Scene.LampDir, l.ID)
	}
	b := a.bitmap(rel)
	if b == nil {
		return
	}
	w, h := b.Size()
	a.drawBitmapAlpha(cp, rel, ui.Rect{X: float32(l.X), Y: float32(l.Y), W: float32(w), H: float32(h)}, alpha)
}

func (a *app) drawSignal(cp *ui.Painter, s lampEntry, variant int, alpha float32) {
	rel := ""
	if variant != 0 && a.proj != nil && a.proj.Scene != nil {
		rel = a.proj.Scene.SignalPaths[s.ID]
	}
	if rel == "" && a.proj != nil && a.proj.Scene != nil {
		rel = fmt.Sprintf("%s/%d.png", a.proj.Scene.SignalDir, s.ID)
	}
	a.drawSignalRel(cp, rel, s, alpha)
}

func (a *app) drawSignalRel(cp *ui.Painter, rel string, s lampEntry, alpha float32) {
	b := a.bitmap(rel)
	if b == nil {
		return
	}
	w, h := b.Size()
	a.drawBitmapAlpha(cp, rel, ui.Rect{X: float32(s.X), Y: float32(s.Y), W: float32(w), H: float32(h)}, alpha)
}

func (a *app) drawGPSSignal(cp *ui.Painter, strength int, alpha float32) {
	if strength < 2 {
		strength = 2
	}
	if strength > 4 {
		strength = 4
	}
	r := ui.Rect{X: 210, Y: 13, W: 45, H: 45}
	if w := a.proj.Widget("btn_gps_signal"); w != nil {
		r = ui.Rect{X: float32(w.X), Y: float32(w.Y), W: float32(w.W), H: float32(w.H)}
	}
	dir := "dashboard/style/lamps/GPS"
	if a.proj != nil && a.proj.Scene != nil && a.proj.Scene.GPSDir != "" {
		dir = a.proj.Scene.GPSDir
	}
	a.drawBitmapAlpha(cp, fmt.Sprintf("%s/signal_%d.png", dir, strength), r, alpha)
}

func (a *app) drawSwitchScene(cp *ui.Painter) {
	cp.FillGradient(ui.Rect{W: themeW, H: themeH}, ui.LinearGradient{
		From: ui.Hex("#150b0b"), To: ui.Hex("#040404"), Angle: 135,
	}, 0)
	frame := 8
	if a.sceneAnim {
		frame = int(a.scenePhase*10) % 48
	}
	r := ui.Rect{X: 312, Y: 225, W: 400, H: 150}
	cp.Fill(ui.Rect{X: r.X - 10, Y: r.Y - 10, W: r.W + 20, H: r.H + 20}, ui.RGBA(255, 255, 255, 0.04), 12)
	if !a.drawBitmapAt(cp, fmt.Sprintf("switch_theme/style/theme/%d.png", frame), r, ui.FillBox) {
		cp.Text(380, 290, "switch_theme 序列帧缺失", 16, ui.Hex("#ffffff"))
	}
	bar := ui.Rect{X: 312, Y: 400, W: 400, H: 3}
	cp.Fill(bar, ui.RGBA(255, 255, 255, 0.1), 2)
	cp.Fill(ui.Rect{X: bar.X, Y: bar.Y, W: bar.W * float32(frame+1) / 48, H: bar.H}, ui.Hex("#ff5548"), 2)
	cp.Text(312, 412, "主题 UI 换肤 · 48 帧 / 4 秒", 10, ui.RGBA(255, 255, 255, 0.45))
}

func (a *app) drawCountdownScene(cp *ui.Painter) {
	a.drawBitmapAt(cp, "menu/style/count_down/bg.png", ui.Rect{W: themeW, H: themeH}, ui.FillBox)
	n := 3 - int(a.scenePhase)%3
	if !a.sceneAnim {
		n = 2
	}
	a.drawDigitsCenter(cp, "menu/style/count_down/num", strconv.Itoa(n),
		ui.Rect{X: 456, Y: 180, W: 112, H: 174}, 0)
	label := "车辆即将下电"
	sub := "长按电源键取消"
	if a.menuLang == "en_US" {
		label, sub = "Power off soon", "Hold the power key to cancel"
	}
	cp.Text(412, 380, label, 20, ui.Hex("#ffffff"))
	cp.Text(430, 414, sub, 12, ui.RGBA(255, 255, 255, 0.45))
	frac := float32(math.Mod(a.scenePhase, 1))
	bar := ui.Rect{X: 400, Y: 450, W: 224, H: 4}
	cp.Fill(bar, ui.RGBA(255, 255, 255, 0.1), 2)
	cp.Fill(ui.Rect{X: bar.X, Y: bar.Y, W: bar.W * (1 - frac), H: bar.H}, ui.Hex("#ff5548"), 2)
}
