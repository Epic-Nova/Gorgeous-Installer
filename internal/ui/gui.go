//go:build cgo

package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/go-piv/piv-go/piv"
	bundle "gorgeous-installer"
	"gorgeous-installer/internal/api"
	"gorgeous-installer/internal/buildinfo"
	"gorgeous-installer/internal/config"
	"gorgeous-installer/internal/installer"
	"gorgeous-installer/internal/settings"
	"gorgeous-installer/internal/unreal"
	"gorgeous-installer/internal/updater"
)

// ─── GT Brand Design Tokens ──────────────────────────────────────────────────
// Primary palette from darkest to brightest — these define the GT ecosystem.

var (
	gtBg0     = color.NRGBA{R: 4, G: 31, B: 44, A: 255}    // #041f2c  window background
	gtBg1     = color.NRGBA{R: 5, G: 42, B: 59, A: 255}    // #052a3b  panel surface
	gtBg2     = color.NRGBA{R: 9, G: 62, B: 86, A: 255}    // #093e56  card
	gtBg3     = color.NRGBA{R: 11, G: 80, B: 110, A: 255}  // #0b506e  card elevated
	gtBg4     = color.NRGBA{R: 13, G: 89, B: 124, A: 255}  // #0d597c  section header
	gtBg5     = color.NRGBA{R: 17, G: 117, B: 162, A: 255} // #1175a2  accent mid
	gtBg6     = color.NRGBA{R: 26, G: 153, B: 255, A: 255} // #1a99ff  accent bright
	gtPrimary = color.NRGBA{R: 0, G: 175, B: 255, A: 255}  // #00afff  GT primary

	gtSidebar = color.NRGBA{R: 3, G: 13, B: 20, A: 255} // deepest rail

	gtTextPrimary   = color.NRGBA{R: 232, G: 244, B: 250, A: 255}
	gtTextSecondary = color.NRGBA{R: 123, G: 175, B: 202, A: 255}
	gtTextDim       = color.NRGBA{R: 71, G: 120, B: 148, A: 255}
	gtTextMuted     = color.NRGBA{R: 38, G: 68, B: 86, A: 255}

	// Mode-specific accent colors (chosen to complement the GT palette)
	accentInstall   = gtPrimary                                  // teal   – install
	accentRecompile = color.NRGBA{R: 127, G: 79, B: 240, A: 255} // purple – recompile
	accentUpdate    = color.NRGBA{R: 240, G: 165, B: 52, A: 255} // amber  – update
	accentReinstall = gtBg6                                      // bright – reinstall
	accentSuccess   = color.NRGBA{R: 44, G: 182, B: 125, A: 255} // green  – success
	accentError     = color.NRGBA{R: 229, G: 49, B: 112, A: 255} // red    – error
)

// withAlpha returns a copy of c with the given alpha.
func withAlpha(c color.NRGBA, a uint8) color.NRGBA {
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: a}
}

// ─── Panel identifiers ───────────────────────────────────────────────────────

type panelID int

const (
	panelInstaller panelID = iota
	panelSHACheck
	panelProjectMgr
	panelPublisher
	panelSettings
)

// ─── navButton  ──────────────────────────────────────────────────────────────
// A custom sidebar navigation item that supports hover and active-indicator
// rendering without requiring any external dependencies.

type navButton struct {
	widget.BaseWidget
	icon    fyne.Resource
	label   string
	active  bool
	hovered bool
	onTap   func()
}

func newNavButton(icon fyne.Resource, label string, onTap func()) *navButton {
	b := &navButton{icon: icon, label: label, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *navButton) SetActive(a bool) { b.active = a; b.Refresh() }

func (b *navButton) Tapped(_ *fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}
func (b *navButton) MouseIn(_ *desktop.MouseEvent)    { b.hovered = true; b.Refresh() }
func (b *navButton) MouseMoved(_ *desktop.MouseEvent) {}
func (b *navButton) MouseOut()                        { b.hovered = false; b.Refresh() }

func (b *navButton) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 10

	indicator := canvas.NewRectangle(gtPrimary)
	indicator.CornerRadius = 2
	indicator.Hide()

	iconImg := canvas.NewImageFromResource(b.icon)
	iconImg.FillMode = canvas.ImageFillContain

	lbl := canvas.NewText(strings.ToUpper(b.label), gtTextMuted)
	lbl.TextSize = 8
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	lbl.Alignment = fyne.TextAlignCenter

	objs := []fyne.CanvasObject{bg, indicator, iconImg, lbl}

	r := &navButtonRenderer{
		btn: b, bg: bg, indicator: indicator,
		iconImg: iconImg, lbl: lbl,
		objs: objs,
	}
	r.Refresh()
	return r
}

type navButtonRenderer struct {
	btn       *navButton
	bg        *canvas.Rectangle
	indicator *canvas.Rectangle
	iconImg   *canvas.Image
	lbl       *canvas.Text
	objs      []fyne.CanvasObject
}

func (r *navButtonRenderer) Layout(sz fyne.Size) {
	r.bg.Resize(fyne.NewSize(sz.Width-8, sz.Height-4))
	r.bg.Move(fyne.NewPos(4, 2))

	r.indicator.Resize(fyne.NewSize(3, sz.Height*0.54))
	r.indicator.Move(fyne.NewPos(2, sz.Height*0.23))

	iconSz := float32(24)
	iconTop := float32(11)
	r.iconImg.Resize(fyne.NewSize(iconSz, iconSz))
	r.iconImg.Move(fyne.NewPos((sz.Width-iconSz)/2, iconTop))

	lblY := iconTop + iconSz + 5
	r.lbl.Resize(fyne.NewSize(sz.Width, 13))
	r.lbl.Move(fyne.NewPos(0, lblY))
}

func (r *navButtonRenderer) MinSize() fyne.Size {
	return fyne.NewSize(88, 70)
}

func (r *navButtonRenderer) Refresh() {
	b := r.btn
	switch {
	case b.active:
		r.bg.FillColor = withAlpha(gtBg5, 50)
		r.indicator.Show()
		r.lbl.Color = gtTextPrimary
	case b.hovered:
		r.bg.FillColor = withAlpha(gtBg5, 25)
		r.indicator.Hide()
		r.lbl.Color = gtTextSecondary
	default:
		r.bg.FillColor = color.Transparent
		r.indicator.Hide()
		r.lbl.Color = gtTextDim
	}
	canvas.Refresh(r.bg)
	canvas.Refresh(r.indicator)
	canvas.Refresh(r.iconImg)
	canvas.Refresh(r.lbl)
}

func (r *navButtonRenderer) Destroy()                     {}
func (r *navButtonRenderer) Objects() []fyne.CanvasObject { return r.objs }

// ─── accentButton ────────────────────────────────────────────────────────────
// A premium CTA button with a colored background. When SetRunning(true), it
// shows an animated pulsing glow ring to indicate active compilation.

type accentButton struct {
	widget.BaseWidget
	label   string
	accent  color.NRGBA
	onTap   func()
	enabled bool
	running bool
}

func newAccentButton(label string, accent color.NRGBA, onTap func()) *accentButton {
	b := &accentButton{label: label, accent: accent, onTap: onTap, enabled: true}
	b.ExtendBaseWidget(b)
	return b
}

func (b *accentButton) SetLabel(l string)       { b.label = l; b.Refresh() }
func (b *accentButton) SetAccent(a color.NRGBA) { b.accent = a; b.Refresh() }
func (b *accentButton) SetEnabled(e bool)       { b.enabled = e; b.Refresh() }
func (b *accentButton) SetRunning(r bool)       { b.running = r; b.Refresh() }
func (b *accentButton) Trigger() {
	if b.enabled && !b.running && b.onTap != nil {
		b.onTap()
	}
}

func (b *accentButton) Tapped(_ *fyne.PointEvent) { b.Trigger() }

func (b *accentButton) CreateRenderer() fyne.WidgetRenderer {
	glow := canvas.NewCircle(color.Transparent)
	glow.StrokeWidth = 3
	glow.StrokeColor = color.Transparent

	bg := canvas.NewRectangle(b.accent)
	bg.CornerRadius = 14

	lbl := canvas.NewText(b.label, color.White)
	lbl.TextSize = 15
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	lbl.Alignment = fyne.TextAlignCenter

	center := container.NewCenter(lbl)

	r := &accentButtonRenderer{
		btn: b, glow: glow, bg: bg, lbl: lbl, center: center,
		objs: []fyne.CanvasObject{glow, bg, center},
	}
	r.Refresh()
	return r
}

type accentButtonRenderer struct {
	btn         *accentButton
	glow        *canvas.Circle
	bg          *canvas.Rectangle
	lbl         *canvas.Text
	center      fyne.CanvasObject
	objs        []fyne.CanvasObject
	glowAnim    *fyne.Animation
	glowStarted bool
}

func (r *accentButtonRenderer) Layout(sz fyne.Size) {
	pad := float32(10)
	r.glow.Resize(fyne.NewSize(sz.Width+pad*2, sz.Height+pad*2))
	r.glow.Move(fyne.NewPos(-pad, -pad))
	r.bg.Resize(sz)
	r.bg.Move(fyne.NewPos(0, 0))
	r.center.Resize(sz)
	r.center.Move(fyne.NewPos(0, 0))
}

func (r *accentButtonRenderer) MinSize() fyne.Size { return fyne.NewSize(200, 52) }

func (r *accentButtonRenderer) Refresh() {
	b := r.btn
	a := b.accent

	if b.running {
		if !r.glowStarted {
			r.glowAnim = canvas.NewColorRGBAAnimation(
				withAlpha(a, 35),
				withAlpha(a, 115),
				920*time.Millisecond,
				func(c color.Color) {
					r.glow.StrokeColor = c
					canvas.Refresh(r.glow)
				},
			)
			r.glowAnim.AutoReverse = true
			r.glowAnim.RepeatCount = fyne.AnimationRepeatForever
			r.glowAnim.Curve = fyne.AnimationEaseInOut
			r.glowAnim.Start()
			r.glowStarted = true
		}
		r.bg.FillColor = color.NRGBA{R: a.R / 2, G: a.G / 2, B: a.B / 2, A: 210}
		r.lbl.Color = withAlpha(color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 140)
	} else {
		if r.glowStarted {
			r.glowAnim.Stop()
			r.glowAnim = nil
			r.glowStarted = false
			r.glow.StrokeColor = color.Transparent
			canvas.Refresh(r.glow)
		}
		if b.enabled {
			r.bg.FillColor = a
			r.lbl.Color = color.White
		} else {
			r.bg.FillColor = withAlpha(a, 55)
			r.lbl.Color = withAlpha(color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 80)
		}
	}

	r.lbl.Text = b.label
	canvas.Refresh(r.bg)
	canvas.Refresh(r.lbl)
}

func (r *accentButtonRenderer) Destroy() {
	if r.glowAnim != nil {
		r.glowAnim.Stop()
	}
}
func (r *accentButtonRenderer) Objects() []fyne.CanvasObject { return r.objs }

// ─── GUIApp ──────────────────────────────────────────────────────────────────

// GUIApp represents the native GUI application.
type GUIApp struct {
	config           *config.Config
	recompileOnly    bool
	waitForPID       int
	reopenProject    bool
	AutoBuildProject bool
	VerifyCompat     bool
	ProjectPath      string
	installSucceeded bool
	installZipPath   string
	RegenerateProject bool

	win         fyne.Window
	modalLayer  *fyne.Container
	toastLayer  *fyne.Container
	toastLayout *toastLayout

	navItemsBox     *fyne.Container
	navPublisherBtn *navButton
}

// ─── Custom Layouts ──────────────────────────────────────────────────────────

type toastLayout struct {
	offsetY float32
}

func (l *toastLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, 0)
}

func (l *toastLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		min := o.MinSize()
		if min.Width < 300 {
			min.Width = 300
		}
		o.Resize(min)
		o.Move(fyne.NewPos(size.Width-min.Width-20, size.Height-min.Height-20+l.offsetY))
	}
}

type modalGroupLayout struct {
	width, height float32
	scale         float32
	offsetY       float32
}

type logAnimLayout struct {
	height float32
}

func (l *logAnimLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) > 0 {
		return fyne.NewSize(10, l.height)
	}
	return fyne.NewSize(0, 0)
}

func (l *logAnimLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) > 0 {
		objects[0].Resize(fyne.NewSize(size.Width, l.height))
		objects[0].Move(fyne.NewPos(0, 0))
	}
}

func (l *modalGroupLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.width*l.scale, l.height*l.scale+45*l.scale+l.offsetY)
}

func (l *modalGroupLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) >= 2 {
		card := objects[0]
		badge := objects[1]

		w := l.width * l.scale
		h := l.height * l.scale
		card.Resize(fyne.NewSize(w, h))
		card.Move(fyne.NewPos(0, 45*l.scale+l.offsetY))

		badge.Resize(fyne.NewSize(90*l.scale, 90*l.scale))
		badge.Move(fyne.NewPos((w-(90*l.scale))/2, l.offsetY))
	}
}

// NewGUIApp creates a new GUI app instance.
func NewGUIApp(cfg *config.Config, recompileOnly bool, waitPid int, reopenProject bool, autoBuildProject bool, verifyCompat bool, installZip string, regenerateProject bool) *GUIApp {
	return &GUIApp{config: cfg, recompileOnly: recompileOnly, waitForPID: waitPid, reopenProject: reopenProject, AutoBuildProject: autoBuildProject, VerifyCompat: verifyCompat, installZipPath: installZip, RegenerateProject: regenerateProject}
}

func (g *GUIApp) isPackless() bool {
	return len(g.config.AvailableVersions) == 0
}

// ─── Run ─────────────────────────────────────────────────────────────────────

