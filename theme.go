package main

import (
	"archive/zip"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	xdraw "golang.org/x/image/draw"
)

// ThemeConfig is a config.ini kept as its original lines, so comments and
// fields written by the vendor survive an export.
type ThemeConfig struct {
	path  string
	lines []configLine
	index map[string]int
}

type configLine struct {
	raw   string
	key   string
	value string
	quote bool
	known bool
}

var numericConfigKeys = map[string]bool{
	"PackSize": true, "FileSize": true, "FileCount": true,
}

func parseThemeConfig(data []byte) *ThemeConfig {
	c := &ThemeConfig{index: map[string]int{}}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	for _, raw := range strings.Split(text, "\n") {
		line := configLine{raw: raw}
		trim := strings.TrimSpace(raw)
		if trim != "" && !strings.HasPrefix(trim, "#") && !strings.HasPrefix(trim, ";") &&
			!strings.HasPrefix(trim, "[") {
			if eq := strings.Index(trim, "="); eq > 0 {
				key := strings.TrimSpace(trim[:eq])
				val := strings.TrimSpace(trim[eq+1:])
				quote := len(val) >= 2 && strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`)
				if quote {
					val = val[1 : len(val)-1]
				}
				line.key, line.value, line.quote, line.known = key, val, quote, true
				if _, exists := c.index[key]; !exists {
					c.index[key] = len(c.lines)
				}
			}
		}
		c.lines = append(c.lines, line)
	}
	return c
}

func (c *ThemeConfig) Get(key string) string {
	if c == nil {
		return ""
	}
	if i, ok := c.index[key]; ok {
		return c.lines[i].value
	}
	return ""
}

func (c *ThemeConfig) Set(key, value string) {
	if c == nil {
		return
	}
	if i, ok := c.index[key]; ok {
		l := &c.lines[i]
		l.value = value
		indent := l.raw[:len(l.raw)-len(strings.TrimLeft(l.raw, " \t"))]
		if l.quote || !numericConfigKeys[key] {
			l.raw = indent + key + `="` + value + `"`
			l.quote = true
		} else {
			l.raw = indent + key + "=" + value
			l.quote = false
		}
		return
	}
	quote := !numericConfigKeys[key]
	var raw string
	if quote {
		raw = key + `="` + value + `"`
	} else {
		raw = key + "=" + value
	}
	c.index[key] = len(c.lines)
	line := configLine{raw: raw, key: key, value: value, quote: quote, known: true}
	// Insert before trailing blank lines so the new field stays with the
	// other fields, and the file keeps its final newline.
	at := len(c.lines)
	for at > 0 && strings.TrimSpace(c.lines[at-1].raw) == "" {
		at--
	}
	c.lines = append(c.lines, configLine{})
	copy(c.lines[at+1:], c.lines[at:])
	c.lines[at] = line
	c.index = map[string]int{}
	for i, l := range c.lines {
		if l.known {
			if _, ok := c.index[l.key]; !ok {
				c.index[l.key] = i
			}
		}
	}
}

func (c *ThemeConfig) Remove(key string) {
	i, ok := c.index[key]
	if !ok {
		return
	}
	c.lines = append(c.lines[:i], c.lines[i+1:]...)
	c.index = map[string]int{}
	for j, l := range c.lines {
		if l.known {
			if _, exists := c.index[l.key]; !exists {
				c.index[l.key] = j
			}
		}
	}
}

func (c *ThemeConfig) Bytes() []byte {
	if c == nil {
		return nil
	}
	text := make([]string, len(c.lines))
	for i, l := range c.lines {
		text[i] = l.raw
	}
	out := strings.Join(text, "\n")
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out)
}

// Asset is one image in the theme package.
type Asset struct {
	Rel     string
	Size    int64
	ModTime time.Time
}

// ThemeProject is an editable working copy of an outer theme zip, a
// package.zip, or an extracted theme folder. The source is never modified.
type ThemeProject struct {
	SourcePath  string
	SourceKind  string
	SourceName  string
	TempDir     string
	PackDir     string
	ThemeRoot   string
	RootPrefix  string
	Config      *ThemeConfig
	ConfigPath  string
	LayoutPath  string
	LayoutText  string
	Widgets     map[string]*Widget
	OriginalPos map[string]Point
	Assets      []Asset
	AssetDirs   []string
	Original    map[string][]byte
	Dirty       map[string]bool
	Scene       *SceneData
	LayoutDirty bool
	MetaDirty   bool
	bgCache     map[int]string
	BaseMD5     string
	BaseSize    int64
	BaseCount   int
	OpenTime    time.Time
}

func openProject(src string) (*ThemeProject, error) {
	info, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "theme-studio-")
	if err != nil {
		return nil, err
	}
	p := &ThemeProject{
		SourcePath:  src,
		SourceName:  filepath.Base(src),
		TempDir:     tmp,
		Original:    map[string][]byte{},
		Dirty:       map[string]bool{},
		Widgets:     map[string]*Widget{},
		OriginalPos: map[string]Point{},
		OpenTime:    time.Now(),
	}
	cleanup := func(e error) (*ThemeProject, error) {
		os.RemoveAll(tmp)
		return nil, e
	}

	switch {
	case info.IsDir():
		if isOuterDir(src) {
			p.SourceKind = "outer"
			if err := copyFile(filepath.Join(src, "config.ini"), filepath.Join(tmp, "config.ini")); err != nil {
				return cleanup(err)
			}
			packCopy := filepath.Join(tmp, "package.zip")
			if err := copyFile(filepath.Join(src, "package.zip"), packCopy); err != nil {
				return cleanup(err)
			}
			if err := unzipTo(packCopy, filepath.Join(tmp, "package")); err != nil {
				return cleanup(err)
			}
			p.PackDir = filepath.Join(tmp, "package")
		} else if looksLikeThemeRoot(src) {
			p.SourceKind = "folder"
			p.PackDir = filepath.Join(tmp, "package")
			if err := copyTree(src, p.PackDir); err != nil {
				return cleanup(err)
			}
		} else {
			return cleanup(errors.New("这个文件夹里没有 config.ini + package.zip，也不是解压后的主题目录"))
		}

	case strings.EqualFold(filepath.Ext(src), ".zip"):
		if err := p.readZipSource(src, tmp); err != nil {
			return cleanup(err)
		}
	default:
		return cleanup(errors.New("请选择主题 zip、package.zip，或解压后的主题文件夹"))
	}

	root, err := findThemeRoot(p.PackDir)
	if err != nil {
		return cleanup(err)
	}
	p.ThemeRoot = root
	if rel, err := filepath.Rel(p.PackDir, root); err == nil && rel != "." {
		p.RootPrefix = filepath.ToSlash(rel)
	}
	p.LayoutPath = filepath.Join(root, "dashboard", "etc", "layout.ini")
	layout, err := os.ReadFile(p.LayoutPath)
	if err != nil {
		return cleanup(fmt.Errorf("读取仪表布局失败：%w", err))
	}
	p.LayoutText = string(layout)
	p.Widgets = parseWidgets(p.LayoutText)
	for name, w := range p.Widgets {
		p.OriginalPos[name] = Point{X: w.X, Y: w.Y}
	}

	cfgPath := filepath.Join(tmp, "config.ini")
	if data, err := os.ReadFile(cfgPath); err == nil {
		p.ConfigPath = cfgPath
		p.Config = parseThemeConfig(data)
	} else {
		p.ConfigPath = cfgPath
		p.Config = defaultConfig(p)
	}
	p.BaseMD5 = md5OfFile(filepath.Join(tmp, "package.zip"))
	p.BaseSize = dirSize(p.PackDir)
	p.BaseCount = countFiles(p.PackDir)
	if err := p.scanAssets(); err != nil {
		return cleanup(err)
	}
	p.Scene = loadSceneData(p)
	return p, nil
}

func (p *ThemeProject) readZipSource(src, tmp string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	names := map[string]*zip.File{}
	for _, f := range zr.File {
		names[path.Clean(f.Name)] = f
	}
	cfg, hasCfg := names["config.ini"]
	pack, hasPack := names["package.zip"]
	if hasCfg && hasPack {
		p.SourceKind = "outer"
		if err := extractZipFile(cfg, filepath.Join(tmp, "config.ini")); err != nil {
			return err
		}
		packCopy := filepath.Join(tmp, "package.zip")
		if err := extractZipFile(pack, packCopy); err != nil {
			return err
		}
		p.PackDir = filepath.Join(tmp, "package")
		return unzipTo(packCopy, p.PackDir)
	}
	// A plain package.zip: unpack it as the resource tree.
	p.SourceKind = "package"
	p.PackDir = filepath.Join(tmp, "package")
	if err := unzipTo(src, p.PackDir); err != nil {
		return err
	}
	return nil
}

func isOuterDir(dir string) bool {
	_, e1 := os.Stat(filepath.Join(dir, "config.ini"))
	_, e2 := os.Stat(filepath.Join(dir, "package.zip"))
	return e1 == nil && e2 == nil
}

func looksLikeThemeRoot(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "dashboard", "etc", "layout.ini"))
	return err == nil
}

func findThemeRoot(packDir string) (string, error) {
	if looksLikeThemeRoot(packDir) {
		return packDir, nil
	}
	var found string
	filepath.WalkDir(packDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if d.IsDir() && looksLikeThemeRoot(p) {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", errors.New("主题包里找不到 dashboard/etc/layout.ini")
	}
	return found, nil
}

func defaultConfig(p *ThemeProject) *ThemeConfig {
	name := p.RootPrefix
	if name == "" {
		name = strings.TrimSuffix(p.SourceName, filepath.Ext(p.SourceName))
	}
	c := parseThemeConfig(nil)
	c.Set("ThemeName", name)
	c.Set("ThemeID", "")
	c.Set("ThemeVer", "0.0.1")
	c.Set("SerialNum", "02")
	c.Set("PackName", name)
	c.Set("ThemeType", "")
	c.Set("ThemeConf", "S15")
	c.Set("BuildDate", time.Now().Format("060102"))
	c.Set("BuildTime", time.Now().Format("150405"))
	c.Set("StartVersion", "")
	c.Set("PackSize", "0")
	c.Set("FileSize", "0")
	c.Set("FileCount", "0")
	c.Set("Md5", "")
	c.Set("ResourcePath", "/data/theme/"+name)
	return c
}

func (p *ThemeProject) scanAssets() error {
	p.Assets = nil
	dirs := map[string]bool{}
	err := filepath.WalkDir(p.ThemeRoot, func(fp string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(fp))
		if !isImageExt(ext) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(p.ThemeRoot, fp)
		rel = filepath.ToSlash(rel)
		p.Assets = append(p.Assets, Asset{Rel: rel, Size: info.Size(), ModTime: info.ModTime()})
		dirs[path.Dir(rel)] = true
		return nil
	})
	if err != nil {
		return err
	}
	p.AssetDirs = p.AssetDirs[:0]
	for d := range dirs {
		p.AssetDirs = append(p.AssetDirs, d)
	}
	sort.Strings(p.AssetDirs)
	sort.Slice(p.Assets, func(i, j int) bool { return naturalLess(p.Assets[i].Rel, p.Assets[j].Rel) })
	return nil
}

// naturalLess orders names with numbers in them as a person counts, so
// 2.png comes before 10.png.
func naturalLess(a, b string) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if isDigit(a[i]) && isDigit(b[j]) {
			si, sj := i, j
			for i < len(a) && isDigit(a[i]) {
				i++
			}
			for j < len(b) && isDigit(b[j]) {
				j++
			}
			na := strings.TrimLeft(a[si:i], "0")
			nb := strings.TrimLeft(b[sj:j], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			continue
		}
		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
	return len(a) < len(b)
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func isImageExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".bmp", ".gif":
		return true
	}
	return false
}

func (p *ThemeProject) Abs(rel string) string {
	return filepath.Join(p.ThemeRoot, filepath.FromSlash(rel))
}

func (p *ThemeProject) Has(rel string) bool {
	_, err := os.Stat(p.Abs(rel))
	return err == nil
}

// ReplaceAsset writes srcPath over rel, resized to the old dimensions. A
// background is cropped to fill instead of being stretched.
func (p *ThemeProject) ReplaceAsset(rel, srcPath string, cover bool) error {
	target := p.Abs(rel)
	oldW, oldH := imageSize(target)
	img, _, err := decodeImageFile(srcPath)
	if err != nil {
		return fmt.Errorf("读取图片失败：%w", err)
	}
	if oldW <= 0 || oldH <= 0 {
		b := img.Bounds()
		oldW, oldH = b.Dx(), b.Dy()
	}
	out := image.NewNRGBA(image.Rect(0, 0, oldW, oldH))
	if cover {
		scale := maxf(float64(oldW)/float64(img.Bounds().Dx()), float64(oldH)/float64(img.Bounds().Dy()))
		w := int(float64(img.Bounds().Dx()) * scale)
		h := int(float64(img.Bounds().Dy()) * scale)
		tmp := image.NewNRGBA(image.Rect(0, 0, w, h))
		xdraw.CatmullRom.Scale(tmp, tmp.Bounds(), img, img.Bounds(), draw.Over, nil)
		offX, offY := (w-oldW)/2, (h-oldH)/2
		draw.Draw(out, out.Bounds(), tmp, image.Pt(offX, offY), draw.Src)
	} else {
		xdraw.CatmullRom.Scale(out, out.Bounds(), img, img.Bounds(), draw.Over, nil)
	}
	if _, ok := p.Original[rel]; !ok {
		if old, err := os.ReadFile(target); err == nil {
			p.Original[rel] = old
		}
	}
	tmp := target + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := png.Encode(f, out); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		return err
	}
	p.Dirty[rel] = true
	p.refreshAssetMeta(rel)
	return nil
}

func (p *ThemeProject) refreshAssetMeta(rel string) {
	info, err := os.Stat(p.Abs(rel))
	if err != nil {
		return
	}
	for i := range p.Assets {
		if p.Assets[i].Rel == rel {
			p.Assets[i].Size = info.Size()
			p.Assets[i].ModTime = info.ModTime()
			break
		}
	}
}

// ReplaceFolder copies every file from dir whose name matches a file in the
// selected asset's folder, so a numbered set (0.png ... 9.png) can be
// replaced in one go.
func (p *ThemeProject) ReplaceFolder(rel, dir string) (int, error) {
	folder := path.Dir(rel)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	byName := map[string]string{}
	for _, a := range p.Assets {
		if path.Dir(a.Rel) == folder {
			byName[strings.ToLower(path.Base(a.Rel))] = a.Rel
		}
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dstRel, ok := byName[strings.ToLower(e.Name())]
		if !ok {
			continue
		}
		if err := p.ReplaceAsset(dstRel, filepath.Join(dir, e.Name()), strings.Contains(dstRel, "/bg/")); err != nil {
			return n, err
		}
		n++
	}
	if n == 0 {
		return 0, errors.New("文件夹里没有与当前目录同名的图片")
	}
	return n, nil
}

func (p *ThemeProject) RevertAsset(rel string) error {
	data, ok := p.Original[rel]
	if !ok {
		return errors.New("这个文件还没有被替换过")
	}
	if err := os.WriteFile(p.Abs(rel), data, 0o644); err != nil {
		return err
	}
	delete(p.Original, rel)
	delete(p.Dirty, rel)
	p.refreshAssetMeta(rel)
	return nil
}

func (p *ThemeProject) Asset(rel string) (Asset, bool) {
	for _, a := range p.Assets {
		if a.Rel == rel {
			return a, true
		}
	}
	return Asset{}, false
}

func (p *ThemeProject) Widget(name string) *Widget {
	return p.Widgets[name]
}

func (p *ThemeProject) MoveWidget(name string, x, y int) bool {
	w := p.Widgets[name]
	if w == nil {
		return false
	}
	if x == w.X && y == w.Y {
		return false
	}
	text, ok := moveWidgetInText(p.LayoutText, name, x, y)
	if !ok {
		return false
	}
	p.LayoutText = text
	p.Widgets = parseWidgets(text)
	p.LayoutDirty = true
	return true
}

func (p *ThemeProject) dirtyAssetCount() int {
	return len(p.Dirty)
}

func (p *ThemeProject) meta() themeMeta {
	return themeMeta{
		DisplayName:  p.Config.Get("CustomName"),
		ThemeName:    p.Config.Get("ThemeName"),
		ThemeID:      p.Config.Get("ThemeID"),
		ThemeVer:     p.Config.Get("ThemeVer"),
		ThemeType:    p.Config.Get("ThemeType"),
		ThemeConf:    p.Config.Get("ThemeConf"),
		PackName:     p.Config.Get("PackName"),
		StartVersion: p.Config.Get("StartVersion"),
		ResourcePath: p.Config.Get("ResourcePath"),
		Author:       p.Config.Get("CustomAuthor"),
		Description:  p.Config.Get("CustomDesc"),
	}
}

type themeMeta struct {
	DisplayName  string
	ThemeName    string
	ThemeID      string
	ThemeVer     string
	ThemeType    string
	ThemeConf    string
	PackName     string
	StartVersion string
	ResourcePath string
	Author       string
	Description  string
}

func (p *ThemeProject) applyMeta(m themeMeta) {
	if m.DisplayName != "" {
		p.Config.Set("CustomName", m.DisplayName)
	} else {
		p.Config.Remove("CustomName")
	}
	p.Config.Set("ThemeName", m.ThemeName)
	p.Config.Set("ThemeID", m.ThemeID)
	p.Config.Set("ThemeVer", m.ThemeVer)
	p.Config.Set("ThemeType", m.ThemeType)
	p.Config.Set("ThemeConf", m.ThemeConf)
	p.Config.Set("PackName", m.PackName)
	p.Config.Set("StartVersion", m.StartVersion)
	p.Config.Set("ResourcePath", m.ResourcePath)
	if m.Author != "" {
		p.Config.Set("CustomAuthor", m.Author)
	} else {
		p.Config.Remove("CustomAuthor")
	}
	if m.Description != "" {
		p.Config.Set("CustomDesc", m.Description)
	} else {
		p.Config.Remove("CustomDesc")
	}
}

type ExportOptions struct {
	Path        string
	BumpVersion bool
}

type ExportResult struct {
	Path      string
	Size      int64
	MD5       string
	PackSize  int64
	FileCount int
	Version   string
}

func (p *ThemeProject) Export(m themeMeta, opts ExportOptions) (*ExportResult, error) {
	if opts.Path == "" {
		return nil, errors.New("没有选择导出位置")
	}
	if p.LayoutDirty {
		if err := os.WriteFile(p.LayoutPath, []byte(p.LayoutText), 0o644); err != nil {
			return nil, err
		}
		p.LayoutDirty = false
	}
	if opts.BumpVersion {
		m.ThemeVer = bumpPatch(m.ThemeVer)
	}
	if m.ThemeVer == "" {
		m.ThemeVer = "0.0.1"
	}
	p.applyMeta(m)

	packPath := filepath.Join(p.TempDir, "export_package.zip")
	size, count, sum, err := zipThemeDir(p.PackDir, packPath)
	if err != nil {
		return nil, err
	}
	// Keep the vendor's FileSize convention: an installer-size hint larger
	// than the archive itself. A theme without one gets the archive size.
	fileSize := size
	if oldPack, err1 := strconv.ParseInt(p.Config.Get("PackSize"), 10, 64); err1 == nil {
		if oldFile, err2 := strconv.ParseInt(p.Config.Get("FileSize"), 10, 64); err2 == nil && oldFile > oldPack {
			fileSize = size + (oldFile - oldPack)
		}
	}
	p.Config.Set("PackSize", strconv.FormatInt(size, 10))
	p.Config.Set("FileSize", strconv.FormatInt(fileSize, 10))
	p.Config.Set("FileCount", strconv.Itoa(count))
	p.Config.Set("Md5", sum)
	p.Config.Set("BuildDate", time.Now().Format("060102"))
	p.Config.Set("BuildTime", time.Now().Format("150405"))
	cfgData := p.Config.Bytes()
	if err := os.WriteFile(p.ConfigPath, cfgData, 0o644); err != nil {
		return nil, err
	}

	tmpOut := opts.Path + ".tmp"
	out, err := os.Create(tmpOut)
	if err != nil {
		return nil, err
	}
	zw := zip.NewWriter(out)
	if err := addZipBytes(zw, "config.ini", cfgData); err != nil {
		zw.Close()
		out.Close()
		os.Remove(tmpOut)
		return nil, err
	}
	if err := addZipFile(zw, "package.zip", packPath); err != nil {
		zw.Close()
		out.Close()
		os.Remove(tmpOut)
		return nil, err
	}
	if err := zw.Close(); err != nil {
		out.Close()
		os.Remove(tmpOut)
		return nil, err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmpOut)
		return nil, err
	}
	if err := os.Rename(tmpOut, opts.Path); err != nil {
		os.Remove(tmpOut)
		return nil, err
	}
	st, err := os.Stat(opts.Path)
	if err != nil {
		return nil, err
	}
	return &ExportResult{
		Path: opts.Path, Size: st.Size(), MD5: sum, PackSize: size,
		FileCount: count, Version: p.Config.Get("ThemeVer"),
	}, nil
}

func (p *ThemeProject) defaultExportName(m themeMeta) string {
	name := m.DisplayName
	if name == "" {
		name = m.ThemeName
	}
	if name == "" {
		name = p.SourceName
	}
	typ := m.ThemeType
	if typ == "" {
		typ = "theme"
	}
	ver := m.ThemeVer
	if ver == "" {
		ver = "0.0.1"
	}
	return sanitizeName(fmt.Sprintf("%s_%s_v%s_studio.zip", name, typ, ver))
}

func (p *ThemeProject) quickExportPath() string {
	dir := filepath.Dir(p.SourcePath)
	if st, err := os.Stat(p.SourcePath); err == nil && st.IsDir() {
		dir = filepath.Dir(p.SourcePath)
	}
	base := strings.TrimSuffix(filepath.Base(p.SourcePath), filepath.Ext(p.SourcePath))
	if base == "" || base == "." {
		base = "theme"
	}
	return filepath.Join(dir, sanitizeName(base)+"_studio.zip")
}

func bumpPatch(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "0.0.1"
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 {
		return "0.0.1"
	}
	last := parts[len(parts)-1]
	n, err := strconv.Atoi(last)
	if err != nil {
		return strings.TrimSuffix(v, ".") + ".1"
	}
	parts[len(parts)-1] = strconv.Itoa(n + 1)
	return strings.Join(parts, ".")
}

func sanitizeName(s string) string {
	repl := strings.NewReplacer(`\`, "_", "/", "_", ":", "_", "*", "_", "?", "_",
		`"`, "_", "<", "_", ">", "_", "|", "_")
	return strings.TrimSpace(repl.Replace(s))
}

func zipThemeDir(dir, out string) (size int64, count int, md5sum string, err error) {
	f, err := os.Create(out)
	if err != nil {
		return 0, 0, "", err
	}
	zw := zip.NewWriter(f)
	var files []string
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if d.IsDir() {
			_, err := zw.CreateHeader(&zip.FileHeader{Name: name + "/", Method: zip.Store, Modified: time.Now()})
			return err
		}
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()})
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, in)
		in.Close()
		if err != nil {
			return err
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		zw.Close()
		f.Close()
		return 0, 0, "", err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return 0, 0, "", err
	}
	if err := f.Close(); err != nil {
		return 0, 0, "", err
	}
	st, err := os.Stat(out)
	if err != nil {
		return 0, 0, "", err
	}
	return st.Size(), len(files), md5OfFile(out), nil
}

