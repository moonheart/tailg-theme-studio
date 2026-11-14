package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	themeW = 1024
	themeH = 600
)

type hitItem struct {
	Name   string
	Label  string
	Folder string
	X, Y   int
	W, H   int
}

type dragState struct {
	Active bool
	Name   string
	StartX float32
	StartY float32
	BaseX  int
	BaseY  int
	DX     int
	DY     int
	Moved  bool
	Start  int64
}

func (a *app) invalidatePreview() {
	a.previewVer++
}

func (a *app) bitmap(rel string) *ui.Bitmap {
	if a.usedAssets != nil {
		a.usedAssets[rel] = true
	}
	if a.bitmaps == nil {
		a.bitmaps = map[string]*ui.Bitmap{}
	}
	p := a.proj
	if p == nil {
		return nil
	}
	abs := p.Abs(rel)
	st, err := os.Stat(abs)
	if err != nil {
		return nil
	}
	key := rel + "|" + strconv.FormatInt(st.ModTime().UnixNano(), 10) + "|" + strconv.FormatInt(st.Size(), 10)
	if b, ok := a.bitmaps[key]; ok {
		return b
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	b, err := ui.DecodeBitmap(data)
	if err != nil {
		return nil
	}
	a.bitmaps[key] = b
	return b
}

func (a *app) thumb(rel string) *ui.Bitmap {
	if a.thumbs == nil {
		a.thumbs = map[string]*ui.Bitmap{}
	}
	p := a.proj
	if p == nil {
		return nil
	}
	abs := p.Abs(rel)
	st, err := os.Stat(abs)
	if err != nil {
		return nil
	}
	key := rel + "|" + strconv.FormatInt(st.ModTime().UnixNano(), 10) + "|" + strconv.FormatInt(st.Size(), 10)
	if b, ok := a.thumbs[key]; ok {
		return b
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		b, e2 := ui.DecodeBitmap(data)
		if e2 != nil {
			return nil
		}
		a.thumbs[key] = b
		return b
	}
	b := img.Bounds()
	const maxSide = 320
	if b.Dx() <= maxSide && b.Dy() <= maxSide {
		nb := ui.NewBitmap(img)
		a.thumbs[key] = nb
		return nb
	}
	scale := math.Min(float64(maxSide)/float64(b.Dx()), float64(maxSide)/float64(b.Dy()))
	w, h := int(float64(b.Dx())*scale), int(float64(b.Dy())*scale)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	xdrawScale(dst, img)
	nb := ui.NewBitmap(dst)
	a.thumbs[key] = nb
	return nb
}

func xdrawScale(dst *image.NRGBA, src image.Image) {
	sb := src.Bounds()
	db := dst.Bounds()
	// A small separable box filter is enough for a 160px thumbnail.
	for y := db.Min.Y; y < db.Max.Y; y++ {
		sy0 := sb.Min.Y + y*sb.Dy()/db.Dy()
		sy1 := sb.Min.Y + (y+1)*sb.Dy()/db.Dy()
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for x := db.Min.X; x < db.Max.X; x++ {
			sx0 := sb.Min.X + x*sb.Dx()/db.Dx()
			sx1 := sb.Min.X + (x+1)*sb.Dx()/db.Dx()
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var r, g, b, a, n uint64
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					cr := color.NRGBAModel.Convert(src.At(sx, sy)).(color.NRGBA)
					r += uint64(cr.R)
					g += uint64(cr.G)
					b += uint64(cr.B)
					a += uint64(cr.A)
					n++
				}
			}
			if n == 0 {
				continue
			}
			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r / n), G: uint8(g / n),
				B: uint8(b / n), A: uint8(a / n),
			})
		}
	}
}

func (a *app) assetColor(rel string) ui.Color {
	if a.colors == nil {
		a.colors = map[string]ui.Color{}
	}
	p := a.proj
	if p == nil {
		return ui.RGBA(255, 255, 255, 0.9)
	}
	st, err := os.Stat(p.Abs(rel))
	if err != nil {
		return ui.RGBA(255, 255, 255, 0.9)
	}
	key := rel + "|" + strconv.FormatInt(st.ModTime().UnixNano(), 10)
	if c, ok := a.colors[key]; ok {
		return c
	}
	data, err := os.ReadFile(p.Abs(rel))
	if err != nil {
		return ui.RGBA(255, 255, 255, 0.9)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ui.RGBA(255, 255, 255, 0.9)
	}
	b := img.Bounds()
	var r, g, bl, cnt uint64
	for y := b.Min.Y; y < b.Max.Y; y += 2 {
		for x := b.Min.X; x < b.Max.X; x += 2 {
			cr, cg, cb, ca := img.At(x, y).RGBA()
			if ca < 0x8000 {
				continue
			}
			r += uint64(cr)
			g += uint64(cg)
			bl += uint64(cb)
			cnt++
		}
	}
	c := ui.RGBA(255, 255, 255, 0.9)
	if cnt > 0 {
		c = ui.RGBA(uint8((r/cnt)>>8), uint8((g/cnt)>>8), uint8((bl/cnt)>>8), 0.95)
	}
	a.colors[key] = c
	return c
}