// Run starts the Gorgeous Installer GUI.
func (g *GUIApp) Run() {
	application := app.NewWithID("com.gorgeouscore.installer")
	iconRes := loadIconResource()
	if iconRes != nil {
		application.SetIcon(iconRes)
	}

	var win fyne.Window
	if runtime.GOOS == "windows" {
		if d, ok := application.Driver().(desktop.Driver); ok {
			win = d.CreateSplashWindow()
		} else {
			win = application.NewWindow("Gorgeous Installer")
		}
	} else {
		win = application.NewWindow("Gorgeous Installer")
	}
	win.SetTitle("Gorgeous Installer")
	if iconRes != nil {
		win.SetIcon(iconRes)
	}
	win.SetPadded(false)
	win.Resize(fyne.NewSize(920, 560))
	win.SetFixedSize(false)
	win.CenterOnScreen()
	g.win = win

	win.SetCloseIntercept(func() {
		// Try to kill UnrealBuildTool in case it was orphaned
		unreal.KillUBT()
		if g.VerifyCompat {
			os.Exit(2)
		}
		win.Close()
	})

	// ── Shared installation state ─────────────────────────────────────────────
	var (
		mu             sync.Mutex
		installCancel  context.CancelFunc
		installRunning bool

		projectPath      string
		autoPackVersion  string
		manualVersionSel bool
		lastFingerprint  string
		plannedAction    = installer.InstallActionInstall

		logText strings.Builder
	)

	isPackless := g.isPackless()
	versions := g.availableVersions()

	// ── Log entry ─────────────────────────────────────────────────────────────
	logLbl := widget.NewLabel("")
	logLbl.Wrapping = fyne.TextWrapWord
	logScroll := container.NewScroll(logLbl)

	autoScroll := true
	var autoScrollCheck *widget.Check
	autoScrollCheck = widget.NewCheck("Auto Scroll", func(on bool) {
		autoScroll = on
		if on {
			logScroll.ScrollToBottom()
		}
	})
	autoScrollCheck.SetChecked(true)

	autoScrollCheck.SetChecked(true)

	lastLineLbl := canvas.NewText("Ready.", gtTextSecondary)
	lastLineLbl.TextSize = 11

	var uiUpdateQueued bool

	appendStatus := func(msg string, args ...any) {
		line := fmt.Sprintf(msg, args...)
		cleanLine := strings.TrimRight(line, "\r\n")

		mu.Lock()
		logText.WriteString(cleanLine + "\n")

		str := logText.String()
		newlineCount := strings.Count(str, "\n")
		if newlineCount > 1000 {
			idx := 0
			for i := 0; i < newlineCount-1000; i++ {
				nextIdx := strings.Index(str[idx:], "\n")
				if nextIdx == -1 {
					break
				}
				idx += nextIdx + 1
			}
			newStr := str[idx:]
			logText.Reset()
			logText.WriteString(newStr)
			str = newStr
		}

		if !uiUpdateQueued {
			uiUpdateQueued = true
			go func() {
				time.Sleep(50 * time.Millisecond) // Throttle to max 20 FPS
				mu.Lock()
				text := logText.String()
				uiUpdateQueued = false
				mu.Unlock()

				fyne.Do(func() {
					logLbl.SetText(text)
					if autoScroll {
						logScroll.ScrollToBottom()
					}
					setCanvasText(lastLineLbl, cleanLine)
				})
			}()
		}
		mu.Unlock()
	}
	appendStatus("Ready.")
	go g.validateConfiguredPackSHAs(appendStatus)

	navBtns := map[panelID]*navButton{}
	panelObjs := map[panelID]fyne.CanvasObject{}

	g.modalLayer = container.NewStack()
	g.toastLayout = &toastLayout{offsetY: 200}
	g.toastLayer = container.New(g.toastLayout)
	var overlayContainer *fyne.Container = container.NewStack(g.modalLayer, g.toastLayer)
	var mainShell fyne.CanvasObject

	var currentVisiblePanel fyne.CanvasObject

	switchToPanel := func(id panelID) {
		for pid, btn := range navBtns {
			btn.SetActive(pid == id)
		}

		target := panelObjs[id]
		if target == nil || target == currentVisiblePanel {
			return
		}

		if currentVisiblePanel != nil {
			// Fade out old, slide in new.
			// Fyne 2.0 doesn't natively support cross-fade swapping easily without custom widgets,
			// but we can just use Show/Hide and let the container relayout.
			currentVisiblePanel.Hide()
		}

		target.Show()
		currentVisiblePanel = target

		// Slide-in animation for target
		target.Move(fyne.NewPos(0, 20))
		anim := canvas.NewPositionAnimation(fyne.NewPos(0, 20), fyne.NewPos(0, 0), 200*time.Millisecond, func(p fyne.Position) {
			target.Move(p)
			canvas.Refresh(target)
		})
		anim.Curve = fyne.AnimationEaseOut
		anim.Start()

		if id == panelPublisher {
			if !win.FullScreen() {
				win.Resize(fyne.NewSize(1000, 680))
			}
		} else {
			if !win.FullScreen() {
				win.Resize(fyne.NewSize(920, 560))
			}
		}
	}

	// ── Version selector (pack mode only) ─────────────────────────────────────
	var refreshPlanState func()

	versionSelect := widget.NewSelect(versions, func(_ string) {
		if refreshPlanState != nil {
			refreshPlanState()
		}
	})
	versionSelect.PlaceHolder = "Select pack version"
	if len(versions) > 0 {
		versionSelect.SetSelected(versions[0])
	}

	versionSelectRow := container.NewHBox(
		newGTLabel("Version:"),
		versionSelect,
	)
	versionSelectRow.Hide()

	// ── Engine info label ─────────────────────────────────────────────────────
	engineVal := canvas.NewText("Not detected", gtTextSecondary)
	engineVal.TextSize = 14
	engineVal.TextStyle = fyne.TextStyle{Bold: true}

	// ── Status card (pack mode) ───────────────────────────────────────────────
	statusIcon := canvas.NewText("◉", gtTextDim)
	statusIcon.TextSize = 18
	statusText := canvas.NewText("Select a project to begin", gtTextDim)
	statusText.TextSize = 13
	statusBg := canvas.NewRectangle(withAlpha(gtBg3, 180))
	statusBg.CornerRadius = 12
	statusCard := container.NewStack(
		statusBg,
		container.NewPadded(
			container.NewHBox(statusIcon, container.NewPadded(statusText), layout.NewSpacer()),
		),
	)

	updateStatusCard := func(action installer.InstallAction, msg string) {
		statusText.Text = msg
		switch action {
		case installer.InstallActionUpdate:
			statusIcon.Text = "↑"
			statusIcon.Color = accentUpdate
			statusBg.FillColor = withAlpha(accentUpdate, 22)
		case installer.InstallActionReinstall:
			statusIcon.Text = "✓"
			statusIcon.Color = accentSuccess
			statusBg.FillColor = withAlpha(accentSuccess, 22)
		default:
			statusIcon.Text = "↓"
			statusIcon.Color = accentInstall
			statusBg.FillColor = withAlpha(accentInstall, 22)
		}
		canvas.Refresh(statusIcon)
		canvas.Refresh(statusText)
		canvas.Refresh(statusBg)
	}

	// ── Action button ─────────────────────────────────────────────────────────
	actionBtn := newAccentButton("↓  Install", accentInstall, nil) // onTap wired below

	setActionForMode := func(action installer.InstallAction) {
		plannedAction = action
		if isPackless || g.recompileOnly {
			actionBtn.SetAccent(accentRecompile)
			actionBtn.SetLabel("↺  Recompile Plugin")
			return
		}
		switch action {
		case installer.InstallActionUpdate:
			actionBtn.SetAccent(accentUpdate)
			actionBtn.SetLabel("↑  Update")
		case installer.InstallActionReinstall:
			actionBtn.SetAccent(accentReinstall)
			actionBtn.SetLabel("⟳  Reinstall")
		default:
			actionBtn.SetAccent(accentInstall)
			actionBtn.SetLabel("↓  Install")
		}
	}

	// ── Project loader ────────────────────────────────────────────────────────
	projectEntry := widget.NewEntry()
	projectEntry.SetPlaceHolder("Select a .uproject file…")
	projectEntry.Disable()

	loadProject := func(selected string) {
		projectPath = selected
		lastFingerprint = ""
		projectEntry.Enable()
		projectEntry.SetText(projectPath)
		projectEntry.Disable()
		appendStatus("Project selected: %s", filepath.Base(projectPath))

		engineVersion, enginePath, detectErr := unreal.GetEngineVersionFromProject(projectPath)
		if detectErr != nil {
			autoPackVersion = ""
			manualVersionSel = true
			if len(versions) > 0 && versionSelect.Selected == "" {
				versionSelect.SetSelected(versions[0])
			}
			if !isPackless {
				versionSelectRow.Show()
			}
			setCanvasText(engineVal, "Not detected")
			appendStatus("Engine detection failed: %v", detectErr)
			if refreshPlanState != nil {
				refreshPlanState()
			}
			return
		}

		setCanvasText(engineVal, "UE "+engineVersion)
		appendStatus("Detected engine version from EngineAssociation: %s", engineVersion)
		appendStatus("Resolved engine path: %s", enginePath)

		best := chooseClosestVersion(engineVersion, versions)
		if best != "" {
			bestMajor, bestMinor := parseVersion(best)
			engineMajor, engineMinor := parseVersion(engineVersion)
			isConcrete := !(bestMajor > engineMajor || (bestMajor == engineMajor && bestMinor > engineMinor))
			if isConcrete {
				autoPackVersion = best
				manualVersionSel = false
				versionSelect.SetSelected(best)
				versionSelectRow.Hide()
				appendStatus("Selected pack version from project: %s", best)
				if refreshPlanState != nil {
					refreshPlanState()
				}
				return
			}
		}

		if isVersionInSupportedList(engineVersion, supportedVersionsForVersion("Universal", g.config.AvailableVersions)) {
			univVer := "Universal"
			autoPackVersion = univVer
			manualVersionSel = false
			versionSelect.SetSelected(univVer)
			versionSelectRow.Hide()
			appendStatus("Selected Universal build: covers UE %s", engineVersion)
			if refreshPlanState != nil {
				refreshPlanState()
			}
			return
		}

		autoPackVersion = ""
		manualVersionSel = false
		if !isPackless {
			versionSelectRow.Hide()
		}
		appendStatus("No configured pack versions were available for automatic selection")
		if refreshPlanState != nil {
			refreshPlanState()
		}
	}

	// ── Plan refresh ──────────────────────────────────────────────────────────
	refreshPlanState = func() {
		mu.Lock()
		running := installRunning
		mu.Unlock()
		if running {
			return
		}
		if strings.TrimSpace(projectPath) == "" {
			setActionForMode(installer.InstallActionInstall)
			return
		}
		if isPackless {
			setActionForMode(installer.InstallActionReinstall)
			return
		}
		selectedVersion := currentSelectedVersion(autoPackVersion, manualVersionSel, versionSelect)
		if selectedVersion == "" {
			setActionForMode(installer.InstallActionInstall)
			return
		}

		inspectPath := projectPath
		inspectVer := selectedVersion
		appendStatus("Analyzing installed files for pack version %s", inspectVer)

		go func(p, v string) {
			plan, err := g.inspectInstallPlan(p, v)
			fyne.Do(func() {
				if p != projectPath || v != currentSelectedVersion(autoPackVersion, manualVersionSel, versionSelect) {
					return
				}
				if err != nil {
					setActionForMode(installer.InstallActionInstall)
					appendStatus("Could not determine install state: %v", err)
					return
				}
				setActionForMode(plan.Action)
				updateStatusCard(plan.Action, buildStatusMsg(plan))

				fp := fmt.Sprintf("%s|%s|%s|%d|%d|%d|%d",
					p, v, plan.Action, plan.TotalSourceFiles, plan.ExistingFiles,
					len(plan.MissingFiles), len(plan.DifferentFiles))
				if len(plan.ChangedFiles) > 0 {
					fp += "|" + strings.Join(plan.ChangedFiles, "|")
				}
				if fp == lastFingerprint {
					return
				}
				lastFingerprint = fp

				switch plan.Action {
				case installer.InstallActionUpdate:
					appendStatus("Update available: %d files differ", len(plan.ChangedFiles))
					for _, f := range plan.ChangedFiles {
						appendStatus("DIFF: %s", f)
					}
				case installer.InstallActionReinstall:
					appendStatus("Pack version %s is already installed", plan.PackVersion)
					appendStatus("All %d files match; Reinstall is available", plan.TotalSourceFiles)
				default:
					if plan.ExistingFiles > 0 && len(plan.ChangedFiles) == 0 {
						appendStatus("Pack files are already up to date (%d matching files)", plan.UnchangedFiles)
					} else {
						appendStatus("No existing pack files detected; ready for install (%d files)", plan.TotalSourceFiles)
					}
				}
			})
		}(inspectPath, inspectVer)
	}

	// ── Close handler ─────────────────────────────────────────────────────────
	win.SetCloseIntercept(func() {
		mu.Lock()
		cancel := installCancel
		running := installRunning
		installCancel = nil
		installRunning = false
		mu.Unlock()

		if running && cancel != nil {
			appendStatus("Stopping active compilation before closing...")
			cancel()
		}
		win.SetCloseIntercept(nil)
		win.Close()
	})

	// ── Install flow (wired to actionBtn.onTap) ───────────────────────────────
	doInstall := func() {
		if projectPath == "" {
			found := findUProjectUpwards()
			if found != "" {
				loadProject(found)
			} else {
				g.showAnimatedDialog("Error", "please select a .uproject file first", true)
				return
			}
		}

		selectedVersion := currentSelectedVersion(autoPackVersion, manualVersionSel, versionSelect)
		if !isPackless && selectedVersion == "" && g.installZipPath == "" {
			if manualVersionSel {
				g.showAnimatedDialog("Error", "please select a pack version", true)
			} else {
				g.showAnimatedDialog("Error", "could not determine a pack version from the selected project", true)
			}
			return
		}

		action := plannedAction
		if action == "" {
			action = installer.InstallActionInstall
		}

		actionBtn.SetEnabled(false)
		actionBtn.SetRunning(true)

		if g.waitForPID > 0 {
			appendStatus("Waiting for Unreal Engine process (PID %d) to exit...", g.waitForPID)
			waitTicker := time.NewTicker(time.Second)
			defer waitTicker.Stop()
		waitLoop:
			for {
				select {
				case <-time.After(30 * time.Second):
					break waitLoop
				case <-waitTicker.C:
					p, err := os.FindProcess(g.waitForPID)
					if err != nil {
						break waitLoop
					}
					if runtime.GOOS != "windows" {
						if err := p.Signal(os.Signal(nil)); err != nil {
							break waitLoop
						}
						continue
					}
					break waitLoop
				}
			}
		}

		appendStatus("Starting %s for pack version %s", action, selectedVersion)

		installCtx, cancelInstall := context.WithCancel(context.Background())
		mu.Lock()
		installCancel = cancelInstall
		installRunning = true
		mu.Unlock()

		go func(p, v string, installAction installer.InstallAction) {
			defer func() {
				cancelInstall()
				mu.Lock()
				installCancel = nil
				installRunning = false
				mu.Unlock()
			}()

			err := g.performInstall(installCtx, p, v, installAction, appendStatus)

			fyne.Do(func() {
				actionBtn.SetRunning(false)
				actionBtn.SetEnabled(true)

				if err != nil && errors.Is(err, context.Canceled) {
					appendStatus("Installation canceled")
					if refreshPlanState != nil {
						refreshPlanState()
					}
					return
				}

				if err != nil {
					appendStatus("Installation failed: %v", err)
					resultActionLabel := ""
					var resultAction func()
					if isCompileFailure(err) {
						resultActionLabel = "Back to Logs"
						resultAction = func() {
							g.dismissModal()
							if refreshPlanState != nil {
								refreshPlanState()
							}
						}
					}
					g.showInstallResult(false, err.Error(), iconRes, resultActionLabel, resultAction, "Close Installer", nil, "installation")
					return
				}

				g.installSucceeded = true
				appendStatus("Installation completed successfully")

				resultTitle := "Successfully Installed!"
				var primaryAction func()
				var primaryLabel string
				if g.reopenProject {
					resultTitle = "Ready to Rock!"
					primaryLabel = "Restart Engine"
					primaryAction = func() {
						appendStatus("Restarting Unreal Engine...")
						unreal.LaunchUnrealEditor(projectPath)
						win.Close()
					}
				}
				g.showInstallResult(true, resultTitle, iconRes, primaryLabel, primaryAction, "Close Installer", nil, "installation")
			})
		}(projectPath, selectedVersion, action)
	}

	actionBtn.onTap = doInstall

	// Initial button state
	setActionForMode(installer.InstallActionInstall)

	// ── Browse button ─────────────────────────────────────────────────────────
	browseBtn := widget.NewButtonWithIcon("Browse", theme.FolderOpenIcon(), func() {
		d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				g.showAnimatedDialog("Error", err.Error(), true)
				appendStatus("Project selection error: %v", err)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()
			selected := normalizeURIPath(reader.URI())
			if selected == "" {
				appendStatus("Invalid file URI returned by file dialog")
				return
			}
			loadProject(selected)
		}, win)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".uproject"}))
		d.Show()
	})
	browseBtn.Importance = widget.HighImportance

	exitBtn := widget.NewButton("Exit", func() { win.Close() })

	// ── Build panels ──────────────────────────────────────────────────────────

	// Installer panel
	installerPanel := g.buildInstallerPanel(win, iconRes,
		isPackless, projectEntry, browseBtn, versionSelectRow,
		engineVal, statusCard, actionBtn, exitBtn,
		logScroll,
		lastLineLbl,
		autoScrollCheck)
	panelObjs[panelInstaller] = installerPanel

	// SHA panel (hidden in packless mode)
	if !isPackless {
		backToShell := func() {
			animateWindowOpen(win, mainShell)
			switchToPanel(panelSHACheck)
		}
		shaPanel := g.buildSHAValidatorPanel(win, iconRes, &mainShell, backToShell, appendStatus)
		panelObjs[panelSHACheck] = shaPanel
	}

	// New dynamic panels
	panelObjs[panelProjectMgr] = g.buildProjectsPanel(win, appendStatus)
	panelObjs[panelPublisher] = g.buildPublisherPanel(win, appendStatus)
	panelObjs[panelSettings] = g.buildSettingsPanel(win, appendStatus)

	// Stack all panels; only installer starts visible
	var panelList []fyne.CanvasObject
	for id, p := range panelObjs {
		if id != panelInstaller {
			p.Hide()
		} else {
			currentVisiblePanel = p
		}
		panelList = append(panelList, p)
	}
	contentStack := container.NewStack(panelList...)

	// ── Nav rail ──────────────────────────────────────────────────────────────
	navInstaller := newNavButton(theme.DownloadIcon(), "Installer", func() { switchToPanel(panelInstaller) })
	navInstaller.SetActive(true)
	navBtns[panelInstaller] = navInstaller

	if !isPackless {
		navSHA := newNavButton(theme.SearchIcon(), "SHA Check", func() { switchToPanel(panelSHACheck) })
		navBtns[panelSHACheck] = navSHA
	}

	navProjMgr := newNavButton(theme.FolderIcon(), "Projects", func() { switchToPanel(panelProjectMgr) })
	navBtns[panelProjectMgr] = navProjMgr

	navPublisher := newNavButton(theme.UploadIcon(), "Publisher", func() { switchToPanel(panelPublisher) })
	navBtns[panelPublisher] = navPublisher

	navSettings := newNavButton(theme.SettingsIcon(), "Settings", func() { switchToPanel(panelSettings) })
	navBtns[panelSettings] = navSettings

	// Build sidebar content
	var logoWidget fyne.CanvasObject
	if iconRes != nil {
		img := canvas.NewImageFromResource(iconRes)
		img.FillMode = canvas.ImageFillContain
		logoWidget = container.NewGridWrap(fyne.NewSize(38, 38), img)
	} else {
		circle := canvas.NewCircle(gtPrimary)
		logoWidget = container.NewGridWrap(fyne.NewSize(38, 38), circle)
	}
	logoSection := container.NewPadded(container.NewCenter(logoWidget))

	topDivider := canvas.NewRectangle(withAlpha(gtBg5, 60))
	topDivider.SetMinSize(fyne.NewSize(1, 1))

	futureDivider := canvas.NewRectangle(withAlpha(gtBg2, 200))
	futureDivider.SetMinSize(fyne.NewSize(1, 1))

	// Width anchor ensures sidebar is always 90px
	widthAnchor := canvas.NewRectangle(color.Transparent)
	widthAnchor.SetMinSize(fyne.NewSize(90, 0))

	navItems := container.NewVBox(widthAnchor, logoSection, topDivider)
	navItems.Add(navInstaller)
	if nb, ok := navBtns[panelSHACheck]; ok {
		navItems.Add(nb)
	}
	navItems.Add(futureDivider)
	navItems.Add(navProjMgr)

	// The Publisher button is now dynamically toggled via YubiKey listener

	// Make navItems and navPublisher globally accessible for settings panel if we want real-time toggle
	// We can use a package-level variable or just require app restart. We'll use a package variable.
	g.navItemsBox = navItems
	g.navPublisherBtn = navPublisher

	sidebarContent := container.NewBorder(navItems, container.NewPadded(navSettings), nil, nil, nil)
	sidebar := container.NewStack(newGTRoundedSurface(gtSidebar, 20, sidebarContent))

	// Subtle vertical separator between sidebar and content
	sidebarSep := canvas.NewRectangle(withAlpha(gtBg4, 100))
	sidebarSep.SetMinSize(fyne.NewSize(1, 0))

	// ── Main shell ────────────────────────────────────────────────────────────

	// Create an inner wrapper for the right content so it's fully padded and rounded properly
	rightContent := container.NewBorder(nil, nil, sidebarSep, nil, contentStack)
	appInner := container.NewBorder(nil, nil, sidebar, nil, rightContent)

	appBorder := canvas.NewRectangle(color.Transparent)
	appBorder.CornerRadius = 20
	appBorder.StrokeColor = withAlpha(gtBg6, 30)
	appBorder.StrokeWidth = 1

	// Liquid glass border animation
	time.AfterFunc(100*time.Millisecond, func() {
		fyne.Do(func() {
			borderAnim := canvas.NewColorRGBAAnimation(
				withAlpha(gtBg6, 20),
				withAlpha(gtPrimary, 110),
				2800*time.Millisecond,
				func(c color.Color) { appBorder.StrokeColor = c; canvas.Refresh(appBorder) },
			)
			borderAnim.AutoReverse = true
			borderAnim.Curve = fyne.AnimationEaseInOut
			borderAnim.RepeatCount = fyne.AnimationRepeatForever
			borderAnim.Start()
		})
	})

	appFrame := container.NewStack(
		newGTRoundedSurface(withAlpha(gtBg1, 230), 20, nil), // translucent glass bg fill
		appBorder,                     // border glow
		container.NewPadded(appInner), // Use padding to separate shell edges
	)

	if g.AutoBuildProject {
		appFrame.Hide()
	}

	backdrop := canvas.NewRectangle(gtBg0)
	blobA := canvas.NewCircle(withAlpha(gtBg5, 35))
	blobB := canvas.NewCircle(withAlpha(gtPrimary, 20))

	mainShell = container.NewStack(backdrop, blobA, blobB, container.NewPadded(appFrame), overlayContainer)

	// Drift animations for ambient blobs
	time.AfterFunc(400*time.Millisecond, func() {
		fyne.Do(func() {
			canvasSize := currentCanvasSize(win)
			blobASize := fyne.NewSize(canvasSize.Width*0.6, canvasSize.Width*0.6)
			blobA.Resize(blobASize)
			blobA.Move(fyne.NewPos(-blobASize.Width*0.35, -blobASize.Height*0.4))

			blobBSize := fyne.NewSize(canvasSize.Width*0.5, canvasSize.Width*0.5)
			blobB.Resize(blobBSize)
			blobB.Move(fyne.NewPos(canvasSize.Width-blobBSize.Width*0.65, canvasSize.Height-blobBSize.Height*0.65))

			driftA := canvas.NewPositionAnimation(
				blobA.Position(), fyne.NewPos(blobA.Position().X+22, blobA.Position().Y+14),
				3200*time.Millisecond,
				func(p fyne.Position) { blobA.Move(p); canvas.Refresh(blobA) },
			)
			driftA.AutoReverse = true
			driftA.Curve = fyne.AnimationEaseInOut
			driftA.RepeatCount = fyne.AnimationRepeatForever
			driftA.Start()

			driftB := canvas.NewPositionAnimation(
				blobB.Position(), fyne.NewPos(blobB.Position().X-18, blobB.Position().Y-10),
				3800*time.Millisecond,
				func(p fyne.Position) { blobB.Move(p); canvas.Refresh(blobB) },
			)
			driftB.AutoReverse = true
			driftB.Curve = fyne.AnimationEaseInOut
			driftB.RepeatCount = fyne.AnimationRepeatForever
			driftB.Start()
		})
	})

	// ── Pre-fill project (verify-compatibility mode) ──────────────────────────
	if g.ProjectPath == "" {
		g.ProjectPath = findUProjectUpwards()
	}
	appendStatus("ProjectPath: %s, RegenerateProject: %v, recompileOnly: %v", g.ProjectPath, g.RegenerateProject, g.recompileOnly)
	if g.ProjectPath != "" {
		loadProject(g.ProjectPath)
		if g.RegenerateProject {
			appendStatus("RegenerateProject flag detected, starting recompilation...")
		} else if !g.AutoBuildProject && !g.VerifyCompat && g.recompileOnly && !isPackless {
			time.AfterFunc(800*time.Millisecond, func() {
				fyne.Do(func() { actionBtn.Trigger() })
			})
		} else if g.installZipPath != "" {
			time.AfterFunc(800*time.Millisecond, func() {
				fyne.Do(func() { actionBtn.Trigger() })
			})
		}
	}

	appSettings, _ := settings.LoadSettings()

	// Determine DevMode based on context
	isDev := appSettings.DevMode
	if appSettings.InstalledNatively {
		isDev = appSettings.BinDevMode
	}

	// Apply ForceHTTP setting
	if appSettings.ForceHTTP {
		api.BaseURL = "http://api.gorgeous.simsalabim.studio/api/v1"
		api.IsDevMode = true
		api.IsOffline = false
	} else if isDev {
		api.IsDevMode = true
	}

	// The source bin should never get updates when in dev mode
	skipUpdateCheck := isDev && !appSettings.InstalledNatively
	if !skipUpdateCheck {
		go func() {
			time.Sleep(2 * time.Second) // Wait for boot anim
			newVer, ok := updater.CheckForUpdates(buildinfo.Version, appSettings.InstalledNatively)
			if ok {
				fyne.Do(func() {
					g.showUpdateToast(newVer, func() {
						switchToPanel(panelSettings)
					})
				})
			}
		}()
	}

	if api.IsOffline {
		go func() {
			time.Sleep(4 * time.Second) // Stagger slightly after boot anim
			fyne.Do(func() {
				g.showOfflineToast()
			})
		}()
	} else if api.IsDevMode {
		go func() {
			time.Sleep(4 * time.Second) // Stagger slightly after boot anim
			fyne.Do(func() {
				g.showDevModeToast()
			})
		}()
	}

	// YubiKey Listener for Publisher UI
	go func() {
		isPublisherVisible := false
		var lastCardName string
		lastCardConfigured := false

		for {
			cards, err := piv.Cards()
			yubiConnected := err == nil && len(cards) > 0

			configured := false
			if yubiConnected {
				cardName := cards[0]
				if cardName == lastCardName {
					// Use cached result to avoid hitting the hardware repeatedly
					// (which causes the green LED to stay solid/blink constantly)
					configured = lastCardConfigured
				} else {
					// quick test: can we open and read the signature cert?
					yk, err := piv.Open(cardName)
					if err == nil {
						_, err = yk.Certificate(piv.SlotSignature)
						if err == nil {
							configured = true
						}
						yk.Close()
					}
					lastCardName = cardName
					lastCardConfigured = configured
				}
			} else {
				lastCardName = ""
				lastCardConfigured = false
			}

			// Must be DevMode AND have a configured YubiKey
			currentSettings, _ := settings.LoadSettings()
			shouldShow := configured && currentSettings.DevMode

			if shouldShow && !isPublisherVisible {
				isPublisherVisible = true
				fyne.Do(func() {
					found := false
					for _, obj := range navItems.Objects {
						if obj == navPublisher {
							found = true
							break
						}
					}
					if !found {
						navItems.Add(navPublisher)
						navItems.Refresh()
						g.showPublisherUnlockedToast()
					}
				})
			} else if !shouldShow && isPublisherVisible {
				isPublisherVisible = false
				fyne.Do(func() {
					navItems.Remove(navPublisher)
					navItems.Refresh()
					if currentVisiblePanel == panelObjs[panelPublisher] {
						switchToPanel(panelInstaller)
					}
				})
			}

			time.Sleep(3 * time.Second)
		}
	}()

	playBootSequence(win, iconRes, isPackless, mainShell)
	startRoundedWindowStyling(win.Title(), 32)

	// Check for a previous update error log and show it after the boot sequence
	if errPath, err := settings.ErrorFilePath(); err == nil {
		time.AfterFunc(1800*time.Millisecond, func() {
			if data, err := os.ReadFile(errPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
				msg := strings.TrimSpace(string(data))
				os.Remove(errPath)
				fyne.Do(func() {
					g.showAnimatedDialog("Update Error", "The previous update encountered an error:\n\n"+msg, true)
				})
			}
		})
	}

	if (g.AutoBuildProject || g.VerifyCompat) && g.ProjectPath != "" {
		taskType := ProjectTaskAutoLaunch
		if g.VerifyCompat {
			taskType = ProjectTaskVerifyCompat
		}

		time.AfterFunc(1500*time.Millisecond, func() {
			fyne.Do(func() {
				g.showProjectTaskModal(g.ProjectPath, "", taskType)
			})
		})
	}

	if g.installZipPath != "" && g.ProjectPath != "" {
		time.AfterFunc(1500*time.Millisecond, func() {
			fyne.Do(func() {
				g.showProjectTaskModal(g.ProjectPath, "", ProjectTaskInstallZip)
			})
		})
	}

	// Handle RegenerateProject mode after window is shown
	if g.RegenerateProject && g.ProjectPath != "" {
		go func() {
			time.Sleep(500 * time.Millisecond)
			_, enginePath, detectErr := unreal.GetEngineVersionFromProject(g.ProjectPath)
			if detectErr != nil {
				fyne.Do(func() {
					appendStatus("Failed to detect engine path: %v", detectErr)
				})
				return
			}
			pluginPath, err := unreal.LocatePluginByName(filepath.Dir(g.ProjectPath), enginePath, g.config.PackName)
			if err != nil {
				pluginPath = filepath.Join(filepath.Dir(g.ProjectPath), "Plugins", "GorgeousThings", g.config.PackName)
			}
			fyne.Do(func() {
				appendStatus("Starting recompilation: PluginPath=%s, EnginePath=%s, ProjectPath=%s", pluginPath, enginePath, g.ProjectPath)
			})
			inst := installer.NewInstaller(pluginPath, "code", nil, "", g.ProjectPath, enginePath)
			inst.RecompileOnly = true
			inst.StatusLogger = appendStatus
			inst.SetRunContext(context.Background())
			if err := inst.Install(); err != nil {
				fyne.Do(func() {
					appendStatus("Regeneration failed: %v", err)
				})
				return
			}
			fyne.Do(func() {
				appendStatus("Regeneration complete. Exiting...")
			})
			os.Exit(0)
		}()
	}

	win.ShowAndRun()

	unreal.KillUBT()

	if (g.recompileOnly || isPackless) && !g.installSucceeded {
		os.Exit(1)
	}
}

