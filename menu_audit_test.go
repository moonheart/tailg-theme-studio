package main

import (
	"os"
	"strings"
	"testing"
)

func TestMenuPageGeometry(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("sample theme not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)

	want := map[string][2]int{
		"dash_set":                     {128, 191},
		"abs":                          {128, 191},
		"TcsSet":                       {128, 191},
		"DayOrNight":                   {128, 191},
		"ride_mode":                    {128, 191},
		"time":                         {128, 191},
		"maintain":                     {128, 191},
		"dvr/set":                      {160, 249},
		"dvr/set/quality":              {160, 249},
		"dvr/set/camera":               {160, 249},
		"car_set/set/SiderubInduction": {364, 110},
		"BTConnect":                    {364, 110},
		"tpms/tire_disp":               {277, 90},
	}
	seen := map[string]bool{}
	for _, pg := range p.Scene.MenuPages {
		if xy, ok := want[pg.Key]; ok {
			seen[pg.Key] = true
			if pg.Window.X != xy[0] || pg.Window.Y != xy[1] {
				t.Errorf("%s window = (%d,%d), want (%d,%d)", pg.Key, pg.Window.X, pg.Window.Y, xy[0], xy[1])
			}
		}
		// All child pages must inherit their parent window origin.
		if strings.HasPrefix(pg.Key, "dvr/set/") && (pg.Window.X != 160 || pg.Window.Y != 249) {
			t.Errorf("%s window = (%d,%d), want parent origin (160,249)", pg.Key, pg.Window.X, pg.Window.Y)
		}
		if strings.HasPrefix(pg.Key, "car_set/set/") && (pg.Window.X != 364 || pg.Window.Y != 110) {
			t.Errorf("%s window = (%d,%d), want parent origin (364,110)", pg.Key, pg.Window.X, pg.Window.Y)
		}
		// The 768×218 pages are 0,0 local spaces and must be centered.
		if pg.Window.W == 768 && pg.Window.H == 218 && (pg.Window.X != 128 || pg.Window.Y != 191) {
			t.Errorf("%s window = (%d,%d), want centered (128,191)", pg.Key, pg.Window.X, pg.Window.Y)
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("page %q missing", key)
		}
	}
}

func TestOptionPagesAreOnlyShortRows(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("sample theme not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)

	want := map[string]bool{
		"abs": true, "TcsSet": true, "DayOrNight": true, "ride_mode": true, "time": true,
		"maintain": false, "dvr/set": false, "dvr/set/quality": false,
	}
	seen := map[string]bool{}
	for _, pg := range p.Scene.MenuPages {
		if expected, ok := want[pg.Key]; ok {
			seen[pg.Key] = true
			_, got := optionPageRect(&pg)
			if got != expected {
				t.Errorf("%s option page = %v, want %v (list=%d win=%+v)", pg.Key, got, expected, len(pg.List), pg.Window)
			}
		}
	}
	for key := range want {
		if !seen[key] {
			t.Errorf("page %q missing", key)
		}
	}
}

func TestCompactRectWithSpaces(t *testing.T) {
	body := `
		type = "button"
		x = 232;y = 10;w = 240;h = 40
		style="middle_28"
		flags={
			no_event
		}
	`
	w, ok := parseMenuWidget("quality_topical", body)
	if !ok {
		t.Fatal("compact rect with spaces was not parsed")
	}
	if w.Rect.X != 232 || w.Rect.Y != 10 || w.Rect.W != 240 || w.Rect.H != 40 {
		t.Fatalf("rect = %+v, want (232,10,240,40)", w.Rect)
	}
}

func TestNestedWindowOffsetsAndHiddenState(t *testing.T) {
	if _, err := os.Stat(sampleTheme); err != nil {
		t.Skip("sample theme not present")
	}
	p, err := openProject(sampleTheme)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(p.TempDir)

	var maintain, timePage, dvrList *menuPage
	for i := range p.Scene.MenuPages {
		switch p.Scene.MenuPages[i].Key {
		case "maintain":
			maintain = &p.Scene.MenuPages[i]
		case "time":
			timePage = &p.Scene.MenuPages[i]
		case "dvr/list":
			dvrList = &p.Scene.MenuPages[i]
		}
	}
	if maintain == nil || timePage == nil || dvrList == nil {
		t.Fatal("missing maintain/time/dvr-list page")
	}

	preview := findMenuWidget(dvrList.Widgets, "btn_show_pic")
	if preview == nil || !preview.Hidden {
		t.Fatalf("btn_show_pic hidden = %v, want true", preview != nil && preview.Hidden)
	}

	save := findMenuWidget(maintain.Widgets, "win_save_maintain")
	if save == nil || !save.Hidden {
		t.Fatalf("win_save_maintain hidden = %v, want true", save != nil && save.Hidden)
	}

	op := findMenuWidget(timePage.Widgets, "time_operate_win")
	if op == nil || len(op.Children) != 5 {
		t.Fatalf("time_operate_win children = %d, want 5", len(op.Children))
	}
	year := op.Children[0]
	if year.Type != "window" || year.Rect.X != 0 || year.Rect.W != 102 {
		t.Fatalf("year group = %+v", year.Rect)
	}
	digits := 0
	for _, w := range year.Children {
		if w.Style == "time_style_num" {
			digits++
		}
	}
	if digits != 4 {
		t.Fatalf("year digits = %d, want 4", digits)
	}
	// The editor is a child window; its global origin must be the page
	// origin plus the parent offsets, not a hardcoded screen position.
	gx := timePage.Window.X + op.Rect.X + year.Rect.X
	gy := timePage.Window.Y + op.Rect.Y + year.Rect.Y
	if gx != 332 || gy != 347 {
		t.Fatalf("time editor origin = (%d,%d), want (332,347)", gx, gy)
	}
}
