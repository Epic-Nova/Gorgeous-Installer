package ui

import (
	"fmt"
	"image/color"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"gorgeous-installer/internal/settings"
	"gorgeous-installer/internal/unreal"
)

func (g *GUIApp) buildProjectsPanel(win fyne.Window, appendStatus func(string, ...any)) fyne.CanvasObject {
	appSettings, err := settings.LoadSettings()
	if err != nil {
		appSettings = settings.DefaultSettings()
	}

	var installBanner fyne.CanvasObject
	if !appSettings.InstalledNatively {
		installBtn := widget.NewButtonWithIcon("Install Now", theme.DownloadIcon(), func() {
			if err := installNatively(appSettings); err != nil {
				g.showAnimatedDialog("Error", err.Error(), true)
				appendStatus("Native install failed: %v", err)
				return
			}
			appSettings.InstalledNatively = true
			_ = settings.SaveSettings(appSettings)
			appendStatus("Successfully installed natively to %s and registered desktop entry.", appSettings.LocalBinPath)
			if installBanner != nil {
				installBanner.Hide()
			}
			g.showAnimatedDialog("Success", "Installed natively. You can now open .uproject files directly.", false)
		})
		installBtn.Importance = widget.HighImportance

		installBtn.Icon = theme.DownloadIcon()

		infoBox := container.NewHBox(
			widget.NewIcon(theme.InfoIcon()),
			canvas.NewText("Install natively to enable double-clicking .uproject files.", gtTextPrimary),
			layout.NewSpacer(),
			installBtn,
		)

		installBanner = container.NewPadded(newGTRoundedSurface(withAlpha(gtPrimary, 40), 12, container.NewPadded(infoBox)))
	} else {
		installBanner = container.NewVBox() // empty
	}

	projectsGrid := container.NewGridWrap(fyne.NewSize(192, 192))
	scrollArea := container.NewScroll(projectsGrid)

	var allTiles []*tileWidget

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		projectsGrid.Objects = nil
		projectsGrid.Refresh()
		allTiles = nil
		go func() {
			projects := scanForProjects(appSettings.SearchPaths)
			fyne.Do(func() {
				for _, p := range projects {
					tileObj, tileW := buildProjectTile(g, win, p, appendStatus, func(selectedTile *tileWidget) {
						for _, t := range allTiles {
							t.SetSelected(t == selectedTile)
						}
					})
					allTiles = append(allTiles, tileW)
					projectsGrid.Add(tileObj)
				}
				if len(projects) == 0 {
					projectsGrid.Add(newGTLabel("No projects found. Check your Search Paths in Settings."))
				}
				projectsGrid.Refresh()
			})
		}()
	})
	refreshBtn.Importance = widget.LowImportance

	header := container.NewVBox(
		installBanner,
		container.NewHBox(newGTLabel("Detected Projects:"), layout.NewSpacer(), refreshBtn),
	)

	body := container.NewBorder(header, nil, nil, nil, scrollArea)

	// Initial load
	refreshBtn.Tapped(&fyne.PointEvent{})

	return container.NewPadded(body)
}

type projectInfo struct {
	Path          string
	Name          string
	EngineVersion string
	HasBinaries   bool
	ThumbnailPath string
}

func scanForProjects(paths []string) []projectInfo {
	var results []projectInfo
	for _, searchPath := range paths {
		if stat, err := os.Stat(searchPath); err != nil || !stat.IsDir() {
			continue
		}
		_ = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				// Don't recurse too deep, or into Binaries/Intermediate
				if info.Name() == "Binaries" || info.Name() == "Intermediate" || info.Name() == "Saved" || strings.HasPrefix(info.Name(), ".") {
					return filepath.SkipDir
				}
				rel, _ := filepath.Rel(searchPath, path)
				if strings.Count(rel, string(os.PathSeparator)) > 3 {
					return filepath.SkipDir
				}
				return nil
			}

			if strings.HasSuffix(info.Name(), ".uproject") {
				name := strings.TrimSuffix(info.Name(), ".uproject")
				engineVer, _, _ := unreal.GetEngineVersionFromProject(path)

				dir := filepath.Dir(path)
				hasBinaries := unreal.CheckProjectBinaries(path)

				thumbPath := ""
				for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp"} {
					tp := filepath.Join(dir, name+ext)
					if _, err := os.Stat(tp); err == nil {
						thumbPath = tp
						break
					}
				}

				results = append(results, projectInfo{
					Path:          path,
					Name:          name,
					EngineVersion: engineVer,
					HasBinaries:   hasBinaries,
					ThumbnailPath: thumbPath,
				})
			}
			return nil
		})
	}
	return results
}

