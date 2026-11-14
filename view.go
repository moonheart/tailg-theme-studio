package main

import (
	"fmt"
	"math"
	"path/filepath"

	"github.com/egoist/mygo/ui"
)

var (
	icDashboard = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="7" height="9" rx="1.5"/><rect x="14" y="3" width="7" height="5" rx="1.5"/><rect x="14" y="12" width="7" height="9" rx="1.5"/><rect x="3" y="16" width="7" height="5" rx="1.5"/></svg>`))
	icImage     = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2.5"/><circle cx="8.5" cy="8.5" r="1.5"/><path d="m21 15-5-5L5 21"/></svg>`))
	icInfo      = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 11v5"/><path d="M12 8h.01"/></svg>`))
	icUpload    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 16V4"/><path d="m7 9 5-5 5 5"/><path d="M5 20h14"/></svg>`))
	icFolder    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/></svg>`))
	icReplace   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 0 1 15-6l3 3"/><path d="M21 3v6h-6"/><path d="M21 12a9 9 0 0 1-15 6l-3-3"/><path d="M3 21v-6h6"/></svg>`))
	icReset     = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5"/></svg>`))
	icCheck     = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="m4 12 5 5L20 6"/></svg>`))
	icWarning   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3 2 21h20z"/><path d="M12 9v5"/><path d="M12 18h.01"/></svg>`))
	icPackage   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m12 3 9 5v8l-9 5-9-5V8z"/><path d="m3 8 9 5 9-5"/><path d="M12 13v9"/></svg>`))
	icEye       = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/></svg>`))
	icScenes    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m12 3 9 5-9 5-9-5z"/><path d="m3 12 9 5 9-5"/><path d="m3 16 9 5 9-5"/></svg>`))
)

func (a *app) view(c *ui.Context) {
	t := c.Theme()
	a.shortcuts(c)
	ui.Column(c).Fill().Background(t.Background).Children(func() {
		a.header(c)
		ui.Divider(c)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.sidebar(c)
			ui.Box(c).Width(1).FillHeight().Background(t.Border)
			ui.Box(c).Grow(1).FillHeight().Clip().Children(func() {
				a.center(c)
			})
			ui.Box(c).Width(1).FillHeight().Background(t.Border)
			a.inspector(c)
		})
		ui.Divider(c)
		a.statusBar(c)
	})
}

func (a *app) shortcuts(c *ui.Context) {
	if c.FocusTakesText() || a.page != pageDashboard {
		return
	}
	switch {
	case c.Shortcut(ui.Ctrl, ui.KeyZ):
		a.undoLayout()
		return
	case c.Shortcut(ui.Ctrl|ui.Shift, ui.KeyZ) || c.Shortcut(ui.Ctrl, ui.KeyY):
		a.redoLayout()
		return
	case c.Shortcut(0, ui.KeyH):
		a.hideSelection = !a.hideSelection
		a.invalidatePreview()
		return
	}
	if a.sel == "" {
		return
	}
	step := 1
	mod := ui.Modifiers(0)
	if c.Modifiers()&ui.Shift != 0 {
		step = 10
		mod = ui.Shift
	}
	switch {
	case c.Shortcut(mod, ui.KeyLeft):
		a.nudgeSelection(-step, 0)
	case c.Shortcut(mod, ui.KeyRight):
		a.nudgeSelection(step, 0)
	case c.Shortcut(mod, ui.KeyUp):
		a.nudgeSelection(0, -step)
	case c.Shortcut(mod, ui.KeyDown):
		a.nudgeSelection(0, step)
	}
}