func addZipBytes(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()})
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func addZipFile(zw *zip.Writer, name, path string) error {
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()})
	if err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}

func unzipTo(zipPath, dst string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	root, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	links := map[string]string{}
	for _, f := range zr.File {
		name := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		if name == "." || name == "" || strings.HasPrefix(name, "../") || strings.HasPrefix(name, "/") {
			continue
		}
		target := filepath.Join(dst, filepath.FromSlash(name))
		if !pathWithin(target, root) {
			return fmt.Errorf("主题包里有非法路径：%s", f.Name)
		}
		if f.Mode()&os.ModeSymlink != 0 {
			// The official package uses symlinks for libspecial.so* and
			// for repeated P_0..P_3.png gear images. Extract the target
			// file's bytes instead: Windows cannot make symlinks without
			// privilege, and a copied file still loads and exports fine.
			rc, err := f.Open()
			if err != nil {
				return err
			}
			data, err := io.ReadAll(io.LimitReader(rc, 4096))
			rc.Close()
			if err != nil {
				return err
			}
			links[target] = strings.TrimRight(string(data), "\x00\r\n")
			continue
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	for link, targetText := range links {
		resolved, ok := resolveSymlink(link, targetText, links, root, 0)
		if !ok {
			return fmt.Errorf("主题包里的符号链接指向不存在的文件：%s -> %s", filepath.Base(link), targetText)
		}
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			return err
		}
		if err := copyFile(resolved, link); err != nil {
			return err
		}
	}
	return nil
}

func pathWithin(target, root string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel))
}