// buildProjectTile creates a custom interactive tile for a project.
func buildProjectTile(g *GUIApp, win fyne.Window, p projectInfo, appendStatus func(string, ...any), onSelect func(*tileWidget)) (fyne.CanvasObject, *tileWidget) {
	var thumbContainer fyne.CanvasObject
	if p.ThumbnailPath != "" {
		thumbContainer = createRoundedImage(p.ThumbnailPath, 192, 12)
	} else {
		// Placeholder
		initials := "UE"
		if len(p.Name) > 0 {
			initials = string(p.Name[0])
		}
		c := canvas.NewRectangle(gtBg3)
		c.CornerRadius = 12
		t := canvas.NewText(initials, gtTextSecondary)
		t.TextSize = 48
		t.TextStyle = fyne.TextStyle{Bold: true}

		spacer := canvas.NewRectangle(color.Transparent)
		spacer.SetMinSize(fyne.NewSize(0, 40))
		thumbContainer = container.NewStack(c, container.NewBorder(nil, spacer, nil, nil, container.NewCenter(t)))
	}

	nameTxt := canvas.NewText(p.Name, gtTextPrimary)
	nameTxt.TextSize = 14
	nameTxt.TextStyle = fyne.TextStyle{Bold: true}

	verTxt := canvas.NewText(p.EngineVersion, gtTextSecondary)
	verTxt.TextSize = 10

	statusDot := canvas.NewCircle(accentSuccess)
	if !p.HasBinaries {
		statusDot.FillColor = accentUpdate
	}
	statusDot.Resize(fyne.NewSize(8, 8))
	statusDotContainer := container.NewCenter(container.NewGridWrap(fyne.NewSize(8, 8), statusDot))

	bottomBar := canvas.NewRectangle(withAlpha(gtBg1, 220))
	bottomContent := container.NewHBox(
		container.NewVBox(nameTxt, verTxt),
		layout.NewSpacer(),
		statusDotContainer,
	)
	bottomArea := container.NewStack(bottomBar, container.NewPadded(bottomContent))

	tileContent := container.NewStack(thumbContainer, container.NewBorder(nil, bottomArea, nil, nil))

	var tile *tileWidget
	// We use a custom widget for interaction
	tile = newTileWidget(tileContent, func() {
		if onSelect != nil {
			onSelect(tile)
		}
	}, func() {
		// Double tap
		if p.HasBinaries {
			appendStatus("Opening project: %s", p.Name)
			g.showLaunchToast(p.Name)
			go unreal.OpenProject(p.Path)
		} else {
			g.showProjectTaskModal(p.Path, p.Name, ProjectTaskBuild)
		}
	}, func(ev *fyne.PointEvent) {
		// Right click
		menu := fyne.NewMenu("",
			fyne.NewMenuItem("Open", func() { unreal.OpenProject(p.Path) }),
			fyne.NewMenuItem("Build", func() { g.showProjectTaskModal(p.Path, p.Name, ProjectTaskBuild) }),
			fyne.NewMenuItem("Generate Project Files", func() {
				g.showProjectTaskModal(p.Path, p.Name, ProjectTaskGenerate)
			}),
		)
		widget.ShowPopUpMenuAtPosition(menu, win.Canvas(), ev.AbsolutePosition)
	})

	return tile, tile
}

// tileWidget handles selection, hover, double click and right click
type tileWidget struct {
	widget.BaseWidget
	content    fyne.CanvasObject
	onTap      func()
	onDouble   func()
	onRightTap func(*fyne.PointEvent)
	selected   bool
	bg         *canvas.Rectangle
	overlay    *fyne.Container
}

func newTileWidget(content fyne.CanvasObject, onTap, onDouble func(), onRight func(*fyne.PointEvent)) *tileWidget {
	bg := canvas.NewRectangle(gtBg2)
	bg.CornerRadius = 12
	overlay := container.NewWithoutLayout()
	t := &tileWidget{
		onTap:      onTap,
		onDouble:   onDouble,
		onRightTap: onRight,
		bg:         bg,
		overlay:    overlay,
	}
	t.content = container.NewStack(bg, content, overlay)
	t.ExtendBaseWidget(t)
	return t
}

func (t *tileWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.content)
}

func (t *tileWidget) SetSelected(selected bool) {
	t.selected = selected
	if selected {
		t.bg.FillColor = gtBg5
		t.bg.StrokeColor = color.White
		t.bg.StrokeWidth = 4
	} else {
		t.bg.FillColor = gtBg2
		t.bg.StrokeColor = color.Transparent
		t.bg.StrokeWidth = 0
	}
	t.bg.Refresh()
	t.Refresh()
}

func (t *tileWidget) MouseIn(_ *desktop.MouseEvent) {
	if !t.selected {
		t.bg.FillColor = gtBg3
		t.bg.Refresh()
		t.Refresh()
	}
}

func (t *tileWidget) MouseMoved(_ *desktop.MouseEvent) {}

