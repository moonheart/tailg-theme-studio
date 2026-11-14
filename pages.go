package main

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"
)

// ---- Dashboard inspector ---------------------------------------------------

func (a *app) dashboardInspector(c *ui.Context) {
	t := c.Theme()
	if a.proj == nil {
		section(c, "开始", "打开一个主题包后，这里会显示选中元素的位置和可替换的图片。", func() {
			ui.Text(c, "编辑不会修改原包。导出时会重新计算 MD5、包大小和文件数。").FontSize(10.5).LineHeight(1.55).TextColor(t.TextMuted)
		})
		return
	}
	if a.sel != "" {
		if it, ok := a.itemByName(a.sel); ok {
			a.selectedElementCard(c, it)
		}
	} else {
		section(c, "仪表盘", "在预览中点选一个元素，或从下面的列表选择。", func() {
			ui.Text(c, "拖动元素调整位置；方向键 1px，Shift + 方向键 10px，Ctrl+Z 撤销。").FontSize(10).LineHeight(1.5).TextColor(t.TextMuted)
		})
	}

	ui.Row(c).FillWidth().Padding(3).Radius(8).Background(t.Background).Gap(4).Children(func() {
		tabs := []string{"布局", "灯光", "附加"}
		for idx, label := range tabs {
			i := idx
			btn := ui.Row(c).Key(fmt.Sprintf("dash-tab-%d", i)).Role(ui.RoleButton).Label(label).
				Grow(1).Height(28).Radius(6).Center()
			if a.dashTab == i {
				btn.Background(t.Surface).Border(1, t.Border)
			} else if btn.Hovered() {
				btn.Background(t.Surface.Alpha(0.5))
			}
			btn.Children(func() {
				col := t.TextMuted
				if a.dashTab == i {
					col = t.Text
				}
				ui.Text(c, label).FontSize(10.5).Bold().TextColor(col)
			})
			btn.OnClick(func() {
				a.dashTab = i
				if a.win != nil {
					a.win.Invalidate()
				}
			})
		}
	})

	switch a.dashTab {
	case 0:
		section(c, "可见元素", "这些控件会出现在实时预览里，位置来自 layout.ini。", func() {
			for _, it := range a.hitItems() {
				if it.Name == "main_win" {
					continue
				}
				row := ui.Row(c).Key("select-"+it.Name).Role(ui.RoleButton).Label(it.Label).Height(30).FillWidth().Padding(0, 6).Gap(7).Radius(5).AlignItems(ui.Center)
				if a.sel == it.Name {
					row.Background(t.Accent.Alpha(0.13))
				} else if row.Hovered() {
					row.Background(t.Background)
				}
				row.Children(func() {
					ui.Text(c, it.Label).FontSize(10.5)
					ui.Spacer(c)
					ui.Text(c, fmt.Sprintf("%d, %d", it.X, it.Y)).FontSize(9.5).TextColor(t.TextMuted)
				})
				row.OnClick(func() {
					a.sel = it.Name
					a.win.Invalidate()
				})
			}
		})

		section(c, "画布与历史", "", func() {
			ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					c.Key("grid-switch-2")
					if ui.Switch(c, &a.showGrid).Changed() {
						a.saveSettings()
					}
					ui.Text(c, "点阵网格").FontSize(10)
				})
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					c.Key("hide-sel-switch")
					if ui.Switch(c, &a.hideSelection).Changed() {
						a.invalidatePreview()
					}
					ui.Text(c, "隐藏选框 (H)").FontSize(10)
				})
			})
			ui.Row(c).Gap(8).Children(func() {
				undoLabel := "撤销 (Ctrl+Z)"
				if len(a.layoutUndo) > 0 {
					undoLabel = fmt.Sprintf("撤销 (%d)", len(a.layoutUndo))
				}
				ui.Button(c, undoLabel).Key("layout-undo").Grow(1).OnClick(a.undoLayout)
				redoLabel := "重做"
				if len(a.layoutRedo) > 0 {
					redoLabel = fmt.Sprintf("重做 (%d)", len(a.layoutRedo))
				}
				ui.Button(c, redoLabel).Key("layout-redo").Grow(1).OnClick(a.redoLayout)
			})
			ui.Button(c, "复位全部控件位置").Key("reset-all-layout").OnClick(a.resetAllLayout)
			ui.Text(c, "只把 x/y 写回 layout.ini；宽高、样式和脚本逻辑保持不变。").FontSize(9.5).LineHeight(1.45).TextColor(t.TextMuted)
		})
	case 1:
		section(c, "自检 / 告警灯", "18 盏警示灯、4 个状态灯和 GPS 信号；编号来自 light.ini。", func() {
			a.lampGallery(c)
		})

		section(c, "仪表温度", "ECU 和电机温度显示在仪表底部，数值随循环播放轻微变化。", func() {
			a.tempControls(c)
		})
	case 2:
		section(c, "仪表附加", "天气、指南针、海拔、功率、骑行数据和充电浮窗。", func() {
			a.hudControls(c)
		})
	}
}

func (a *app) tempControls(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(8).Children(func() {
		ui.Column(c).Gap(3).Grow(1).Children(func() {
			ui.Text(c, "ECU °C").FontSize(9.5).TextColor(t.TextMuted)
			v := float64(a.ecuTemp)
			c.Key("ecu-temp")
			if ui.NumberInput(c, &v, -40, 150, 1).Changed() {
				a.ecuTemp = int(v)
				a.invalidatePreview()
			}
		})
		ui.Column(c).Gap(3).Grow(1).Children(func() {
			ui.Text(c, "电机 °C").FontSize(9.5).TextColor(t.TextMuted)
			v := float64(a.motorTemp)
			c.Key("motor-temp")
			if ui.NumberInput(c, &v, -40, 180, 1).Changed() {
				a.motorTemp = int(v)
				a.invalidatePreview()
			}
		})
	})
	ui.Text(c, "位置和图标来自 layout.ini 的 btn_ecu_* / btn_motor_* 与 style.rc；数字使用 dashboard/style/num。").FontSize(9).LineHeight(1.4).TextColor(t.TextMuted)
}