func (a *app) header(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(58).FillWidth().Padding(0, 16).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Row(c).Size(30, 30).Center().Radius(8).Background(t.Accent).Children(func() {
			ui.Text(c, "T").FontSize(15).Bold().TextColor(t.AccentText)
		})
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, "Theme Studio").FontSize(14).Bold().NoWrap()
			ui.Text(c, "台铃仪表主题编辑器").FontSize(10.5).TextColor(t.TextMuted).NoWrap()
		})
		ui.Box(c).Width(16)
		if a.proj != nil {
			name := a.meta.DisplayName
			if name == "" {
				name = a.meta.ThemeName
			}
			if name == "" {
				name = a.proj.SourceName
			}
			ui.Text(c, name).FontSize(13).NoWrap().MaxWidth(320)
			typ := a.meta.ThemeType
			if typ == "" {
				typ = "未知车型"
			}
			ui.Badge(c, typ)
			ui.Text(c, "v"+valueOr(a.meta.ThemeVer, "0.0.1")).FontSize(11).TextColor(t.TextMuted)
			if a.unsaved {
				ui.Badge(c, "未导出")
			}
		} else {
			ui.Text(c, "未打开主题").FontSize(12).TextColor(t.TextMuted)
		}
		ui.Spacer(c)
		if a.proj != nil {
			ui.Text(c, fmt.Sprintf("%d 张图片", len(a.proj.Assets))).FontSize(11).TextColor(t.TextMuted)
			ui.Box(c).Width(6)
		}
		ui.Button(c, "打开").Key("open-theme").OnClick(a.openDialog)
		ui.PrimaryButton(c, "导出").Key("open-export").Disabled(a.proj == nil).OnClick(func() {
			a.page = pageExport
			a.win.Invalidate()
		})
	})
}

func (a *app) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(240).FillHeight().Padding(12, 14).Gap(12).Children(func() {
		a.sidebarThemeCard(c)
		ui.Row(c).Gap(8).Children(func() {
			ui.PrimaryButton(c, "打开主题").Key("open-theme-side").Grow(1).OnClick(a.openDialog)
			ui.Button(c, "文件夹").Key("open-folder").OnClick(a.openFolderDialog)
		})
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(3).Children(func() {
				a.navRow(c, "nav-dashboard", "仪表盘", icDashboard, a.page == pageDashboard, func() { a.page = pageDashboard })
				a.navRow(c, "nav-scenes", "界面预览", icScenes, a.page == pageScenes, a.enterScenes)
				a.navRow(c, "nav-assets", "图片资源", icImage, a.page == pageAssets, func() { a.page = pageAssets })
				a.navRow(c, "nav-info", "主题信息", icInfo, a.page == pageInfo, func() { a.page = pageInfo })
				a.navRow(c, "nav-export", "导出发布", icUpload, a.page == pageExport, func() { a.page = pageExport })
				ui.Box(c).Height(10)
				if len(a.recent) > 0 {
					ui.Text(c, "最近打开").FontSize(10).TextColor(t.TextMuted).Padding(8, 6)
					for i, r := range a.recent {
						if i >= 4 {
							break
						}
						a.recentRow(c, r)
					}
				}
			})
		})
		ui.Column(c).Gap(4).Padding(10).Radius(8).Background(t.Surface.Alpha(0.55)).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(6, 6).Radius(3).Background(t.Success)
				ui.Text(c, "本地编辑").FontSize(10.5).Bold()
			})
			ui.Text(c, "所有改动先写入临时副本，原主题包不会被修改。").FontSize(10).LineHeight(1.45).TextColor(t.TextMuted)
		})
	})
}

func (a *app) sidebarThemeCard(c *ui.Context) {
	t := c.Theme()
	if a.proj == nil {
		ui.Column(c).Height(138).FillWidth().Radius(9).Clip().Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Column(c).Fill().Center().Gap(8).Children(func() {
				ic := ui.Icon(c, icPackage)
				ic.FontSize(28).TextColor(t.TextMuted)
				ui.Text(c, "还没有打开主题").FontSize(11).TextColor(t.TextMuted)
			})
		})
		return
	}
	bg := a.proj.backgroundRel(a.mode)
	ui.Box(c).Height(126).FillWidth().Radius(9).Clip().Background(ui.Hex("#101013")).
		Draw(func(p *ui.Painter, r ui.Rect) {
			if b := a.thumb(bg); b != nil {
				p.Image(b, r, ui.Cover)
			}
			p.FillGradient(r, ui.LinearGradient{
				From: ui.RGBA(0, 0, 0, 0), To: ui.RGBA(0, 0, 0, 0.55), Angle: 90,
			}, 0)
		})
	ui.Column(c).Gap(3).Children(func() {
		name := a.meta.DisplayName
		if name == "" {
			name = a.meta.ThemeName
		}
		if name == "" {
			name = a.proj.SourceName
		}
		ui.Text(c, name).FontSize(13).Bold().NoWrap()
		ui.Text(c, shortenPath(a.proj.SourcePath, 32)).FontSize(9.5).TextColor(t.TextMuted).NoWrap()
		ui.Row(c).Gap(5).Children(func() {
			ui.Badge(c, valueOr(a.meta.ThemeType, "未知车型"))
			ui.Badge(c, "v"+valueOr(a.meta.ThemeVer, "0.0.1"))
			ui.Badge(c, fmt.Sprintf("%d 图", len(a.proj.Assets)))
		})
	})
}

