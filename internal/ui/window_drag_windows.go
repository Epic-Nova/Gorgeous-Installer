//go:build cgo && windows

package ui

import (
	"math"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32ProcDLL             = syscall.NewLazyDLL("user32.dll")
	findWindowWProc           = user32ProcDLL.NewProc("FindWindowW")
	isWindowProc              = user32ProcDLL.NewProc("IsWindow")
	getWindowRectProc         = user32ProcDLL.NewProc("GetWindowRect")
	setWindowPosProc          = user32ProcDLL.NewProc("SetWindowPos")
	setWindowRgnProc          = user32ProcDLL.NewProc("SetWindowRgn")
	releaseCaptureProc        = user32ProcDLL.NewProc("ReleaseCapture")
	sendMessageWProc          = user32ProcDLL.NewProc("SendMessageW")
	gdi32ProcDLL              = syscall.NewLazyDLL("gdi32.dll")
	createRoundRectRgnProc    = gdi32ProcDLL.NewProc("CreateRoundRectRgn")
	deleteObjectProc          = gdi32ProcDLL.NewProc("DeleteObject")
	dwmapiProcDLL             = syscall.NewLazyDLL("dwmapi.dll")
	dwmSetWindowAttributeProc = dwmapiProcDLL.NewProc("DwmSetWindowAttribute")

	cachedWindowTitle string
	cachedWindowHWND  uintptr
	cachedWindowX     int32
	cachedWindowY     int32
	cachedPositionSet bool
	carryX            float64
	carryY            float64
	roundedWidth      int32
	roundedHeight     int32
	roundedRadius     int32
)

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

const (
	swpNoSize                   = 0x0001
	swpNoZOrder                 = 0x0004
	wmNCLButtonDown             = 0x00A1
	htCaption                   = 0x0002
	dwmwaWindowCornerPreference = 33
	dwmwcpRound                 = 2
)

func startRoundedWindowStyling(title string, radius int32) {
	if title == "" || radius <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(180 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			hwnd := resolveWindowHandle(title)
			if hwnd == 0 {
				continue
			}

			applyDwmCornerPreference(hwnd)
			applyRoundedRegion(hwnd, radius)
		}
	}()
}

func applyDwmCornerPreference(hwnd uintptr) {
	preference := uint32(dwmwcpRound)
	dwmSetWindowAttributeProc.Call(
		hwnd,
		uintptr(dwmwaWindowCornerPreference),
		uintptr(unsafe.Pointer(&preference)),
		unsafe.Sizeof(preference),
	)
}

func applyRoundedRegion(hwnd uintptr, radius int32) {
	if radius <= 0 {
		return
	}

	var rect winRect
	ok, _, _ := getWindowRectProc.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ok == 0 {
		return
	}

	width := rect.Right - rect.Left
	height := rect.Bottom - rect.Top
	if width <= 0 || height <= 0 {
		return
	}

	if roundedWidth == width && roundedHeight == height && roundedRadius == radius {
		return
	}

	rgn, _, _ := createRoundRectRgnProc.Call(
		0,
		0,
		uintptr(width+1),
		uintptr(height+1),
		uintptr(radius),
		uintptr(radius),
	)
	if rgn == 0 {
		return
	}

	applied, _, _ := setWindowRgnProc.Call(hwnd, rgn, 1)
	if applied == 0 {
		deleteObjectProc.Call(rgn)
		return
	}

	roundedWidth = width
	roundedHeight = height
	roundedRadius = radius
}

func beginWindowDrag(title string) bool {
	if title == "" {
		return false
	}

	hwnd := resolveWindowHandle(title)
	if hwnd == 0 {
		return false
	}

	releaseCaptureProc.Call()
	sendMessageWProc.Call(hwnd, wmNCLButtonDown, htCaption, 0)

	// Window was moved by the native drag loop; refresh cached position lazily.
	cachedPositionSet = false
	carryX = 0
	carryY = 0

	return true
}

func moveWindowByDelta(title string, dx, dy float32) {
	if title == "" || (dx == 0 && dy == 0) {
		return
	}

	hwnd := resolveWindowHandle(title)
	if hwnd == 0 {
		return
	}

	if !cachedPositionSet && !cacheWindowPosition(hwnd) {
		return
	}

	carryX += float64(dx)
	carryY += float64(dy)

	stepX, remainingX := wholeStep(carryX)
	stepY, remainingY := wholeStep(carryY)
	carryX = remainingX
	carryY = remainingY

	if stepX == 0 && stepY == 0 {
		return
	}

	cachedWindowX += stepX
	cachedWindowY += stepY

	setWindowPosProc.Call(
		hwnd,
		0,
		uintptr(int(cachedWindowX)),
		uintptr(int(cachedWindowY)),
		0,
		0,
		swpNoSize|swpNoZOrder,
	)
}

func resolveWindowHandle(title string) uintptr {
	if cachedWindowHWND != 0 && cachedWindowTitle == title {
		if alive, _, _ := isWindowProc.Call(cachedWindowHWND); alive != 0 {
			return cachedWindowHWND
		}

		cachedWindowHWND = 0
		cachedPositionSet = false
		roundedWidth = 0
		roundedHeight = 0
		roundedRadius = 0
	}

	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0
	}

	hwnd, _, _ := findWindowWProc.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		return 0
	}

	cachedWindowTitle = title
	cachedWindowHWND = hwnd
	cachedPositionSet = false
	carryX = 0
	carryY = 0

	return hwnd
}

func cacheWindowPosition(hwnd uintptr) bool {
	var rect winRect
	ok, _, _ := getWindowRectProc.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ok == 0 {
		return false
	}

	cachedWindowX = rect.Left
	cachedWindowY = rect.Top
	cachedPositionSet = true

	return true
}

func wholeStep(value float64) (int32, float64) {
	if value > 0 {
		whole := math.Floor(value)
		return int32(whole), value - whole
	}

	if value < 0 {
		whole := math.Ceil(value)
		return int32(whole), value - whole
	}

	return 0, 0
}