func (t *tileWidget) MouseOut() {
	if !t.selected {
		t.bg.FillColor = gtBg2
		t.bg.Refresh()
		t.Refresh()
	}
}

func (t *tileWidget) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonPrimary {
		t.bg.FillColor = gtBg5
		t.bg.StrokeColor = color.White
		t.bg.StrokeWidth = 4
		t.bg.Refresh()
		t.Refresh()

		wave := canvas.NewRectangle(color.NRGBA{R: 255, G: 255, B: 255, A: 200})
		wave.CornerRadius = 200
		center := ev.Position
		wave.Resize(fyne.NewSize(10, 10))
		wave.Move(fyne.NewPos(center.X-5, center.Y-5))

		t.overlay.Add(wave)

		anim := canvas.NewPositionAnimation(fyne.NewPos(0, 0), fyne.NewPos(1, 1), 600*time.Millisecond, func(p fyne.Position) {
			v := p.X
			distX := center.X
			if t.bg.Size().Width-center.X < distX {
				distX = t.bg.Size().Width - center.X
			}
			distY := center.Y
			if t.bg.Size().Height-center.Y < distY {
				distY = t.bg.Size().Height - center.Y
			}
			maxSize := distX
			if distY < maxSize {
				maxSize = distY
			}
			maxSize *= 2 // max diameter

			if maxSize < 10 {
				maxSize = 10 // fallback
			}
			size := float32(10) + (v * maxSize)
			wave.Resize(fyne.NewSize(size, size))
			wave.Move(fyne.NewPos(center.X-size/2, center.Y-size/2))
			wave.FillColor = color.NRGBA{R: 255, G: 255, B: 255, A: uint8(200 * (1 - v))}
			wave.Refresh()
		})
		anim.Curve = fyne.AnimationEaseOut
		anim.Start()

		time.AfterFunc(650*time.Millisecond, func() {
			fyne.Do(func() {
				t.overlay.Remove(wave)
				t.overlay.Refresh()
			})
		})
	}
}

func (t *tileWidget) MouseUp(ev *desktop.MouseEvent) {
	if !t.selected {
		t.bg.FillColor = gtBg3
		t.bg.StrokeColor = color.Transparent
		t.bg.StrokeWidth = 0
	} else {
		t.bg.FillColor = gtBg5
		t.bg.StrokeColor = color.White
		t.bg.StrokeWidth = 4
	}
	t.bg.Refresh()
	t.Refresh()
}

func (t *tileWidget) Tapped(ev *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tileWidget) DoubleTapped(_ *fyne.PointEvent) {
	if t.onDouble != nil {
		t.onDouble()
	}
}

func (t *tileWidget) TappedSecondary(ev *fyne.PointEvent) {
	if t.onRightTap != nil {
		t.onRightTap(ev)
	}
}

// installNatively installs the binary and creates the desktop entry or windows registry
func installNatively(appSettings *settings.AppSettings) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	binDir := appSettings.LocalBinPath
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}

	binName := "gorgeous-installer"
	if runtime.GOOS == "windows" {
		binName = "gorgeous-installer.exe"
	}
	destBin := filepath.Join(binDir, binName)

	// Rename existing to avoid text file busy error if it's currently running
	_ = os.Rename(destBin, destBin+".old")

	src, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(destBin, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		// Windows Registry
		// Define the ProgID for the installer but DO NOT associate .uproject with it automatically.
		_ = exec.Command("reg", "add", `HKCU\Software\Classes\GorgeousInstaller.ProjectFile\shell\open\command`, "/ve", "/d", fmt.Sprintf(`"%s" "%%1"`, destBin), "/f").Run()
		return nil
	}

	// Linux Desktop File
	home, _ := os.UserHomeDir()
	appDir := filepath.Join(home, ".local", "share", "applications")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return err
	}

	desktopFile := filepath.Join(appDir, "gorgeous-installer.desktop")
	desktopContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Gorgeous Installer
Exec=%s %%f
Icon=gorgeous-installer
MimeType=application/x-unreal-project;
Categories=Development;
StartupWMClass=gorgeous-installer
`, destBin)

	if err := os.WriteFile(desktopFile, []byte(desktopContent), 0644); err != nil {
		return err
	}

	// Copy icon
	iconDir := filepath.Join(home, ".local", "share", "icons")
	if err := os.MkdirAll(iconDir, 0755); err == nil {
		if res := loadIconResource(); res != nil {
			_ = os.WriteFile(filepath.Join(iconDir, "gorgeous-installer.png"), res.Content(), 0644)
		}
	}

	// Update desktop database if available
	_ = exec.Command("update-desktop-database", appDir).Run()
	_ = exec.Command("xdg-mime", "default", "gorgeous-installer.desktop", "application/x-unreal-project").Run()

	return nil
}
