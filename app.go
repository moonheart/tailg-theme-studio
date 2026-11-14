package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type pageKind int

const (
	pageDashboard pageKind = iota
	pageScenes
	pageAssets
	pageInfo
	pageExport
)

type layoutEdit struct {
	Name       string
	OldX, OldY int
	NewX, NewY int
}

type prefs struct {
	Recent   []string `json:"recent"`
	Zoom     float64  `json:"zoom"`
	Mode     int      `json:"mode"`
	ShowGrid bool     `json:"showGrid"`
}

type app struct {
	win *mygo.Window

	proj     *ThemeProject
	page     pageKind
	loading  bool
	busy     bool
	errText  string
	status   string
	recent   []string
	startup  string
	settings prefs

	// Preview.
	canvas          *ui.Canvas
	previewVer      int
	drawnVer        int
	mode            int
	speed           int
	soc             int
	gear            string
	clock           string
	zoom            float64
	showGrid        bool
	sel             string
	hover           string
	drag            dragState
	previewScale    float32
	previewBox      ui.Rect
	bitmaps         map[string]*ui.Bitmap
	thumbs          map[string]*ui.Bitmap
	colors          map[string]ui.Color
	layoutEdits     int
	layoutUndo      []layoutEdit
	layoutRedo      []layoutEdit
	dashTab         int
	hideSelection   bool
	forceClose      bool
	unsaved         bool
	scene           sceneKind
	sceneAnim       bool
	sceneSpeed      float64
	scenePhase      float64
	sceneEpoch      time.Time
	menuLang        string
	botState        string
	menuPageKey     string
	menuPageQuery   string
	popupType       string
	nitroMode       string
	ecuTemp         int
	motorTemp       int
	renderEcuTemp   int
	renderMotorTemp int
	weatherID       int
	airTemp         int
	compassDeg      int
	altitude        int
	powerKW         float64
	renderPower     float64
	ridePage        string
	showCharge      bool
	chargePower     float64
	chargeVoltage   float64
	hintID          int
	showGPS         bool
	adasMode        string
	showAllLamps    bool
	particle        *ui.VideoPlayer
	particlePath    string
	particleErr     string
	particleReady   bool
	particleEnabled bool
	usedAssets      map[string]bool
	hillHold        bool
	nominalVoltage  string
	chargeState     int
	imperial        bool
	dtcCode         int
	tpmsAbnormal    bool
	naviIcon        int
	naviArrow       int
	naviLanes       int
	naviLaneBg      int
	linkLoading     bool
	renderSpeed     int
	renderSOC       int
	renderGear      string
	renderClock     string

	// Assets.
	assetTop      string
	assetSub      string
	assetQuery    string
	selectedAsset string
	assetList     ui.ListState

	// Metadata.
	meta         themeMeta
	metaSnapshot themeMeta
	origType     string
	bumpVersion  bool

	// Export.
	exporting    bool
	exportResult *ExportResult
	exportErr    string
}

func prefsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "theme-studio", "prefs.json")
}

func loadPrefs() prefs {
	p := prefs{Zoom: 0, Mode: 0, ShowGrid: true}
	data, err := os.ReadFile(prefsPath())
	if err == nil {
		json.Unmarshal(data, &p)
	}
	return p
}

func (a *app) addRecent(p string) {
	var out []string
	for _, r := range a.recent {
		if !strings.EqualFold(r, p) {
			out = append(out, r)
		}
	}
	out = append([]string{p}, out...)
	if len(out) > 6 {
		out = out[:6]
	}
	a.recent = out
	a.saveSettings()
}

func (a *app) openPathAsync(p string) {
	if p == "" {
		return
	}
	if a.loading {
		return
	}
	a.loading = true
	a.errText = ""
	a.status = "正在打开主题包…"
	a.win.Invalidate()
	go func() {
		proj, err := openProject(p)
		a.win.Update(func() {
			a.loading = false
			if err != nil {
				a.errText = err.Error()
				a.status = "打开失败"
				return
			}
			a.stopParticle()
			if a.proj != nil {
				os.RemoveAll(a.proj.TempDir)
			}
			a.proj = proj
			a.bitmaps = map[string]*ui.Bitmap{}
			a.thumbs = map[string]*ui.Bitmap{}
			a.colors = map[string]ui.Color{}
			a.drawnVer = -1
			a.page = pageDashboard
			a.sel = ""
			a.hover = ""
			a.drag = dragState{}
			a.layoutEdits = 0
			a.layoutUndo = nil
			a.layoutRedo = nil
			a.unsaved = false
			a.exportResult = nil
			a.exportErr = ""
			a.selectedAsset = ""
			a.assetQuery = ""
			a.assetTop = "仪表"
			a.assetSub = "背景"
			a.assetList = ui.ListState{}
			a.loadDefaults()
			a.loadParticle()
			a.syncMeta()
			a.origType = proj.Config.Get("ThemeType")
			a.invalidatePreview()
			a.addRecent(p)
			a.status = "已打开 " + p
			a.win.SetTitle(a.windowTitle())
			a.win.Invalidate()
		})
	}()
}