// currentSelectedVersion returns the active version string.
func currentSelectedVersion(auto string, manual bool, sel *widget.Select) string {
	if manual {
		return strings.TrimSpace(sel.Selected)
	}
	return strings.TrimSpace(auto)
}

// buildStatusMsg creates a concise status line from an install plan.
func buildStatusMsg(plan *installer.InstallPlan) string {
	switch plan.Action {
	case installer.InstallActionUpdate:
		return fmt.Sprintf("Update available — %d files changed", len(plan.ChangedFiles))
	case installer.InstallActionReinstall:
		return fmt.Sprintf("v%s installed — %d files matched · Reinstall available", plan.PackVersion, plan.TotalSourceFiles)
	default:
		if plan.TotalSourceFiles > 0 {
			return fmt.Sprintf("Ready to install — %d files", plan.TotalSourceFiles)
		}
		return "Ready to install"
	}
}

// ─── Installer Panel ─────────────────────────────────────────────────────────

func (g *GUIApp) buildInstallerPanel(
	win fyne.Window,
	iconRes fyne.Resource,
	isPackless bool,
	projectEntry *widget.Entry,
	browseBtn *widget.Button,
	versionSelectRow *fyne.Container,
	engineVal *canvas.Text,
	statusCard fyne.CanvasObject,
	actionBtn *accentButton,
	exitBtn *widget.Button,
	logScroll *container.Scroll,
	lastLineLbl *canvas.Text,
	autoScrollCheck *widget.Check,
) fyne.CanvasObject {

	// ── Shared log drawer ─────────────────────────────────────────────────────
	logExpanded := true
	var toggleBtn *widget.Button

	logLayout := &logAnimLayout{height: 140}

	// Create visual bounds similar to widget.Entry to satisfy "without changing actual log box" visually
	logBoxBg := newGTRoundedSurface(withAlpha(gtBg0, 100), 4, nil)
	paddedLogScroll := container.NewPadded(logScroll)
	logContainer := container.NewStack(logBoxBg, paddedLogScroll)

	animLogContainer := container.New(logLayout, logContainer)

	toggleBtn = widget.NewButtonWithIcon("", theme.MenuDropUpIcon(), func() {
		if logExpanded {
			// animate collapse
			logExpanded = false
			lastLineLbl.Show()
			toggleBtn.SetIcon(theme.MenuDropDownIcon())

			anim := fyne.NewAnimation(300*time.Millisecond, func(v float32) {
				logLayout.height = 140 * (1 - v)
				animLogContainer.Refresh()
			})
			anim.Curve = fyne.AnimationEaseInOut
			anim.Start()
		} else {
			// animate expand
			logExpanded = true
			lastLineLbl.Hide()
			toggleBtn.SetIcon(theme.MenuDropUpIcon())

			anim := fyne.NewAnimation(300*time.Millisecond, func(v float32) {
				logLayout.height = 140 * v
				animLogContainer.Refresh()
			})
			anim.Curve = fyne.AnimationEaseInOut
			anim.Start()
		}
	})
	toggleBtn.Importance = widget.LowImportance

	logTitle := canvas.NewText("LIVE LOG", gtTextDim)
	logTitle.TextSize = 10
	logTitle.TextStyle = fyne.TextStyle{Bold: true}
	lastLineLbl.Hide() // hidden when expanded

	drawerHeader := container.NewBorder(nil, nil,
		container.NewHBox(logTitle, autoScrollCheck),
		toggleBtn,
		lastLineLbl,
	)
	drawerInner := container.NewBorder(drawerHeader, nil, nil, nil, animLogContainer)
	logDrawer := newGTRoundedSurface(withAlpha(gtBg2, 220), 14, container.NewPadded(drawerInner))

	if isPackless {
		return g.buildPacklessPanel(win, iconRes, engineVal, actionBtn, exitBtn, logDrawer)
	}
	return g.buildPackPanel(win, iconRes, projectEntry, browseBtn, versionSelectRow, engineVal, statusCard, actionBtn, exitBtn, logDrawer)
}