func (a *app) navRow(c *ui.Context, key, label string, icon *ui.SVG, selected bool, on func()) {
	t := c.Theme()
	row := ui.Row(c).Key(key).Role(ui.RoleButton).Label(label).Height(35).FillWidth().Padding(0, 10).Gap(9).Radius(7).AlignItems(ui.Center)
	if selected {
		row.Background(t.Accent.Alpha(0.13))
	} else if row.Hovered() {
		row.Background(t.Surface)
	}
	row.Children(func() {
		ic := ui.Icon(c, icon)
		ic.FontSize(16)
		if selected {
			ic.TextColor(t.Accent)
		} else {
			ic.TextColor(t.TextMuted)
		}
		txt := ui.Text(c, label)
		txt.FontSize(12.5)
		if selected {
			txt.Bold().TextColor(t.Text)
		}
		ui.Spacer(c)
		if selected {
			ui.Box(c).Width(3).Height(15).Radius(2).Background(t.Accent)
		}
	})
	row.OnClick(func() {
		a.invalidatePreview()
		on()
	})
}

func (a *app) recentRow(c *ui.Context, p string) {
	t := c.Theme()
	base := filepath.Base(p)
	row := ui.Row(c).Role(ui.RoleButton).Label(base).Height(30).FillWidth().Padding(0, 8).Gap(7).Radius(6).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(t.Surface)
	}
	row.Children(func() {
		ic := ui.Icon(c, icPackage)
		ic.FontSize(13).TextColor(t.TextMuted)
		ui.Text(c, base).FontSize(10.5).NoWrap().Grow(1)
	})
	row.OnClick(func() { a.requestOpenPath(p) })
}

func (a *app) statusBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(26).FillWidth().Padding(0, 14).Gap(8).AlignItems(ui.Center).Background(t.Surface.Alpha(0.5)).Children(func() {
		col := t.TextMuted
		if a.errText != "" {
			col = t.Danger
		} else if a.unsaved {
			col = t.Warning
		} else if a.proj != nil {
			col = t.Success
		}
		ui.Box(c).Size(6, 6).Radius(3).Background(col)
		ui.Text(c, a.nowStatus()).Key("status-text").FontSize(10.5).TextColor(col).NoWrap().Grow(1).MaxWidth(900)
		ui.Spacer(c)
		if a.proj != nil {
			ui.Text(c, fmt.Sprintf("%d×%d", themeW, themeH)).FontSize(10).TextColor(t.TextMuted)
			ui.Text(c, "·").FontSize(10).TextColor(t.TextMuted)
			ui.Text(c, valueOr(a.meta.ThemeType, "未设车型")).FontSize(10).TextColor(t.TextMuted)
			ui.Text(c, "·").FontSize(10).TextColor(t.TextMuted)
			ui.Text(c, fmt.Sprintf("%d%%", int(math.Round(float64(a.effectiveScale())*100)))).FontSize(10).TextColor(t.TextMuted)
		}
	})
}

func (a *app) effectiveScale() float32 {
	if a.zoom > 0 {
		return float32(a.zoom)
	}
	return a.fitScale()
}