func (a *app) hudControls(c *ui.Context) {
	t := c.Theme()
	if a.particleAvailable() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			c.Key("particle-toggle")
			if ui.Switch(c, &a.particleEnabled).Changed() {
				if a.particleEnabled {
					a.loadParticle()
				} else {
					a.stopParticle()
				}
				a.invalidatePreview()
			}
			ui.Text(c, "烟花背景视频").FontSize(10)
			ui.Spacer(c)
			status := "静态背景"
			if a.particle != nil {
				status = "已加载 " + path.Base(a.particlePath)
				if a.mode != 0 {
					status += " · 动力 1 才播放"
				}
			}
			ui.Text(c, status).FontSize(9).TextColor(t.TextMuted)
		})
		if a.particleErr != "" {
			ui.Text(c, "视频打开失败，使用静态背景："+a.particleErr).FontSize(9).LineHeight(1.4).TextColor(t.Warning)
		}
	}
	ui.Row(c).Gap(8).Children(func() {
		ui.Column(c).Gap(3).Grow(1).Children(func() {
			ui.Text(c, "天气").FontSize(9.5).TextColor(t.TextMuted)
			labels := []string{"晴", "雨", "多云", "雪", "雷雨", "阴", "雾", "大风"}
			ids := []int{1, 2, 3, 4, 6, 7, 8, 9}
			cur := labels[0]
			for i, id := range ids {
				if id == a.weatherID {
					cur = labels[i]
				}
			}
			c.Key("hud-weather")
			if ui.Select(c, &cur, labels).Changed() {
				for i, l := range labels {
					if l == cur {
						a.weatherID = ids[i]
						a.invalidatePreview()
					}
				}
			}
		})
		ui.Column(c).Gap(3).Width(96).Children(func() {
			ui.Text(c, "气温 °C").FontSize(9.5).TextColor(t.TextMuted)
			v := float64(a.airTemp)
			c.Key("hud-airtemp")
			if ui.NumberInput(c, &v, -40, 60, 1).Changed() {
				a.airTemp = int(v)
				a.invalidatePreview()
			}
		})
	})
	ui.Row(c).Gap(8).Children(func() {
		ui.Column(c).Gap(3).Grow(1).Children(func() {
			ui.Text(c, "指南针 °").FontSize(9.5).TextColor(t.TextMuted)
			v := float64(a.compassDeg)
			c.Key("hud-compass")
			if ui.NumberInput(c, &v, 0, 359, 1).Changed() {
				a.compassDeg = int(v)
				a.invalidatePreview()
			}
		})
		ui.Column(c).Gap(3).Grow(1).Children(func() {
			ui.Text(c, "海拔 m").FontSize(9.5).TextColor(t.TextMuted)
			v := float64(a.altitude)
			c.Key("hud-altitude")
			if ui.NumberInput(c, &v, -500, 9000, 1).Changed() {
				a.altitude = int(v)
				a.invalidatePreview()
			}
		})
	})
	ui.Row(c).Gap(8).Children(func() {
		ui.Column(c).Gap(3).Width(104).Children(func() {
			ui.Text(c, "功率 kW").FontSize(9.5).TextColor(t.TextMuted)
			v := a.powerKW
			c.Key("hud-power")
			if ui.NumberInput(c, &v, -5, 20, 0.1).Changed() {
				a.powerKW = v
				a.invalidatePreview()
			}
		})
		ui.Column(c).Gap(3).Grow(1).Children(func() {
			ui.Text(c, "骑行数据").FontSize(9.5).TextColor(t.TextMuted)
			labels := []string{"自动轮播", "ODO", "TRIP", "平均速度", "骑行时间"}
			vals := []string{"auto", "odo", "trip", "speed", "time"}
			cur := labels[0]
			for i, v := range vals {
				if v == a.ridePage {
					cur = labels[i]
				}
			}
			c.Key("hud-ridepage")
			if ui.Select(c, &cur, labels).Changed() {
				for i, l := range labels {
					if l == cur {
						a.ridePage = vals[i]
						a.invalidatePreview()
					}
				}
			}
		})
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		c.Key("hud-charge")
		if ui.Switch(c, &a.showCharge).Changed() {
			a.invalidatePreview()
		}
		ui.Text(c, "显示充电浮窗").FontSize(10)
		ui.Spacer(c)
		c.Key("hud-gps")
		if ui.Switch(c, &a.showGPS).Changed() {
			a.invalidatePreview()
		}
		ui.Text(c, "GPS 信号").FontSize(10)
	})
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		c.Key("hud-hillhold")
		if ui.Switch(c, &a.hillHold).Changed() {
			a.invalidatePreview()
		}
		ui.Text(c, "坡道 H").FontSize(10)
		ui.Spacer(c)
		c.Key("hud-imperial")
		if ui.Switch(c, &a.imperial).Changed() {
			a.invalidatePreview()
		}
		ui.Text(c, "英制单位").FontSize(10)
	})
	ui.Row(c).Gap(8).Children(func() {
		ui.Column(c).Gap(3).Grow(1).Children(func() {
			ui.Text(c, "额定电压").FontSize(9.5).TextColor(t.TextMuted)
			labels := []string{"无", "36V", "48V", "60V", "64V", "72V", "error"}
			vals := []string{"", "36V", "48V", "60V", "64V", "72V", "error"}
			cur := labels[0]
			for i, v := range vals {
				if v == a.nominalVoltage {
					cur = labels[i]
				}
			}
			c.Key("hud-nominal")
			if ui.Select(c, &cur, labels).Changed() {
				for i, l := range labels {
					if l == cur {
						a.nominalVoltage = vals[i]
						a.invalidatePreview()
					}
				}
			}
		})
	})
	if a.showCharge {
		ui.Row(c).Gap(8).Children(func() {
			ui.Column(c).Gap(3).Grow(1).Children(func() {
				ui.Text(c, "充电功率 kW").FontSize(9.5).TextColor(t.TextMuted)
				v := a.chargePower
				c.Key("hud-charge-power")
				if ui.NumberInput(c, &v, 0, 5, 0.1).Changed() {
					a.chargePower = v
					a.invalidatePreview()
				}
			})
			ui.Column(c).Gap(3).Grow(1).Children(func() {
				ui.Text(c, "电压 V").FontSize(9.5).TextColor(t.TextMuted)
				v := a.chargeVoltage
				c.Key("hud-charge-voltage")
				if ui.NumberInput(c, &v, 0, 100, 0.1).Changed() {
					a.chargeVoltage = v
					a.invalidatePreview()
				}
			})
		})
		ui.Column(c).Gap(3).Children(func() {
			ui.Text(c, "充电状态").FontSize(9.5).TextColor(t.TextMuted)
			labels := []string{"充电中", "已充满", "异常"}
			cur := labels[0]
			if a.chargeState >= 1 && a.chargeState <= 3 {
				cur = labels[a.chargeState-1]
			}
			c.Key("hud-charge-state")
			if ui.Select(c, &cur, labels).Changed() {
				for i, l := range labels {
					if l == cur {
						a.chargeState = i + 1
						a.invalidatePreview()
					}
				}
			}
		})
	}
	ui.Row(c).Gap(8).Children(func() {
		ui.Column(c).Gap(3).Grow(1).Children(func() {
			ui.Text(c, "提示消息").FontSize(9.5).TextColor(t.TextMuted)
			labels := []string{"无", "坡道驻车", "防溜坡", "边撑", "坐垫"}
			ids := []int{0, 42, 43, 44, 45}
			cur := labels[0]
			for i, id := range ids {
				if id == a.hintID {
					cur = labels[i]
				}
			}
			c.Key("hud-hint")
			sel := ui.Select(c, &cur, labels)
			// ui.Select sets MinWidth(theme.Space(35)); clear it so two
			// selects fit side by side in the 320px inspector.
			sel.MinWidth(0).FillWidth()
			if sel.Changed() {
				for i, l := range labels {
					if l == cur {
						a.hintID = ids[i]
						a.invalidatePreview()
					}
				}
			}
		})
		ui.Column(c).Gap(3).Grow(1).Children(func() {
			ui.Text(c, "ADAS 警告").FontSize(9.5).TextColor(t.TextMuted)
			labels := []string{"无", "左侧", "右侧", "后方", "左右"}
			vals := []string{"none", "left", "right", "rear", "sides"}
			cur := labels[0]
			for i, v := range vals {
				if v == a.adasMode {
					cur = labels[i]
				}
			}
			c.Key("hud-adas")
			sel := ui.Select(c, &cur, labels)
			sel.MinWidth(0).FillWidth()
			if sel.Changed() {
				for i, l := range labels {
					if l == cur {
						a.adasMode = vals[i]
						a.invalidatePreview()
					}
				}
			}
		})
	})
	ui.Text(c, "位置来自 layout.ini；天气图标和名字来自 weather 目录，日期用 date 的数字图，星期用 week 的图片。").FontSize(9).LineHeight(1.4).TextColor(t.TextMuted)
}