func resolveSymlink(link, targetText string, links map[string]string, root string, depth int) (string, bool) {
	if depth > 12 {
		return "", false
	}
	next := filepath.FromSlash(targetText)
	if !filepath.IsAbs(next) {
		next = filepath.Join(filepath.Dir(link), next)
	}
	next = filepath.Clean(next)
	if !pathWithin(next, root) {
		return "", false
	}
	if text, ok := links[next]; ok {
		return resolveSymlink(next, text, links, root, depth+1)
	}
	st, err := os.Stat(next)
	if err != nil || !st.Mode().IsRegular() {
		return "", false
	}
	return next, true
}

func extractZipFile(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyTree(src, dst string) error {
	if abs, err := filepath.Abs(src); err == nil {
		src = abs
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if d.Type()&os.ModeSymlink != 0 {
			link, err := os.Readlink(p)
			if err != nil {
				return nil
			}
			if !filepath.IsAbs(link) {
				link = filepath.Join(filepath.Dir(p), link)
			}
			link = filepath.Clean(link)
			if !pathWithin(link, src) {
				return nil
			}
			if st, err := os.Stat(link); err == nil && st.Mode().IsRegular() {
				return copyFile(link, target)
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(p, target)
	})
}

func dirSize(dir string) int64 {
	var n int64
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, e := d.Info(); e == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

func countFiles(dir string) int {
	n := 0
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			n++
		}
		return nil
	})
	return n
}

func md5OfFile(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := md5.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil))
}

func md5OfBytes(b []byte) string {
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}

func imageSize(p string) (int, int) {
	f, err := os.Open(p)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func decodeImageFile(p string) (image.Image, string, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	return image.Decode(f)
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func buildDateNow() (string, string) {
	t := time.Now()
	return t.Format("060102"), t.Format("150405")
}