func (a *app) loadDefaults() {
	if a.proj == nil {
		return
	}
	if a.speed <= 0 {
		a.speed = 68
	}
	if a.soc <= 0 {
		a.soc = 78
	}
	if a.gear == "" {
		a.gear = "3"
	}
	if a.clock == "" {
		a.clock = "0941"
	}
	if a.meta.ThemeVer == "" {
		a.syncMeta()
	}
}

func (a *app) syncMeta() {
	if a.proj == nil {
		return
	}
	a.meta = a.proj.meta()
	a.metaSnapshot = a.meta
}

func (a *app) windowTitle() string {
	if a.proj == nil {
		return "Theme Studio · 台铃仪表主题编辑器"
	}
	name := a.meta.DisplayName
	if name == "" {
		name = a.meta.ThemeName
	}
	if name == "" {
		name = a.proj.SourceName
	}
	mark := ""
	if a.unsaved {
		mark = " •"
	}
	return "Theme Studio · " + name + mark
}

func (a *app) markDirty() {
	a.unsaved = true
	if a.win != nil {
		a.win.SetTitle(a.windowTitle())
	}
}

func (a *app) openDialog() {
	if a.loading {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win,
			Title:  "打开台铃主题包",
			Filters: []mygo.FileFilter{
				{Name: "主题包", Extensions: []string{"zip"}},
			},
			Multiple: false,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.requestOpenPath(paths[0])
	}()
}

func (a *app) openFolderDialog() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.win,
			Title:     "打开解压后的主题文件夹",
			Directory: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.requestOpenPath(paths[0])
	}()
}

func (a *app) replaceAssetDialog() {
	if a.proj == nil || a.selectedAsset == "" || a.busy {
		return
	}
	a.busy = true
	rel := a.selectedAsset
	folder := strings.Contains(rel, "/bg/")
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win,
			Title:  "替换 " + path.Base(rel),
			Filters: []mygo.FileFilter{
				{Name: "图片", Extensions: []string{"png", "jpg", "jpeg", "webp", "bmp", "gif"}},
			},
			Multiple: false,
		})
		if err != nil || len(paths) == 0 {
			a.win.Update(func() { a.busy = false })
			return
		}
		err = a.proj.ReplaceAsset(rel, paths[0], folder)
		a.win.Update(func() {
			a.busy = false
			if err != nil {
				a.errText = err.Error()
				a.status = "替换失败"
				return
			}
			a.afterAssetChange(rel)
			a.status = "已替换 " + path.Base(rel)
			a.errText = ""
		})
	}()
}

func (a *app) replaceFolderDialog() {
	if a.proj == nil || a.selectedAsset == "" || a.busy {
		return
	}
	a.busy = true
	rel := a.selectedAsset
	folder := path.Dir(rel)
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.win,
			Title:     "选择素材文件夹（按同名文件覆盖）",
			Directory: true,
		})
		if err != nil || len(paths) == 0 {
			a.win.Update(func() { a.busy = false })
			return
		}
		n, err := a.proj.ReplaceFolder(rel, paths[0])
		a.win.Update(func() {
			a.busy = false
			if err != nil {
				a.errText = err.Error()
				a.status = "批量替换失败"
				return
			}
			a.clearFolderBitmaps(folder)
			a.afterAssetChange(rel)
			a.status = fmt.Sprintf("已覆盖 %d 张图片", n)
			a.errText = ""
		})
	}()
}

func (a *app) clearFolderBitmaps(folder string) {
	prefix := folder + "/"
	if folder == "." || folder == "" {
		prefix = ""
	}
	for k := range a.bitmaps {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			delete(a.bitmaps, k)
		}
	}
	for k := range a.thumbs {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			delete(a.thumbs, k)
		}
	}
	for k := range a.colors {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			delete(a.colors, k)
		}
	}
}