var lampLabels = map[int]string{
	1: "远光", 3: "左转", 4: "右转", 10: "ABS", 21: "READY",
	27: "电机故障", 38: "云动力", 39: "喇叭", 40: "ECU", 41: "电机过温",
	43: "边撑", 50: "胎压", 52: "整车", 70: "自动大灯", 71: "警告",
	73: "TCS", 75: "信号", 104: "防滑",
}

var signalLabels = map[int]string{1: "手机", 2: "Wi-Fi", 10: "蓝牙", 12: "状态 12"}

func (a *app) lampGallery(c *ui.Context) {
	t := c.Theme()
	d := a.proj.Scene
	if d == nil {
		return
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		c.Key("lamps-preview")
		if ui.Switch(c, &a.showAllLamps).Changed() {
			a.invalidatePreview()
		}
		ui.Text(c, "在预览中显示全部").FontSize(10)
		ui.Spacer(c)
		ui.Text(c, fmt.Sprintf("%d 灯 + %d 状态", len(d.Page0Lamps), len(d.Page0Signal))).FontSize(9).TextColor(t.TextMuted)
	})
	ui.Grid(c).Columns(5).Gap(5).Children(func() {
		for _, l := range d.Page0Lamps {
			rel := d.LampPaths[l.ID]
			if rel == "" {
				rel = fmt.Sprintf("%s/%d.png", d.LampDir, l.ID)
			}
			a.lampTile(c, rel, lampLabel(l.ID))
		}
		for _, s := range d.Page0Signal {
			rel := d.SignalPaths[s.ID]
			if rel == "" {
				rel = fmt.Sprintf("%s/%d.png", d.SignalDir, s.ID)
			}
			a.lampTile(c, rel, signalLabel(s.ID))
		}
		a.lampTile(c, d.GPSDir+"/signal_4.png", "GPS")
	})
	ui.Text(c, "点一个图标选中，再替换它的图片；名称按图标和主题推断，编号对应 light.ini 的 id。").FontSize(9).LineHeight(1.4).TextColor(t.TextMuted)
	ui.Row(c).Gap(6).Children(func() {
		ui.Button(c, "打开告警灯目录").Key("open-lamps").Grow(1).OnClick(func() {
			a.openAssetFolder(d.LampDir)
		})
		if a.isLampAsset(a.selectedAsset) {
			ui.Button(c, "替换选中…").Key("replace-lamp").Grow(1).OnClick(a.replaceAssetDialog)
		}
	})
	if a.isLampAsset(a.selectedAsset) {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "已选 "+path.Base(a.selectedAsset)).FontSize(9).TextColor(t.TextMuted)
			ui.Spacer(c)
			if _, exists := a.proj.Original[a.selectedAsset]; exists {
				ui.Button(c, "还原").Key("revert-lamp").OnClick(func() { a.revertAsset(a.selectedAsset) })
			}
		})
	}
}