// buildPacklessPanel builds the simplified recompile-only installer panel.
func (g *GUIApp) buildPacklessPanel(
	_ fyne.Window,
	iconRes fyne.Resource,
	engineVal *canvas.Text,
	actionBtn *accentButton,
	exitBtn *widget.Button,
	logDrawer fyne.CanvasObject,
) fyne.CanvasObject {

	// Identity card
	var logoWidget fyne.CanvasObject
	if iconRes != nil {
		img := canvas.NewImageFromResource(iconRes)
		img.FillMode = canvas.ImageFillContain
		logoWidget = container.NewGridWrap(fyne.NewSize(52, 52), img)
	} else {
		c := canvas.NewCircle(gtPrimary)
		logoWidget = container.NewGridWrap(fyne.NewSize(52, 52), c)
	}

	pluginNameTxt := canvas.NewText(g.config.PluginName, gtTextPrimary)
	pluginNameTxt.TextSize = 22
	pluginNameTxt.TextStyle = fyne.TextStyle{Bold: true}

	subtitleTxt := canvas.NewText("Plugin Recompiler — Packless Mode", gtTextDim)
	subtitleTxt.TextSize = 12

	identityContent := container.NewBorder(nil, nil, container.NewPadded(logoWidget), nil,
		container.NewVBox(pluginNameTxt, subtitleTxt),
	)
	identityCard := newGTRoundedSurface(gtBg2, 16, container.NewPadded(identityContent))

	// Engine badge
	engineIconTxt := canvas.NewText("⚙", gtTextSecondary)
	engineIconTxt.TextSize = 14
	engineLabelTxt := canvas.NewText("Engine", gtTextDim)
	engineLabelTxt.TextSize = 11
	engineBadge := newGTRoundedSurface(withAlpha(gtBg3, 180), 10,
		container.NewPadded(container.NewHBox(engineIconTxt, engineLabelTxt, engineVal)),
	)

	packlessLabel := canvas.NewText("No pack required — recompile only", gtTextDim)
	packlessLabel.TextSize = 11
	packlessLabel.Alignment = fyne.TextAlignCenter
	modeBadge := newGTRoundedSurface(withAlpha(accentRecompile, 22), 8,
		container.NewPadded(packlessLabel),
	)

	centerContent := container.NewVBox(
		identityCard,
		canvas.NewRectangle(color.Transparent), // 8px space
		container.NewCenter(container.NewHBox(engineBadge, modeBadge)),
		canvas.NewRectangle(color.Transparent), // 8px space
		canvas.NewRectangle(color.Transparent), // 8px space
		container.NewCenter(container.NewGridWrap(fyne.NewSize(280, 52), actionBtn)),
		canvas.NewRectangle(color.Transparent), // 8px space
		container.NewCenter(exitBtn),
	)

	body := container.NewBorder(nil, logDrawer, nil, nil, container.NewCenter(centerContent))

	return container.NewPadded(body)
}

// buildPackPanel builds the full pack installer panel.
func (g *GUIApp) buildPackPanel(
	_ fyne.Window,
	_ fyne.Resource,
	projectEntry *widget.Entry,
	browseBtn *widget.Button,
	versionSelectRow *fyne.Container,
	engineVal *canvas.Text,
	statusCard fyne.CanvasObject,
	actionBtn *accentButton,
	exitBtn *widget.Button,
	logDrawer fyne.CanvasObject,
) fyne.CanvasObject {

	// Project card
	engineRow := container.NewHBox(
		newGTLabel("Engine:"),
		engineVal,
		layout.NewSpacer(),
		newGTLabel("Plugin:"),
		newGTValueLabel(g.config.PluginName),
	)

	projectSection := newGTCard("Project", "Select your .uproject file",
		container.NewVBox(
			container.NewBorder(nil, nil, nil, browseBtn, projectEntry),
			engineRow,
			versionSelectRow,
		),
	)

	// Status card section
	statusSection := newGTCard("Install Status", "Current installation state",
		container.NewPadded(statusCard),
	)

	// Action row
	actionRow := container.NewHBox(
		layout.NewSpacer(),
		container.NewGridWrap(fyne.NewSize(220, 52), actionBtn),
		exitBtn,
	)

	topContent := container.NewVBox(projectSection, statusSection)
	bottomContent := container.NewVBox(actionRow, logDrawer)

	body := container.NewBorder(topContent, bottomContent, nil, nil, nil)
	return container.NewPadded(body)
}

// ─── SHA Validator Panel ──────────────────────────────────────────────────────

func (g *GUIApp) buildSHAValidatorPanel(
	win fyne.Window,
	iconRes fyne.Resource,
	mainShell *fyne.CanvasObject,
	backToShell func(),
	appendStatus func(string, ...any),
) fyne.CanvasObject {

	versions := g.availableVersions()

	engineSelect := widget.NewSelect(versions, nil)
	engineSelect.PlaceHolder = "Select engine version"
	if len(versions) > 0 {
		engineSelect.SetSelected(versions[0])
	}

	shaFileEntry := widget.NewEntry()
	shaFileEntry.SetPlaceHolder("Optional: choose a .sha/.sha256 file")
	shaFileEntry.Disable()

	selectedPackValue := newGTValueLabel("Not selected")
	manifestValue := newGTValueLabel("Auto from config")
	var selectedSHAPath string

	var shaPanel fyne.CanvasObject

	engineSelect.OnChanged = func(version string) {
		trimmed := strings.TrimSpace(version)
		if trimmed == "" {
			setCanvasText(selectedPackValue, "Not selected")
			setCanvasText(manifestValue, "Auto from config")
			return
		}
		setCanvasText(selectedPackValue, trimmed)
		if strings.TrimSpace(selectedSHAPath) != "" {
			setCanvasText(manifestValue, filepath.Base(selectedSHAPath))
			return
		}
		packVersion, err := g.selectedPackVersion(trimmed)
		if err != nil {
			setCanvasText(manifestValue, "Version not found in config")
			return
		}
		resolved, found := installer.ResolvePackSHAManifestPath(packVersion)
		if !found || strings.TrimSpace(resolved) == "" {
			setCanvasText(manifestValue, "No configured manifest")
			return
		}
		setCanvasText(manifestValue, resolved)
	}
	engineSelect.OnChanged(engineSelect.Selected)

	browseSHABtn := widget.NewButtonWithIcon("Browse SHA", theme.FolderOpenIcon(), func() {
		d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				g.showAnimatedDialog("Error", err.Error(), true)
				appendStatus("SHA file selection error: %v", err)
				return
			}
			if reader == nil {
				return
			}
			defer reader.Close()
			selected := normalizeURIPath(reader.URI())
			if selected == "" {
				appendStatus("Invalid SHA file URI returned by file dialog")
				return
			}
			selectedSHAPath = selected
			shaFileEntry.Enable()
			shaFileEntry.SetText(selectedSHAPath)
			shaFileEntry.Disable()
			setCanvasText(manifestValue, filepath.Base(selectedSHAPath))
			appendStatus("SHA file selected: %s", selectedSHAPath)
		}, win)
		d.SetFilter(storage.NewExtensionFileFilter([]string{".sha", ".sha256", ".txt"}))
		d.Show()
	})

	var validateBtn *widget.Button
	validateBtn = widget.NewButtonWithIcon("Validate", theme.ConfirmIcon(), func() {
		selectedVersion := strings.TrimSpace(engineSelect.Selected)
		if selectedVersion == "" {
			g.showAnimatedDialog("Error", "please select an engine version", true)
			return
		}
		selectedPack, err := g.selectedPackVersion(selectedVersion)
		if err != nil {
			g.showAnimatedDialog("Error", err.Error(), true)
			return
		}
		setCanvasText(selectedPackValue, selectedVersion)

		manifestPath := strings.TrimSpace(selectedSHAPath)
		if manifestPath == "" {
			resolvedPath, found := installer.ResolvePackSHAManifestPath(selectedPack)
			if !found || strings.TrimSpace(resolvedPath) == "" {
				g.showAnimatedDialog("Error", fmt.Sprintf("no SHA file selected and no shaFile configured for version %s", selectedVersion), true)
				return
			}
			manifestPath = resolvedPath
			setCanvasText(manifestValue, manifestPath)
		}

		validateBtn.Disable()
		browseSHABtn.Disable()
		appendStatus("Validating SHA for version %s with %s", selectedVersion, manifestPath)

		go func(pack *config.PackVersion, shaPath string) {
			report, validateErr := installer.ValidatePackSHA(pack, shaPath)
			fyne.Do(func() {
				validateBtn.Enable()
				browseSHABtn.Enable()

				if validateErr != nil {
					appendStatus("SHA validation failed: %v", validateErr)
					g.showInstallResult(false, validateErr.Error(), iconRes, "", nil, "Close", func() {}, "sha validation")
					return
				}
				resultMessage := formatSHAValidationMessage(report)
				if report.IsValid() {
					appendStatus("SHA validation successful for %s", report.PackVersion)
					g.showInstallResult(true, resultMessage, iconRes, "", nil, "Close", func() {}, "sha validation")
					return
				}
				appendStatus("SHA validation reported differences for %s", report.PackVersion)
				g.showInstallResult(false, resultMessage, iconRes, "", nil, "Close", func() {}, "sha validation")
			})
		}(selectedPack, manifestPath)
	})

	infoSection := newGTCard("SHA Validation", "Compare pack files against a manifest",
		container.NewVBox(
			container.NewHBox(newGTLabel("Engine version:"), engineSelect),
			container.NewBorder(nil, nil, nil, browseSHABtn, shaFileEntry),
			container.NewHBox(newGTLabel("Selected pack:"), selectedPackValue),
			container.NewHBox(newGTLabel("Manifest:"), manifestValue),
		),
	)

	actionRow := container.NewHBox(layout.NewSpacer(), validateBtn, layout.NewSpacer())
	body := container.NewBorder(infoSection, actionRow, nil, nil,
		container.NewCenter(widget.NewLabel("Select an engine version and optional SHA file, then click Validate.")),
	)
	shaPanel = container.NewPadded(body)
	return shaPanel
}