func (p *ThemeProject) backgroundRel(mode int) string {
	if p.bgCache != nil {
		if rel, ok := p.bgCache[mode]; ok && p.Has(rel) {
			return rel
		}
	} else {
		p.bgCache = map[int]string{}
	}
	if data, err := os.ReadFile(p.Abs("dashboard/etc/dynamic-load.ini")); err == nil {
		text := string(data)
		for _, key := range []string{fmt.Sprintf("night_%d", mode), fmt.Sprintf("day_%d", mode)} {
			re := regexp.MustCompile(key + `\s*=\s*"([^"]+)"`)
			if m := re.FindStringSubmatch(text); m != nil {
				rel := strings.TrimPrefix(strings.ReplaceAll(m[1], "\\", "/"), "./")
				if p.Has(rel) {
					p.bgCache[mode] = rel
					return rel
				}
			}
		}
	}
	var candidates []string
	for _, a := range p.Assets {
		base := path.Base(a.Rel)
		if strings.HasPrefix(a.Rel, "dashboard/") && strings.HasPrefix(base, "bg_") &&
			strings.HasSuffix(base, ".png") {
			candidates = append(candidates, a.Rel)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool { return naturalLess(candidates[i], candidates[j]) })
	preferred := fmt.Sprintf("dashboard/style/nightime/bg/bg_%d.png", mode+1)
	for _, c := range candidates {
		if c == preferred {
			return c
		}
	}
	if mode >= 0 && mode < len(candidates) {
		return candidates[mode]
	}
	return candidates[0]
}

func (a *app) drawTheme(cp *ui.Painter) {
	if !a.videoBgActive() {
		cp.Fill(ui.Rect{W: themeW, H: themeH}, ui.Hex("#0b0b0d"), 0)
	}
	if a.proj == nil {
		cp.Text(32, 40, "打开一个主题包开始", 16, ui.Hex("#ffffff"))
		return
	}
	a.renderSpeed, a.renderSOC, a.renderGear, a.renderClock = a.speed, a.soc, a.gear, a.clock
	a.renderEcuTemp, a.renderMotorTemp = a.ecuTemp, a.motorTemp
	a.renderPower = a.powerKW
	if a.page == pageScenes {
		a.prepareSceneRender()
		switch a.scene {
		case sceneMenu:
			a.drawMenuScene(cp)
		case sceneMenuSub:
			a.drawMenuSubScene(cp)
		case sceneMenuPage:
			a.drawMenuPageScene(cp)
		case scenePopup:
			a.drawPopupScene(cp)
		case sceneBot:
			a.drawBotScene(cp)
		case sceneCharge:
			a.drawChargeScene(cp)
		case sceneLock:
			a.drawLockScene(cp)
		case sceneLink:
			a.drawLinkScene(cp)
		case sceneLights:
			a.drawDashboardScene(cp)
			a.drawLightsScene(cp)
			a.drawBotAt(cp, 0, "normal", a.sceneAnim)
		case sceneSwitch:
			a.drawSwitchScene(cp)
		case sceneCountdown:
			a.drawCountdownScene(cp)
		default:
			a.drawDashboardScene(cp)
			a.drawBotAt(cp, 0, "normal", a.sceneAnim)
		}
		return
	}
	a.drawDashboardScene(cp)
	a.drawBotAt(cp, 0, "normal", false)
}

func (a *app) drawDashboardScene(cp *ui.Painter) {
	p := a.proj
	if p == nil {
		return
	}
	if !a.videoBgActive() {
		bg := p.backgroundRel(a.mode)
		if bg != "" {
			if b := a.bitmap(bg); b != nil {
				cp.Image(b, ui.Rect{W: themeW, H: themeH}, ui.FillBox)
			}
		}
	}

	a.drawRideData(cp)
	a.drawADAS(cp)

	// SOC process bar and number.
	if w := p.Widget("soc_meter"); w != nil {
		box := a.widgetDrawRect(w)
		if b := a.bitmap("dashboard/style/soc/soc_total/process/bg.png"); b != nil {
			cp.Image(b, box, ui.FillBox)
		}
		pct := float32(a.renderSOC) / 100
		if pct < 0 {
			pct = 0
		}
		if pct > 1 {
			pct = 1
		}
		procRel := "dashboard/style/soc/soc_total/process/process.png"
		if a.renderSOC <= 10 {
			procRel = "dashboard/style/soc/soc_total/process/low_2.png"
		} else if a.renderSOC <= 20 {
			procRel = "dashboard/style/soc/soc_total/process/low_1.png"
		}
		if b := a.bitmap(procRel); b != nil && pct > 0 {
			clip := box
			clip.W *= pct
			cp.Clip(clip, 0, func() { cp.Image(b, box, ui.FillBox) })
		}
	}
	if a.renderSOC <= 15 {
		if w := p.Widget("low_soc_icon"); w != nil {
			rel := styleBG(p, "button", w.Style)
			if rel == "" {
				rel = "dashboard/style/soc/soc_total/low_soc_icon.png"
			}
			a.drawBitmapAt(cp, rel, a.widgetDrawRect(w), ui.FillBox)
		}
	}
	if a.nominalVoltage == "" {
		if w := p.Widget("btn_soc"); w != nil {
			a.drawDigitsRight(cp, "dashboard/style/soc/soc_total/num", strconv.Itoa(a.renderSOC),
				a.widgetDrawRect(w), 0)
		}
		if w := p.Widget("btn_soc_unit"); w != nil {
			if b := a.bitmap("dashboard/style/soc/soc_total/soc_unit.png"); b != nil {
				cp.Image(b, a.widgetDrawRect(w), ui.Contain)
			}
		}
	}

	// Speed, or P when the preview gear is P.
	if a.renderGear == "P" {
		if w := p.Widget("btn_gearP"); w != nil {
			frame := 0
			if a.sceneAnim {
				frame = int(a.scenePhase*3) % 4
			}
			if b := a.bitmap(fmt.Sprintf("dashboard/style/gearmeter/gear_p/P_%d.png", frame)); b != nil {
				cp.Image(b, a.widgetDrawRect(w), ui.FillBox)
			}
		}
	} else {
		if w := p.Widget("speedmeter"); w != nil && !a.hillHold {
			a.drawSpeed(cp, w)
		}
		if w := p.Widget("speed_unit"); w != nil && !a.hillHold {
			rel := "dashboard/style/speedmeter/kmh.png"
			if a.imperial || w.Style == "speed_unit_mph" {
				rel = "dashboard/style/speedmeter/mph.png"
			}
			if b := a.bitmap(rel); b != nil {
				cp.Image(b, a.widgetDrawRect(w), ui.Contain)
			}
		}
		if w := p.Widget("btn_gear"); w != nil {
			rel := fmt.Sprintf("dashboard/style/gearmeter/region_0/nightime/%s.png", a.renderGear)
			if b := a.bitmap(rel); b != nil {
				cp.Image(b, a.widgetDrawRect(w), ui.FillBox)
			}
		}
	}

	if a.hillHold {
		if w := p.Widget("speed_stop"); w != nil {
			a.drawBitmapAt(cp, "dashboard/style/speedmeter/H.png",
				a.widgetDrawRect(w), ui.FillBox)
		}
	}
	if a.nominalVoltage != "" {
		if w := p.Widget("btn_nominal_voltage"); w != nil {
			a.drawBitmapAt(cp, "dashboard/style/nightime/nominal_voltage/"+a.nominalVoltage+".png",
				a.widgetDrawRect(w), ui.FillBox)
		}
	}

	// Clock, five fixed slots so the colon keeps its place.
	if w := p.Widget("mainwin_time"); w != nil {
		a.drawClock(cp, w)
	}
	a.drawNitro(cp, a.liveWidgetSceneRect("n2_icon_0", p.Scene.NitroIcon), a.liveWidgetSceneRect("n2_left_warn_0", p.Scene.NitroSeq))
	a.drawTemps(cp)
	a.drawPower(cp)
	a.drawWeather(cp)
	a.drawCompass(cp)
	a.drawAltitude(cp)
	a.drawHint(cp)
	if a.showGPS {
		a.drawGPSSignal(cp, 4, 0.95)
	}
	if a.showCharge {
		a.drawChargeOverlay(cp)
	}
	if a.showAllLamps && (a.page == pageDashboard || a.scene == sceneDash) {
		a.drawAllLamps(cp)
	}
}

func (a *app) drawAllLamps(cp *ui.Painter) {
	d := a.proj.Scene
	if d == nil {
		return
	}
	for _, l := range d.Page0Lamps {
		if l.Check {
			a.drawLamp(cp, a.proj.Scene.LampPaths[l.ID], l, 0.95)
		}
	}
	for _, s := range d.Page0Signal {
		a.drawSignal(cp, s, 1, 0.95)
	}
	a.drawGPSSignal(cp, 4, 0.95)
}

func (a *app) drawTemps(cp *ui.Painter) {
	d := a.proj.Scene
	if d == nil {
		return
	}
	a.drawTempPair(cp,
		a.liveWidgetRect("btn_ecu_icon", d.EcuIcon),
		a.liveWidgetRect("btn_ecu_temp", d.EcuTemp),
		a.liveWidgetRect("btn_ecu_unit", d.EcuUnit),
		"btn_ecu_icon", "ecu_unit", "dashboard/style/regular/Temp/ecu.png", a.renderEcuTemp)
	a.drawTempPair(cp,
		a.liveWidgetRect("btn_motor_icon", d.MotorIcon),
		a.liveWidgetRect("btn_motor_temp", d.MotorTemp),
		a.liveWidgetRect("btn_motor_unit", d.MotorUnit),
		"btn_motor_icon", "motor_unit", "dashboard/style/regular/Temp/motor.png", a.renderMotorTemp)
}

func (a *app) drawTempPair(cp *ui.Painter, icon, temp, unit ui.Rect,
	iconStyle, unitStyle, fallbackIcon string, value int) {
	if icon.W > 0 {
		rel := styleBG(a.proj, "button", iconStyle)
		if rel == "" {
			rel = fallbackIcon
		}
		a.drawBitmapAt(cp, rel, icon, ui.FillBox)
	}
	if temp.W > 0 {
		a.drawDigitsCenter(cp, "dashboard/style/num", strconv.Itoa(value), temp, 0)
	}
	if unit.W > 0 {
		rel := styleBG(a.proj, "button", unitStyle)
		if rel == "" {
			rel = "dashboard/style/regular/Temp/temp_unit.png"
		}
		a.drawBitmapAt(cp, rel, unit, ui.FillBox)
	}
}

func (a *app) drawRideData(cp *ui.Painter) {
	d := a.proj.Scene
	if d == nil {
		return
	}
	fallback := d.RideData
	if fallback.W <= 0 {
		fallback = SceneRect{X: 2, Y: 555, W: 225, H: 30}
	}
	box := a.liveWidgetRect("lv_ridedata", fallback)
	cp.Clip(box, 0, func() {
		key := a.ridePage
		if key == "" || key == "auto" {
			pages := []string{"odo", "trip", "speed", "time"}
			key = pages[int(a.scenePhase/3)%len(pages)]
		}
		topic, value, unit := "ODO", "00128", "km"
		topicX, topicY, topicW, topicH := 47, 12, 43, 17
		unitH, unitY := 14, 15
		switch key {
		case "trip":
			topic, value = "TRIP", "12.4"
		case "speed":
			topic, value, unit = "SPEED", "24.6", "kmh"
			topicX, topicW, topicH = 0, 86, 20
			unitH, unitY = 16, 13
		case "time":
			topic, value, unit = "TRIP_TIME", "00:42", ""
			topicX, topicW, topicH = 0, 86, 20
		}
		if a.imperial {
			switch key {
			case "odo":
				value = fmt.Sprintf("%05d", int(math.Round(128*0.621371)))
				unit = "miles"
			case "trip":
				value = fmt.Sprintf("%.1f", 12.4*0.621371)
				unit = "miles"
			case "speed":
				value = fmt.Sprintf("%.1f", 24.6*0.621371)
				unit = "mph"
			}
		}
		a.drawBitmapAt(cp, "dashboard/style/ridedata/topic/"+topic+".png",
			ui.Rect{X: box.X + float32(topicX), Y: box.Y + float32(topicY),
				W: float32(topicW), H: float32(topicH)}, ui.FillBox)
		a.drawDigitsRight(cp, "dashboard/style/num", value,
			ui.Rect{X: box.X + 90, Y: box.Y + 2, W: 90, H: 27}, 0)
		if unit != "" {
			a.drawBitmapAt(cp, "dashboard/style/ridedata/unit/"+unit+".png",
				ui.Rect{X: box.X + 185, Y: box.Y + float32(unitY), W: 40, H: float32(unitH)}, ui.FillBox)
		}
	})
}

func (a *app) drawWeather(cp *ui.Painter) {
	d := a.proj.Scene
	if d == nil {
		return
	}
	wbg := a.liveWidgetRect("weather_bg", d.WeatherBG)
	if wbg.W <= 0 {
		return
	}
	bg := styleBG(a.proj, "button", "weather_bg")
	if bg == "" {
		bg = "dashboard/style/nightime/weather/bg.png"
	}
	a.drawBitmapAt(cp, bg, wbg, ui.FillBox)
	id := a.weatherID
	if id <= 0 {
		id = 1
	}
	icon := fmt.Sprintf("dashboard/style/nightime/weather/%d/0.png", id)
	if !a.proj.Has(icon) {
		id = 1
		icon = "dashboard/style/nightime/weather/1/0.png"
	}
	a.drawBitmapAt(cp, icon, rectOf(d.WeatherIcon), ui.FillBox)
	lang := "zh"
	if a.menuLang == "en_US" {
		lang = "en"
	}
	name := fmt.Sprintf("dashboard/style/nightime/weather/name_%s/%d/0.png", lang, id)
	if !a.proj.Has(name) {
		name = fmt.Sprintf("dashboard/style/nightime/weather/name_zh/%d/0.png", id)
	}
	a.drawBitmapAt(cp, name, a.liveWidgetRect("btn_weather_text", d.WeatherText), ui.FillBox)
	a.drawDigitsRight(cp, "dashboard/style/nightime/weather/temp", strconv.Itoa(a.airTemp),
		a.liveWidgetRect("btn_airtemp", d.AirTemp), 0)
	unit := styleBG(a.proj, "button", "temp_c")
	if unit == "" {
		unit = "dashboard/style/nightime/weather/temp/unit.png"
	}
	a.drawBitmapAt(cp, unit, a.liveWidgetRect("btn_tempunit", d.TempUnit), ui.FillBox)
	// date: the theme's ASCII glyphs 46-57 are "." and "0"-"9".
	a.drawDateGlyphs(cp, time.Now().Format("01.02"), a.liveWidgetRect("btn_date", d.DateText))
	week := int(time.Now().Weekday())
	weekDir := "text_zh"
	if a.menuLang == "en_US" {
		weekDir = "text_en"
	}
	a.drawBitmapAt(cp, fmt.Sprintf("dashboard/style/week/%s/%d.png", weekDir, week),
		a.liveWidgetRect("btn_week_text", d.WeekText), ui.FillBox)
}

func (a *app) drawDateGlyphs(cp *ui.Painter, text string, box ui.Rect) {
	type glyph struct {
		b *ui.Bitmap
		w float32
	}
	var glyphs []glyph
	total := float32(0)
	for _, ch := range text {
		b := a.bitmap(fmt.Sprintf("dashboard/style/date/%d.png", int(ch)))
		if b == nil {
			continue
		}
		bw, bh := b.Size()
		w := float32(bw) * box.H / float32(maxInt(1, bh))
		glyphs = append(glyphs, glyph{b: b, w: w})
		total += w
	}
	x := box.X + (box.W-total)/2
	for _, g := range glyphs {
		cp.Image(g.b, ui.Rect{X: x, Y: box.Y, W: g.w, H: box.H}, ui.FillBox)
		x += g.w
	}
}

var compassDirs = []string{
	"north", "northeast", "east", "southeast",
	"south", "southwest", "west", "northwest",
}

func (a *app) drawCompass(cp *ui.Painter) {
	d := a.proj.Scene
	if d == nil {
		return
	}
	iconRect := a.liveWidgetRect("btn_compass_icon", d.CompassIcon)
	if iconRect.W <= 0 {
		return
	}
	deg := a.compassDeg % 360
	if deg < 0 {
		deg += 360
	}
	lang, icon := "zh_CN", "compass_zh.png"
	if a.menuLang == "en_US" {
		lang, icon = "en_US", "compass_us.png"
	}
	a.drawBitmapAt(cp, "dashboard/style/compass/icon/"+icon, iconRect, ui.FillBox)
	dir := compassDirs[((deg+22)/45)%8]
	a.drawBitmapAt(cp, "dashboard/style/compass/text/"+lang+"/"+dir+".png",
		a.liveWidgetRect("btn_compass_text", d.CompassText), ui.FillBox)
	a.drawDigitsRight(cp, "dashboard/style/num", fmt.Sprintf("%03d", deg),
		a.liveWidgetRect("btn_compass_value", d.CompassValue), 0)
	a.drawBitmapAt(cp, "dashboard/style/compass/icon/compass_unit.png",
		a.liveWidgetRect("btn_compass_unit", d.CompassUnit), ui.FillBox)
}

func (a *app) drawAltitude(cp *ui.Painter) {
	d := a.proj.Scene
	if d == nil {
		return
	}
	textRect := a.liveWidgetRect("btn_altitude_text", d.AltitudeText)
	if textRect.W <= 0 {
		return
	}
	lang := "cn"
	if a.menuLang == "en_US" {
		lang = "us"
	}
	a.drawBitmapAt(cp, "dashboard/style/altitude/text/"+lang+".png",
		textRect, ui.FillBox)
	alt, unit := a.altitude, "m_unit.png"
	if a.imperial {
		alt = int(math.Round(float64(alt) * 3.28084))
		unit = "foot_unit.png"
	}
	a.drawDigitsRight(cp, "dashboard/style/num", strconv.Itoa(alt),
		a.liveWidgetRect("btn_altitude_value", d.AltitudeValue), 0)
	a.drawBitmapAt(cp, "dashboard/style/altitude/icon/"+unit,
		a.liveWidgetRect("btn_altitude_unit", d.AltitudeUnit), ui.FillBox)
}

func (a *app) drawPower(cp *ui.Painter) {
	d := a.proj.Scene
	if d == nil {
		return
	}
	numRect := a.liveWidgetRect("power_num", d.PowerNum)
	if numRect.W <= 0 {
		return
	}
	title := styleBG(a.proj, "button", "power_title")
	if title == "" {
		title = "dashboard/style/nightime/power/POWER.png"
	}
	a.drawBitmapAt(cp, title, a.liveWidgetRect("power_title", d.PowerTitle), ui.FillBox)
	folder := "dashboard/style/nightime/power/green"
	if a.renderPower < 0 {
		folder = "dashboard/style/nightime/power/red"
	}
	a.drawDigitsRight(cp, folder, fmt.Sprintf("%.1f", math.Abs(a.renderPower)),
		numRect, 0)
	unit := styleBG(a.proj, "button", "power_unit")
	if unit == "" {
		unit = "dashboard/style/nightime/power/kW.png"
	}
	a.drawBitmapAt(cp, unit, a.liveWidgetRect("power_unit", d.PowerUnit), ui.FillBox)
}

func (a *app) drawHint(cp *ui.Painter) {
	if a.hintID <= 0 {
		return
	}
	w := a.proj.Widget("btn_hint_msg")
	if w == nil {
		return
	}
	lang := "zh"
	if a.menuLang == "en_US" {
		lang = "en"
	}
	a.drawBitmapAt(cp, fmt.Sprintf("dashboard/style/nightime/hint_msg/img_%s/HINT_ID_%d.png", lang, a.hintID),
		ui.Rect{X: float32(w.X), Y: float32(w.Y), W: float32(w.W), H: float32(w.H)}, ui.FillBox)
}

func (a *app) drawADAS(cp *ui.Painter) {
	if a.adasMode == "" || a.adasMode == "none" {
		return
	}
	frame := 0
	if a.sceneAnim {
		frame = int(a.scenePhase*2) % 2
	}
	draw := func(name string) {
		w := a.proj.Widget(name)
		if w == nil {
			return
		}
		side := "left"
		switch {
		case strings.Contains(name, "_rear_"):
			side = "rear"
		case strings.Contains(name, "_right_"):
			side = "right"
		}
		a.drawBitmapAt(cp, fmt.Sprintf("dashboard/style/regular/ADAS/%s/%d.png", side, frame),
			ui.Rect{X: float32(w.X), Y: float32(w.Y), W: float32(w.W), H: float32(w.H)}, ui.FillBox)
	}
	switch a.adasMode {
	case "left":
		draw("adas_left_warn_0")
	case "right":
		draw("adas_right_warn_0")
	case "rear":
		draw("adas_rear_warn_0")
	case "sides":
		draw("adas_left_warn_0")
		draw("adas_right_warn_0")
	}
}

func (a *app) drawChargeOverlay(cp *ui.Painter) {
	d := a.proj.Scene
	if d == nil || d.ChargeWin.W <= 0 {
		return
	}
	win := rectOf(d.ChargeWin)
	bg := styleBG(a.proj, "button", "charge_bg")
	if bg == "" {
		bg = "dashboard/style/soc/charge_info/bg.png"
	}
	a.drawBitmapAt(cp, bg, win, ui.FillBox)
	lang := "zh"
	if a.menuLang == "en_US" {
		lang = "en"
	}
	base := "dashboard/style/soc/charge_info/text_" + lang
	off := func(r SceneRect) ui.Rect {
		return ui.Rect{X: win.X + float32(r.X), Y: win.Y + float32(r.Y),
			W: float32(r.W), H: float32(r.H)}
	}
	title := a.chargeState
	if title < 1 || title > 3 {
		title = 1
	}
	a.drawBitmapAt(cp, fmt.Sprintf("%s/title_%d.png", base, title), off(d.ChargeTitle), ui.FillBox)
	a.drawBitmapAt(cp, base+"/info.png", off(d.ChargeInfo), ui.FillBox)
	a.drawBitmapAt(cp, base+"/hint.png", off(d.ChargeHint), ui.FillBox)
	a.drawDigitsRight(cp, "dashboard/style/num", fmt.Sprintf("%.1f", a.chargePower),
		off(d.ChargePower), 0)
	a.drawDigitsRight(cp, "dashboard/style/num", fmt.Sprintf("%.0f", a.chargeVoltage),
		off(d.ChargeVoltage), 0)
}

func rectOf(r SceneRect) ui.Rect {
	return ui.Rect{X: float32(r.X), Y: float32(r.Y), W: float32(r.W), H: float32(r.H)}
}

func (a *app) widgetDrawRect(w *Widget) ui.Rect {
	dx, dy := a.dragOffset(w.Name)
	return ui.Rect{X: float32(w.X + dx), Y: float32(w.Y + dy), W: float32(w.W), H: float32(w.H)}
}

func (a *app) liveWidgetSceneRect(name string, fallback SceneRect) SceneRect {
	if a.proj != nil {
		if w := a.proj.Widget(name); w != nil && w.W > 0 && w.H > 0 {
			dx, dy := a.dragOffset(name)
			return SceneRect{X: w.X + dx, Y: w.Y + dy, W: w.W, H: w.H}
		}
	}
	return fallback
}

func (a *app) liveWidgetRect(name string, fallback SceneRect) ui.Rect {
	return rectOf(a.liveWidgetSceneRect(name, fallback))
}

// nitroState resolves the flame indicator state. Auto loops ready → boost →
// depleted so the preview shows all three theme icons without a controller.
func (a *app) nitroState() string {
	if a.nitroMode != "" && a.nitroMode != "auto" {
		return a.nitroMode
	}
	t := math.Mod(a.scenePhase, 6.2)
	if t < 0 {
		t += 6.2
	}
	switch {
	case t < 3.0:
		return "ready"
	case t < 4.4:
		return "boost"
	default:
		return "idle"
	}
}

func (a *app) nitroStateLabel() string {
	switch a.nitroState() {
	case "idle":
		return "未就绪 · 灰色火焰"
	case "boost":
		if a.proj != nil && a.proj.Scene != nil && a.proj.Scene.NitroFrames > 0 {
			return fmt.Sprintf("加速中 · 红色火焰 + %d 帧火焰环", a.proj.Scene.NitroFrames)
		}
		return "加速中 · 红色火焰"
	default:
		return "已就绪 · 橙色火焰"
	}
}

func (a *app) drawNitro(cp *ui.Painter, icon, seq SceneRect) {
	if a.proj == nil || a.proj.Scene == nil || icon.W <= 0 || icon.H <= 0 {
		return
	}
	st := a.nitroState()
	seqDir := a.proj.Scene.NitroSeqDir
	if seqDir == "" {
		seqDir = "dashboard/style/regular/Nitrogen/seq/left"
	}
	if st == "boost" && seq.W > 0 && seq.H > 0 && a.proj.Scene.NitroFrames > 0 {
		frame := 0
		if a.sceneAnim {
			frame = int(a.scenePhase/0.033) % a.proj.Scene.NitroFrames
		}
		a.drawBitmapAt(cp, fmt.Sprintf("%s/%d.png", seqDir, frame),
			ui.Rect{X: float32(seq.X), Y: float32(seq.Y), W: float32(seq.W), H: float32(seq.H)}, ui.FillBox)
	}
	num := 1
	switch st {
	case "ready":
		num = 2
	case "boost":
		num = 3
	}
	iconDir := path.Dir(path.Dir(seqDir)) + "/icon"
	r := ui.Rect{X: float32(icon.X), Y: float32(icon.Y), W: float32(icon.W), H: float32(icon.H)}
	if !a.drawBitmapAt(cp, fmt.Sprintf("%s/%d.png", iconDir, num), r, ui.FillBox) {
		a.drawBitmapAt(cp, fmt.Sprintf("dashboard/style/regular/Nitrogen/icon/%d.png", num), r, ui.FillBox)
	}
}

func (a *app) drawSpeed(cp *ui.Painter, w *Widget) {
	box := a.widgetDrawRect(w)
	speed := a.renderSpeed
	if a.imperial {
		speed = int(math.Round(float64(speed) * 0.621371))
	}
	text := strconv.Itoa(speed)
	if speed > 299 {
		text = "299"
	}
	a.drawDigitsCenter(cp, "dashboard/style/speedmeter", text, box, 0)
}

func (a *app) drawClock(cp *ui.Painter, w *Widget) {
	box := a.widgetDrawRect(w)
	var digits []rune
	for _, r := range a.renderClock {
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
	col := a.assetColor("dashboard/style/timepiece/0.png")
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
		rel := "dashboard/style/timepiece/" + string(ch) + ".png"
		b := a.bitmap(rel)
		if b == nil {
			continue
		}
		bw, bh := b.Size()
		ih := box.H
		iw := float32(bw) * ih / float32(maxInt(1, bh))
		cp.Image(b, ui.Rect{X: sx + (slotW-iw)/2, Y: box.Y, W: iw, H: ih}, ui.FillBox)
	}
}

func (a *app) drawDigitsCenter(cp *ui.Painter, folder, text string, box ui.Rect, spacing float32) {
	type glyph struct {
		b *ui.Bitmap
		w float32
	}
	var glyphs []glyph
	total := float32(0)
	for _, ch := range text {
		b := a.bitmap(digitPNG(folder, ch))
		if b == nil {
			continue
		}
		bw, bh := b.Size()
		w := float32(bw) * box.H / float32(maxInt(1, bh))
		glyphs = append(glyphs, glyph{b: b, w: w})
		total += w
	}
	if len(glyphs) > 1 {
		total += spacing * float32(len(glyphs)-1)
	}
	x := box.X + (box.W-total)/2
	for _, g := range glyphs {
		cp.Image(g.b, ui.Rect{X: x, Y: box.Y, W: g.w, H: box.H}, ui.FillBox)
		x += g.w + spacing
	}
}

func (a *app) drawDigitsRight(cp *ui.Painter, folder, text string, box ui.Rect, spacing float32) {
	type glyph struct {
		b *ui.Bitmap
		w float32
	}
	var glyphs []glyph
	total := float32(0)
	for _, ch := range text {
		b := a.bitmap(digitPNG(folder, ch))
		if b == nil {
			continue
		}
		bw, bh := b.Size()
		w := float32(bw) * box.H / float32(maxInt(1, bh))
		glyphs = append(glyphs, glyph{b: b, w: w})
		total += w
	}
	if len(glyphs) > 1 {
		total += spacing * float32(len(glyphs)-1)
	}
	x := box.X + box.W - total
	for _, g := range glyphs {
		cp.Image(g.b, ui.Rect{X: x, Y: box.Y, W: g.w, H: box.H}, ui.FillBox)
		x += g.w + spacing
	}
}

func digitPNG(folder string, ch rune) string {
	switch ch {
	case ':':
		return folder + "/12.png"
	case '-':
		return folder + "/10.png"
	case '.':
		return folder + "/11.png"
	}
	return folder + "/" + string(ch) + ".png"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (a *app) hitItems() []hitItem {
	p := a.proj
	if p == nil {
		return nil
	}
	var items []hitItem
	add := func(name, label, folder string) {
		w := p.Widget(name)
		if w == nil || w.W <= 0 || w.H <= 0 {
			return
		}
		items = append(items, hitItem{Name: name, Label: label, Folder: folder,
			X: w.X, Y: w.Y, W: w.W, H: w.H})
	}
	if w := p.Widget("main_win"); w != nil {
		items = append(items, hitItem{Name: "main_win", Label: "仪表背景", Folder: "dashboard/style",
			X: w.X, Y: w.Y, W: w.W, H: w.H})
	} else {
		bg := p.backgroundRel(a.mode)
		folder := path.Dir(bg)
		items = append(items, hitItem{Name: "main_win", Label: "仪表背景", Folder: folder,
			X: 0, Y: 0, W: themeW, H: themeH})
	}
	add("lv_ridedata", "骑行数据", "dashboard/style/ridedata")
	add("soc_meter", "电量进度条", "dashboard/style/soc")
	add("btn_soc", "电量数字", "dashboard/style/soc")
	add("btn_soc_unit", "电量单位", "dashboard/style/soc")
	gear := a.gear
	if a.page == pageScenes {
		gear = a.renderGear
	}
	if gear != "P" {
		add("speed_unit", "速度单位", "dashboard/style/speedmeter")
		add("speedmeter", "车速数字", "dashboard/style/speedmeter")
		add("btn_gear", "档位数字", "dashboard/style/gearmeter")
	} else {
		add("btn_gearP", "P 档", "dashboard/style/gearmeter")
	}
	add("mainwin_time", "时间", "dashboard/style/timepiece")
	add("btn_ecu_icon", "ECU 温度图标", "dashboard/style/regular/Temp")
	add("btn_ecu_temp", "ECU 温度数字", "dashboard/style/num")
	add("btn_ecu_unit", "ECU 温度单位", "dashboard/style/regular/Temp")
	add("btn_motor_icon", "电机温度图标", "dashboard/style/regular/Temp")
	add("btn_motor_temp", "电机温度数字", "dashboard/style/num")
	add("btn_motor_unit", "电机温度单位", "dashboard/style/regular/Temp")
	add("weather_bg", "天气背景", "dashboard/style/nightime/weather")
	add("btn_weather_text", "天气文字", "dashboard/style/nightime/weather/name_zh")
	add("btn_airtemp", "气温", "dashboard/style/nightime/weather/temp")
	add("btn_tempunit", "气温单位", "dashboard/style/nightime/weather/temp")
	add("btn_date", "日期", "dashboard/style/date")
	add("btn_week_text", "星期", "dashboard/style/week")
	add("btn_compass_icon", "指南针", "dashboard/style/compass")
	add("btn_compass_text", "指南针方向", "dashboard/style/compass/text")
	add("btn_compass_value", "指南针角度", "dashboard/style/num")
	add("btn_compass_unit", "指南针单位", "dashboard/style/compass/icon")
	add("btn_altitude_text", "海拔标题", "dashboard/style/altitude/text")
	add("btn_altitude_value", "海拔数字", "dashboard/style/num")
	add("btn_altitude_unit", "海拔单位", "dashboard/style/altitude/icon")
	add("power_title", "功率标题", "dashboard/style/nightime/power")
	add("power_num", "功率数字", "dashboard/style/nightime/power/green")
	add("power_unit", "功率单位", "dashboard/style/nightime/power")
	seqDir := ""
	if a.proj.Scene != nil {
		seqDir = a.proj.Scene.NitroSeqDir
	}
	if seqDir == "" {
		seqDir = "dashboard/style/regular/Nitrogen/seq/left"
	}
	add("n2_left_warn_0", "氮气火焰环", seqDir)
	add("n2_icon_0", "氮气图标", path.Dir(path.Dir(seqDir))+"/icon")
	return items
}

func (a *app) itemByName(name string) (hitItem, bool) {
	for _, it := range a.hitItems() {
		if it.Name == name {
			return it, true
		}
	}
	return hitItem{}, false
}

func (a *app) hitTest(x, y float32) (hitItem, bool) {
	items := a.hitItems()
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		if x >= float32(it.X) && y >= float32(it.Y) &&
			x <= float32(it.X+it.W) && y <= float32(it.Y+it.H) {
			return it, true
		}
	}
	return hitItem{}, false
}

func (a *app) dragOffset(name string) (int, int) {
	if a.drag.Active && a.drag.Name == name {
		return a.drag.DX, a.drag.DY
	}
	return 0, 0
}

func (a *app) drawOverlay(p *ui.Painter, r ui.Rect, t *ui.Theme) {
	if a.hideSelection {
		return
	}
	scale := r.W / themeW
	if scale <= 0 {
		return
	}
	if a.hover != "" && a.hover != a.sel && !a.drag.Active {
		if it, ok := a.itemByName(a.hover); ok {
			hr := ui.Rect{X: r.X + float32(it.X)*scale, Y: r.Y + float32(it.Y)*scale,
				W: float32(it.W) * scale, H: float32(it.H) * scale}
			p.Stroke(hr, t.Text.Alpha(0.55), 2, 1)
		}
	}
	if a.sel == "" {
		return
	}
	it, ok := a.itemByName(a.sel)
	if !ok {
		return
	}
	dx, dy := a.dragOffset(a.sel)
	x := it.X + dx
	y := it.Y + dy
	box := ui.Rect{X: r.X + float32(x)*scale, Y: r.Y + float32(y)*scale,
		W: float32(it.W) * scale, H: float32(it.H) * scale}
	if box.W < 1 || box.H < 1 {
		return
	}
	p.Stroke(box, t.Accent, 0, 1.5)
	// Corner marks.
	const s = 5
	for _, pt := range [][2]float32{
		{box.X, box.Y}, {box.X + box.W, box.Y},
		{box.X, box.Y + box.H}, {box.X + box.W, box.Y + box.H},
	} {
		p.Fill(ui.Rect{X: pt[0] - s/2, Y: pt[1] - s/2, W: s, H: s}, t.Accent, 1)
	}
	label := it.Label + "  " + strconv.Itoa(x) + ", " + strconv.Itoa(y)
	tw, th := p.MeasureText(0, ui.Span{Text: label, Size: 11, Weight: 500, Color: t.AccentText})
	chip := ui.Rect{X: box.X, Y: box.Y - th - 8, W: tw + 12, H: th + 6}
	if chip.Y < r.Y+4 {
		chip.Y = box.Y + box.H + 6
	}
	if chip.Y+chip.H > r.Y+r.H-4 {
		chip.Y = r.Y + 4
	}
	if chip.X < r.X+4 {
		chip.X = r.X + 4
	}
	if chip.X+chip.W > r.X+r.W-4 {
		chip.X = r.X + r.W - chip.W - 4
	}
	p.Fill(chip, t.Accent, 4)
	p.Text(chip.X+6, chip.Y+3, label, 11, t.AccentText)
}

// handlePreviewInput maps pointer gestures to the 1024×600 canvas.
func (a *app) handlePreviewInput(ev ui.InputEvent) bool {
	if a.proj == nil || a.previewScale <= 0 {
		return false
	}
	toTheme := func(x, y float32) (float32, float32) {
		return x / a.previewScale, y / a.previewScale
	}
	tx, ty := toTheme(ev.X, ev.Y)
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		it, ok := a.hitTest(tx, ty)
		if !ok {
			a.sel = ""
			a.hover = ""
			return true
		}
		a.sel = it.Name
		a.hover = it.Name
		if ev.Clicks >= 2 {
			a.openAssetFolder(it.Folder)
			return true
		}
		if it.Name == "main_win" {
			return true
		}
		a.drag = dragState{Active: true, Name: it.Name, StartX: ev.X, StartY: ev.Y,
			BaseX: it.X, BaseY: it.Y, Start: ev.Time.UnixNano()}
		return true
	case ui.InputPointerMove:
		if a.drag.Active {
			dx := int(math.Round(float64((ev.X - a.drag.StartX) / a.previewScale)))
			dy := int(math.Round(float64((ev.Y - a.drag.StartY) / a.previewScale)))
			if dx != a.drag.DX || dy != a.drag.DY {
				a.drag.DX, a.drag.DY = dx, dy
				if absInt(dx) > 1 || absInt(dy) > 1 {
					a.drag.Moved = true
				}
				a.invalidatePreview()
			}
			return true
		}
		if it, ok := a.hitTest(tx, ty); ok {
			if a.hover != it.Name {
				a.hover = it.Name
			}
		} else if a.hover != "" {
			a.hover = ""
		}
		return true
	case ui.InputPointerUp:
		if a.drag.Active {
			d := a.drag
			a.drag = dragState{}
			if d.Moved && ev.Button == 0 && !ev.Canceled {
				w := a.proj.Widget(d.Name)
				if w != nil {
					nx := clampInt(d.BaseX+d.DX, -w.W+20, themeW-20)
					ny := clampInt(d.BaseY+d.DY, -w.H+20, themeH-20)
					oldX, oldY := w.X, w.Y
					if a.proj.MoveWidget(d.Name, nx, ny) {
						a.recordLayoutEdit(d.Name, oldX, oldY, nx, ny)
						a.markLayoutChanged()
					}
					a.invalidatePreview()
				}
			}
			return true
		}
	}
	return false
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (a *app) markLayoutChanged() {
	a.layoutEdits++
	a.unsaved = true
	a.previewVer++
}

func (a *app) nudgeSelection(dx, dy int) {
	if a.sel == "" || a.sel == "main_win" || a.proj == nil {
		return
	}
	w := a.proj.Widget(a.sel)
	if w == nil {
		return
	}
	nx := clampInt(w.X+dx, -w.W+20, themeW-20)
	ny := clampInt(w.Y+dy, -w.H+20, themeH-20)
	oldX, oldY := w.X, w.Y
	if a.proj.MoveWidget(a.sel, nx, ny) {
		a.recordLayoutEdit(a.sel, oldX, oldY, nx, ny)
		a.markLayoutChanged()
		a.invalidatePreview()
	}
}

func (a *app) resetSelectedWidget() {
	if a.sel == "" || a.sel == "main_win" || a.proj == nil {
		return
	}
	if w := a.proj.Widget(a.sel); w != nil {
		if orig, ok := a.proj.OriginalPos[a.sel]; ok {
			oldX, oldY := w.X, w.Y
			if a.proj.MoveWidget(a.sel, orig.X, orig.Y) {
				a.recordLayoutEdit(a.sel, oldX, oldY, orig.X, orig.Y)
				a.markLayoutChanged()
				a.invalidatePreview()
			}
		}
	}
}