func (a *app) lampTile(c *ui.Context, rel, label string) {
	t := c.Theme()
	tile := ui.Column(c).Gap(2).Padding(3).Height(54).Radius(5).Role(ui.RoleButton).Label(label)
	if a.selectedAsset == rel {
		tile.Background(t.Accent.Alpha(0.16)).Border(1, t.Accent)
	} else if tile.Hovered() {
		tile.Background(t.Background)
	}
	tile.Children(func() {
		a.assetThumb(c, rel, 26)
		ui.Text(c, label).FontSize(8.5).LineHeight(1.1).TextAlign(ui.Center).MaxWidth(50).NoWrap()
	})
	tile.OnClick(func() {
		a.selectedAsset = rel
		a.win.Invalidate()
	})
}

func lampLabel(id int) string {
	if v := lampLabels[id]; v != "" {
		return v
	}
	return fmt.Sprintf("灯 %d", id)
}

func signalLabel(id int) string {
	if v := signalLabels[id]; v != "" {
		return v
	}
	return fmt.Sprintf("信号 %d", id)
}

func (a *app) isLampAsset(rel string) bool {
	if rel == "" || a.proj == nil || a.proj.Scene == nil {
		return false
	}
	for _, p := range a.proj.Scene.LampPaths {
		if p == rel {
			return true
		}
	}
	for _, p := range a.proj.Scene.SignalPaths {
		if p == rel {
			return true
		}
	}
	return strings.Contains(rel, "/lamps/") || strings.Contains(rel, "/signals/")
}

func (a *app) selectedElementCard(c *ui.Context, it hitItem) {
	t := c.Theme()
	if it.Name == "main_win" {
		bg := a.proj.backgroundRel(a.mode)
		section(c, "仪表背景", "动力模式 1–4 可以各用一张背景；当前编辑动力 "+fmt.Sprintf("%d", a.mode+1)+"。", func() {
			if bg != "" {
				a.assetThumb(c, bg, 160)
				kv(c, "当前文件", bg)
			}
			ui.Button(c, "替换当前背景…").Key("replace-background").OnClick(func() {
				if bg != "" {
					a.selectedAsset = bg
					a.replaceAssetDialog()
				}
			})
			ui.Text(c, "动力模式").FontSize(9.5).TextColor(t.TextMuted)
			ui.Row(c).Gap(6).Children(func() {
				for i := 1; i <= 4; i++ {
					idx := i - 1
					b := ui.Button(c, fmt.Sprintf("%d", i)).Key(fmt.Sprintf("bg-%d", i)).Grow(1)
					if a.mode == idx {
						b.Background(t.Accent.Alpha(0.15))
					}
					b.OnClick(func() {
						a.mode = idx
						a.selectedAsset = a.proj.backgroundRel(idx)
						a.invalidatePreview()
						a.saveSettings()
						a.win.Invalidate()
					})
				}
			})
			ui.Button(c, "打开背景图片目录").Key("bg-assets").OnClick(func() {
				a.openAssetFolder("dashboard/style/nightime/bg")
			})
			ui.Text(c, "背景必须按 1024×600 使用；导入其他尺寸时会自动裁切填满。").FontSize(9.5).LineHeight(1.45).TextColor(t.TextMuted)
		})
		return
	}
	section(c, "选中元素", it.Label+" · "+it.Name, func() {
		x := float64(it.X)
		y := float64(it.Y)
		changed := false
		fieldRow(c, func() {
			ui.Column(c).Gap(3).Grow(1).Children(func() {
				ui.Text(c, "X").FontSize(9.5).TextColor(t.TextMuted)
				c.Key("sel-x")
				if ui.NumberInput(c, &x, -500, 1400, 1).Changed() {
					changed = true
				}
			})
			ui.Column(c).Gap(3).Grow(1).Children(func() {
				ui.Text(c, "Y").FontSize(9.5).TextColor(t.TextMuted)
				c.Key("sel-y")
				if ui.NumberInput(c, &y, -500, 900, 1).Changed() {
					changed = true
				}
			})
		})
		if changed {
			nx, ny := int(x), int(y)
			if a.proj.MoveWidget(it.Name, nx, ny) {
				a.recordLayoutEdit(it.Name, it.X, it.Y, nx, ny)
				a.markLayoutChanged()
				a.invalidatePreview()
			}
		}
		kv(c, "宽度", fmt.Sprintf("%d px", it.W))
		kv(c, "高度", fmt.Sprintf("%d px", it.H))
		ui.Row(c).Gap(8).Children(func() {
			ui.Button(c, "复位位置").Key("reset-element").Grow(1).OnClick(a.resetSelectedWidget)
			ui.Button(c, "查看图片").Key("element-assets").Grow(1).OnClick(func() {
				a.openAssetFolder(it.Folder)
			})
		})
		if orig, done := a.proj.OriginalPos[it.Name]; done && (orig.X != it.X || orig.Y != it.Y) {
			ui.Text(c, "位置已修改").FontSize(9).TextColor(t.Warning)
		}
	})
}