func (a *app) revertAsset(rel string) {
	if a.proj == nil {
		return
	}
	if err := a.proj.RevertAsset(rel); err != nil {
		a.errText = err.Error()
		return
	}
	a.afterAssetChange(rel)
	a.status = "已还原 " + path.Base(rel)
}

func (a *app) afterAssetChange(rel string) {
	a.errText = ""
	a.clearBitmap(rel)
	a.unsaved = true
	a.invalidatePreview()
	a.win.SetTitle(a.windowTitle())
}

func (a *app) clearBitmap(rel string) {
	for k := range a.bitmaps {
		if strings.HasPrefix(k, rel+"|") {
			delete(a.bitmaps, k)
		}
	}
	for k := range a.thumbs {
		if strings.HasPrefix(k, rel+"|") {
			delete(a.thumbs, k)
		}
	}
	for k := range a.colors {
		if strings.HasPrefix(k, rel+"|") {
			delete(a.colors, k)
		}
	}
}

func (a *app) quickExport() {
	if a.proj == nil || a.exporting {
		return
	}
	a.startExport(a.proj.quickExportPath())
}

func (a *app) exportDialog() {
	if a.proj == nil || a.exporting {
		return
	}
	name := a.proj.defaultExportName(a.meta)
	dir := filepath.Dir(a.proj.SourcePath)
	go func() {
		p, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      a.win,
			Title:       "导出主题包",
			DefaultPath: filepath.Join(dir, name),
			Filters: []mygo.FileFilter{
				{Name: "主题包", Extensions: []string{"zip"}},
			},
		})
		if err != nil || p == "" {
			return
		}
		a.startExport(p)
	}()
}

func (a *app) startExport(out string) {
	if a.proj == nil || a.exporting {
		return
	}
	a.exporting = true
	a.exportErr = ""
	a.errText = ""
	a.exportResult = nil
	a.status = "正在打包主题…"
	a.win.Invalidate()
	proj := a.proj
	meta := a.meta
	bump := a.bumpVersion
	go func() {
		res, err := proj.Export(meta, ExportOptions{Path: out, BumpVersion: bump})
		a.win.Update(func() {
			a.exporting = false
			if err != nil {
				a.exportErr = err.Error()
				a.status = "导出失败：" + err.Error()
				return
			}
			a.exportResult = res
			a.unsaved = false
			proj.Dirty = map[string]bool{}
			a.layoutEdits = 0
			a.meta.ThemeVer = res.Version
			a.metaSnapshot = a.meta
			a.bumpVersion = false
			a.status = "已导出 " + res.Path
			a.win.SetTitle(a.windowTitle())
			a.win.Invalidate()
		})
	}()
}

func (a *app) revealExport() {
	if a.exportResult == nil {
		return
	}
	mygo.Shell.ShowItemInFolder(a.exportResult.Path)
}

type checkItem struct {
	Label  string
	Detail string
	OK     bool
	Warn   bool
}

