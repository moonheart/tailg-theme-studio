package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

func (a *app) scenesPage(c *ui.Context) {
	if a.proj == nil {
		a.welcome(c)
		return
	}
	t := c.Theme()
	if a.sceneAnim {
		c.AnimationFrame()
		now := time.Now()
		if a.sceneEpoch.IsZero() {
			a.sceneEpoch = now
		}
		dt := now.Sub(a.sceneEpoch).Seconds()
		a.sceneEpoch = now
		if dt > 0.25 {
			dt = 0.25
		}
		a.scenePhase += dt * a.sceneSpeed
		a.invalidatePreview()
	}
	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		ui.Row(c).FillWidth().Height(34).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
				ic := ui.Icon(c, icScenes)
				ic.FontSize(15).TextColor(t.Accent)
				ui.Text(c, "界面预览").FontSize(13).Bold()
			})
			ui.Badge(c, "1024 × 600")
			if a.sceneAnim {
				ui.Badge(c, "动画中")
			} else {
				ui.Badge(c, "已暂停")
			}
			ui.Spacer(c)
			labels := sceneOptionLabels()
			sn := labels[int(a.scene)]
			c.Key("scene-select")
			if ui.Select(c, &sn, labels).Width(230).Changed() {
				for i, l := range labels {
					if l == sn {
						a.selectScene(sceneKind(i))
					}
				}
			}
			if a.scene == sceneMenuPage && a.proj.Scene != nil && len(a.proj.Scene.MenuPages) > 0 {
				if ui.Button(c, "‹").Key("scene-toolbar-prev").Width(28).Clicked() {
					a.menuPageStep(-1)
				}
				ui.Text(c, fmt.Sprintf("%d / %d", a.menuPageIndex()+1, len(a.proj.Scene.MenuPages))).
					FontSize(10.5).Width(52).TextAlign(ui.Center).TextColor(t.TextMuted)
				if ui.Button(c, "›").Key("scene-toolbar-next").Width(28).Clicked() {
					a.menuPageStep(1)
				}
			} else if a.scene == scenePopup && a.proj.Scene != nil && len(a.proj.Scene.Popups) > 0 {
				cur := 1
				for i, item := range a.proj.Scene.Popups {
					if item.Key == a.popupType {
						cur = i + 1
						break
					}
				}
				if ui.Button(c, "‹").Key("scene-toolbar-prev").Width(28).Clicked() {
					a.popupStep(-1)
				}
				ui.Text(c, fmt.Sprintf("%d / %d", cur, len(a.proj.Scene.Popups))).
					FontSize(10.5).Width(52).TextAlign(ui.Center).TextColor(t.TextMuted)
				if ui.Button(c, "›").Key("scene-toolbar-next").Width(28).Clicked() {
					a.popupStep(1)
				}
			}
			playLabel := "暂停"
			if !a.sceneAnim {
				playLabel = "播放"
			}
			if ui.Button(c, playLabel).Key("scene-toggle").Clicked() {
				a.sceneAnim = !a.sceneAnim
				a.sceneEpoch = time.Now()
				a.invalidatePreview()
			}
			ui.Box(c).Width(4)
			if ui.Button(c, "−").Key("scene-zoom-out").Width(30).Clicked() {
				a.stepZoom(-1)
			}
			ui.Text(c, fmt.Sprintf("%d%%", int(a.effectiveScale()*100))).FontSize(11).Width(42).TextAlign(ui.Center)
			if ui.Button(c, "+").Key("scene-zoom-in").Width(30).Clicked() {
				a.stepZoom(1)
			}
			if ui.Button(c, "适应").Key("scene-zoom-fit").Clicked() {
				a.zoom = 0
				a.saveSettings()
			}
		})
		ui.Box(c).Grow(1).Radius(12).Clip().Background(ui.Hex("#141418")).
			Draw(func(p *ui.Painter, r ui.Rect) {
				step := float32(28)
				for x := r.X + step/2; x < r.X+r.W; x += step {
					for y := r.Y + step/2; y < r.Y+r.H; y += step {
						p.Fill(ui.Rect{X: x, Y: y, W: 1, H: 1}, ui.RGBA(255, 255, 255, 0.055), 0)
					}
				}
			}).Children(func() {
			a.sceneStage(c)
		})
		ui.Row(c).FillWidth().Padding(10, 12).Gap(12).Radius(9).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, a.scene.label()).FontSize(11).Bold()
				ui.Text(c, a.scene.desc()).FontSize(9.5).LineHeight(1.45).TextColor(t.TextMuted)
			})
			ui.Button(c, "查看资源").Key("scene-assets").OnClick(func() {
				a.openAssetFolder(a.scene.folder())
			})
			ui.Button(c, "回到仪表盘").Key("scene-to-dashboard").OnClick(func() {
				a.page = pageDashboard
				a.invalidatePreview()
				a.win.Invalidate()
			})
		})
	})
}