// ─── Coming Soon Panel ───────────────────────────────────────────────────────

func buildComingSoonPanel(icon fyne.Resource, name, description string) fyne.CanvasObject {
	iconImg := canvas.NewImageFromResource(icon)
	iconImg.FillMode = canvas.ImageFillContain
	iconWrap := container.NewGridWrap(fyne.NewSize(48, 48), iconImg)

	nameTxt := canvas.NewText(name, gtTextSecondary)
	nameTxt.TextSize = 24
	nameTxt.TextStyle = fyne.TextStyle{Bold: true}

	soonBadge := canvas.NewText("COMING SOON", withAlpha(accentUpdate, 200))
	soonBadge.TextSize = 10
	soonBadge.TextStyle = fyne.TextStyle{Bold: true}

	descTxt := widget.NewLabel(description)
	descTxt.Alignment = fyne.TextAlignCenter
	descTxt.Wrapping = fyne.TextWrapWord

	card := newGTRoundedSurface(withAlpha(gtBg2, 200), 20,
		container.NewPadded(
			container.NewVBox(
				container.NewCenter(iconWrap),
				container.NewCenter(nameTxt),
				container.NewCenter(soonBadge),
				canvas.NewRectangle(color.Transparent),
				container.NewCenter(container.NewGridWrap(fyne.NewSize(440, 80), descTxt)),
			),
		),
	)

	return container.NewCenter(container.NewGridWrap(fyne.NewSize(500, 280), card))
}

// ─── Boot Sequence ───────────────────────────────────────────────────────────

func playBootSequence(win fyne.Window, iconRes fyne.Resource, isPackless bool, finalContent fyne.CanvasObject) {
	bootBg := canvas.NewRectangle(gtBg0)

	glow := canvas.NewCircle(withAlpha(gtPrimary, 60))
	ring := canvas.NewCircle(color.Transparent)
	ring.StrokeColor = withAlpha(gtBg6, 160)
	ring.StrokeWidth = 3

	var iconVisual fyne.CanvasObject
	if iconRes != nil {
		img := canvas.NewImageFromResource(iconRes)
		img.FillMode = canvas.ImageFillContain
		iconVisual = img
	} else {
		iconVisual = canvas.NewCircle(gtPrimary)
	}

	logoSurface := newGTRoundedSurface(gtBg4, 22, container.NewPadded(iconVisual))

	titleText := canvas.NewText("Gorgeous Installer", gtTextPrimary)
	titleText.TextSize = 24
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	subtitle := "Loading installer..."
	if isPackless {
		subtitle = "Initializing recompiler..."
	}
	subtitleText := canvas.NewText(subtitle, gtTextSecondary)
	subtitleText.TextSize = 12

	bootStage := container.NewWithoutLayout(glow, ring, logoSurface, titleText, subtitleText)
	bootContent := container.NewStack(bootBg, bootStage)
	win.SetContent(bootContent)

	layoutBoot := func(iconSize, ringSize float32) {
		canvasSize := currentCanvasSize(win)

		ringObjectSize := fyne.NewSize(ringSize, ringSize)
		ring.Resize(ringObjectSize)
		ring.Move(centeredPos(canvasSize, ringObjectSize, -28))

		glowSize := fyne.NewSize(ringSize+30, ringSize+30)
		glow.Resize(glowSize)
		glow.Move(centeredPos(canvasSize, glowSize, -28))

		iconObjectSize := fyne.NewSize(iconSize, iconSize)
		logoSurface.Resize(iconObjectSize)
		logoSurface.Move(centeredPos(canvasSize, iconObjectSize, -28))

		titleSize := titleText.MinSize()
		titleText.Move(centeredPos(canvasSize, titleSize, 50))

		subtitleSize := subtitleText.MinSize()
		subtitleText.Move(centeredPos(canvasSize, subtitleSize, 78))

		canvas.Refresh(ring)
		canvas.Refresh(glow)
		canvas.Refresh(logoSurface)
		canvas.Refresh(titleText)
		canvas.Refresh(subtitleText)
	}

	layoutBoot(86, 130)

	time.AfterFunc(40*time.Millisecond, func() {
		fyne.Do(func() { layoutBoot(86, 130) })
	})

	ringPulse := canvas.NewSizeAnimation(
		fyne.NewSize(126, 126), fyne.NewSize(170, 170), 640*time.Millisecond,
		func(s fyne.Size) { layoutBoot(86, s.Width) },
	)
	ringPulse.AutoReverse = true
	ringPulse.Curve = fyne.AnimationEaseInOut
	ringPulse.RepeatCount = 2
	ringPulse.Start()

	iconPulse := canvas.NewSizeAnimation(
		fyne.NewSize(82, 82), fyne.NewSize(96, 96), 520*time.Millisecond,
		func(s fyne.Size) { layoutBoot(s.Width, ring.Size().Width) },
	)
	iconPulse.AutoReverse = true
	iconPulse.Curve = fyne.AnimationEaseInOut
	iconPulse.RepeatCount = 2
	iconPulse.Start()

	time.AfterFunc(1300*time.Millisecond, func() {
		fyne.Do(func() { animateWindowOpen(win, finalContent) })
	})
}

// ─── animateWindowOpen ───────────────────────────────────────────────────────

func animateWindowOpen(win fyne.Window, finalContent fyne.CanvasObject) {
	canvasSize := currentCanvasSize(win)
	startSize := fyne.NewSize(canvasSize.Width*0.84, canvasSize.Height*0.84)
	startPos := centeredPos(canvasSize, startSize, 10)

	finalContent.Resize(startSize)
	finalContent.Move(startPos)

	openingLayer := container.NewWithoutLayout(finalContent)
	win.SetContent(openingLayer)

	moveAnim := canvas.NewPositionAnimation(
		startPos, fyne.NewPos(0, 0), 300*time.Millisecond,
		func(p fyne.Position) { finalContent.Move(p); canvas.Refresh(finalContent) },
	)
	moveAnim.Curve = fyne.AnimationEaseInOut

	sizeAnim := canvas.NewSizeAnimation(
		startSize, canvasSize, 300*time.Millisecond,
		func(s fyne.Size) { finalContent.Resize(s); canvas.Refresh(finalContent) },
	)
	sizeAnim.Curve = fyne.AnimationEaseInOut

	moveAnim.Start()
	sizeAnim.Start()

	time.AfterFunc(340*time.Millisecond, func() {
		fyne.Do(func() { win.SetContent(finalContent) })
	})
}

// ─── showInstallResult ───────────────────────────────────────────────────────

func (g *GUIApp) showInstallResult(success bool, message string, iconRes fyne.Resource, actionLabel string, onAction func(), closeLabel string, onClose func(), operationLabel string) {
	_ = iconRes

	operation := strings.ToLower(strings.TrimSpace(operationLabel))
	if operation == "" {
		operation = "installation"
	}
	isSHAValidation := strings.Contains(operation, "sha")

	bgBase := color.NRGBA{R: 4, G: 31, B: 44, A: 255}
	bgPulse := color.NRGBA{R: 6, G: 43, B: 60, A: 255}
	cardColor := color.NRGBA{R: 9, G: 62, B: 86, A: 245}
	cardBorderColor := withAlpha(gtBg6, 190)
	badgeColor := accentSuccess
	badgeGlowColor := withAlpha(accentSuccess, 85)
	messageSurfaceColor := withAlpha(gtBg4, 240)
	statusLabel := "INSTALLATION COMPLETE"
	iconGlyph := "✓"
	title := "Successfully Installed!"
	defaultSuccessMsg := "All selected files were installed and the plugin was updated successfully."
	defaultFailMsg := "An unknown error occurred during installation."

	if isSHAValidation {
		statusLabel = "SHA VALIDATION PASSED"
		title = "SHA Validation Passed"
		defaultSuccessMsg = "All compared files matched the expected checksums."
		defaultFailMsg = "The selected files did not match the expected checksums."
	}

	detailMessage := strings.TrimSpace(message)
	if detailMessage == "" || strings.EqualFold(detailMessage, "successfully installed!") {
		detailMessage = defaultSuccessMsg
	}

	if !success {
		bgBase = color.NRGBA{R: 20, G: 6, B: 12, A: 255}
		bgPulse = color.NRGBA{R: 30, G: 9, B: 17, A: 255}
		cardColor = color.NRGBA{R: 80, G: 20, B: 35, A: 245}
		cardBorderColor = withAlpha(accentError, 210)
		badgeColor = accentError
		badgeGlowColor = withAlpha(accentError, 95)
		messageSurfaceColor = color.NRGBA{R: 100, G: 28, B: 46, A: 235}
		statusLabel = "INSTALLATION FAILED"
		iconGlyph = "▲"
		title = "Installation Failed"
		if isSHAValidation {
			statusLabel = "SHA VALIDATION FAILED"
			title = "SHA Validation Failed"
		}
		if detailMessage == "" {
			detailMessage = defaultFailMsg
		}
	}

	backdrop := canvas.NewRectangle(bgBase)
	blobA := canvas.NewCircle(color.NRGBA{R: 18, G: 110, B: 145, A: 70})
	blobB := canvas.NewCircle(color.NRGBA{R: 14, G: 86, B: 120, A: 90})
	if !success {
		blobA.FillColor = color.NRGBA{R: 165, G: 56, B: 79, A: 95}
		blobB.FillColor = color.NRGBA{R: 95, G: 34, B: 48, A: 105}
	}

	cardFill := canvas.NewRectangle(cardColor)
	cardFill.CornerRadius = 32
	cardBorder := canvas.NewRectangle(color.Transparent)
	cardBorder.CornerRadius = 32
	cardBorder.StrokeColor = cardBorderColor
	cardBorder.StrokeWidth = 2

	badgeGlow := canvas.NewCircle(badgeGlowColor)
	badgeFill := canvas.NewCircle(badgeColor)
	badgeRing := canvas.NewCircle(color.Transparent)
	badgeRing.StrokeColor = color.NRGBA{R: 244, G: 248, B: 250, A: 190}
	badgeRing.StrokeWidth = 2
	badgeIcon := canvas.NewText(iconGlyph, color.NRGBA{R: 244, G: 248, B: 250, A: 255})
	badgeIcon.TextSize = 52
	badgeIcon.TextStyle = fyne.TextStyle{Bold: true}
	badge := container.NewStack(badgeGlow, badgeFill, badgeRing, container.NewCenter(badgeIcon))

	statusText := canvas.NewText(statusLabel, gtTextSecondary)
	statusText.TextSize = 12
	statusText.TextStyle = fyne.TextStyle{Bold: true}

	titleText := canvas.NewText(title, gtTextPrimary)
	titleText.TextSize = 38
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	messageLabel := widget.NewLabel(detailMessage)
	messageLabel.Alignment = fyne.TextAlignCenter
	messageLabel.Wrapping = fyne.TextWrapWord
	messageSurface := newGTRoundedSurface(messageSurfaceColor, 16, container.NewPadded(messageLabel))

	if strings.TrimSpace(closeLabel) == "" {
		closeLabel = "Close Installer"
	}
	closeBtn := widget.NewButton(closeLabel, func() {
		g.modalLayer.Objects = nil
		g.modalLayer.Refresh()
		if onClose != nil {
			onClose()
			return
		}
		g.win.Close()
	})
	closeBtn.Importance = widget.HighImportance

	actionRow := container.NewHBox(layout.NewSpacer(), closeBtn, layout.NewSpacer())
	if onAction != nil && strings.TrimSpace(actionLabel) != "" {
		actionBtn := widget.NewButton(actionLabel, onAction)
		actionRow = container.NewHBox(layout.NewSpacer(), actionBtn, closeBtn, layout.NewSpacer())
	}

	topGap := canvas.NewRectangle(color.Transparent)
	topGap.SetMinSize(fyne.NewSize(0, 62))

	cardBody := container.NewVBox(
		topGap,
		container.NewCenter(statusText),
		container.NewCenter(titleText),
		container.NewPadded(messageSurface),
		layout.NewSpacer(),
		actionRow,
	)
	card := container.NewStack(cardFill, cardBorder, cardBody)
	// Removed legacy stage allocation

	// Layout and Overlay
	layoutState := &modalGroupLayout{width: 700, height: 480, scale: 0.9, offsetY: 25}
	modalGroup := container.New(layoutState, card, badge)

	backdrop.FillColor = color.Transparent
	blobA.Resize(fyne.NewSize(800, 800))
	blobA.Move(fyne.NewPos(-100, 200))
	blobB.Resize(fyne.NewSize(700, 700))
	blobB.Move(fyne.NewPos(400, -200))

	stage := container.NewStack(backdrop, container.NewWithoutLayout(blobA, blobB), container.NewCenter(modalGroup))

	g.modalLayer.Objects = []fyne.CanvasObject{stage}
	g.modalLayer.Refresh()

	// Animations
	bgAnim := canvas.NewColorRGBAAnimation(color.Transparent, bgBase, 400*time.Millisecond, func(c color.Color) {
		backdrop.FillColor = c
		canvas.Refresh(backdrop)
	})
	bgAnim.Start()

	anim := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 400*time.Millisecond, func(p fyne.Position) {
		v := p.X
		layoutState.scale = 0.9 + (0.1 * v)
		layoutState.offsetY = 25 * (1 - v)
		modalGroup.Refresh()
	})
	anim.Curve = fyne.AnimationEaseOut
	anim.Start()

	// Blob drift
	blobAMove := canvas.NewPositionAnimation(
		blobA.Position(), fyne.NewPos(-60, 240), 3000*time.Millisecond,
		func(p fyne.Position) { blobA.Move(p); canvas.Refresh(blobA) },
	)
	blobAMove.AutoReverse = true
	blobAMove.RepeatCount = fyne.AnimationRepeatForever
	blobAMove.Curve = fyne.AnimationEaseInOut
	blobAMove.Start()

	blobBMove := canvas.NewPositionAnimation(
		blobB.Position(), fyne.NewPos(360, -160), 3200*time.Millisecond,
		func(p fyne.Position) { blobB.Move(p); canvas.Refresh(blobB) },
	)
	blobBMove.AutoReverse = true
	blobBMove.RepeatCount = fyne.AnimationRepeatForever
	blobBMove.Curve = fyne.AnimationEaseInOut
	blobBMove.Start()

	time.AfterFunc(520*time.Millisecond, func() {
		fyne.Do(func() {
			pulse := canvas.NewColorRGBAAnimation(bgBase, bgPulse, 1800*time.Millisecond, func(c color.Color) {
				backdrop.FillColor = c
				canvas.Refresh(backdrop)
			})
			pulse.AutoReverse = true
			pulse.RepeatCount = fyne.AnimationRepeatForever
			pulse.Start()
		})
	})
}

// ─── Window helpers ───────────────────────────────────────────────────────────