func (a *app) checks() []checkItem {
	p := a.proj
	if p == nil {
		return nil
	}
	var out []checkItem
	typ := a.meta.ThemeType
	out = append(out, checkItem{
		Label: "车型 / ThemeType", Detail: valueOr(typ, "未填写，车机会拒绝未知型号"), OK: typ != "",
	})
	if a.origType != "" && typ != a.origType {
		out = append(out, checkItem{Label: "车型被修改", Detail: "原包是 " + a.origType + "，请确认目标车机一致", OK: true, Warn: true})
	}
	out = append(out, checkItem{
		Label: "固件下限 / StartVersion", Detail: valueOr(a.meta.StartVersion, "未填写"), OK: a.meta.StartVersion != "",
	})
	bg := p.backgroundRel(a.mode)
	if bg == "" {
		out = append(out, checkItem{Label: "仪表背景", Detail: "找不到 1024×600 的背景图", OK: false})
	} else {
		w, h := imageSize(p.Abs(bg))
		badMode := ""
		for m := 0; m < 4; m++ {
			if mRel := p.backgroundRel(m); mRel != "" {
				mw, mh := imageSize(p.Abs(mRel))
				if mw != themeW || mh != themeH {
					badMode = fmt.Sprintf("%s (%d×%d)", path.Base(mRel), mw, mh)
					break
				}
			}
		}
		detail := fmt.Sprintf("%s · %d×%d", path.Base(bg), w, h)
		if badMode != "" {
			detail = "异常尺寸：" + badMode
		}
		out = append(out, checkItem{Label: "仪表背景", Detail: detail, OK: w == themeW && h == themeH && badMode == ""})
	}
	missing := 0
	for i := 0; i < 10; i++ {
		if !p.Has(fmt.Sprintf("dashboard/style/speedmeter/%d.png", i)) {
			missing++
		}
	}
	out = append(out, checkItem{Label: "速度数字 0–9", Detail: fmt.Sprintf("缺 %d 个", missing), OK: missing == 0})
	out = append(out, checkItem{Label: "仪表布局", Detail: "dashboard/etc/layout.ini", OK: p.Has("dashboard/etc/layout.ini")})
	changeDetail := fmt.Sprintf("%d 张图片 · %d 个控件位置", p.dirtyAssetCount(), a.layoutEdits)
	if a.meta != a.metaSnapshot {
		changeDetail += " · 信息已修改"
	}
	out = append(out, checkItem{
		Label: "本次改动", Detail: changeDetail, OK: true,
	})
	return out
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// ---- Assets -------------------------------------------------------------

type assetCategory struct {
	Name     string
	Children []string
}

// Two levels: a vehicle area, then what the images are for. The order is
// the order of the tree in the asset browser.
var assetTree = []assetCategory{
	{Name: "仪表", Children: []string{
		"背景", "速度与档位", "数字与时间", "SOC 与电池", "里程数据",
		"灯光与告警", "天气与指南针", "其他仪表",
	}},
	{Name: "菜单", Children: []string{
		"主菜单卡片", "背景与弹窗", "导航", "充电", "倒计时", "应用与图标", "其他菜单",
	}},
	{Name: "锁屏", Children: []string{
		"锁屏背景", "锁屏数字", "选择框", "其他锁屏",
	}},
	{Name: "主题换肤", Children: []string{"全部"}},
	{Name: "其他", Children: []string{"bot 动画", "其他"}},
}

func assetUnder(rel string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(rel, p) {
			return true
		}
	}
	return false
}

// assetCategoryOf gives every image exactly one top and sub category, so
// the tree counts add up and no image appears twice.
func assetCategoryOf(rel string) (top, sub string) {
	base := strings.ToLower(path.Base(rel))
	switch {
	case strings.HasPrefix(rel, "dashboard/"):
		top = "仪表"
		switch {
		case strings.Contains(rel, "/bg/") || strings.HasPrefix(base, "bg_"):
			sub = "背景"
		case assetUnder(rel, "dashboard/style/speedmeter/", "dashboard/style/gearmeter/", "dashboard/style/link/speedmeter/", "dashboard/style/link/gearmeter/"):
			sub = "速度与档位"
		case assetUnder(rel, "dashboard/style/num/", "dashboard/style/timepiece/", "dashboard/style/date/", "dashboard/style/link/timepiece/"):
			sub = "数字与时间"
		case assetUnder(rel, "dashboard/style/soc/", "dashboard/style/link/soc/"):
			sub = "SOC 与电池"
		case assetUnder(rel, "dashboard/style/ridedata/"):
			sub = "里程数据"
		case assetUnder(rel, "dashboard/style/lamps/", "dashboard/style/signals/", "dashboard/style/statewin/", "dashboard/style/link/lamps/", "dashboard/style/link/signals/"):
			sub = "灯光与告警"
		case assetUnder(rel, "dashboard/style/weather/", "dashboard/style/compass/", "dashboard/style/altitude/", "dashboard/style/week/"):
			sub = "天气与指南针"
		default:
			sub = "其他仪表"
		}
	case strings.HasPrefix(rel, "menu/"):
		top = "菜单"
		switch {
		case assetUnder(rel, "menu/style/home/", "menu/style/dash_set/"):
			sub = "主菜单卡片"
		case assetUnder(rel, "menu/style/Night/bg/", "menu/style/bg/", "menu/style/Night/list/") || strings.HasSuffix(rel, "/TipsBG.png"):
			sub = "背景与弹窗"
		case assetUnder(rel, "menu/style/navi/"):
			sub = "导航"
		case assetUnder(rel, "menu/style/charge/"):
			sub = "充电"
		case assetUnder(rel, "menu/style/count_down/"):
			sub = "倒计时"
		case assetUnder(rel, "menu/style/App/", "menu/style/mvc/", "menu/style/dtc/", "menu/style/warn/", "menu/style/music/", "menu/style/connect/", "menu/style/asr/", "menu/style/Night/connect/", "menu/style/Night/music/", "menu/style/Night/tpms/", "menu/style/Night/asr/") || rel == "menu/style/switch.png":
			sub = "应用与图标"
		default:
			sub = "其他菜单"
		}
	case strings.HasPrefix(rel, "tdSuspendMgr/"):
		top = "锁屏"
		switch {
		case strings.Contains(rel, "/number/") || strings.Contains(rel, "/select_num/"):
			sub = "锁屏数字"
		case strings.HasSuffix(rel, "/select.png"):
			sub = "选择框"
		case strings.HasSuffix(rel, "/bg.png"):
			sub = "锁屏背景"
		default:
			sub = "其他锁屏"
		}
	case strings.HasPrefix(rel, "switch_theme/"):
		top, sub = "主题换肤", "全部"
	case strings.HasPrefix(rel, "bot/"):
		top, sub = "其他", "bot 动画"
	default:
		top, sub = "其他", "其他"
	}
	return top, sub
}