func (a *app) sceneStage(c *ui.Context) {
	zoomed := a.zoom > 0 && float32(a.zoom) > a.fitScale()+0.002
	if zoomed {
		ui.ScrollBoth(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(22).Children(func() {
				a.sceneCanvas(c)
			})
		})
		return
	}
	ui.Box(c).Fill().Center().Padding(22).Children(func() {
		a.sceneCanvas(c)
	})
}

func (a *app) sceneCanvas(c *ui.Context) {
	scale := a.effectiveScale()
	a.previewScale = scale
	w := float32(themeW) * scale
	h := float32(themeH) * scale
	box := ui.Box(c).Key("scene-canvas").Size(w, h).Label("场景预览")
	box.Children(func() {
		if a.particle != nil {
			if a.showsParticle() && !(a.page == pageScenes && !a.sceneAnim) {
				a.particle.Play()
			} else {
				a.particle.Pause()
			}
		}
		if a.showsParticle() {
			v := ui.Video(c, a.particle).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.FillBox)
			v.HitTest(func(x, y float32, r ui.Rect) bool { return false })
		}
		if a.particle != nil {
			ready := a.particle.Ready()
			if ready != a.particleReady {
				a.particleReady = ready
				a.invalidatePreview()
			}
		}
		overlay := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0)
		overlay.HitTest(func(x, y float32, r ui.Rect) bool { return false })
		overlay.Draw(func(p *ui.Painter, r ui.Rect) {
			if !a.videoBgActive() {
				p.Shadow(r, 6, 0, 6, 18, 0, ui.RGBA(0, 0, 0, 0.45))
			}
			if a.canvas == nil {
				a.canvas = ui.NewCanvas(themeW, themeH)
				a.drawnVer = -1
			}
			if a.drawnVer != a.previewVer {
				a.canvas.Draw(p, func(cp *ui.Painter) { a.drawTheme(cp) })
				a.drawnVer = a.previewVer
			}
			p.Image(a.canvas.Bitmap(), r, ui.FillBox)
		})
	})
}