// ---- Assets ----------------------------------------------------------------

func (a *app) assetsPage(c *ui.Context) {
	if a.proj == nil {
		a.welcome(c)
		return
	}
	t := c.Theme()
	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		ui.Row(c).FillWidth().Height(34).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "图片资源").FontSize(16).Bold()
			breadcrumb := a.assetTop + " / " + a.assetSub
			if a.assetTop == "全部" {
				breadcrumb = "全部图片"
			}
			ui.Text(c, breadcrumb).FontSize(10.5).TextColor(t.TextMuted)
			n := len(a.filteredAssets())
			if a.assetQuery != "" {
				ui.Text(c, fmt.Sprintf("找到 %d 张", n)).FontSize(10).TextColor(t.TextMuted)
			} else {
				ui.Text(c, fmt.Sprintf("%d 张", n)).FontSize(10).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			c.Key("asset-search")
			if ui.TextInput(c, &a.assetQuery).Placeholder("搜索当前分类…").Label("搜索图片").Width(220).Changed() {
				a.assetList = ui.ListState{}
				if a.win != nil {
					a.win.Invalidate()
				}
			}
			if a.assetQuery != "" {
				ui.Button(c, "清空").OnClick(func() {
					a.assetQuery = ""
					a.assetList = ui.ListState{}
					if a.win != nil {
						a.win.Invalidate()
					}
				})
			}
		})
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Gap(12).Children(func() {
			a.assetCategoryTree(c)
			ui.Box(c).Width(1).FillHeight().Background(t.Border)
			ui.Box(c).Grow(1).FillHeight().Clip().Children(func() {
				assets := a.filteredAssets()
				if len(assets) == 0 {
					ui.Column(c).Fill().Center().Gap(8).Children(func() {
						ic := ui.Icon(c, icImage)
						ic.FontSize(30).TextColor(t.TextMuted)
						msg := "这个分类里没有图片"
						if a.assetQuery != "" {
							msg = "当前分类里没有匹配的图片"
						}
						ui.Text(c, msg).FontSize(12).TextColor(t.TextMuted)
						ui.Text(c, "可以切到「全部图片」或清空搜索。").FontSize(10).TextColor(t.TextMuted)
					})
					return
				}
				// The center column loses 240 + 320 for the app rails,
				// 32 for page padding, 188 for the category tree and the
				// gaps/dividers. Compute columns from the real width so a
				// card that does not fit moves to the next row instead of
				// being clipped at the right edge.
				winW, _ := c.Size()
				gridW := winW - 807
				const minCard = 170
				cols := int((gridW + 10) / (minCard + 10))
				if cols < 2 {
					cols = 2
				}
				if cols > 6 {
					cols = 6
				}
				rows := (len(assets) + cols - 1) / cols
				a.assetList.Key = func(i int) any { return i }
				ui.List(c, &a.assetList, rows, func(i int) {
					ui.Row(c).FillWidth().Padding(3, 2).Gap(10).Children(func() {
						for col := 0; col < cols; col++ {
							idx := i*cols + col
							if idx >= len(assets) {
								ui.Box(c).Grow(1).MinWidth(0).Height(146)
								continue
							}
							a.assetCell(c, assets[idx])
						}
					})
				}).Grow(1).PaddingY(4)
			})
		})
	})
}

func (a *app) assetCategoryTree(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(188).FillHeight().Padding(6).Gap(2).Radius(10).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, "分类").FontSize(9.5).TextColor(t.TextMuted).Padding(4, 5)
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(1).Children(func() {
				a.assetCatRow(c, "asset-cat-all", "全部图片", "全部", "全部", 0, true)
				a.assetCatRow(c, "asset-cat-dirty", "已修改", "已修改", "已修改", 0, true)
				for ti, node := range assetTree {
					a.assetCatRow(c, fmt.Sprintf("asset-cat-top-%d", ti), node.Name, node.Name, "全部", 0, true)
					if len(node.Children) == 1 && node.Children[0] == "全部" {
						continue
					}
					for si, child := range node.Children {
						a.assetCatRow(c, fmt.Sprintf("asset-cat-%d-%d", ti, si), child, node.Name, child, 1, false)
					}
					ui.Box(c).Height(3)
				}
			})
		})
	})
}

func (a *app) assetCatRow(c *ui.Context, key, label, top, sub string, indent int, parent bool) {
	t := c.Theme()
	selected := a.assetTop == top && a.assetSub == sub
	inPath := a.assetTop == top
	row := ui.Row(c).Key(key).Role(ui.RoleButton).Label(label).
		Height(23).FillWidth().Padding(0, 5).Gap(3).Radius(4).AlignItems(ui.Center)
	if selected {
		row.Background(t.Accent.Alpha(0.14))
	} else if row.Hovered() {
		row.Background(t.Background)
	}
	row.Children(func() {
		if indent > 0 {
			ui.Box(c).Width(10)
		}
		txt := ui.Text(c, label).FontSize(10).Grow(1).NoWrap()
		if parent {
			txt.Bold()
		}
		if inPath {
			txt.TextColor(t.Accent)
		}
		ui.Text(c, fmt.Sprintf("%d", a.countCategory(top, sub))).FontSize(8).TextColor(t.TextMuted)
	})
	row.OnClick(func() { a.setAssetCategory(top, sub) })
}