func assetInCategory(rel, top, sub string) bool {
	if top == "全部" || top == "" {
		return true
	}
	t, s := assetCategoryOf(rel)
	if t != top {
		return false
	}
	return sub == "全部" || sub == "" || s == sub
}

func (a *app) filteredAssets() []Asset {
	if a.proj == nil {
		return nil
	}
	q := strings.ToLower(strings.TrimSpace(a.assetQuery))
	var out []Asset
	for _, asset := range a.proj.Assets {
		if a.assetTop == "已修改" {
			if !a.proj.Dirty[asset.Rel] {
				continue
			}
		} else if !assetInCategory(asset.Rel, a.assetTop, a.assetSub) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(asset.Rel), q) {
			continue
		}
		out = append(out, asset)
	}
	return out
}

func (a *app) countCategory(top, sub string) int {
	if a.proj == nil {
		return 0
	}
	if top == "已修改" {
		return a.proj.dirtyAssetCount()
	}
	n := 0
	for _, asset := range a.proj.Assets {
		if assetInCategory(asset.Rel, top, sub) {
			n++
		}
	}
	return n
}

func categoryForFolder(folder string) (top, sub string) {
	folder = strings.TrimSuffix(strings.ReplaceAll(folder, "\\", "/"), "/")
	if folder == "tdSuspendMgr/style/lock" {
		return "锁屏", "锁屏背景"
	}
	// A representative child name is enough: assetCategoryOf inspects the
	// directory part and the file name.
	return assetCategoryOf(folder + "/index.png")
}

func (a *app) selectedAssetInfo() (Asset, bool) {
	if a.proj == nil || a.selectedAsset == "" {
		return Asset{}, false
	}
	return a.proj.Asset(a.selectedAsset)
}

func (a *app) setAssetCategory(top, sub string) {
	a.assetTop, a.assetSub = top, sub
	a.assetList = ui.ListState{}
	a.selectedAsset = ""
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *app) openAssetFolder(folder string) {
	if a.proj == nil {
		return
	}
	a.page = pageAssets
	a.assetTop, a.assetSub = categoryForFolder(folder)
	a.assetQuery = ""
	a.assetList = ui.ListState{}
	a.selectedAsset = ""
	for _, asset := range a.proj.Assets {
		if path.Dir(asset.Rel) == folder {
			a.selectedAsset = asset.Rel
			break
		}
	}
	if a.win != nil {
		a.win.Invalidate()
	}
}

// ---- Preferences / startup ------------------------------------------------

func (a *app) saveSettings() {
	a.settings.Zoom = a.zoom
	a.settings.Mode = a.mode
	a.settings.ShowGrid = a.showGrid
	a.settings.Recent = a.recent
	os.MkdirAll(filepath.Dir(prefsPath()), 0o755)
	b, _ := json.MarshalIndent(a.settings, "", "  ")
	os.WriteFile(prefsPath(), b, 0o644)
}

func (a *app) nowStatus() string {
	if a.loading {
		return "正在打开…"
	}
	if a.exporting {
		return "正在打包…"
	}
	if a.errText != "" {
		return a.errText
	}
	if a.status != "" {
		return a.status
	}
	if a.proj == nil {
		return "打开一个主题包开始编辑"
	}
	return a.proj.SourcePath
}

func (a *app) imageSizeOf(rel string) (int, int) {
	if a.proj == nil {
		return 0, 0
	}
	return imageSize(a.proj.Abs(rel))
}

