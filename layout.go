package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Point struct{ X, Y int }

// Widget is one control block of dashboard/etc/layout.ini. Only the
// controls the preview can show are addressed by name.
type Widget struct {
	Name    string
	Type    string
	Style   string
	Caption string
	X, Y    int
	W, H    int
	Block   string
}

var (
	blockStartRe = regexp.MustCompile(`(?m)^[ \t]*([A-Za-z0-9_]+)[ \t]*=[ \t]*\{`)
	typeRe       = regexp.MustCompile(`(?m)^[ \t]*type[ \t]*=[ \t]*"([^"]*)"`)
	styleRe      = regexp.MustCompile(`(?m)^[ \t]*style[ \t]*=[ \t]*"([^"]*)"`)
	captionRe    = regexp.MustCompile(`(?m)^[ \t]*caption[ \t]*=[ \t]*"([^"]*)"`)
	xywhRe       = regexp.MustCompile(`x[ \t]*=[ \t]*(-?\d+)[ \t]*;[ \t]*y[ \t]*=[ \t]*(-?\d+)[ \t]*;[ \t]*w[ \t]*=[ \t]*(\d+)[ \t]*;[ \t]*h[ \t]*=[ \t]*(\d+)`)
)

func parseWidgets(text string) map[string]*Widget {
	out := map[string]*Widget{}
	for _, loc := range blockStartRe.FindAllStringSubmatchIndex(text, -1) {
		name := text[loc[2]:loc[3]]
		open := strings.IndexByte(text[loc[0]:loc[1]], '{') + loc[0]
		closeAt := matchingBrace(text, open)
		if closeAt < 0 {
			continue
		}
		block := text[open+1 : closeAt]
		w := &Widget{Name: name, Block: block}
		if m := xywhRe.FindStringSubmatchIndex(block); m != nil {
			w.X = atoi(block[m[2]:m[3]])
			w.Y = atoi(block[m[4]:m[5]])
			w.W = atoi(block[m[6]:m[7]])
			w.H = atoi(block[m[8]:m[9]])
		} else {
			var okX, okY, okW, okH bool
			w.X, okX = intField(block, "x")
			w.Y, okY = intField(block, "y")
			w.W, okW = intField(block, "w")
			w.H, okH = intField(block, "h")
			if !okX || !okY || !okW || !okH {
				continue
			}
		}
		if sm := typeRe.FindStringSubmatch(block); sm != nil {
			w.Type = sm[1]
		}
		if sm := styleRe.FindStringSubmatch(block); sm != nil {
			w.Style = sm[1]
		}
		if sm := captionRe.FindStringSubmatch(block); sm != nil {
			w.Caption = sm[1]
		}
		if _, exists := out[name]; !exists {
			out[name] = w
		}
	}
	return out
}

func intField(block, key string) (int, bool) {
	re := regexp.MustCompile(`(?m)^[ 	]*` + key + `[ 	]*=[ 	]*(-?\d+)`)
	m := re.FindStringSubmatch(block)
	if m == nil {
		return 0, false
	}
	return atoi(m[1]), true
}

func matchingBrace(text string, open int) int {
	depth := 0
	inString := false
	comment := false
	for i := open; i < len(text); i++ {
		ch := text[i]
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
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

var (
	xLineRe = regexp.MustCompile(`(?m)^([ \t]*x[ \t]*=[ \t]*)(-?\d+)`)
	yLineRe = regexp.MustCompile(`(?m)^([ \t]*y[ \t]*=[ \t]*)(-?\d+)`)
)

func moveWidgetInText(text, name string, x, y int) (string, bool) {
	re := regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(name) + `[ \t]*=[ \t]*\{`)
	loc := re.FindStringIndex(text)
	if loc == nil {
		return text, false
	}
	open := strings.IndexByte(text[loc[0]:loc[1]], '{') + loc[0]
	closeAt := matchingBrace(text, open)
	if closeAt < 0 {
		return text, false
	}
	block := text[open+1 : closeAt]
	if m := xywhRe.FindStringSubmatchIndex(block); m != nil {
		newXY := fmt.Sprintf("x=%d;y=%d;w=%s;h=%s", x, y, block[m[6]:m[7]], block[m[8]:m[9]])
		block = block[:m[0]] + newXY + block[m[1]:]
		return text[:open+1] + block + text[closeAt:], true
	}
	xm := xLineRe.FindStringSubmatchIndex(block)
	ym := yLineRe.FindStringSubmatchIndex(block)
	if xm == nil || ym == nil {
		return text, false
	}
	// Replace in reverse offset order so earlier indices stay valid.
	if xm[0] < ym[0] {
		block = block[:ym[4]] + strconv.Itoa(y) + block[ym[5]:]
		block = block[:xm[4]] + strconv.Itoa(x) + block[xm[5]:]
	} else {
		block = block[:xm[4]] + strconv.Itoa(x) + block[xm[5]:]
		block = block[:ym[4]] + strconv.Itoa(y) + block[ym[5]:]
	}
	return text[:open+1] + block + text[closeAt:], true
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func widgetLabel(name string) string {
	labels := map[string]string{
		"main_win":      "仪表背景",
		"speedmeter":    "车速数字",
		"speed_unit":    "速度单位",
		"gearP":         "P 档",
		"btn_gearP":     "P 档",
		"btn_gear":      "档位数字",
		"mainwin_time":  "时间",
		"soc_meter":     "电量进度条",
		"btn_soc":       "电量数字",
		"btn_soc_unit":  "电量单位",
		"lv_ridedata":   "里程条",
		"btn_date":      "日期",
		"btn_weather":   "天气",
		"weather_bg":    "天气背景",
		"btn_compass":   "指南针",
		"speed_stop":    "停车标志",
		"low_soc_icon":  "低电量图标",
		"btn_airtemp":   "气温数字",
		"btn_tempunit":  "温度单位",
		"btn_week_text": "星期",
	}
	if s, ok := labels[name]; ok {
		return s
	}
	return name
}