func currentCanvasSize(win fyne.Window) fyne.Size {
	const (
		defaultWidth  = float32(920)
		defaultHeight = float32(560)
		maxWidth      = float32(1920)
		maxHeight     = float32(1200)
	)
	s := win.Canvas().Size()
	if s.Width < 1 || s.Height < 1 {
		return fyne.NewSize(defaultWidth, defaultHeight)
	}
	if s.Width > maxWidth {
		s.Width = maxWidth
	}
	if s.Height > maxHeight {
		s.Height = maxHeight
	}
	return s
}

func centeredPos(canvasSize, objectSize fyne.Size, yOffset float32) fyne.Position {
	return fyne.NewPos(
		(canvasSize.Width-objectSize.Width)/2,
		(canvasSize.Height-objectSize.Height)/2+yOffset,
	)
}

func animateWindowResize(win fyne.Window, target fyne.Size, duration time.Duration) {
	if win == nil {
		return
	}
	start := clampSize(currentCanvasSize(win), fyne.NewSize(420, 280), fyne.NewSize(1920, 1200))
	target = clampSize(target, fyne.NewSize(420, 280), fyne.NewSize(1280, 800))
	if start == target {
		return
	}
	go func() {
		const steps = 18
		stepDuration := duration / steps
		if stepDuration <= 0 {
			stepDuration = 16 * time.Millisecond
		}
		ticker := time.NewTicker(stepDuration)
		defer ticker.Stop()
		for step := 1; step <= steps; step++ {
			<-ticker.C
			t := float32(step) / float32(steps)
			eased := float32(1 - (1-t)*(1-t))
			w := start.Width + (target.Width-start.Width)*eased
			h := start.Height + (target.Height-start.Height)*eased
			fyne.Do(func() { win.Resize(fyne.NewSize(w, h)) })
		}
	}()
}

func clampSize(value, min, max fyne.Size) fyne.Size {
	if value.Width < min.Width {
		value.Width = min.Width
	}
	if value.Height < min.Height {
		value.Height = min.Height
	}
	if value.Width > max.Width {
		value.Width = max.Width
	}
	if value.Height > max.Height {
		value.Height = max.Height
	}
	return value
}

// ─── Text helpers ─────────────────────────────────────────────────────────────

// newGTLabel returns a small secondary-color label.
func newGTLabel(text string) *canvas.Text {
	l := canvas.NewText(text, gtTextSecondary)
	l.TextSize = 13
	return l
}

// newGTValueLabel returns a bold primary-color value label.
func newGTValueLabel(text string) *canvas.Text {
	v := canvas.NewText(text, gtTextPrimary)
	v.TextSize = 13
	v.TextStyle = fyne.TextStyle{Bold: true}
	return v
}

// Kept for compatibility
func newSurfaceLabelText(text string) *canvas.Text { return newGTLabel(text) }
func newSurfaceValueText(text string) *canvas.Text { return newGTValueLabel(text) }

func setCanvasText(label *canvas.Text, value string) {
	if label == nil {
		return
	}
	label.Text = value
	label.Refresh()
}

func logLineColor(line string) color.Color {
	trimmed := strings.TrimSpace(strings.ToLower(line))
	switch {
	case strings.Contains(trimmed, "error"), strings.Contains(trimmed, "failed"), strings.Contains(trimmed, "fatal"):
		return accentError
	case strings.Contains(trimmed, "warning"), strings.Contains(trimmed, "warn"):
		return accentUpdate
	default:
		return gtTextSecondary
	}
}

func isCompileFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "plugin recompilation failed") ||
		strings.Contains(msg, "unrealbuildtool compile failed") ||
		strings.Contains(msg, "buildplugin")
}

// ─── Surface builders ─────────────────────────────────────────────────────────

// newGTRoundedSurface creates a rounded card surface using the GT palette.
func newGTRoundedSurface(fill color.Color, radius float32, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(fill)
	bg.CornerRadius = radius
	if content == nil {
		return container.NewStack(bg)
	}
	return container.NewStack(bg, content)
}

// newRoundedSurface is kept for compatibility with showInstallResult.
func newRoundedSurface(fill color.Color, radius float32, content fyne.CanvasObject) fyne.CanvasObject {
	return newGTRoundedSurface(fill, radius, content)
}

// newGTCard builds a titled, rounded card section using GT brand tokens.
func newGTCard(title, subtitle string, content fyne.CanvasObject) fyne.CanvasObject {
	titleTxt := canvas.NewText(title, gtTextPrimary)
	titleTxt.TextSize = 16
	titleTxt.TextStyle = fyne.TextStyle{Bold: true}

	subTxt := canvas.NewText(subtitle, gtTextDim)
	subTxt.TextSize = 11

	sep := canvas.NewRectangle(withAlpha(gtBg5, 80))
	sep.SetMinSize(fyne.NewSize(0, 1))

	header := container.NewVBox(titleTxt, subTxt, sep)
	inner := container.NewBorder(header, nil, nil, nil, content)

	return newGTRoundedSurface(withAlpha(gtBg2, 230), 16, container.NewPadded(inner))
}

// newSection is kept for compatibility.
func newSection(title, subtitle string, content fyne.CanvasObject) fyne.CanvasObject {
	return newGTCard(title, subtitle, content)
}

// ─── Drag surface ─────────────────────────────────────────────────────────────

type dragHandle struct {
	widget.BaseWidget
	onDrag     func(float32, float32)
	onDragInit func() bool
	nativeDrag bool
}

func newDragHandle(onDrag func(float32, float32), onDragInit func() bool) *dragHandle {
	h := &dragHandle{onDrag: onDrag, onDragInit: onDragInit}
	h.ExtendBaseWidget(h)
	return h
}

func (h *dragHandle) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonPrimary && h.onDragInit != nil {
		h.nativeDrag = h.onDragInit()
	}
}
func (h *dragHandle) MouseUp(_ *desktop.MouseEvent) { h.nativeDrag = false }

func (h *dragHandle) Dragged(ev *fyne.DragEvent) {
	if h.nativeDrag || h.onDrag == nil {
		return
	}
	h.onDrag(ev.Dragged.DX, ev.Dragged.DY)
}

func (h *dragHandle) DragEnd() { h.nativeDrag = false }
func (h *dragHandle) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func newDragSurface(content fyne.CanvasObject, onDrag func(float32, float32), onDragInit func() bool) fyne.CanvasObject {
	return container.NewStack(newDragHandle(onDrag, onDragInit), container.NewPadded(content))
}

// ─── Version helpers ──────────────────────────────────────────────────────────

func chooseClosestVersion(engineVersion string, versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	normalized, err := unreal.NormalizeVersion(engineVersion)
	if err != nil {
		normalized = engineVersion
	}
	for _, v := range versions {
		if v == normalized {
			return v
		}
	}

	engineMajor, engineMinor := parseVersion(normalized)
	var best string
	var bestMajor, bestMinor int
	for _, v := range versions {
		major, minor := parseVersion(v)
		if major > engineMajor || (major == engineMajor && minor > engineMinor) {
			continue
		}
		if best == "" || major > bestMajor || (major == bestMajor && minor > bestMinor) {
			best = v
			bestMajor = major
			bestMinor = minor
		}
	}
	return best
}

func supportedVersionsForVersion(ver string, availVersions []config.PackVersion) []string {
	if !strings.EqualFold(ver, "Universal") {
		return nil
	}
	for _, pv := range availVersions {
		if strings.EqualFold(pv.Version, "Universal") {
			return pv.SupportedVersions
		}
	}
	return nil
}

func isVersionInSupportedList(engineVer string, supported []string) bool {
	if len(supported) == 0 {
		return true
	}
	normEngine, err := unreal.NormalizeVersion(engineVer)
	if err != nil {
		normEngine = engineVer
	}
	for _, sv := range supported {
		if sv == normEngine {
			return true
		}
	}
	return false
}

func compareVersionStrings(a, b string) int {
	aMajor, aMinor := parseVersion(a)
	bMajor, bMinor := parseVersion(b)
	if aMajor != bMajor {
		if aMajor > bMajor {
			return 1
		}
		return -1
	}
	if aMinor != bMinor {
		if aMinor > bMinor {
			return 1
		}
		return -1
	}
	return strings.Compare(a, b)
}

func parseVersion(v string) (int, int) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) == 0 {
		return 0, 0
	}
	major, _ := strconv.Atoi(parts[0])
	minor := 0
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return major, minor
}

func normalizeURIPath(uri fyne.URI) string {
	if uri == nil {
		return ""
	}
	p := uri.Path()
	if runtime.GOOS == "windows" && strings.HasPrefix(p, "/") && len(p) >= 3 && p[2] == ':' {
		p = p[1:]
	}
	return filepath.Clean(p)
}

func wrapLogLine(line string, maxChars int) []string {
	if maxChars < 32 {
		maxChars = 32
	}
	runes := []rune(strings.TrimRight(line, "\r\n"))
	if len(runes) == 0 {
		return []string{""}
	}
	wrapped := make([]string, 0, len(runes)/maxChars+1)
	start := 0
	for start < len(runes) {
		end := start + maxChars
		if end >= len(runes) {
			wrapped = append(wrapped, string(runes[start:]))
			break
		}
		split := end
		for i := end; i > start+maxChars/2; i-- {
			if runes[i-1] == ' ' || runes[i-1] == '\t' {
				split = i
				break
			}
		}
		segment := strings.TrimRight(string(runes[start:split]), " \t")
		if segment == "" {
			segment = string(runes[start:split])
		}
		wrapped = append(wrapped, segment)
		start = split
		for start < len(runes) && (runes[start] == ' ' || runes[start] == '\t') {
			start++
		}
	}
	return wrapped
}

// ─── Icon resource ────────────────────────────────────────────────────────────

func loadIconResource() fyne.Resource {
	if data, err := bundle.ReadFile("icon.png"); err == nil {
		return fyne.NewStaticResource("icon.png", data)
	}
	candidates := []string{"icon.png"}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append([]string{filepath.Join(exeDir, "icon.png")}, candidates...)
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			return fyne.NewStaticResource("icon.png", data)
		}
	}
	return nil
}

// ─── Business logic (performInstall, inspectInstallPlan, etc.) ───────────────

func (g *GUIApp) availableVersions() []string {
	versions := make([]string, 0, len(g.config.AvailableVersions))
	for _, pv := range g.config.AvailableVersions {
		versions = append(versions, pv.Version)
	}
	sort.SliceStable(versions, func(i, j int) bool {
		return compareVersionStrings(versions[i], versions[j]) > 0
	})
	return versions
}

func (g *GUIApp) selectedPackVersion(selectedVersion string) (*config.PackVersion, error) {
	for i := range g.config.AvailableVersions {
		if g.config.AvailableVersions[i].Version == selectedVersion {
			return &g.config.AvailableVersions[i], nil
		}
	}
	return nil, fmt.Errorf("selected pack version %q not found in config", selectedVersion)
}

func (g *GUIApp) inspectInstallPlan(projectPath, selectedVersion string) (*installer.InstallPlan, error) {
	if strings.TrimSpace(projectPath) == "" {
		return nil, fmt.Errorf("project path is empty")
	}
	selectedPack, err := g.selectedPackVersion(selectedVersion)
	if err != nil {
		return nil, err
	}
	_, enginePath, detectErr := unreal.GetEngineVersionFromProject(projectPath)
	if detectErr != nil {
		enginePath = ""
	}
	pluginNameForSearch := g.config.PackName
	if pluginNameForSearch == "" {
		pluginNameForSearch = g.config.PluginName
	}
	pluginPath, err := unreal.LocatePluginByName(filepath.Dir(projectPath), enginePath, pluginNameForSearch)
	if err != nil {
		if g.config.PackType == "hybrid" {
			pluginPath = filepath.Join(filepath.Dir(projectPath), "Plugins", "GorgeousThings", pluginNameForSearch)
		} else {
			if detectErr != nil {
				return nil, fmt.Errorf("failed to locate plugin %q in project plugins after engine detection failure: %w", pluginNameForSearch, err)
			}
			return nil, fmt.Errorf("failed to locate plugin %q: %w", pluginNameForSearch, err)
		}
	}
	inst := installer.NewInstaller(pluginPath, g.config.PackType, selectedPack, g.config.InstallPath, projectPath, enginePath)
	return inst.BuildInstallPlan()
}

func (g *GUIApp) performInstall(ctx context.Context, projectPath, selectedVersion string, action installer.InstallAction, appendStatus func(string, ...any)) error {
	if projectPath == "" {
		return fmt.Errorf("project path is empty")
	}
	appendStatus("Resolving engine and plugin paths")

	_, enginePath, detectErr := unreal.GetEngineVersionFromProject(projectPath)
	if detectErr != nil {
		appendStatus("Engine detection failed for plugin lookup fallback: %v", detectErr)
	}
	if enginePath != "" {
		appendStatus("Engine path: %s", enginePath)
	}

	pluginNameForSearch := g.config.PackName
	if pluginNameForSearch == "" {
		pluginNameForSearch = g.config.PluginName
	}
	pluginPath, err := unreal.LocatePluginByName(filepath.Dir(projectPath), enginePath, pluginNameForSearch)
	if err != nil {
		if g.config.PackType == "hybrid" {
			// Fallback: This might be a brand new plugin installation.
			// Default to placing it in Plugins/GorgeousThings/<PackName>
			pluginPath = filepath.Join(filepath.Dir(projectPath), "Plugins", "GorgeousThings", pluginNameForSearch)
			appendStatus("Plugin %q not found locally. Preparing to install into: %s", pluginNameForSearch, pluginPath)
		} else {
			if detectErr != nil {
				return fmt.Errorf("failed to locate plugin %q in project plugins after engine detection failure: %w", pluginNameForSearch, err)
			}
			return fmt.Errorf("failed to locate plugin %q: %w", pluginNameForSearch, err)
		}
	}
	appendStatus("Plugin path: %s", pluginPath)

	var selectedPack *config.PackVersion
	if len(g.config.AvailableVersions) > 0 {
		selectedPack, err = g.selectedPackVersion(selectedVersion)
		if err != nil {
			return err
		}
		appendStatus("Installing %s %s with action %s", g.config.PackType, selectedPack.Version, action)
	} else {
		appendStatus("Recompiling %s with action %s", g.config.PluginName, action)
	}

	inst := installer.NewInstaller(pluginPath, g.config.PackType, selectedPack, g.config.InstallPath, projectPath, enginePath)
	inst.RecompileOnly = g.recompileOnly || selectedPack == nil
	inst.SetInstallAction(action)
	inst.SetStatusLogger(appendStatus)
	inst.SetRunContext(ctx)
	if err := inst.Install(); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}
	return nil
}