func (a *app) assetCell(c *ui.Context, asset Asset) {
	t := c.Theme()
	selected := a.selectedAsset == asset.Rel
	cell := ui.Column(c).Key("asset-" + asset.Rel).Role(ui.RoleButton).Label(path.Base(asset.Rel)).AccessSelected(selected).Grow(1).MinWidth(0).Height(146).Padding(7).Gap(5).Radius(8)
	if selected {
		cell.Background(t.Accent.Alpha(0.10)).Border(2, t.Accent)
	} else if cell.Hovered() {
		cell.Background(t.Surface).Border(1, t.Border)
	} else {
		cell.Border(1, t.Border)
	}
	if cell.FileDragOver() {
		cell.Background(t.Accent.Alpha(0.18)).Border(2, t.Accent)
	}
	cell.Children(func() {
		a.assetThumb(c, asset.Rel, 92)
		name := path.Base(asset.Rel)
		if a.proj.Dirty[asset.Rel] {
			ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, name).FontSize(10.5).Bold().NoWrap().Grow(1)
				ui.Badge(c, "已改").Background(t.Warning.Alpha(0.18)).TextColor(t.Warning)
			})
		} else {
			ui.Text(c, name).FontSize(10.5).Bold().NoWrap()
		}
		dir := path.Base(path.Dir(asset.Rel))
		if dir == "." {
			dir = ""
		}
		ui.Text(c, dir).FontSize(8.5).TextColor(t.TextMuted).NoWrap()
	})
	cell.OnClick(func() {
		a.selectedAsset = asset.Rel
		a.win.Invalidate()
	})
	if files := cell.DroppedFiles(); len(files) > 0 {
		a.replaceAssetFrom(asset.Rel, files[0])
	}
	if cell.DoubleClicked() {
		a.selectedAsset = asset.Rel
		a.replaceAssetDialog()
	}
}

func (a *app) assetsInspector(c *ui.Context) {
	t := c.Theme()
	asset, ok := a.selectedAssetInfo()
	if !ok {
		section(c, "图片资源", "从左侧选一张图片查看尺寸、路径，并替换它。", func() {
			ui.Text(c, "可以直接把图片拖到卡片上替换；背景图会自动裁切填满 1024×600。").FontSize(10).LineHeight(1.55).TextColor(t.TextMuted)
		})
		return
	}
	section(c, "选中图片", path.Base(asset.Rel), func() {
		a.assetThumb(c, asset.Rel, 190)
		kv(c, "路径", asset.Rel)
		top, sub := assetCategoryOf(asset.Rel)
		kv(c, "分类", top+" / "+sub)
		w, h := a.imageSizeOf(asset.Rel)
		kv(c, "尺寸", fmt.Sprintf("%d × %d px", w, h))
		kv(c, "大小", formatBytes(asset.Size))
	})
	section(c, "替换", "会保持原来的像素尺寸和文件名。", func() {
		ui.Button(c, "替换这张图片…").Key("replace-asset").OnClick(a.replaceAssetDialog)
		ui.Button(c, "用文件夹批量覆盖…").Key("replace-folder").OnClick(a.replaceFolderDialog)
		if _, exists := a.proj.Original[asset.Rel]; exists {
			ui.Button(c, "还原为原图").Key("revert-asset").OnClick(func() { a.revertAsset(asset.Rel) })
		}
		ui.Text(c, "批量覆盖适合一整套数字：文件夹里放 0.png…9.png，同名文件会被逐一替换。").FontSize(9.5).LineHeight(1.45).TextColor(t.TextMuted)
	})
	a.siblingAssets(c, asset)
}

func (a *app) siblingAssets(c *ui.Context, asset Asset) {
	t := c.Theme()
	folder := path.Dir(asset.Rel)
	var siblings []Asset
	for _, x := range a.proj.Assets {
		if path.Dir(x.Rel) == folder {
			siblings = append(siblings, x)
		}
	}
	if len(siblings) <= 1 {
		return
	}
	section(c, "同目录图片", fmt.Sprintf("%s · %d 张", folder, len(siblings)), func() {
		shown := 0
		for _, s := range siblings {
			if shown >= 24 {
				ui.Text(c, fmt.Sprintf("还有 %d 张…", len(siblings)-shown)).FontSize(9.5).TextColor(t.TextMuted)
				break
			}
			row := ui.Row(c).Role(ui.RoleButton).Label(path.Base(s.Rel)).Height(26).Gap(7).AlignItems(ui.Center).Padding(0, 5).Radius(5)
			if row.Hovered() {
				row.Background(t.Background)
			}
			row.Children(func() {
				ui.Text(c, path.Base(s.Rel)).FontSize(10)
				ui.Spacer(c)
				if a.proj.Dirty[s.Rel] {
					ui.Text(c, "已替换").FontSize(8.5).TextColor(t.Warning)
				}
			})
			row.OnClick(func() {
				a.selectedAsset = s.Rel
				a.win.Invalidate()
			})
			shown++
		}
	})
}

// ---- Theme information -----------------------------------------------------