func (a *app) fitScale() float32 {
	w, h := float32(0), float32(0)
	if a.win != nil {
		cw, ch := a.win.ContentSize()
		w, h = float32(cw), float32(ch)
	}
	if w < 900 || h < 600 {
		return 0.72
	}
	availW := w - 240 - 320 - 88
	availH := h - 58 - 26 - 56 - 100
	s := minf(availW/themeW, availH/themeH)
	if s > 1 {
		s = 1
	}
	if s < 0.32 {
		s = 0.32
	}
	return s
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func (a *app) center(c *ui.Context) {
	switch a.page {
	case pageDashboard:
		a.dashboardPage(c)
	case pageScenes:
		a.scenesPage(c)
	case pageAssets:
		a.assetsPage(c)
	case pageInfo:
		a.infoPage(c)
	case pageExport:
		a.exportPage(c)
	}
}

func (a *app) welcome(c *ui.Context) {
	t := c.Theme()
	ui.Box(c).Fill().Center().Padding(32).Children(func() {
		ui.Column(c).MaxWidth(560).Padding(28).Gap(14).Radius(14).Background(t.Surface).Border(1, t.Border).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Size(44, 44).Center().Radius(12).Background(t.Accent).Children(func() {
					ui.Text(c, "T").FontSize(22).Bold().TextColor(t.AccentText)
				})
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, "打开一个台铃仪表主题").FontSize(19).Bold()
					ui.Text(c, "在桌面上预览 1024×600 仪表、替换素材、重新打包上传。").FontSize(12).TextColor(t.TextMuted)
				})
			})
			ui.Text(c, "支持外层主题 zip（config.ini + package.zip）、单独的 package.zip，或已解压的主题文件夹。").FontSize(11).LineHeight(1.6).TextColor(t.TextMuted)
			ui.Row(c).Gap(10).Children(func() {
				ui.PrimaryButton(c, "打开主题 zip").Key("welcome-open").OnClick(a.openDialog)
				ui.Button(c, "打开文件夹").OnClick(a.openFolderDialog)
			})
			if a.loading {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Spinner(c)
					ui.Text(c, "正在读取主题包…").FontSize(11).TextColor(t.TextMuted)
				})
			}
			if a.errText != "" {
				ui.Column(c).Padding(10).Radius(7).Background(t.Danger.Alpha(0.12)).Children(func() {
					ui.Text(c, a.errText).FontSize(11).TextColor(t.Danger)
				})
			}
			if len(a.recent) > 0 {
				ui.Divider(c)
				ui.Text(c, "最近打开").FontSize(10).TextColor(t.TextMuted)
				for i, r := range a.recent {
					if i >= 5 {
						break
					}
					a.welcomeRecent(c, r)
				}
			}
		})
	})
}

func (a *app) welcomeRecent(c *ui.Context, p string) {
	t := c.Theme()
	base := filepath.Base(p)
	row := ui.Row(c).Role(ui.RoleButton).Label(base).Height(32).FillWidth().Padding(0, 8).Gap(8).Radius(6).AlignItems(ui.Center)
	if row.Hovered() {
		row.Background(t.Background)
	}
	row.Children(func() {
		ic := ui.Icon(c, icPackage)
		ic.FontSize(14).TextColor(t.TextMuted)
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, base).FontSize(11).NoWrap()
			ui.Text(c, shortenPath(p, 58)).FontSize(9).TextColor(t.TextMuted).NoWrap()
		})
	})
	row.OnClick(func() { a.requestOpenPath(p) })
}

func shortenPath(p string, max int) string {
	if len(p) <= max {
		return p
	}
	half := (max - 1) / 2
	return p[:half] + "…" + p[len(p)-half:]
}

func (a *app) dashboardPage(c *ui.Context) {
	if a.proj == nil {
		a.welcome(c)
		return
	}
	t := c.Theme()
	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		ui.Row(c).FillWidth().Height(34).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
				ic := ui.Icon(c, icEye)
				ic.FontSize(15).TextColor(t.Accent)
				ui.Text(c, "实时预览").FontSize(13).Bold()
			})
			ui.Badge(c, "1024 × 600")
			ui.Text(c, "拖动选中元素可调整位置").FontSize(10).TextColor(t.TextMuted)
			ui.Spacer(c)
			// Zoom.
			if ui.Button(c, "−").Key("zoom-out").Width(30).Clicked() {
				a.stepZoom(-1)
			}
			ui.Text(c, fmt.Sprintf("%d%%", int(math.Round(float64(a.effectiveScale())*100)))).FontSize(11).Width(42).TextAlign(ui.Center)
			if ui.Button(c, "+").Key("zoom-in").Width(30).Clicked() {
				a.stepZoom(1)
			}
			if ui.Button(c, "适应").Key("zoom-fit").Clicked() {
				a.zoom = 0
				a.saveSettings()
			}
			ui.Box(c).Width(4)
			modes := []string{"动力 1", "动力 2", "动力 3", "动力 4"}
			modeName := modes[a.mode]
			c.Key("preview-mode")
			if ui.Select(c, &modeName, modes).Width(92).Changed() {
				for i, n := range modes {
					if n == modeName {
						a.mode = i
					}
				}
				a.saveSettings()
				a.invalidatePreview()
			}
		})
		ui.Box(c).Grow(1).Radius(12).Clip().Background(ui.Hex("#141418")).
			Draw(func(p *ui.Painter, r ui.Rect) {
				if a.showGrid {
					step := float32(28)
					for x := r.X + step/2; x < r.X+r.W; x += step {
						for y := r.Y + step/2; y < r.Y+r.H; y += step {
							p.Fill(ui.Rect{X: x, Y: y, W: 1, H: 1}, ui.RGBA(255, 255, 255, 0.055), 0)
						}
					}
				}
			}).Children(func() {
			a.previewStage(c)
		})
		a.previewDataBar(c)
	})
}

