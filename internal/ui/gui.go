//go:build cgo

package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
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

	bundle "gorgeous-installer"
	"gorgeous-installer/internal/config"
	"gorgeous-installer/internal/installer"
	"gorgeous-installer/internal/unreal"
)

// GUIApp represents the native Windows GUI application.
type GUIApp struct {
	config *config.Config
}

// NewGUIApp creates a new GUI app instance.
func NewGUIApp(cfg *config.Config) *GUIApp {
	return &GUIApp{config: cfg}
}

// Run starts the native Windows GUI.
func (g *GUIApp) Run() {
	application := app.NewWithID("com.gorgeouscore.installer")
	iconRes := loadIconResource()
	if iconRes != nil {
		application.SetIcon(iconRes)
	}

	var win fyne.Window
	if d, ok := application.Driver().(desktop.Driver); ok {
		win = d.CreateSplashWindow()
	} else {
		win = application.NewWindow("Gorgeous Installer")
	}

	win.SetTitle("Gorgeous Installer")
	if iconRes != nil {
		win.SetIcon(iconRes)
	}
	win.SetPadded(false)
	baseWindowSize := fyne.NewSize(500, 300)
	compileWindowSize := fyne.NewSize(860, 560)
	resultWindowSize := fyne.NewSize(860, 560)
	win.Resize(baseWindowSize)
	win.SetFixedSize(false)
	win.CenterOnScreen()

	projectEntry := widget.NewEntry()
	projectEntry.SetPlaceHolder("Select a .uproject file")
	projectEntry.Disable()

	engineValue := newSurfaceValueText("Not detected")

	versions := g.availableVersions()
	plannedInstallAction := installer.InstallActionInstall
	lastPlanFingerprint := ""
	var refreshInstallPlanState func()

	versionSelect := widget.NewSelect(versions, func(_ string) {
		if refreshInstallPlanState != nil {
			refreshInstallPlanState()
		}
	})
	versionSelect.PlaceHolder = "Select pack version"
	if len(versions) > 0 {
		versionSelect.SetSelected(versions[0])
	}
	autoPackVersion := ""
	manualVersionSelection := false

	versionSelectRow := container.NewHBox(
		newSurfaceLabelText("Version:"),
		versionSelect,
	)
	versionSelectRow.Hide()

	statusLines := container.NewVBox()
	statusScroll := container.NewScroll(statusLines)
	statusScroll.SetMinSize(fyne.NewSize(0, 170))

	spinner := widget.NewProgressBarInfinite()
	spinner.Hide()

	var installMu sync.Mutex
	var installCancel context.CancelFunc
	installRunning := false

	appendStatus := func(msg string, args ...any) {
		line := fmt.Sprintf(msg, args...)
		fyne.Do(func() {
			lineColor := logLineColor(line)
			wrappedLines := wrapLogLine(line, 118)
			for _, wrapped := range wrappedLines {
				entry := canvas.NewText(wrapped, lineColor)
				entry.TextSize = 12
				statusLines.Add(entry)
			}
			statusLines.Refresh()
			statusScroll.ScrollToBottom()

			// Force a second tail-follow tick to keep up with rapid compiler output bursts.
			time.AfterFunc(10*time.Millisecond, func() {
				fyne.Do(func() {
					statusScroll.ScrollToBottom()
				})
			})
		})
	}
	appendStatus("Ready.")

	go g.validateConfiguredPackSHAs(appendStatus)

	var projectPath string
	var mainContent fyne.CanvasObject

	win.SetCloseIntercept(func() {
		installMu.Lock()
		cancel := installCancel
		running := installRunning
		installCancel = nil
		installRunning = false
		installMu.Unlock()

		if running && cancel != nil {
			appendStatus("Stopping active compilation before closing...")
			cancel()
		}

		win.SetCloseIntercept(nil)
		win.Close()
	})

	browseBtn := widget.NewButtonWithIcon("Browse Project", theme.FolderOpenIcon(), func() {
		d := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, win)
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

			projectPath = selected
			lastPlanFingerprint = ""
			projectEntry.Enable()
			projectEntry.SetText(projectPath)
			projectEntry.Disable()
			appendStatus("Project selected: %s", filepath.Base(projectPath))

			engineVersion, enginePath, detectErr := unreal.GetEngineVersionFromProject(projectPath)
			if detectErr != nil {
				autoPackVersion = ""
				manualVersionSelection = true
				if len(versions) > 0 && versionSelect.Selected == "" {
					versionSelect.SetSelected(versions[0])
				}
				versionSelectRow.Show()
				setCanvasText(engineValue, "Could not detect engine version")
				appendStatus("Engine detection failed: %v", detectErr)
				appendStatus("Version selection enabled due detection failure")
				if refreshInstallPlanState != nil {
					refreshInstallPlanState()
				}
				return
			}

			setCanvasText(engineValue, "UE "+engineVersion)
			appendStatus("Detected engine version from EngineAssociation: %s", engineVersion)
			appendStatus("Resolved engine path: %s", enginePath)

			if best := chooseClosestVersion(engineVersion, versions); best != "" {
				autoPackVersion = best
				manualVersionSelection = false
				versionSelect.SetSelected(best)
				versionSelectRow.Hide()
				appendStatus("Selected pack version from project: %s", best)
				if refreshInstallPlanState != nil {
					refreshInstallPlanState()
				}
				return
			}

			autoPackVersion = ""
			manualVersionSelection = false
			versionSelectRow.Hide()
			appendStatus("No configured pack versions were available for automatic selection")
			if refreshInstallPlanState != nil {
				refreshInstallPlanState()
			}
		}, win)

		d.SetFilter(storage.NewExtensionFileFilter([]string{".uproject"}))
		d.Show()
	})

	var installBtn *widget.Button
	currentSelectedVersion := func() string {
		selectedVersion := autoPackVersion
		if manualVersionSelection {
			selectedVersion = versionSelect.Selected
		}

		return strings.TrimSpace(selectedVersion)
	}

	setInstallButtonForAction := func(action installer.InstallAction) {
		plannedInstallAction = action
		if installBtn == nil {
			return
		}

		switch action {
		case installer.InstallActionUpdate:
			installBtn.SetText("Update")
			installBtn.SetIcon(theme.ViewRefreshIcon())
		case installer.InstallActionReinstall:
			installBtn.SetText("Reinstall")
			installBtn.SetIcon(theme.ViewRefreshIcon())
		default:
			installBtn.SetText("Install")
			installBtn.SetIcon(theme.DownloadIcon())
		}
	}

	refreshInstallPlanState = func() {
		installMu.Lock()
		running := installRunning
		installMu.Unlock()

		if running {
			return
		}

		if strings.TrimSpace(projectPath) == "" {
			setInstallButtonForAction(installer.InstallActionInstall)
			return
		}

		selectedVersion := currentSelectedVersion()
		if selectedVersion == "" {
			setInstallButtonForAction(installer.InstallActionInstall)
			return
		}

		inspectProjectPath := projectPath
		inspectVersion := selectedVersion
		appendStatus("Analyzing installed files for pack version %s", inspectVersion)

		go func(p, v string) {
			plan, err := g.inspectInstallPlan(p, v)

			fyne.Do(func() {
				if p != projectPath || v != currentSelectedVersion() {
					return
				}

				if err != nil {
					setInstallButtonForAction(installer.InstallActionInstall)
					appendStatus("Could not determine install state: %v", err)
					return
				}

				setInstallButtonForAction(plan.Action)

				fingerprint := fmt.Sprintf("%s|%s|%s|%d|%d|%d|%d", p, v, plan.Action, plan.TotalSourceFiles, plan.ExistingFiles, len(plan.MissingFiles), len(plan.DifferentFiles))
				if len(plan.ChangedFiles) > 0 {
					fingerprint += "|" + strings.Join(plan.ChangedFiles, "|")
				}
				if fingerprint == lastPlanFingerprint {
					return
				}
				lastPlanFingerprint = fingerprint

				switch plan.Action {
				case installer.InstallActionUpdate:
					appendStatus("Update available: %d files differ", len(plan.ChangedFiles))
					for _, file := range plan.ChangedFiles {
						appendStatus("DIFF: %s", file)
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
		}(inspectProjectPath, inspectVersion)
	}

	installBtn = widget.NewButtonWithIcon("Install", theme.DownloadIcon(), func() {
		if projectPath == "" {
			dialog.ShowError(fmt.Errorf("please select a .uproject file first"), win)
			return
		}

		selectedVersion := currentSelectedVersion()
		if selectedVersion == "" {
			if manualVersionSelection {
				dialog.ShowError(fmt.Errorf("please select a pack version"), win)
			} else {
				dialog.ShowError(fmt.Errorf("could not determine a pack version from the selected project"), win)
			}
			return
		}

		action := plannedInstallAction
		if action == "" {
			action = installer.InstallActionInstall
		}

		installBtn.Disable()
		browseBtn.Disable()
		if manualVersionSelection {
			versionSelect.Disable()
		}
		spinner.Show()
		spinner.Start()
		appendStatus("Starting %s for pack version %s", action, selectedVersion)

		if strings.EqualFold(g.config.PackType, "code") {
			animateWindowResize(win, compileWindowSize, 360*time.Millisecond)
		}

		installCtx, cancelInstall := context.WithCancel(context.Background())
		installMu.Lock()
		installCancel = cancelInstall
		installRunning = true
		installMu.Unlock()

		go func(p, v string, installAction installer.InstallAction) {
			defer func() {
				cancelInstall()
				installMu.Lock()
				installCancel = nil
				installRunning = false
				installMu.Unlock()
			}()

			err := g.performInstall(installCtx, p, v, installAction, appendStatus)

			fyne.Do(func() {
				spinner.Stop()
				spinner.Hide()
				installBtn.Enable()
				browseBtn.Enable()
				if manualVersionSelection {
					versionSelect.Enable()
				}

				if err != nil && errors.Is(err, context.Canceled) {
					if strings.EqualFold(g.config.PackType, "code") {
						animateWindowResize(win, baseWindowSize, 260*time.Millisecond)
					}
					appendStatus("Installation canceled")
					if refreshInstallPlanState != nil {
						refreshInstallPlanState()
					}
					return
				}

				if err != nil {
					if strings.EqualFold(g.config.PackType, "code") {
						animateWindowResize(win, resultWindowSize, 220*time.Millisecond)
					}
					appendStatus("Installation failed: %v", err)

					resultActionLabel := ""
					var resultAction func()
					if isCompileFailure(err) {
						resultActionLabel = "Back to Logs"
						resultAction = func() {
							animateWindowOpen(win, mainContent)
							if refreshInstallPlanState != nil {
								refreshInstallPlanState()
							}
						}
					}

					showInstallResult(win, false, err.Error(), iconRes, resultActionLabel, resultAction, "Close Installer", nil, "installation")
					return
				}

				if strings.EqualFold(g.config.PackType, "code") {
					animateWindowResize(win, resultWindowSize, 220*time.Millisecond)
				}
				appendStatus("Installation completed successfully")
				showInstallResult(win, true, "Successfully Installed!", iconRes, "", nil, "Close Installer", nil, "installation")
			})
		}(projectPath, selectedVersion, action)
	})
	setInstallButtonForAction(installer.InstallActionInstall)

	exitBtn := widget.NewButton("Exit", func() {
		win.Close()
	})
	validateSHABtn := widget.NewButtonWithIcon("Validate SHA", theme.SearchIcon(), func() {
		g.showSHAValidationScreen(win, iconRes, mainContent, appendStatus)
	})

	pluginValue := newSurfaceValueText(g.config.PluginName)
	packTypeLabel := newSurfaceLabelText(fmt.Sprintf("Pack type: %s", g.config.PackType))

	controlSection := newSection(
		"Project",
		"Choose the Unreal project and pack",
		container.NewVBox(
			container.NewHBox(newSurfaceLabelText("Target plugin:"), pluginValue),
			container.NewBorder(nil, nil, nil, browseBtn, projectEntry),
			container.NewHBox(
				newSurfaceLabelText("Engine:"),
				engineValue,
				layout.NewSpacer(),
			),
			versionSelectRow,
			packTypeLabel,
		),
	)

	statusSection := newSection(
		"Installer Log",
		"Live installation output",
		container.NewBorder(
			container.NewHBox(newSurfaceLabelText("Status"), layout.NewSpacer(), spinner),
			nil,
			nil,
			nil,
			statusScroll,
		),
	)

	actionRow := container.NewHBox(layout.NewSpacer(), validateSHABtn, installBtn, exitBtn)

	windowTitle := canvas.NewText("Gorgeous Installer", color.NRGBA{R: 225, G: 238, B: 243, A: 255})
	windowTitle.TextSize = 18
	windowTitle.TextStyle = fyne.TextStyle{Bold: true}

	var logo fyne.CanvasObject
	if iconRes != nil {
		img := canvas.NewImageFromResource(iconRes)
		img.FillMode = canvas.ImageFillContain
		logo = container.NewGridWrap(fyne.NewSize(40, 40), img)
	} else {
		fallbackLogo := canvas.NewCircle(color.NRGBA{R: 32, G: 125, B: 160, A: 255})
		logo = container.NewGridWrap(fyne.NewSize(40, 40), fallbackLogo)
	}
	dragWindow := func(dx, dy float32) {
		moveWindowByDelta(win.Title(), dx, dy)
	}
	startWindowDrag := func() bool {
		return beginWindowDrag(win.Title())
	}
	dragTitle := newDragSurface(
		container.NewHBox(logo, windowTitle, layout.NewSpacer()),
		dragWindow,
		startWindowDrag,
	)
	topBar := newRoundedSurface(
		color.NRGBA{R: 15, G: 77, B: 104, A: 255},
		16,
		container.NewPadded(dragTitle),
	)

	body := container.NewBorder(
		controlSection,
		actionRow,
		nil,
		nil,
		statusSection,
	)

	chrome := container.NewBorder(
		topBar,
		nil,
		nil,
		nil,
		body,
	)

	panel := newRoundedSurface(
		color.NRGBA{R: 10, G: 66, B: 90, A: 255},
		18,
		container.NewPadded(chrome),
	)

	backdrop := canvas.NewRectangle(color.NRGBA{R: 4, G: 37, B: 52, A: 255})
	mainContent = container.NewStack(backdrop, container.NewPadded(panel))
	playBootSequence(win, iconRes, mainContent)
	startRoundedWindowStyling(win.Title(), 32)
	win.ShowAndRun()
}

func playBootSequence(win fyne.Window, iconRes fyne.Resource, finalContent fyne.CanvasObject) {
	bootBg := canvas.NewRectangle(color.NRGBA{R: 3, G: 32, B: 46, A: 255})

	glow := canvas.NewCircle(color.NRGBA{R: 23, G: 126, B: 169, A: 70})
	ring := canvas.NewCircle(color.Transparent)
	ring.StrokeColor = color.NRGBA{R: 101, G: 171, B: 200, A: 170}
	ring.StrokeWidth = 3

	var iconVisual fyne.CanvasObject
	if iconRes != nil {
		img := canvas.NewImageFromResource(iconRes)
		img.FillMode = canvas.ImageFillContain
		iconVisual = img
	} else {
		iconVisual = canvas.NewCircle(color.NRGBA{R: 32, G: 125, B: 160, A: 255})
	}

	logoSurface := newRoundedSurface(
		color.NRGBA{R: 14, G: 77, B: 104, A: 255},
		22,
		container.NewPadded(iconVisual),
	)

	titleText := canvas.NewText("Gorgeous Installer", color.NRGBA{R: 229, G: 240, B: 244, A: 255})
	titleText.TextSize = 24
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	subtitleText := canvas.NewText("Loading installer...", color.NRGBA{R: 177, G: 209, B: 220, A: 255})
	subtitleText.TextSize = 12

	bootStage := container.NewWithoutLayout(glow, ring, logoSurface, titleText, subtitleText)
	bootContent := container.NewStack(bootBg, bootStage)
	win.SetContent(bootContent)

	layoutBoot := func(iconSize, ringSize float32) {
		canvasSize := currentCanvasSize(win)

		ringObjectSize := fyne.NewSize(ringSize, ringSize)
		ring.Resize(ringObjectSize)
		ring.Move(centeredPos(canvasSize, ringObjectSize, -28))

		glowSize := fyne.NewSize(ringSize+26, ringSize+26)
		glow.Resize(glowSize)
		glow.Move(centeredPos(canvasSize, glowSize, -28))

		iconObjectSize := fyne.NewSize(iconSize, iconSize)
		logoSurface.Resize(iconObjectSize)
		logoSurface.Move(centeredPos(canvasSize, iconObjectSize, -28))

		titleSize := titleText.MinSize()
		titleText.Move(centeredPos(canvasSize, titleSize, 46))

		subtitleSize := subtitleText.MinSize()
		subtitleText.Move(centeredPos(canvasSize, subtitleSize, 74))

		canvas.Refresh(ring)
		canvas.Refresh(glow)
		canvas.Refresh(logoSurface)
		canvas.Refresh(titleText)
		canvas.Refresh(subtitleText)
	}

	layoutBoot(86, 128)

	time.AfterFunc(40*time.Millisecond, func() {
		fyne.Do(func() {
			layoutBoot(86, 128)
		})
	})

	ringPulse := canvas.NewSizeAnimation(
		fyne.NewSize(124, 124),
		fyne.NewSize(166, 166),
		640*time.Millisecond,
		func(s fyne.Size) {
			layoutBoot(86, s.Width)
		},
	)
	ringPulse.AutoReverse = true
	ringPulse.Curve = fyne.AnimationEaseInOut
	ringPulse.RepeatCount = 2
	ringPulse.Start()

	iconPulse := canvas.NewSizeAnimation(
		fyne.NewSize(82, 82),
		fyne.NewSize(96, 96),
		520*time.Millisecond,
		func(s fyne.Size) {
			layoutBoot(s.Width, ring.Size().Width)
		},
	)
	iconPulse.AutoReverse = true
	iconPulse.Curve = fyne.AnimationEaseInOut
	iconPulse.RepeatCount = 2
	iconPulse.Start()

	time.AfterFunc(1300*time.Millisecond, func() {
		fyne.Do(func() {
			animateWindowOpen(win, finalContent)
		})
	})
}

func animateWindowOpen(win fyne.Window, finalContent fyne.CanvasObject) {
	canvasSize := currentCanvasSize(win)
	startSize := fyne.NewSize(canvasSize.Width*0.84, canvasSize.Height*0.84)
	startPos := centeredPos(canvasSize, startSize, 10)

	finalContent.Resize(startSize)
	finalContent.Move(startPos)

	openingLayer := container.NewWithoutLayout(finalContent)
	win.SetContent(openingLayer)

	moveAnim := canvas.NewPositionAnimation(
		startPos,
		fyne.NewPos(0, 0),
		300*time.Millisecond,
		func(p fyne.Position) {
			finalContent.Move(p)
			canvas.Refresh(finalContent)
		},
	)
	moveAnim.Curve = fyne.AnimationEaseInOut

	sizeAnim := canvas.NewSizeAnimation(
		startSize,
		canvasSize,
		300*time.Millisecond,
		func(s fyne.Size) {
			finalContent.Resize(s)
			canvas.Refresh(finalContent)
		},
	)
	sizeAnim.Curve = fyne.AnimationEaseInOut

	moveAnim.Start()
	sizeAnim.Start()

	time.AfterFunc(340*time.Millisecond, func() {
		fyne.Do(func() {
			win.SetContent(finalContent)
		})
	})
}

func showInstallResult(win fyne.Window, success bool, message string, iconRes fyne.Resource, actionLabel string, onAction func(), closeLabel string, onClose func(), operationLabel string) {
	_ = iconRes

	operation := strings.ToLower(strings.TrimSpace(operationLabel))
	if operation == "" {
		operation = "installation"
	}
	isSHAValidation := strings.Contains(operation, "sha")

	bgBase := color.NRGBA{R: 4, G: 37, B: 52, A: 255}
	bgPulse := color.NRGBA{R: 7, G: 48, B: 68, A: 255}
	cardColor := color.NRGBA{R: 14, G: 82, B: 107, A: 245}
	cardBorderColor := color.NRGBA{R: 104, G: 180, B: 206, A: 190}
	badgeColor := color.NRGBA{R: 67, G: 210, B: 145, A: 255}
	badgeGlowColor := color.NRGBA{R: 52, G: 192, B: 134, A: 85}
	messageSurfaceColor := color.NRGBA{R: 17, G: 94, B: 122, A: 240}
	statusLabel := "INSTALLATION COMPLETE"
	iconGlyph := "✓"
	title := "Successfully Installed!"
	defaultSuccessMessage := "All selected files were installed and the plugin was updated successfully."
	defaultFailureMessage := "An unknown error occurred during installation."
	if isSHAValidation {
		statusLabel = "SHA VALIDATION PASSED"
		title = "SHA Validation Passed"
		defaultSuccessMessage = "All compared files matched the expected checksums."
		defaultFailureMessage = "The selected files did not match the expected checksums."
	}
	detailMessage := strings.TrimSpace(message)
	if detailMessage == "" || strings.EqualFold(detailMessage, "successfully installed!") {
		detailMessage = defaultSuccessMessage
	}

	if !success {
		bgBase = color.NRGBA{R: 34, G: 11, B: 22, A: 255}
		bgPulse = color.NRGBA{R: 48, G: 14, B: 30, A: 255}
		cardColor = color.NRGBA{R: 121, G: 33, B: 52, A: 245}
		cardBorderColor = color.NRGBA{R: 241, G: 138, B: 152, A: 210}
		badgeColor = color.NRGBA{R: 255, G: 112, B: 127, A: 255}
		badgeGlowColor = color.NRGBA{R: 255, G: 113, B: 128, A: 95}
		messageSurfaceColor = color.NRGBA{R: 140, G: 44, B: 63, A: 235}
		statusLabel = "INSTALLATION FAILED"
		iconGlyph = "▲"
		title = "Installation Failed"
		if isSHAValidation {
			statusLabel = "SHA VALIDATION FAILED"
			title = "SHA Validation Failed"
		}
		if detailMessage == "" {
			detailMessage = defaultFailureMessage
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

	badge := container.NewStack(
		badgeGlow,
		badgeFill,
		badgeRing,
		container.NewCenter(badgeIcon),
	)

	statusText := canvas.NewText(statusLabel, color.NRGBA{R: 214, G: 232, B: 239, A: 255})
	statusText.TextSize = 12
	statusText.TextStyle = fyne.TextStyle{Bold: true}

	titleText := canvas.NewText(title, color.NRGBA{R: 236, G: 244, B: 248, A: 255})
	titleText.TextSize = 38
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	messageLabel := widget.NewLabel(detailMessage)
	messageLabel.Alignment = fyne.TextAlignCenter
	messageLabel.Wrapping = fyne.TextWrapWord

	messageSurface := newRoundedSurface(
		messageSurfaceColor,
		16,
		container.NewPadded(messageLabel),
	)

	if strings.TrimSpace(closeLabel) == "" {
		closeLabel = "Close Installer"
	}

	closeBtn := widget.NewButton(closeLabel, func() {
		if onClose != nil {
			onClose()
			return
		}

		win.Close()
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
		container.NewPadded(actionRow),
	)

	card := container.NewStack(
		cardFill,
		cardBorder,
		container.NewPadded(cardBody),
	)

	stage := container.NewWithoutLayout(backdrop, blobA, blobB, card, badge)
	win.SetContent(stage)

	canvasSize := currentCanvasSize(win)
	backdrop.Resize(canvasSize)

	cardWidth := canvasSize.Width * 0.92
	if cardWidth > 760 {
		cardWidth = 760
	}
	maxCanvasWidth := canvasSize.Width - 22
	if cardWidth > maxCanvasWidth {
		cardWidth = maxCanvasWidth
	}
	if cardWidth < 300 {
		cardWidth = canvasSize.Width
	}

	cardHeight := canvasSize.Height * 0.86
	if cardHeight > 560 {
		cardHeight = 560
	}
	maxCanvasHeight := canvasSize.Height - 18
	if cardHeight > maxCanvasHeight {
		cardHeight = maxCanvasHeight
	}
	if cardHeight < 220 {
		cardHeight = canvasSize.Height
	}

	finalCardSize := fyne.NewSize(cardWidth, cardHeight)
	startCardSize := fyne.NewSize(finalCardSize.Width*0.9, finalCardSize.Height*0.9)

	finalCardPos := centeredPos(canvasSize, finalCardSize, 8)
	startCardPos := centeredPos(canvasSize, startCardSize, 22)

	card.Resize(startCardSize)
	card.Move(startCardPos)

	setBadgePos := func(cardPos fyne.Position, cardSize fyne.Size, scale float32, yOffset float32) {
		badgeSize := fyne.NewSize(108*scale, 108*scale)
		badge.Resize(badgeSize)
		badgePos := fyne.NewPos(
			cardPos.X+(cardSize.Width-badgeSize.Width)/2,
			cardPos.Y-badgeSize.Height*0.36+yOffset,
		)
		badge.Move(badgePos)
		canvas.Refresh(badge)
	}

	setBadgePos(startCardPos, startCardSize, 0.84, 0)

	blobASize := fyne.NewSize(canvasSize.Width*0.82, canvasSize.Width*0.82)
	blobA.Resize(blobASize)
	blobA.Move(fyne.NewPos(-blobASize.Width*0.3, -blobASize.Height*0.4))

	blobBSize := fyne.NewSize(canvasSize.Width*0.74, canvasSize.Width*0.74)
	blobB.Resize(blobBSize)
	blobB.Move(fyne.NewPos(canvasSize.Width-blobBSize.Width*0.72, canvasSize.Height-blobBSize.Height*0.64))

	cardMove := canvas.NewPositionAnimation(
		startCardPos,
		finalCardPos,
		360*time.Millisecond,
		func(p fyne.Position) {
			card.Move(p)
			setBadgePos(p, card.Size(), 1.0, 0)
			canvas.Refresh(card)
		},
	)
	cardMove.Curve = fyne.AnimationEaseInOut

	cardGrow := canvas.NewSizeAnimation(
		startCardSize,
		finalCardSize,
		360*time.Millisecond,
		func(s fyne.Size) {
			card.Resize(s)
			setBadgePos(card.Position(), s, 1.0, 0)
			canvas.Refresh(card)
		},
	)
	cardGrow.Curve = fyne.AnimationEaseInOut

	badgePop := canvas.NewSizeAnimation(
		fyne.NewSize(84, 84),
		fyne.NewSize(108, 108),
		420*time.Millisecond,
		func(s fyne.Size) {
			scale := s.Width / 108
			setBadgePos(card.Position(), card.Size(), scale, 0)
		},
	)
	badgePop.Curve = fyne.AnimationEaseOut

	bgPulseAnim := canvas.NewColorRGBAAnimation(
		bgBase,
		bgPulse,
		1800*time.Millisecond,
		func(c color.Color) {
			backdrop.FillColor = c
			backdrop.Refresh()
		},
	)
	bgPulseAnim.AutoReverse = true
	bgPulseAnim.Curve = fyne.AnimationEaseInOut
	bgPulseAnim.RepeatCount = fyne.AnimationRepeatForever

	blobADrift := canvas.NewPositionAnimation(
		blobA.Position(),
		fyne.NewPos(blobA.Position().X+18, blobA.Position().Y+10),
		2800*time.Millisecond,
		func(p fyne.Position) {
			blobA.Move(p)
			canvas.Refresh(blobA)
		},
	)
	blobADrift.AutoReverse = true
	blobADrift.Curve = fyne.AnimationEaseInOut
	blobADrift.RepeatCount = fyne.AnimationRepeatForever

	blobBDrift := canvas.NewPositionAnimation(
		blobB.Position(),
		fyne.NewPos(blobB.Position().X-20, blobB.Position().Y-12),
		3200*time.Millisecond,
		func(p fyne.Position) {
			blobB.Move(p)
			canvas.Refresh(blobB)
		},
	)
	blobBDrift.AutoReverse = true
	blobBDrift.Curve = fyne.AnimationEaseInOut
	blobBDrift.RepeatCount = fyne.AnimationRepeatForever

	cardMove.Start()
	cardGrow.Start()
	badgePop.Start()
	bgPulseAnim.Start()
	blobADrift.Start()
	blobBDrift.Start()

	time.AfterFunc(450*time.Millisecond, func() {
		fyne.Do(func() {
			finalBadgePos := badge.Position()
			badgeBob := canvas.NewPositionAnimation(
				fyne.NewPos(finalBadgePos.X, finalBadgePos.Y-4),
				fyne.NewPos(finalBadgePos.X, finalBadgePos.Y+4),
				1200*time.Millisecond,
				func(p fyne.Position) {
					badge.Move(p)
					canvas.Refresh(badge)
				},
			)
			badgeBob.AutoReverse = true
			badgeBob.Curve = fyne.AnimationEaseInOut
			badgeBob.RepeatCount = fyne.AnimationRepeatForever
			badgeBob.Start()
		})
	})
}

func currentCanvasSize(win fyne.Window) fyne.Size {
	const (
		defaultWidth  = float32(500)
		defaultHeight = float32(300)
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
	target = clampSize(target, fyne.NewSize(420, 280), fyne.NewSize(1180, 760))
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

			fyne.Do(func() {
				win.Resize(fyne.NewSize(w, h))
			})
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

func newSurfaceLabelText(text string) *canvas.Text {
	l := canvas.NewText(text, color.NRGBA{R: 212, G: 232, B: 241, A: 255})
	l.TextSize = 13
	return l
}

func newSurfaceValueText(text string) *canvas.Text {
	v := canvas.NewText(text, color.NRGBA{R: 237, G: 246, B: 250, A: 255})
	v.TextSize = 13
	v.TextStyle = fyne.TextStyle{Bold: true}
	return v
}

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
		return color.NRGBA{R: 255, G: 124, B: 136, A: 255}
	case strings.Contains(trimmed, "warning"), strings.Contains(trimmed, "warn"):
		return color.NRGBA{R: 255, G: 214, B: 126, A: 255}
	default:
		return color.NRGBA{R: 220, G: 232, B: 238, A: 255}
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

	pluginPath, err := unreal.LocatePluginByName(filepath.Dir(projectPath), enginePath, g.config.PluginName)
	if err != nil {
		if detectErr != nil {
			return nil, fmt.Errorf("failed to locate plugin %q in project plugins after engine detection failure: %w", g.config.PluginName, err)
		}
		return nil, fmt.Errorf("failed to locate plugin %q: %w", g.config.PluginName, err)
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

	pluginPath, err := unreal.LocatePluginByName(filepath.Dir(projectPath), enginePath, g.config.PluginName)
	if err != nil {
		if detectErr != nil {
			return fmt.Errorf("failed to locate plugin %q in project plugins after engine detection failure: %w", g.config.PluginName, err)
		}
		return fmt.Errorf("failed to locate plugin %q: %w", g.config.PluginName, err)
	}

	appendStatus("Plugin path: %s", pluginPath)

	selectedPack, err := g.selectedPackVersion(selectedVersion)
	if err != nil {
		return err
	}

	appendStatus("Installing %s %s with action %s", g.config.PackType, selectedPack.Version, action)

	inst := installer.NewInstaller(pluginPath, g.config.PackType, selectedPack, g.config.InstallPath, projectPath, enginePath)
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

	if manifestCount == 0 {
		appendStatus("No SHA manifest files were found for startup validation")
	}
}

func (g *GUIApp) showSHAValidationScreen(win fyne.Window, iconRes fyne.Resource, mainContent fyne.CanvasObject, appendStatus func(string, ...any)) {
	versions := g.availableVersions()
	engineSelect := widget.NewSelect(versions, nil)
	engineSelect.PlaceHolder = "Select engine version"
	if len(versions) > 0 {
		engineSelect.SetSelected(versions[0])
	}

	shaFileEntry := widget.NewEntry()
	shaFileEntry.SetPlaceHolder("Optional: choose a .sha/.sha256 file")
	shaFileEntry.Disable()

	selectedPackValue := newSurfaceValueText("Not selected")
	manifestValue := newSurfaceValueText("Auto from config")
	var selectedSHAPath string
	var validationScreen fyne.CanvasObject

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
				dialog.ShowError(err, win)
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

	backBtn := widget.NewButton("Back", func() {
		animateWindowOpen(win, mainContent)
	})

	var validateBtn *widget.Button
	validateBtn = widget.NewButtonWithIcon("Validate", theme.ConfirmIcon(), func() {
		selectedVersion := strings.TrimSpace(engineSelect.Selected)
		if selectedVersion == "" {
			dialog.ShowError(fmt.Errorf("please select an engine version"), win)
			return
		}

		selectedPack, err := g.selectedPackVersion(selectedVersion)
		if err != nil {
			dialog.ShowError(err, win)
			return
		}

		setCanvasText(selectedPackValue, selectedVersion)

		manifestPath := strings.TrimSpace(selectedSHAPath)
		if manifestPath == "" {
			resolvedPath, found := installer.ResolvePackSHAManifestPath(selectedPack)
			if !found || strings.TrimSpace(resolvedPath) == "" {
				dialog.ShowError(fmt.Errorf("no SHA file selected and no shaFile configured for version %s", selectedVersion), win)
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
					showInstallResult(win, false, validateErr.Error(), iconRes, "", nil, "Close", func() {
						animateWindowOpen(win, validationScreen)
					}, "sha validation")
					return
				}

				resultMessage := formatSHAValidationMessage(report)
				if report.IsValid() {
					appendStatus("SHA validation successful for %s", report.PackVersion)
					showInstallResult(win, true, resultMessage, iconRes, "", nil, "Close", func() {
						animateWindowOpen(win, validationScreen)
					}, "sha validation")
					return
				}

				appendStatus("SHA validation reported differences for %s", report.PackVersion)
				showInstallResult(win, false, resultMessage, iconRes, "", nil, "Close", func() {
					animateWindowOpen(win, validationScreen)
				}, "sha validation")
			})
		}(selectedPack, manifestPath)
	})

	validationSection := newSection(
		"SHA Validation",
		"Compare pack files against a SHA manifest",
		container.NewVBox(
			container.NewHBox(newSurfaceLabelText("Engine version:"), engineSelect),
			container.NewBorder(nil, nil, nil, browseSHABtn, shaFileEntry),
			container.NewHBox(newSurfaceLabelText("Selected pack:"), selectedPackValue),
			container.NewHBox(newSurfaceLabelText("Manifest:"), manifestValue),
		),
	)

	actions := container.NewHBox(layout.NewSpacer(), validateBtn, backBtn)
	body := container.NewBorder(validationSection, actions, nil, nil, container.NewPadded(widget.NewLabel("Select an engine version and optional SHA file, then validate.")))

	chrome := newRoundedSurface(
		color.NRGBA{R: 10, G: 66, B: 90, A: 255},
		18,
		container.NewPadded(body),
	)

	validationBackdrop := canvas.NewRectangle(color.NRGBA{R: 4, G: 37, B: 52, A: 255})
	validationScreen = container.NewStack(validationBackdrop, container.NewPadded(chrome))
	animateWindowOpen(win, validationScreen)
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

func newSection(title, subtitle string, content fyne.CanvasObject) fyne.CanvasObject {
	titleText := canvas.NewText(title, color.White)
	titleText.TextSize = 20
	titleText.TextStyle = fyne.TextStyle{Bold: true}

	subText := canvas.NewText(subtitle, color.NRGBA{R: 176, G: 211, B: 222, A: 255})
	subText.TextSize = 12

	sectionContent := container.NewVBox(
		titleText,
		subText,
		widget.NewSeparator(),
		content,
	)

	return newRoundedSurface(
		color.NRGBA{R: 18, G: 88, B: 116, A: 230},
		20,
		container.NewPadded(sectionContent),
	)
}

func newRoundedSurface(fill color.Color, radius float32, content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(fill)
	bg.CornerRadius = radius
	return container.NewStack(bg, content)
}

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

func (h *dragHandle) MouseUp(_ *desktop.MouseEvent) {
	h.nativeDrag = false
}

func (h *dragHandle) Dragged(ev *fyne.DragEvent) {
	if h.nativeDrag {
		return
	}

	if h.onDrag == nil {
		return
	}

	h.onDrag(ev.Dragged.DX, ev.Dragged.DY)
}

func (h *dragHandle) DragEnd() {
	h.nativeDrag = false
}

func (h *dragHandle) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}

func newDragSurface(content fyne.CanvasObject, onDrag func(float32, float32), onDragInit func() bool) fyne.CanvasObject {
	return container.NewStack(newDragHandle(onDrag, onDragInit), container.NewPadded(content))
}

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
	best := versions[0]
	bestMajor, bestMinor := parseVersion(best)

	for _, v := range versions {
		major, minor := parseVersion(v)

		if major > engineMajor || (major == engineMajor && minor > engineMinor) {
			continue
		}

		if major > bestMajor || (major == bestMajor && minor > bestMinor) {
			best = v
			bestMajor = major
			bestMinor = minor
		}
	}

	return best
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