func (g *GUIApp) validateConfiguredPackSHAs(appendStatus func(string, ...any)) {
	if appendStatus == nil {
		return
	}
	manifestCount := 0
	for i := range g.config.AvailableVersions {
		packVersion := g.config.AvailableVersions[i]
		manifestPath, found := installer.ResolvePackSHAManifestPath(&packVersion)
		if !found || strings.TrimSpace(manifestPath) == "" {
			if strings.TrimSpace(packVersion.SHAFile) != "" {
				appendStatus("Configured SHA manifest missing for %s: %s", packVersion.Version, packVersion.SHAFile)
			}
			continue
		}
		manifestCount++
		report, err := installer.ValidatePackSHA(&packVersion, manifestPath)
		if err != nil {
			appendStatus("SHA validation error for %s: %v", packVersion.Version, err)
			continue
		}
		if report.IsValid() {
			appendStatus("SHA OK for %s (%d files)", packVersion.Version, report.TotalEntries)
			continue
		}
		appendStatus("SHA issues for %s: %d missing, %d mismatched", packVersion.Version, len(report.MissingFiles), len(report.Mismatches))
	}
	if manifestCount == 0 && !g.isPackless() {
		appendStatus("No SHA manifest files were found for startup validation")
	}
}

func formatSHAValidationMessage(report *installer.SHAValidationReport) string {
	if report == nil {
		return "No SHA validation report was produced."
	}
	lines := []string{
		fmt.Sprintf("Version: %s", report.PackVersion),
		fmt.Sprintf("Manifest: %s", report.ManifestPath),
		fmt.Sprintf("Entries: %d", report.TotalEntries),
		fmt.Sprintf("Matched: %d", report.MatchedFiles),
	}
	if len(report.MissingFiles) > 0 {
		lines = append(lines, fmt.Sprintf("Missing files: %d", len(report.MissingFiles)))
		limit := len(report.MissingFiles)
		if limit > 8 {
			limit = 8
		}
		for i := 0; i < limit; i++ {
			lines = append(lines, "- MISSING: "+report.MissingFiles[i])
		}
		if len(report.MissingFiles) > limit {
			lines = append(lines, fmt.Sprintf("... and %d more missing files", len(report.MissingFiles)-limit))
		}
	}
	if len(report.Mismatches) > 0 {
		lines = append(lines, fmt.Sprintf("Mismatched files: %d", len(report.Mismatches)))
		limit := len(report.Mismatches)
		if limit > 8 {
			limit = 8
		}
		for i := 0; i < limit; i++ {
			lines = append(lines, "- MISMATCH: "+report.Mismatches[i].FilePath)
		}
		if len(report.Mismatches) > limit {
			lines = append(lines, fmt.Sprintf("... and %d more mismatched files", len(report.Mismatches)-limit))
		}
	}
	if report.IsValid() {
		lines = append(lines, "All SHA checks passed.")
	}
	return strings.Join(lines, "\n")
}

func (g *GUIApp) dismissModal() {
	if len(g.modalLayer.Objects) == 0 {
		return
	}
	stage := g.modalLayer.Objects[0]
	animOut := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 300*time.Millisecond, func(p fyne.Position) {
		v := p.X
		// Just fade it out quickly
		if stack, ok := stage.(*fyne.Container); ok && len(stack.Objects) > 0 {
			if rect, ok := stack.Objects[0].(*canvas.Rectangle); ok {
				rect.FillColor = withAlpha(color.NRGBA{0, 0, 0, 255}, uint8(255*(1-v)))
				canvas.Refresh(rect)
			}
		}
	})
	animOut.Start()
	time.AfterFunc(300*time.Millisecond, func() {
		fyne.Do(func() {
			g.modalLayer.Objects = nil
			g.modalLayer.Refresh()
		})
	})
}

func (g *GUIApp) showUpdateToast(newVer string, onUpdateTap func()) {
	updateBtn := newAccentButton("Update Now", accentUpdate, func() {
		g.toastLayer.Objects = nil
		g.toastLayer.Refresh()
		onUpdateTap()
	})

	iconTxt := canvas.NewText("✨", color.White)
	iconTxt.TextSize = 22
	iconBox := container.NewCenter(iconTxt)

	title := canvas.NewText("Update Available", gtTextPrimary)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 14

	msg := canvas.NewText("Version "+newVer+" is ready.", gtTextSecondary)
	msg.TextSize = 12

	textCol := container.NewVBox(title, msg)

	content := container.NewBorder(nil, nil, container.NewPadded(iconBox), container.NewPadded(updateBtn), container.NewVBox(layout.NewSpacer(), textCol, layout.NewSpacer()))

	toastCard := newGTRoundedSurface(withAlpha(gtBg2, 240), 16, container.NewPadded(content))
	g.toastLayer.Add(toastCard)
	g.toastLayer.Refresh()

	// Animate in by sliding up
	animIn := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 400*time.Millisecond, func(p fyne.Position) {
		v := p.X
		g.toastLayout.offsetY = 150 * (1 - v) // slide up from 150px below
		g.toastLayer.Refresh()
	})
	animIn.Curve = fyne.AnimationEaseOut
	animIn.Start()

	// Animate away
	time.AfterFunc(5*time.Second, func() {
		fyne.Do(func() {
			animOut := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 400*time.Millisecond, func(p fyne.Position) {
				v := p.X
				g.toastLayout.offsetY = 150 * v // slide back down
				g.toastLayer.Refresh()
			})
			animOut.Curve = fyne.AnimationEaseIn
			animOut.Start()

			time.AfterFunc(450*time.Millisecond, func() {
				fyne.Do(func() {
					g.toastLayer.Remove(toastCard)
					g.toastLayer.Refresh()
				})
			})
		})
	})
}

func (g *GUIApp) showDevModeToast() {
	g.toastLayer.Objects = nil
	g.toastLayer.Refresh()

	iconTxt := canvas.NewText("⚠️", color.NRGBA{R: 240, G: 165, B: 52, A: 255})
	iconTxt.TextSize = 22
	iconBox := container.NewCenter(iconTxt)

	title := canvas.NewText("Dev Mode Activated", gtTextPrimary)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 14

	msg := canvas.NewText("Operating in HTTP is dangerous.", gtTextSecondary)
	msg.TextSize = 12

	textCol := container.NewVBox(title, msg)

	content := container.NewBorder(nil, nil, container.NewPadded(iconBox), nil, container.NewVBox(layout.NewSpacer(), textCol, layout.NewSpacer()))

	toastCard := newGTRoundedSurface(withAlpha(gtBg2, 240), 16, container.NewPadded(content))
	g.toastLayer.Add(toastCard)
	g.toastLayer.Refresh()

	// Animate in by sliding up
	animIn := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 400*time.Millisecond, func(p fyne.Position) {
		v := p.X
		g.toastLayout.offsetY = 150 * (1 - v) // slide up from 150px below
		g.toastLayer.Refresh()
	})
	animIn.Curve = fyne.AnimationEaseOut
	animIn.Start()

	// Animate away
	time.AfterFunc(8*time.Second, func() {
		fyne.Do(func() {
			animOut := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 400*time.Millisecond, func(p fyne.Position) {
				v := p.X
				g.toastLayout.offsetY = 150 * v // slide back down
				g.toastLayer.Refresh()
			})
			animOut.Curve = fyne.AnimationEaseIn
			animOut.Start()

			time.AfterFunc(450*time.Millisecond, func() {
				fyne.Do(func() {
					g.toastLayer.Remove(toastCard)
					g.toastLayer.Refresh()
				})
			})
		})
	})
}

func (g *GUIApp) showPublisherUnlockedToast() {
	g.toastLayer.Objects = nil
	g.toastLayer.Refresh()

	iconTxt := canvas.NewText("🔑", accentSuccess)
	iconTxt.TextSize = 22
	iconBox := container.NewCenter(iconTxt)

	title := canvas.NewText("Publisher UI Unlocked", accentSuccess)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 14

	msg := canvas.NewText("YubiKey connected and configured.", gtTextSecondary)
	msg.TextSize = 12

	textCol := container.NewVBox(title, msg)

	content := container.NewBorder(nil, nil, container.NewPadded(iconBox), nil, container.NewVBox(layout.NewSpacer(), textCol, layout.NewSpacer()))

	toastCard := newGTRoundedSurface(withAlpha(gtBg2, 240), 16, container.NewPadded(content))
	g.toastLayer.Add(toastCard)
	g.toastLayer.Refresh()

	// Animate in by sliding up
	animIn := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 400*time.Millisecond, func(p fyne.Position) {
		v := p.X
		g.toastLayout.offsetY = 150 * (1 - v)
		g.toastLayer.Refresh()
	})
	animIn.Curve = fyne.AnimationEaseOut
	animIn.Start()

	// Animate away
	time.AfterFunc(8*time.Second, func() {
		fyne.Do(func() {
			animOut := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 400*time.Millisecond, func(p fyne.Position) {
				v := p.X
				g.toastLayout.offsetY = 150 * v
				g.toastLayer.Refresh()
			})
			animOut.Curve = fyne.AnimationEaseIn
			animOut.Start()

			time.AfterFunc(450*time.Millisecond, func() {
				fyne.Do(func() {
					g.toastLayer.Remove(toastCard)
					g.toastLayer.Refresh()
				})
			})
		})
	})
}

func (g *GUIApp) showOfflineToast() {
	g.toastLayer.Objects = nil
	g.toastLayer.Refresh()

	iconTxt := canvas.NewText("⚠️", color.NRGBA{R: 240, G: 165, B: 52, A: 255})
	iconTxt.TextSize = 22
	iconBox := container.NewCenter(iconTxt)

	title := canvas.NewText("Offline Mode", gtTextPrimary)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 14

	msg := canvas.NewText("We are offline and operating in HTTP is dangerous.", gtTextSecondary)
	msg.TextSize = 12

	textCol := container.NewVBox(title, msg)

	content := container.NewBorder(nil, nil, container.NewPadded(iconBox), nil, container.NewVBox(layout.NewSpacer(), textCol, layout.NewSpacer()))

	toastCard := newGTRoundedSurface(withAlpha(gtBg2, 240), 16, container.NewPadded(content))
	g.toastLayer.Add(toastCard)
	g.toastLayer.Refresh()

	// Animate in by sliding up
	animIn := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 400*time.Millisecond, func(p fyne.Position) {
		v := p.X
		g.toastLayout.offsetY = 150 * (1 - v) // slide up from 150px below
		g.toastLayer.Refresh()
	})
	animIn.Curve = fyne.AnimationEaseOut
	animIn.Start()

	// Animate away
	time.AfterFunc(8*time.Second, func() {
		fyne.Do(func() {
			animOut := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 400*time.Millisecond, func(p fyne.Position) {
				v := p.X
				g.toastLayout.offsetY = 150 * v // slide back down
				g.toastLayer.Refresh()
			})
			animOut.Curve = fyne.AnimationEaseIn
			animOut.Start()

			time.AfterFunc(450*time.Millisecond, func() {
				fyne.Do(func() {
					g.toastLayer.Remove(toastCard)
					g.toastLayer.Refresh()
				})
			})
		})
	})
}

func (g *GUIApp) showLaunchToast(projName string) {
	g.toastLayer.Objects = nil
	g.toastLayer.Refresh()

	iconTxt := canvas.NewText("🚀", color.White)
	iconTxt.TextSize = 22
	iconBox := container.NewCenter(iconTxt)

	title := canvas.NewText("Launching Project", gtTextPrimary)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 14

	msg := canvas.NewText("Opening "+projName+" in Unreal Editor...", gtTextSecondary)
	msg.TextSize = 12

	textCol := container.NewVBox(title, msg)

	content := container.NewBorder(nil, nil, container.NewPadded(iconBox), nil, container.NewVBox(layout.NewSpacer(), textCol, layout.NewSpacer()))

	toastCard := newGTRoundedSurface(withAlpha(gtBg2, 240), 16, container.NewPadded(content))
	g.toastLayer.Add(toastCard)
	g.toastLayer.Refresh()

	// Animate in by sliding up
	animIn := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 400*time.Millisecond, func(p fyne.Position) {
		v := p.X
		g.toastLayout.offsetY = 150 * (1 - v) // slide up from 150px below
		g.toastLayer.Refresh()
	})
	animIn.Curve = fyne.AnimationEaseOut
	animIn.Start()

	// Animate away
	time.AfterFunc(6*time.Second, func() {
		fyne.Do(func() {
			animOut := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 400*time.Millisecond, func(p fyne.Position) {
				v := p.X
				g.toastLayout.offsetY = 150 * v // slide back down
				g.toastLayer.Refresh()
			})
			animOut.Curve = fyne.AnimationEaseIn
			animOut.Start()

			time.AfterFunc(450*time.Millisecond, func() {
				fyne.Do(func() {
					g.toastLayer.Remove(toastCard)
					g.toastLayer.Refresh()
				})
			})
		})
	})
}

// ─── Animated Dialogs ────────────────────────────────────────────────────────

type genericModalLayout struct {
	width, height float32
	scale         float32
	offsetY       float32
}

func (l *genericModalLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(l.width*l.scale, l.height*l.scale+l.offsetY)
}

func (l *genericModalLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) > 0 {
		card := objects[0]
		w := l.width * l.scale
		h := l.height * l.scale
		card.Resize(fyne.NewSize(w, h))
		card.Move(fyne.NewPos(0, l.offsetY))
	}
}

func (g *GUIApp) showAnimatedDialog(title, message string, isError bool) {
	lbl := widget.NewLabel(message)
	lbl.Wrapping = fyne.TextWrapWord
	content := container.NewPadded(lbl)
	g.showAnimatedCustomDialog(title, content, 500, 200, "Close", func() { g.dismissModal() })
}

func (g *GUIApp) showAnimatedCustomDialog(title string, content fyne.CanvasObject, width, height float32, dismissLabel string, onDismiss func()) {
	if dismissLabel == "" {
		dismissLabel = "Close"
	}
	if onDismiss == nil {
		onDismiss = func() { g.dismissModal() }
	}

	closeBtn := widget.NewButton(dismissLabel, onDismiss)
	closeBtn.Importance = widget.HighImportance

	cardBody := container.NewBorder(
		container.NewPadded(container.NewCenter(canvas.NewText(title, gtTextPrimary))),
		container.NewPadded(container.NewHBox(layout.NewSpacer(), closeBtn, layout.NewSpacer())),
		nil, nil,
		content,
	)

	cardFill := newGTRoundedSurface(gtBg1, 16, nil)
	card := container.NewStack(cardFill, cardBody)

	layoutState := &genericModalLayout{width: width, height: height, scale: 0.9, offsetY: 25}
	modalGroup := container.New(layoutState, card)

	backdrop := canvas.NewRectangle(color.Transparent)
	stage := container.NewStack(backdrop, container.NewCenter(modalGroup))

	g.modalLayer.Objects = []fyne.CanvasObject{stage}
	g.modalLayer.Refresh()

	// Animations
	bgAnim := canvas.NewColorRGBAAnimation(color.Transparent, withAlpha(gtBg0, 180), 300*time.Millisecond, func(c color.Color) {
		backdrop.FillColor = c
		canvas.Refresh(backdrop)
	})
	bgAnim.Start()

	anim := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 300*time.Millisecond, func(p fyne.Position) {
		v := p.X
		layoutState.scale = 0.9 + (0.1 * v)
		layoutState.offsetY = 25 * (1 - v)
		modalGroup.Refresh()
	})
	anim.Curve = fyne.AnimationEaseOut
	anim.Start()
}