func (a *app) launchOpen() {
	if a.startup != "" {
		a.openPathAsync(a.startup)
	}
}

func (a *app) replaceAssetFrom(rel, src string) {
	if a.proj == nil {
		return
	}
	cover := strings.Contains(rel, "/bg/")
	go func() {
		err := a.proj.ReplaceAsset(rel, src, cover)
		a.win.Update(func() {
			if err != nil {
				a.errText = err.Error()
				a.status = "替换失败"
				return
			}
			a.selectedAsset = rel
			a.afterAssetChange(rel)
			a.status = "已替换 " + path.Base(rel)
			a.errText = ""
		})
	}()
}

func (a *app) resetAllLayout() {
	if a.proj == nil {
		return
	}
	n := 0
	for name, pt := range a.proj.OriginalPos {
		if name == "main_win" {
			continue
		}
		if a.proj.MoveWidget(name, pt.X, pt.Y) {
			n++
		}
	}
	a.layoutEdits = 0
	a.layoutUndo = nil
	a.layoutRedo = nil
	a.proj.LayoutDirty = false
	a.unsaved = a.proj.dirtyAssetCount() > 0 || a.meta != a.metaSnapshot
	if a.win != nil {
		a.win.SetTitle(a.windowTitle())
	}
	if n > 0 {
		a.invalidatePreview()
		a.status = fmt.Sprintf("已复位 %d 个控件", n)
	} else {
		a.status = "控件已经在原始位置"
	}
}

func (a *app) recordLayoutEdit(name string, oldX, oldY, newX, newY int) {
	if oldX == newX && oldY == newY {
		return
	}
	a.layoutUndo = append(a.layoutUndo, layoutEdit{
		Name: name, OldX: oldX, OldY: oldY, NewX: newX, NewY: newY,
	})
	if len(a.layoutUndo) > 100 {
		a.layoutUndo = a.layoutUndo[len(a.layoutUndo)-100:]
	}
	a.layoutRedo = nil
}

func (a *app) undoLayout() {
	if a.proj == nil || len(a.layoutUndo) == 0 {
		return
	}
	ed := a.layoutUndo[len(a.layoutUndo)-1]
	a.layoutUndo = a.layoutUndo[:len(a.layoutUndo)-1]
	if a.proj.MoveWidget(ed.Name, ed.OldX, ed.OldY) {
		a.layoutRedo = append(a.layoutRedo, ed)
		if a.layoutEdits > 0 {
			a.layoutEdits--
		}
		a.sel = ed.Name
		a.markDirty()
		a.invalidatePreview()
		a.status = fmt.Sprintf("已撤销：%s → %d, %d", widgetLabel(ed.Name), ed.OldX, ed.OldY)
		if a.win != nil {
			a.win.Invalidate()
		}
	}
}

func (a *app) redoLayout() {
	if a.proj == nil || len(a.layoutRedo) == 0 {
		return
	}
	ed := a.layoutRedo[len(a.layoutRedo)-1]
	a.layoutRedo = a.layoutRedo[:len(a.layoutRedo)-1]
	if a.proj.MoveWidget(ed.Name, ed.NewX, ed.NewY) {
		a.layoutUndo = append(a.layoutUndo, ed)
		a.layoutEdits++
		a.sel = ed.Name
		a.markDirty()
		a.invalidatePreview()
		a.status = fmt.Sprintf("已重做：%s → %d, %d", widgetLabel(ed.Name), ed.NewX, ed.NewY)
		if a.win != nil {
			a.win.Invalidate()
		}
	}
}

func (a *app) confirmDiscard(title, msg string, onProceed func()) {
	if !a.unsaved || a.win == nil {
		onProceed()
		return
	}
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:             a.win,
			Type:               mygo.MessageWarning,
			Title:              title,
			Message:            msg,
			Detail:             "当前主题还有未导出的改动，继续操作将丢弃这些修改。",
			Buttons:            []string{"放弃改动", "取消"},
			DefaultButton:      1,
			CancelButton:       1,
			DestructiveButtons: []int{0},
		})
		if err == nil && res.Button == 0 {
			a.win.Update(onProceed)
		}
	}()
}

func (a *app) requestOpenPath(p string) {
	if p == "" || a.loading {
		return
	}
	a.confirmDiscard("打开新主题", "放弃当前未导出的改动并打开新主题？", func() {
		a.openPathAsync(p)
	})
}