func (a *app) infoPage(c *ui.Context) {
	if a.proj == nil {
		a.welcome(c)
		return
	}
	t := c.Theme()
	c.Key("info-scroll")
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(24).Gap(14).MaxWidth(820).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Column(c).Gap(3).Children(func() {
					ui.Text(c, "主题信息").FontSize(20).Bold()
					ui.Text(c, "这些字段会写入 config.ini，导出时一起打包。").FontSize(11).TextColor(t.TextMuted)
				})
				ui.Spacer(c)
				ui.Badge(c, valueOr(a.meta.ThemeType, "未设车型"))
				ui.Badge(c, "v"+valueOr(a.meta.ThemeVer, "0.0.1"))
			})

			section(c, "身份", "显示名称只影响 App 和主题列表；ThemeName / ThemeID 是车机识别主题的字段。", func() {
				field(c, "显示名称（CustomName）", "meta-display", &a.meta.DisplayName, "例如 岁新骑悦 · 我的版本")
				fieldRow(c, func() {
					field(c, "主题名称 ThemeName", "meta-themename", &a.meta.ThemeName, "台铃K075")
					field(c, "主题 ID ThemeID", "meta-themeid", &a.meta.ThemeID, "0203DEMO…")
				})
				fieldRow(c, func() {
					field(c, "版本 ThemeVer", "meta-themever", &a.meta.ThemeVer, "0.0.9")
					field(c, "车型 ThemeType", "meta-themetype", &a.meta.ThemeType, "S19")
				})
				fieldRow(c, func() {
					field(c, "配置 ThemeConf", "meta-themeconf", &a.meta.ThemeConf, "S15")
					field(c, "固件下限 StartVersion", "meta-startversion", &a.meta.StartVersion, "1.1.4")
				})
			})

			section(c, "资源与说明", "PackName / ResourcePath 决定车机解压路径，除非你清楚后果，否则不要改。", func() {
				fieldRow(c, func() {
					field(c, "包名 PackName", "meta-packname", &a.meta.PackName, "NewRideJoy")
					field(c, "ResourcePath", "meta-resourcepath", &a.meta.ResourcePath, "/data/theme/NewRideJoy")
				})
				fieldRow(c, func() {
					field(c, "作者 CustomAuthor", "meta-author", &a.meta.Author, "可选")
					field(c, "简介 CustomDesc", "meta-desc", &a.meta.Description, "可选")
				})
			})

			section(c, "导出行为", "", func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					c.Key("bump-version")
					ui.Switch(c, &a.bumpVersion)
					ui.Column(c).Gap(2).Children(func() {
						ui.Text(c, "导出时版本号 +1").FontSize(11)
						ui.Text(c, "同一主题更新时递增 ThemeVer，车机会认为这是新版本。").FontSize(9.5).TextColor(t.TextMuted)
					})
				})
				ui.Text(c, "PackSize、FileSize、FileCount、Md5、BuildDate/Time 会在导出时自动重算。").FontSize(9.5).LineHeight(1.45).TextColor(t.TextMuted)
			})

			if a.meta.ThemeType == "" {
				ui.Column(c).Padding(12).Radius(8).Background(t.Warning.Alpha(0.12)).Children(func() {
					ui.Text(c, "车型为空").FontSize(11).Bold().TextColor(t.Warning)
					ui.Text(c, "ThemeType 必须与目标车机一致（例如 S19）。填错会上传失败，甚至让车机拒绝主题。").FontSize(10).LineHeight(1.5).TextColor(t.Warning)
				})
			}
			if a.origType != "" && a.meta.ThemeType != a.origType {
				ui.Column(c).Padding(12).Radius(8).Background(t.Danger.Alpha(0.12)).Children(func() {
					ui.Text(c, "车型已被修改").FontSize(11).Bold().TextColor(t.Danger)
					ui.Text(c, fmt.Sprintf("原包是 %s，当前是 %s。不同车型的主题不能互刷。", a.origType, a.meta.ThemeType)).
						FontSize(10).LineHeight(1.5).TextColor(t.Danger)
				})
			}
			a.infoChangedTracker(c)
		})
	})
}

func (a *app) infoChangedTracker(c *ui.Context) {
	// Text inputs already wrote into a.meta during this build. A
	// difference from the snapshot means there is something to export.
	if a.meta != a.metaSnapshot && !a.unsaved {
		a.unsaved = true
		if a.win != nil {
			a.win.SetTitle(a.windowTitle())
		}
	}
}

func (a *app) infoInspector(c *ui.Context) {
	t := c.Theme()
	section(c, "导出检查", "", func() {
		a.checkList(c)
	})
	section(c, "本次改动", "", func() {
		a.dirtySummary(c)
		for _, ch := range a.checks() {
			if ch.Warn && ch.Label == "车型被修改" {
				ui.Text(c, ch.Detail).FontSize(9.5).TextColor(t.Warning)
			}
		}
	})
	section(c, "下一步", "", func() {
		ui.Text(c, "信息改完后，到「导出发布」生成新的主题包。上传前请先停止投屏。").FontSize(10).LineHeight(1.5).TextColor(t.TextMuted)
		ui.PrimaryButton(c, "去导出").Key("go-export").OnClick(func() {
			a.page = pageExport
			a.win.Invalidate()
		})
	})
}

// ---- Export ----------------------------------------------------------------

