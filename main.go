package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	a := &app{
		speed:           68,
		soc:             78,
		gear:            "3",
		clock:           "0941",
		page:            pageDashboard,
		scene:           sceneDash,
		sceneAnim:       true,
		sceneSpeed:      1,
		sceneEpoch:      time.Now(),
		menuLang:        "zh_CN",
		botState:        "normal",
		menuPageKey:     "brightness",
		popupType:       "bt_msg",
		nitroMode:       "auto",
		ecuTemp:         36,
		motorTemp:       42,
		weatherID:       1,
		airTemp:         24,
		compassDeg:      45,
		altitude:        128,
		powerKW:         1.8,
		ridePage:        "auto",
		chargePower:     0.6,
		chargeVoltage:   54.2,
		adasMode:        "none",
		particleEnabled: true,
		chargeState:     1,
		dtcCode:         44,
		naviIcon:        1,
		naviArrow:       0,
		naviLanes:       2,
		naviLaneBg:      1,
	}
	a.settings = loadPrefs()
	a.recent = a.settings.Recent
	a.zoom = a.settings.Zoom
	a.mode = a.settings.Mode
	if a.mode < 0 || a.mode > 3 {
		a.mode = 0
	}
	a.showGrid = a.settings.ShowGrid
	if a.zoom < 0 || a.zoom > 1.5 {
		a.zoom = 0
	}
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		a.startup = os.Args[1]
	}

	mygo.App.WhenReady(func() {
		a.win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "Theme Studio · 台铃仪表主题编辑器",
			Width:     1440,
			Height:    920,
			MinWidth:  1180,
			MinHeight: 760,
			StateKey:  "theme-studio",
			Content:   ui.View(a.view),
		})
		a.win.OnClose(func(e *mygo.CloseEvent) {
			if !a.unsaved || a.forceClose {
				return
			}
			e.PreventDefault()
			a.confirmDiscard("放弃未保存的修改？", "当前主题有尚未导出的修改，确定要退出吗？", func() {
				a.forceClose = true
				a.win.Close()
			})
		})
		a.win.OnClosed(func() {
			a.stopParticle()
			if a.proj != nil {
				os.RemoveAll(a.proj.TempDir)
			}
		})
		a.win.OnFileDrop(func(e *mygo.FileDropEvent) {
			if len(e.Paths) > 0 {
				a.requestOpenPath(e.Paths[0])
			}
		})
		a.win.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Label: "文件", Submenu: []*mygo.MenuItem{
				{ID: "open", Label: "打开主题…", Accelerator: "CmdOrCtrl+O", Click: func(*mygo.MenuItem, *mygo.Window) { a.openDialog() }},
				{ID: "open-folder", Label: "打开文件夹…", Click: func(*mygo.MenuItem, *mygo.Window) { a.openFolderDialog() }},
				mygo.Separator(),
				{ID: "quick-export", Label: "快速导出", Accelerator: "CmdOrCtrl+Shift+S", Click: func(*mygo.MenuItem, *mygo.Window) { a.quickExport() }},
				{ID: "export", Label: "导出为…", Accelerator: "CmdOrCtrl+S", Click: func(*mygo.MenuItem, *mygo.Window) { a.exportDialog() }},
				mygo.Separator(),
				{Role: mygo.RoleQuit},
			}},
			{Label: "编辑", Submenu: []*mygo.MenuItem{
				{Role: mygo.RoleUndo},
				{Role: mygo.RoleRedo},
				mygo.Separator(),
				{Role: mygo.RoleCut},
				{Role: mygo.RoleCopy},
				{Role: mygo.RolePaste},
				{Role: mygo.RoleSelectAll},
			}},
		}))
		a.launchOpen()
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