// ─── Project Task Modal ──────────────────────────────────────────────────────

type ProjectTaskType int

const (
	ProjectTaskAutoLaunch ProjectTaskType = iota
	ProjectTaskBuild
	ProjectTaskGenerate
	ProjectTaskVerifyCompat
	ProjectTaskInstallZip
)

func (g *GUIApp) showProjectTaskModal(projectPath string, pName string, taskType ProjectTaskType) {
	title := "Building Project"
	if taskType == ProjectTaskGenerate {
		title = "Generating Project Files"
	}
	if pName == "" {
		pName = filepath.Base(projectPath)
		pName = strings.TrimSuffix(pName, ".uproject")
	}

	statusLbl := widget.NewLabel("Preparing...")
	statusLbl.Alignment = fyne.TextAlignCenter
	statusLbl.Wrapping = fyne.TextWrapWord

	progressInfinite := widget.NewProgressBarInfinite()
	progress := widget.NewProgressBar()
	progress.Max = 1.0
	progress.SetValue(0)
	progress.Hide()

	progressContainer := container.NewStack(progressInfinite, progress)

	isInfinite := true

	var logLines []string
	logLbl := widget.NewLabel("")
	logLbl.Wrapping = fyne.TextWrapWord
	logScroll := container.NewScroll(logLbl)

	logContainer := container.NewGridWrap(fyne.NewSize(600, 260), newGTRoundedSurface(withAlpha(gtBg0, 150), 8, logScroll))
	logContainer.Hide()

	autoScroll := true
	autoScrollCheck := widget.NewCheck("Auto Scroll", func(on bool) {
		autoScroll = on
		if on {
			logScroll.ScrollToBottom()
		}
	})
	autoScrollCheck.SetChecked(true)
	autoScrollCheck.Hide()

	cancelCtx, cancelFunc := context.WithCancel(context.Background())

	var layoutState *genericModalLayout
	var modalGroup *fyne.Container

	logBtn := widget.NewButton("Show Logs", nil)
	logBtn.OnTapped = func() {
		if logContainer.Hidden {
			logContainer.Show()
			autoScrollCheck.Show()
			logBtn.SetText("Hide Logs")

			// Animate expand
			anim := fyne.NewAnimation(300*time.Millisecond, func(v float32) {
				if layoutState != nil && modalGroup != nil {
					layoutState.height = 160 + (300 * v)
					modalGroup.Refresh()
				}
			})
			anim.Curve = fyne.AnimationEaseInOut
			anim.Start()
		} else {
			logBtn.SetText("Show Logs")
			autoScrollCheck.Hide()

			// Animate collapse
			anim := fyne.NewAnimation(300*time.Millisecond, func(v float32) {
				if layoutState != nil && modalGroup != nil {
					layoutState.height = 460 - (300 * v)
					modalGroup.Refresh()
				}
			})
			anim.Curve = fyne.AnimationEaseInOut
			anim.Start()

			time.AfterFunc(300*time.Millisecond, func() {
				fyne.Do(func() { logContainer.Hide() })
			})
		}
	}
	logBtn.Importance = widget.LowImportance

	cancelBtn := widget.NewButton("Cancel", func() {
		cancelFunc()
		statusLbl.SetText("Canceling...")
		unreal.KillUBT()
		if taskType == ProjectTaskAutoLaunch || taskType == ProjectTaskVerifyCompat {
			if g.win != nil {
				g.win.Close()
			} else {
				os.Exit(2)
			}
		}
	})
	cancelBtn.Importance = widget.DangerImportance

	closeBtn := newAccentButton("Close", accentUpdate, func() {
		cancelFunc()
		unreal.KillUBT()
		if taskType == ProjectTaskAutoLaunch || taskType == ProjectTaskVerifyCompat {
			if g.win != nil {
				g.win.Close()
			} else {
				if g.installSucceeded {
					os.Exit(0)
				} else {
					os.Exit(2)
				}
			}
		} else {
			g.dismissModal()
		}
	})
	closeBtn.Hide()

	buttons := container.NewHBox(layout.NewSpacer(), autoScrollCheck, logBtn, cancelBtn, closeBtn, layout.NewSpacer())

	content := container.NewVBox(
		container.NewPadded(container.NewVBox(
			container.NewHBox(canvas.NewText(pName, gtTextPrimary)),
			canvas.NewText("Status", gtTextDim),
			statusLbl,
		)),
		container.NewPadded(progressContainer),
		buttons,
		logContainer,
	)

	// Build custom modal so we can animate height
	cardBody := container.NewBorder(
		container.NewPadded(container.NewCenter(canvas.NewText(title, gtTextPrimary))),
		nil, nil, nil,
		content,
	)

	cardFill := newGTRoundedSurface(gtBg1, 16, nil)
	card := container.NewStack(cardFill, cardBody)

	layoutState = &genericModalLayout{width: 640, height: 160, scale: 0.9, offsetY: 25}
	modalGroup = container.New(layoutState, card)

	backdrop := canvas.NewRectangle(color.Transparent)
	stage := container.NewStack(backdrop, container.NewCenter(modalGroup))

	g.modalLayer.Objects = []fyne.CanvasObject{stage}
	g.modalLayer.Refresh()

	// Animations in
	bgAnim := canvas.NewColorRGBAAnimation(color.Transparent, withAlpha(gtBg0, 180), 300*time.Millisecond, func(c color.Color) {
		backdrop.FillColor = c
		canvas.Refresh(backdrop)
	})
	bgAnim.Start()

	animIn := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 300*time.Millisecond, func(p fyne.Position) {
		v := p.X
		layoutState.scale = 0.9 + (0.1 * v)
		layoutState.offsetY = 25 * (1 - v)
		modalGroup.Refresh()
	})
	animIn.Curve = fyne.AnimationEaseOut
	animIn.Start()

	animRunning := true

	// Auto-scroll detection
	go func() {
		for animRunning {
			time.Sleep(200 * time.Millisecond)
			fyne.Do(func() {
				if autoScroll && logScroll.Content != nil {
					maxOffset := logScroll.Content.MinSize().Height - logScroll.Size().Height
					if maxOffset > 0 && logScroll.Offset.Y < maxOffset-30 {
						// User scrolled up manually!
						autoScrollCheck.SetChecked(false)
					}
				}
			})
		}
	}()

	// Regex to strip absolute paths down to just the filename.ext
	pathStripRegex := regexp.MustCompile(`(?:[a-zA-Z]:[\\/]|/)[^\s]+[\\/]([^\s]+\.\w+)`)
	// Regex for [XXX/YYY] compilation progress
	progRegex := regexp.MustCompile(`\[(\d+)/(\d+)\]`)

	var uiUpdateQueued bool
	var mu sync.Mutex

	logFn := func(msg string, args ...any) {
		line := fmt.Sprintf(msg, args...)
		cleanLine := strings.TrimRight(line, "\r\n")

		// Strip absolute paths for cleaner logs
		cleanLine = pathStripRegex.ReplaceAllString(cleanLine, "$1")

		mu.Lock()
		logLines = append(logLines, cleanLine)
		if len(logLines) > 500 {
			logLines = logLines[len(logLines)-500:]
		}

		match := progRegex.FindStringSubmatch(cleanLine)

		if !uiUpdateQueued {
			uiUpdateQueued = true
			go func() {
				time.Sleep(50 * time.Millisecond) // Throttle to max 20 FPS
				mu.Lock()
				text := strings.Join(logLines, "\n")
				uiUpdateQueued = false
				mu.Unlock()
				fyne.Do(func() {
					logLbl.SetText(text)
					if autoScroll {
						logScroll.ScrollToBottom()
					}
				})
			}()
		}
		mu.Unlock()

		// Progress UI updates are fine to do immediately as they are much lighter
		// than rebuilding a 500-line string, but we wrap in fyne.Do
		fyne.Do(func() {
			if isInfinite && !strings.Contains(cleanLine, "Adaptive Build") {
				statusLbl.SetText(cleanLine)
			}

			if len(match) == 3 {
				if isInfinite {
					isInfinite = false
					progressInfinite.Hide()
					progress.Show()
				}
				cur, _ := strconv.ParseFloat(match[1], 64)
				tot, _ := strconv.ParseFloat(match[2], 64)
				if tot > 0 {
					progress.SetValue(cur / tot)
				}

				// Strip the [XXX/YYY] prefix from the status label to make it cleaner
				cleanStatus := strings.TrimSpace(progRegex.ReplaceAllString(cleanLine, ""))
				statusLbl.SetText(fmt.Sprintf("Compiling (%.0f / %.0f): %s", cur, tot, cleanStatus))
			}
		})
	}

	go func() {
		var err error
		if taskType == ProjectTaskInstallZip {
			logFn("Installing update from zip...")
			err = installer.ProcessZipUpdate(g.installZipPath, projectPath, g.waitForPID)
		} else if taskType == ProjectTaskAutoLaunch || taskType == ProjectTaskVerifyCompat || taskType == ProjectTaskGenerate {
			logFn("Generating project files for %s...", pName)
			err = unreal.GenerateProjectFiles(cancelCtx, projectPath, logFn)
		}

		if err == nil && (taskType == ProjectTaskAutoLaunch || taskType == ProjectTaskVerifyCompat || taskType == ProjectTaskBuild) {
			logFn("Building project %s...", pName)
			err = unreal.BuildProject(cancelCtx, projectPath, logFn)
		}

		animRunning = false

		fyne.Do(func() {
			cancelBtn.Hide()
			closeBtn.Show()

			if err != nil && errors.Is(err, context.Canceled) {
				if !isInfinite {
					progress.SetValue(0.0)
				}
				statusLbl.SetText("Canceled by user")
			} else if err != nil {
				if !isInfinite {
					progress.SetValue(1.0)
				}
				statusLbl.SetText("Failed: " + err.Error())
				if logContainer.Hidden {
					logBtn.OnTapped()
				}
			} else {
				if isInfinite {
					progressInfinite.Hide()
					progress.Show()
				}
				progress.SetValue(1.0)
				statusLbl.SetText("Success!")
				if taskType == ProjectTaskAutoLaunch {
					logFn("Launching project...")
					unreal.OpenProject(projectPath)
					os.Exit(0)
				} else if taskType == ProjectTaskVerifyCompat || taskType == ProjectTaskInstallZip {
					g.installSucceeded = true
					logFn("Installation complete. Returning to Unreal Engine...")
					time.AfterFunc(2*time.Second, func() {
						fyne.Do(func() {
							if g.win != nil {
								g.win.Close()
							}
						})
					})
				} else {
					closeBtn.SetLabel("Done")
					closeBtn.SetAccent(accentSuccess)
				}
			}
		})
	}()
}

func findUProjectUpwards() string {
	// 1. Try search from working directory
	if dir, err := os.Getwd(); err == nil {
		if path := findUProjectUpwardsFromDir(dir); path != "" {
			return path
		}
	}
	// 2. Try search from executable directory
	if exePath, err := os.Executable(); err == nil {
		dir := filepath.Dir(exePath)
		if path := findUProjectUpwardsFromDir(dir); path != "" {
			return path
		}
	}
	return ""
}

func findUProjectUpwardsFromDir(dir string) string {
	for {
		matches, err := filepath.Glob(filepath.Join(dir, "*.uproject"))
		if err == nil && len(matches) > 0 {
			return matches[0]
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// playUpdateSequence transitions the window to a full-screen boot-like update
// animation with a progress bar and pulsing rings. It returns a status setter
// function that the caller uses to update the subtitle text.
func (g *GUIApp) playUpdateSequence(win fyne.Window, iconRes fyne.Resource) func(string) {
	updateBg := canvas.NewRectangle(gtBg0)

	glow := canvas.NewCircle(withAlpha(gtPrimary, 40))
	ring := canvas.NewCircle(color.Transparent)
	ring.StrokeColor = withAlpha(gtBg6, 140)
	ring.StrokeWidth = 3

	var iconVisual fyne.CanvasObject
	if iconRes != nil {
		img := canvas.NewImageFromResource(iconRes)
		img.FillMode = canvas.ImageFillContain
		iconVisual = img
	} else {
		iconVisual = canvas.NewCircle(gtPrimary)
	}
	logoSurface := newGTRoundedSurface(gtBg4, 22, container.NewPadded(iconVisual))

	titleText := canvas.NewText("Gorgeous Installer", gtTextPrimary)
	titleText.TextSize = 24
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	subtitleText := canvas.NewText("Downloading update...", gtTextSecondary)
	subtitleText.TextSize = 12

	progBar := widget.NewProgressBarInfinite()

	updateStage := container.NewWithoutLayout(glow, ring, logoSurface, titleText, subtitleText)
	updateContent := container.NewStack(updateBg, updateStage, container.NewPadded(
		container.NewBorder(nil, container.NewPadded(progBar), nil, nil, nil),
	))

	layoutUpdate := func(iconSize, ringSize float32) {
		canvasSize := currentCanvasSize(win)

		ringObjectSize := fyne.NewSize(ringSize, ringSize)
		ring.Resize(ringObjectSize)
		ring.Move(centeredPos(canvasSize, ringObjectSize, -28))

		glowSize := fyne.NewSize(ringSize+30, ringSize+30)
		glow.Resize(glowSize)
		glow.Move(centeredPos(canvasSize, glowSize, -28))

		iconObjectSize := fyne.NewSize(iconSize, iconSize)
		logoSurface.Resize(iconObjectSize)
		logoSurface.Move(centeredPos(canvasSize, iconObjectSize, -28))

		titleSize := titleText.MinSize()
		titleText.Move(centeredPos(canvasSize, titleSize, 50))

		subtitleSize := subtitleText.MinSize()
		subtitleText.Move(centeredPos(canvasSize, subtitleSize, 72))

		canvas.Refresh(ring)
		canvas.Refresh(glow)
		canvas.Refresh(logoSurface)
		canvas.Refresh(titleText)
		canvas.Refresh(subtitleText)
	}

	win.SetContent(updateContent)
	layoutUpdate(86, 130)

	time.AfterFunc(40*time.Millisecond, func() {
		fyne.Do(func() { layoutUpdate(86, 130) })
	})

	ringPulse := canvas.NewSizeAnimation(
		fyne.NewSize(126, 126), fyne.NewSize(170, 170), 700*time.Millisecond,
		func(s fyne.Size) { layoutUpdate(86, s.Width) },
	)
	ringPulse.AutoReverse = true
	ringPulse.Curve = fyne.AnimationEaseInOut
	ringPulse.RepeatCount = fyne.AnimationRepeatForever
	ringPulse.Start()

	// Return a setter so the caller can update the status subtitle
	return func(status string) {
		fyne.Do(func() {
			subtitleText.Text = status
			canvas.Refresh(subtitleText)
			layoutUpdate(86, ring.Size().Width)
		})
	}
}