func (a *app) exportPage(c *ui.Context) {
	t := c.Theme()
	if a.proj == nil {
		a.welcome(c)
		return
	}
	c.Key("export-scroll")
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).FillWidth().Padding(24).Gap(14).MaxWidth(820).Children(func() {
			grad := t.Accent
			grad2 := t.Accent.Mix(ui.Hex("#000000"), 0.38)
			curVer := valueOr(a.meta.ThemeVer, "0.0.1")
			verLabel := "v" + curVer
			if a.bumpVersion {
				verLabel += " → v" + bumpPatch(curVer)
			}
			ui.Column(c).FillWidth().Padding(22).Radius(13).
				Draw(func(p *ui.Painter, r ui.Rect) {
					p.FillGradient(r, ui.LinearGradient{From: grad, To: grad2, Angle: 135}, 13)
				}).Children(func() {
				ui.Text(c, "导出可用的主题包").FontSize(22).Bold().TextColor(t.AccentText)
				name := valueOr(a.meta.DisplayName, valueOr(a.meta.ThemeName, a.proj.SourceName))
				ui.Text(c, name+" · "+valueOr(a.meta.ThemeType, "未设车型")+" · "+verLabel).
					FontSize(11).TextColor(t.AccentText.Alpha(0.85))
				ui.Text(c, "外层 zip = config.ini + package.zip，可直接用手机 App 的主题上传。").FontSize(10).TextColor(t.AccentText.Alpha(0.8))
			})
			var failedChecks []string
			for _, ch := range a.checks() {
				if !ch.OK {
					failedChecks = append(failedChecks, ch.Label+"（"+ch.Detail+"）")
				}
			}
			if len(failedChecks) > 0 {
				ui.Column(c).FillWidth().Padding(12).Radius(8).Gap(4).
					Background(t.Warning.Alpha(0.12)).Border(1, t.Warning.Alpha(0.5)).Children(func() {
					ui.Text(c, "导出前请注意").FontSize(11).Bold().TextColor(t.Warning)
					ui.Text(c, strings.Join(failedChecks, "；")).FontSize(10).LineHeight(1.45).TextColor(t.Warning)
				})
			}
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.PrimaryButton(c, "快速导出").Key("export-quick").OnClick(a.quickExport)
					ui.Button(c, "另存为…").Key("export-dialog").OnClick(a.exportDialog)
					ui.Spacer(c)
					c.Key("export-bump-version")
					ui.Switch(c, &a.bumpVersion)
					ui.Text(c, "导出时版本号 +1").FontSize(10.5)
				})
				ui.Text(c, "快速导出位置："+a.proj.quickExportPath()).FontSize(9.5).TextColor(t.TextMuted)
			})

			if a.exporting {
				ui.Row(c).Padding(12).Radius(8).Background(t.Surface).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c)
					ui.Text(c, "正在重新打包并计算 MD5…").FontSize(11)
				})
			}
			if a.exportErr != "" {
				ui.Column(c).Padding(12).Radius(8).Background(t.Danger.Alpha(0.12)).Children(func() {
					ui.Text(c, "导出失败").FontSize(11).Bold().TextColor(t.Danger)
					ui.Text(c, a.exportErr).FontSize(10).TextColor(t.Danger)
				})
			}
			if a.exportResult != nil {
				res := a.exportResult
				section(c, "导出完成", "新的主题包已经写好，原包没有被修改。", func() {
					kv(c, "文件", res.Path)
					kv(c, "版本", "v"+res.Version)
					kv(c, "大小", formatBytes(res.Size))
					kv(c, "package.zip", formatBytes(res.PackSize))
					kv(c, "文件数", fmt.Sprintf("%d", res.FileCount))
					kv(c, "package.zip MD5", res.MD5)
					ui.Button(c, "在文件夹中显示").Key("reveal-export").OnClick(a.revealExport)
				})
			}

			section(c, "导出前检查", "", func() {
				a.checkList(c)
			})

			section(c, "输出位置", "", func() {
				kv(c, "快速导出", a.proj.quickExportPath())
				kv(c, "另存为", "自选目录和文件名")
				ui.Text(c, "快速导出会覆盖同名 _studio.zip；原主题包始终不动。").FontSize(9.5).LineHeight(1.45).TextColor(t.TextMuted)
			})

			section(c, "上传到车机", "建议先停止投屏，再在手机 App 里上传。", func() {
				steps := []string{
					"确认车机在线，电量充足，车辆处于驻车状态。",
					"手机 App → 设置 → 车机外观 → 主题与壁纸。",
					"选择刚导出的 zip，确认 ThemeType / StartVersion 与车辆一致。",
					"上传会自动停止投屏；完成后重新启动投屏。",
				}
				for i, s := range steps {
					ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
						ui.Box(c).Size(18, 18).Radius(9).Background(t.Accent.Alpha(0.15)).Children(func() {
							ui.Text(c, fmt.Sprintf("%d", i+1)).FontSize(9).TextColor(t.Accent).Width(18).TextAlign(ui.Center)
						})
						ui.Text(c, s).FontSize(10.5).LineHeight(1.5).Grow(1)
					})
				}
			})
		})
	})
}

func (a *app) exportInspector(c *ui.Context) {
	t := c.Theme()
	if a.proj == nil {
		return
	}
	section(c, "打包规则", "", func() {
		for _, s := range []string{
			"package.zip 内的资源树原样保留，只替换改动过的图片。",
			"PackSize = package.zip 字节数；Md5 = 它的 MD5。",
			"FileCount = 文件数量；BuildDate/Time = 当前时间。",
			"FileSize 沿用原包的安装体积差值。",
			"ThemeType / StartVersion 只按你填写的值写入。",
		} {
			ui.Row(c).Gap(6).AlignItems(ui.Start).Children(func() {
				ui.Box(c).Size(4, 4).Radius(2).Background(t.TextMuted).Margin(6, 0, 0, 0)
				ui.Text(c, s).FontSize(9.5).LineHeight(1.45).Grow(1).TextColor(t.TextMuted)
			})
		}
	})
	section(c, "安全提示", "", func() {
		ui.Text(c, "不要跨车型上传主题。上传前保留官方主题或 OTA 回退路径，不要在骑行中安装。").FontSize(9.5).LineHeight(1.5).TextColor(t.Warning)
	})
	if a.exportResult != nil {
		section(c, "最近导出", "", func() {
			ui.Text(c, filepath.Base(a.exportResult.Path)).FontSize(11).Bold()
			ui.Text(c, formatBytes(a.exportResult.Size)).FontSize(9.5).TextColor(t.TextMuted)
			ui.Button(c, "在文件夹中显示").OnClick(a.revealExport)
		})
	}
}
