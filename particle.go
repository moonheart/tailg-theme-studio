package main

import (
	"os"
	"path"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui/video"
)

// particleRel finds the theme's animated background video from
// dashboard/etc/dynamic-load.ini:
//
//	[particle_load]
//	particle_stat=1
//	particle_path="dashboard/style/particle"
//
// The file itself is night.mp4 in this theme; other themes may name it
// differently, so the directory is scanned when the usual names are absent.
func (a *app) particleRel() string {
	if a.proj == nil {
		return ""
	}
	cfg := parseThemeConfig([]byte(readRel(a.proj, "dashboard/etc/dynamic-load.ini")))
	if cfg.Get("particle_stat") == "0" {
		return ""
	}
	base := strings.TrimSpace(strings.ReplaceAll(cfg.Get("particle_path"), "\\", "/"))
	base = strings.Trim(base, "/")
	if base == "" {
		base = "dashboard/style/particle"
	}
	for _, name := range []string{"night.mp4", "day.mp4", "bg.mp4", "particle.mp4"} {
		rel := base + "/" + name
		if a.proj.Has(rel) {
			return rel
		}
	}
	entries, err := os.ReadDir(a.proj.Abs(base))
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".mp4") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return ""
	}
	return path.Join(base, names[0])
}

func (a *app) loadParticle() {
	a.stopParticle()
	if a.proj == nil || !a.particleEnabled {
		return
	}
	rel := a.particleRel()
	if rel == "" {
		return
	}
	player, err := video.Open(a.proj.Abs(rel))
	if err != nil {
		a.particleErr = err.Error()
		return
	}
	player.SetLoop(true).SetMuted(true)
	a.particle = player
	a.particlePath = rel
	a.particleErr = ""
	a.particleReady = false
}

func (a *app) stopParticle() {
	if a.particle != nil {
		a.particle.Close()
		a.particle = nil
	}
	a.particleReady = false
}

func (a *app) particleAvailable() bool {
	return a.particle != nil || a.particleRel() != ""
}

// showsParticle tells whether the current page wants the animated
// background behind its canvas.
func (a *app) showsParticle() bool {
	if a.particle == nil || !a.particleEnabled || a.mode != 0 {
		return false
	}
	return a.page == pageDashboard || (a.page == pageScenes && a.scene == sceneDash)
}

// videoBgActive is true once the video has a picture and can replace the
// static bg_N.png background. Before that, the canvas paints the static
// background over the video.
func (a *app) videoBgActive() bool {
	return a.showsParticle() && a.particle.Ready()
}