func (a *app) enterScenes() {
	a.page = pageScenes
	a.sceneEpoch = time.Now()
	a.scenePhase = 0
	a.invalidatePreview()
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *app) selectScene(s sceneKind) {
	a.scene = s
	a.sceneEpoch = time.Now()
	a.scenePhase = 0
	a.invalidatePreview()
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *app) scenesInspector(c *ui.Context) {
	t := c.Theme()
	if a.proj == nil {
		return
	}
	section(c, "动画", "", func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			c.Key("scene-anim-ins")
			if ui.Switch(c, &a.sceneAnim).Changed() {
				a.sceneEpoch = time.Now()
				a.invalidatePreview()
			}
			ui.Text(c, "循环播放").FontSize(10.5)
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%.2fx", a.sceneSpeed)).FontSize(9.5).TextColor(t.TextMuted)
		})
		c.Key("scene-speed")
		ui.Slider(c, &a.sceneSpeed, 0.25, 3)
		ui.Text(c, "速度、动画和序列帧都按这个倍率播放。").FontSize(9).TextColor(t.TextMuted)
		c.Key("scene-lang")
		lang := "中文"
		if a.menuLang == "en_US" {
			lang = "English"
		}
		if ui.Select(c, &lang, []string{"中文", "English"}).Changed() {
			if lang == "English" {
				a.menuLang = "en_US"
			} else {
				a.menuLang = "zh_CN"
			}
			a.invalidatePreview()
		}
		if !a.sceneAnim {
			ui.Text(c, "动画关闭时显示一个固定画面；重新打开会从当前时刻继续。").FontSize(9).LineHeight(1.4).TextColor(t.TextMuted)
		}
	})
	if (a.scene == sceneDash || a.scene == sceneLink) && a.proj.Scene != nil &&
		(a.proj.Scene.NitroIcon.W > 0 || a.proj.Scene.NitroLinkIcon.W > 0) {
		section(c, "氮气加速状态", "主题 layout.ini 的 n2_icon_0 / n2_left_warn_0；灰色未就绪、橙色已就绪、红色加速中并播放火焰环。", func() {
			labels := []string{"自动演示", "未就绪", "已就绪", "加速中"}
			values := []string{"auto", "idle", "ready", "boost"}
			cur := labels[0]
			mode := a.nitroMode
			if mode == "" {
				mode = "auto"
			}
			for i, v := range values {
				if v == mode {
					cur = labels[i]
				}
			}
			c.Key("nitro-mode")
			if ui.Select(c, &cur, labels).Changed() {
				for i, l := range labels {
					if l == cur {
						a.nitroMode = values[i]
						a.sceneEpoch = time.Now()
						a.scenePhase = 0
						a.invalidatePreview()
					}
				}
			}
			ui.Text(c, "当前："+a.nitroStateLabel()).FontSize(9.5).TextColor(t.TextMuted)
			if a.nitroMode == "auto" && !a.sceneAnim {
				ui.Text(c, "自动演示需要打开循环播放；暂停时会停在当前状态的固定帧。").FontSize(9).LineHeight(1.4).TextColor(t.TextMuted)
			}
		})
	}
	if a.scene == sceneLink {
		section(c, "投屏状态", "投屏 HUD 的状态灯、提示、H 和加载页。", func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				c.Key("link-hillhold")
				if ui.Switch(c, &a.hillHold).Changed() {
					a.invalidatePreview()
				}
				ui.Text(c, "坡道 H").FontSize(10)
				ui.Spacer(c)
				c.Key("link-loading")
				if ui.Switch(c, &a.linkLoading).Changed() {
					a.invalidatePreview()
				}
				ui.Text(c, "导航加载页").FontSize(10)
			})
			labels := []string{"无", "坡道驻车", "防溜坡", "边撑", "坐垫"}
			ids := []int{0, 42, 43, 44, 45}
			cur := labels[0]
			for i, id := range ids {
				if id == a.hintID {
					cur = labels[i]
				}
			}
			c.Key("link-hint")
			if ui.Select(c, &cur, labels).Changed() {
				for i, l := range labels {
					if l == cur {
						a.hintID = ids[i]
						a.invalidatePreview()
					}
				}
			}
		})
	}
	if a.scene == sceneDash && a.proj.Scene != nil &&
		(a.proj.Scene.EcuIcon.W > 0 || a.proj.Scene.MotorIcon.W > 0) {
		section(c, "仪表温度", "底部 ECU / 电机温度，位置来自 layout.ini。", func() {
			a.tempControls(c)
		})
	}
	if a.scene == sceneDash && a.proj.Scene != nil && a.proj.Scene.WeatherBG.W > 0 {
		section(c, "仪表附加", "天气、指南针、海拔、功率、骑行数据和充电浮窗。", func() {
			a.hudControls(c)
		})
	}
	if a.scene == sceneMenuPage {
		section(c, "菜单页面", "主题里的全部菜单布局；通用渲染背景、按钮、文本和列表。", func() {
			pages := a.proj.Scene.MenuPages
			if len(pages) == 0 {
				ui.Text(c, "没有找到菜单布局").FontSize(10).TextColor(t.TextMuted)
				return
			}
			c.Key("menu-page-search")
			ui.TextInput(c, &a.menuPageQuery).Placeholder("搜索页面…").Label("搜索菜单页面")
			q := strings.ToLower(strings.TrimSpace(a.menuPageQuery))
			var idxs []int
			for i, pg := range pages {
				if q == "" || strings.Contains(strings.ToLower(pg.Title+" "+pg.Key+" "+pg.Group), q) {
					idxs = append(idxs, i)
				}
			}
			if len(idxs) == 0 {
				ui.Text(c, "没有匹配的页面").FontSize(10).TextColor(t.TextMuted)
			} else {
				if q != "" {
					curKey := pages[a.menuPageIndex()].Key
					found := false
					for _, i := range idxs {
						if pages[i].Key == curKey {
							found = true
							break
						}
					}
					if !found {
						a.setMenuPage(pages[idxs[0]].Key)
					}
				}
				labels := make([]string, len(idxs))
				seen := map[string]int{}
				for j, i := range idxs {
					l := pages[i].Group + " · " + pages[i].Title
					seen[l]++
					if seen[l] > 1 {
						l += " (" + pages[i].Key + ")"
					}
					labels[j] = l
				}
				cur := labels[0]
				for j, i := range idxs {
					if pages[i].Key == pages[a.menuPageIndex()].Key {
						cur = labels[j]
					}
				}
				c.Key("menu-page")
				if ui.Select(c, &cur, labels).Changed() {
					for j, l := range labels {
						if l == cur {
							a.setMenuPage(pages[idxs[j]].Key)
						}
					}
				}
			}
			ui.Row(c).Gap(8).Children(func() {
				ui.Button(c, "上一页").Key("menu-page-prev").Grow(1).OnClick(func() { a.menuPageStep(-1) })
				ui.Button(c, "下一页").Key("menu-page-next").Grow(1).OnClick(func() { a.menuPageStep(1) })
			})
			ui.Text(c, "menu/etc/layout/"+pages[a.menuPageIndex()].Key+".ini").FontSize(9).TextColor(t.TextMuted)
			if pages[a.menuPageIndex()].Key == "tpms/tire_disp" {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					c.Key("tpms-abnormal")
					if ui.Switch(c, &a.tpmsAbnormal).Changed() {
						a.invalidatePreview()
					}
					ui.Text(c, "胎压异常状态").FontSize(10)
				})
			}
			if pages[a.menuPageIndex()].Key == "navi/BTNaviLayout" {
				num := func(key, label string, v *int, lo, hi int) {
					ui.Column(c).Gap(2).Grow(1).Children(func() {
						ui.Text(c, label).FontSize(9).TextColor(t.TextMuted)
						f := float64(*v)
						c.Key(key)
						if ui.NumberInput(c, &f, float64(lo), float64(hi), 1).Changed() {
							*v = int(f)
							a.invalidatePreview()
						}
					})
				}
				ui.Row(c).Gap(8).Children(func() {
					num("navi-icon", "图标", &a.naviIcon, 1, 46)
					num("navi-arrow", "箭头", &a.naviArrow, 0, 48)
				})
				ui.Row(c).Gap(8).Children(func() {
					num("navi-lanes", "车道", &a.naviLanes, 1, 8)
					num("navi-lanebg", "底图", &a.naviLaneBg, 1, 8)
				})
			}
		})
	}
	if a.scene == scenePopup {
		section(c, "弹窗类型", "消息、来电、故障、语音、壁纸、胎压和清除里程。", func() {
			items := a.proj.Scene.Popups
			if len(items) == 0 {
				ui.Text(c, "没有找到弹窗资源").FontSize(10).TextColor(t.TextMuted)
				return
			}
			labels := make([]string, len(items))
			cur := items[0].Title
			for i, item := range items {
				labels[i] = item.Title
				if item.Key == a.popupType {
					cur = item.Title
				}
			}
			c.Key("popup-type")
			if ui.Select(c, &cur, labels).Changed() {
				for _, item := range items {
					if item.Title == cur {
						a.popupType = item.Key
						a.sceneEpoch = time.Now()
						a.scenePhase = 0
						a.invalidatePreview()
					}
				}
			}
			ui.Row(c).Gap(8).Children(func() {
				ui.Button(c, "上一个").Key("popup-prev").Grow(1).OnClick(func() { a.popupStep(-1) })
				ui.Button(c, "下一个").Key("popup-next").Grow(1).OnClick(func() { a.popupStep(1) })
			})
			if a.popupType == "dtc" {
				labels := []string{"9", "39", "40", "41", "44", "71"}
				ids := []int{9, 39, 40, 41, 44, 71}
				cur := labels[0]
				for i, id := range ids {
					if id == a.dtcCode {
						cur = labels[i]
					}
				}
				c.Key("popup-dtc")
				if ui.Select(c, &cur, labels).Changed() {
					for i, l := range labels {
						if l == cur {
							a.dtcCode = ids[i]
							a.invalidatePreview()
						}
					}
				}
			}
		})
	}
	if a.scene == sceneBot {
		section(c, "bot 状态", "四套各 50 帧；init 前几帧是透明淡入。", func() {
			labels := []string{"初始化", "待机", "菜单", "消息"}
			names := []string{"init", "normal", "menu", "msg"}
			cur := 1
			for i, n := range names {
				if n == a.botState {
					cur = i
				}
			}
			label := labels[cur]
			c.Key("bot-state")
			if ui.Select(c, &label, labels).Changed() {
				for i, l := range labels {
					if l == label {
						a.botState = names[i]
					}
				}
				a.sceneEpoch = time.Now()
				a.scenePhase = 0
				a.invalidatePreview()
			}
			ui.Text(c, "在仪表/菜单/投屏场景里，bot 也会按 bot_rules.ini 的位置显示。").FontSize(9).LineHeight(1.4).TextColor(t.TextMuted)
		})
	}
	section(c, "仪表模式", "和仪表盘页面共享，切换后背景会立即更新。", func() {
		modes := []string{"动力 1", "动力 2", "动力 3", "动力 4"}
		mi := a.mode
		if mi < 0 || mi >= len(modes) {
			mi = 0
		}
		mn := modes[mi]
		c.Key("scene-mode")
		if ui.Select(c, &mn, modes).Changed() {
			for i, n := range modes {
				if n == mn {
					a.mode = i
					a.saveSettings()
					a.invalidatePreview()
				}
			}
		}
		ui.Text(c, "背景资源："+a.proj.backgroundRel(a.mode)).FontSize(9).TextColor(t.TextMuted)
	})
	section(c, "场景资源", a.scene.folder(), func() {
		ui.Button(c, "在图片资源中打开").Key("scene-open-assets").OnClick(func() {
			a.openAssetFolder(a.scene.folder())
		})
		ui.Button(c, "回到仪表盘编辑").Key("scene-edit-dashboard").OnClick(func() {
			a.page = pageDashboard
			a.invalidatePreview()
			a.win.Invalidate()
		})
		ui.Text(c, "界面预览只做展示，不会改动主题；替换图片时到「图片资源」操作。").FontSize(9).LineHeight(1.4).TextColor(t.TextMuted)
	})
}