func (a *app) stepZoom(dir int) {
	cur := a.effectiveScale()
	steps := []float32{0.4, 0.5, 0.6, 0.75, 0.9, 1.0, 1.25}
	if dir < 0 {
		for i := len(steps) - 1; i >= 0; i-- {
			if steps[i] < cur-0.01 {
				a.zoom = float64(steps[i])
				a.saveSettings()
				return
			}
		}
		a.zoom = float64(steps[0])
	} else {
		for i := 0; i < len(steps); i++ {
			if steps[i] > cur+0.01 {
				a.zoom = float64(steps[i])
				a.saveSettings()
				return
			}
		}
		a.zoom = float64(steps[len(steps)-1])
	}
	a.saveSettings()
}

func (a *app) previewStage(c *ui.Context) {
	zoomed := a.zoom > 0 && float32(a.zoom) > a.fitScale()+0.002
	if zoomed {
		ui.ScrollBoth(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(22).Children(func() {
				a.previewCanvas(c)
			})
		})
		return
	}
	ui.Box(c).Fill().Center().Padding(22).Children(func() {
		a.previewCanvas(c)
	})
}

func (a *app) previewCanvas(c *ui.Context) {
	scale := a.effectiveScale()
	a.previewScale = scale
	w := float32(themeW) * scale
	h := float32(themeH) * scale
	box := ui.Box(c).Key("preview-canvas").Size(w, h).Focusable().Label("仪表预览")
	box.HandleInput(a.handlePreviewInput)
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
			a.previewBox = r
			a.previewScale = r.W / themeW
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
			a.drawOverlay(p, r, c.Theme())
		})
	})
	if box.Clicked() {
		box.Focus()
	}
}

func (a *app) previewDataBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Padding(10, 12).Gap(16).Radius(9).Background(t.Surface).Border(1, t.Border).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Width(92).Gap(3).Children(func() {
			ui.Text(c, "预览车速").FontSize(9.5).TextColor(t.TextMuted)
			sp := float64(a.speed)
			c.Key("preview-speed")
			if ui.NumberInput(c, &sp, 0, 299, 1).Changed() {
				a.speed = int(sp)
				a.invalidatePreview()
			}
		})
		ui.Column(c).Width(190).Gap(3).Children(func() {
			ui.Row(c).Children(func() {
				ui.Text(c, "电量").FontSize(9.5).TextColor(t.TextMuted)
				ui.Spacer(c)
				ui.Text(c, fmt.Sprintf("%d%%", a.soc)).FontSize(9.5).TextColor(t.TextMuted)
			})
			so := float64(a.soc)
			c.Key("preview-soc")
			if ui.Slider(c, &so, 0, 100).Changed() {
				a.soc = int(so)
				a.invalidatePreview()
			}
		})
		ui.Column(c).Width(70).Gap(3).Children(func() {
			ui.Text(c, "档位").FontSize(9.5).TextColor(t.TextMuted)
			opts := []string{"P", "0", "1", "2", "3", "4", "5"}
			c.Key("preview-gear")
			if ui.Select(c, &a.gear, opts).Changed() {
				a.invalidatePreview()
			}
		})
		ui.Column(c).Width(78).Gap(3).Children(func() {
			ui.Text(c, "时间").FontSize(9.5).TextColor(t.TextMuted)
			c.Key("preview-clock")
			if ui.TextInput(c, &a.clock).Changed() {
				a.invalidatePreview()
			}
		})
		ui.Spacer(c)
		ui.Column(c).Gap(2).AlignItems(ui.End).Children(func() {
			if a.sel != "" {
				if it, ok := a.itemByName(a.sel); ok {
					ui.Text(c, "已选中 "+it.Label).FontSize(10).Bold()
					ui.Text(c, fmt.Sprintf("x %d · y %d · %d×%d", it.X, it.Y, it.W, it.H)).FontSize(9.5).TextColor(t.TextMuted)
				}
			} else {
				ui.Text(c, "点击画面里的元素进行选择").FontSize(10).TextColor(t.TextMuted)
				ui.Text(c, "方向键微调 1px，Shift + 方向键 10px").FontSize(9.5).TextColor(t.TextMuted)
			}
		})
	})
}

// ---- Shared UI bits --------------------------------------------------------

func (a *app) inspector(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Width(320).FillHeight().Background(t.Background).Children(func() {
		c.Key(fmt.Sprintf("inspector-%d", a.page))
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(14).Gap(12).Children(func() {
				switch a.page {
				case pageDashboard:
					a.dashboardInspector(c)
				case pageScenes:
					a.scenesInspector(c)
				case pageAssets:
					a.assetsInspector(c)
				case pageInfo:
					a.infoInspector(c)
				case pageExport:
					a.exportInspector(c)
				}
			})
		})
	})
}

func section(c *ui.Context, title, subtitle string, body func()) {
	t := c.Theme()
	ui.Column(c).FillWidth().Gap(9).Padding(12).Radius(9).Background(t.Surface).Border(1, t.Border).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(12).Bold()
		}
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(10).LineHeight(1.45).TextColor(t.TextMuted)
		}
		body()
	})
}

func field(c *ui.Context, label, key string, value *string, placeholder string) {
	t := c.Theme()
	ui.Column(c).Gap(3).Grow(1).Children(func() {
		ui.Text(c, label).FontSize(9.5).TextColor(t.TextMuted)
		c.Key(key)
		ui.TextInput(c, value).Placeholder(placeholder).Label(label).FontSize(12)
	})
}

func fieldRow(c *ui.Context, children func()) {
	ui.Row(c).FillWidth().Gap(10).Children(children)
}

func kv(c *ui.Context, k, v string) {
	t := c.Theme()
	ui.Row(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, k).FontSize(10).TextColor(t.TextMuted).Width(92)
		ui.Text(c, v).FontSize(10.5).Grow(1)
	})
}

func (a *app) checkList(c *ui.Context) {
	t := c.Theme()
	checks := a.checks()
	for _, ch := range checks {
		col := t.Success
		if !ch.OK {
			col = t.Danger
		} else if ch.Warn {
			col = t.Warning
		}
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(col).Margin(5, 0, 0, 0)
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				ui.Row(c).Gap(6).Children(func() {
					ui.Text(c, ch.Label).FontSize(10.5)
					if ch.Warn {
						ui.Text(c, "注意").FontSize(9).TextColor(t.Warning)
					}
				})
				ui.Text(c, ch.Detail).FontSize(9.5).TextColor(t.TextMuted)
			})
		})
	}
}

func (a *app) dirtySummary(c *ui.Context) {
	if a.proj == nil {
		return
	}
	t := c.Theme()
	summary := fmt.Sprintf("图片 %d 张 · 控件位置 %d 个", a.proj.dirtyAssetCount(), a.layoutEdits)
	if a.meta != a.metaSnapshot {
		summary += " · 信息已修改"
	}
	ui.Text(c, summary).FontSize(10.5).TextColor(t.TextMuted)
	if a.unsaved {
		ui.Text(c, "有未导出的改动").FontSize(10.5).TextColor(t.Warning)
	} else {
		ui.Text(c, "已与导出的主题包一致").FontSize(10.5).TextColor(t.Success)
	}
}

func (a *app) assetThumb(c *ui.Context, rel string, height float32) {
	t := c.Theme()
	ui.Box(c).FillWidth().Height(height).Radius(6).Clip().Background(ui.Hex("#101013")).
		Draw(func(p *ui.Painter, r ui.Rect) {
			a.drawChecker(p, r, 9, ui.Hex("#2b2b31"), ui.Hex("#35353c"))
			if b := a.thumb(rel); b != nil {
				pad := float32(5)
				p.Image(b, ui.Rect{X: r.X + pad, Y: r.Y + pad, W: r.W - pad*2, H: r.H - pad*2}, ui.Contain)
			} else {
				p.Text(r.X+8, r.Y+r.H/2-6, "无法预览", 10, t.TextMuted)
			}
		})
}

func (a *app) drawChecker(p *ui.Painter, r ui.Rect, cell float32, a1, b1 ui.Color) {
	nx := int(math.Ceil(float64(r.W / cell)))
	ny := int(math.Ceil(float64(r.H / cell)))
	for y := 0; y < ny; y++ {
		for x := 0; x < nx; x++ {
			col := a1
			if (x+y)%2 == 0 {
				col = b1
			}
			p.Fill(ui.Rect{X: r.X + float32(x)*cell, Y: r.Y + float32(y)*cell, W: cell, H: cell}, col, 0)
		}
	}
}
