//go:build cgo && windows

package ui

import (
	"log"
	"math"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

var (
	user32ProcDLL             = syscall.NewLazyDLL("user32.dll")
	getWindowRectProc         = user32ProcDLL.NewProc("GetWindowRect")
	setWindowPosProc          = user32ProcDLL.NewProc("SetWindowPos")
	setWindowRgnProc          = user32ProcDLL.NewProc("SetWindowRgn")
	gdi32ProcDLL              = syscall.NewLazyDLL("gdi32.dll")
	createRoundRectRgnProc    = gdi32ProcDLL.NewProc("CreateRoundRectRgn")
	deleteObjectProc          = gdi32ProcDLL.NewProc("DeleteObject")
	dwmapiProcDLL             = syscall.NewLazyDLL("dwmapi.dll")
	dwmSetWindowAttributeProc = dwmapiProcDLL.NewProc("DwmSetWindowAttribute")
)

type winRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

const (
	htCaption                   = 0x0002
	dwmwaWindowCornerPreference = 33
	dwmwcpRound                 = 2
)

func startRoundedWindowStyling(window fyne.Window, radius int32) {
	native, ok := window.(driver.NativeWindow)
	if !ok || radius <= 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(180 * time.Millisecond)
		defer ticker.Stop()

		// GLFW creates the HWND lazily on Show. Never search another process by
		// title, and stop polling once installed or after a bounded startup wait.
		for attempt := 0; attempt < 60; attempt++ {
			<-ticker.C
			var hwnd uintptr
			installed := false
			fyne.DoAndWait(func() {
				native.RunNative(func(context any) {
					if context, ok := context.(driver.WindowsWindowContext); ok {
						hwnd = context.HWND
					}
				})
				if hwnd != 0 {
					installed = installNativeDragFrame(hwnd, radius)
				}
			})
			if hwnd == 0 {
				continue
			}
			if !installed {
				log.Print("Could not install the Windows installer drag area")
			}
			return
		}
	}()
}

var (
	getWindowLongPtrProc = user32ProcDLL.NewProc("GetWindowLongPtrW")
	setWindowLongPtrProc = user32ProcDLL.NewProc("SetWindowLongPtrW")
	callWindowProcProc   = user32ProcDLL.NewProc("CallWindowProcW")
	defWindowProcProc    = user32ProcDLL.NewProc("DefWindowProcW")
	getDpiForWindowProc  = user32ProcDLL.NewProc("GetDpiForWindow")
	isZoomedProc         = user32ProcDLL.NewProc("IsZoomed")
	monitorFromWindow    = user32ProcDLL.NewProc("MonitorFromWindow")
	getMonitorInfoProc   = user32ProcDLL.NewProc("GetMonitorInfoW")
	copyNativeMemory     = syscall.NewLazyDLL("ntdll.dll").NewProc("RtlMoveMemory")
	frameProcedure       = syscall.NewCallback(nativeDragFrameProc)
	frameMu              sync.RWMutex
	frames               = map[uintptr]nativeDragFrame{}
)

type nativeDragFrame struct {
	previous uintptr
	radius   int32
	fallback bool
	width    int32
	height   int32
}

func installNativeDragFrame(hwnd uintptr, radius int32) bool {
	frameMu.RLock()
	_, installed := frames[hwnd]
	frameMu.RUnlock()
	if installed {
		return true
	}
	previous, _, _ := getWindowLongPtrProc.Call(hwnd, ^uintptr(3)) // GWLP_WNDPROC=-4
	if previous == 0 {
		return false
	}
	frameMu.Lock()
	frames[hwnd] = nativeDragFrame{previous: previous, radius: radius}
	frameMu.Unlock()
	if old, _, _ := setWindowLongPtrProc.Call(hwnd, ^uintptr(3), frameProcedure); old == 0 {
		frameMu.Lock()
		delete(frames, hwnd)
		frameMu.Unlock()
		return false
	}
	style, _, _ := getWindowLongPtrProc.Call(hwnd, ^uintptr(15)) // GWL_STYLE=-16
	// Keep WS_CAPTION off. Native resize/maximize styles enable Windows move,
	// snap and resize semantics; WM_NCCALCSIZE below keeps the client frameless.
	setWindowLongPtrProc.Call(hwnd, ^uintptr(15), style|0x00040000|0x00010000)
	setWindowPosProc.Call(hwnd, 0, 0, 0, 0, 0, 0x0001|0x0002|0x0004|0x0010|0x0020)
	if !applyDwmCornerPreference(hwnd) {
		frameMu.Lock()
		frame := frames[hwnd]
		frame.fallback = true
		frames[hwnd] = frame
		frameMu.Unlock()
		applyRoundedRegion(hwnd, radius)
	}
	return true
}

func nativeFrameHit(rect winRect, x, y int32, dpi uint32, maximized bool) uintptr {
	if x < rect.Left || x >= rect.Right || y < rect.Top || y >= rect.Bottom {
		return 0
	}
	if dpi == 0 {
		dpi = 96
	}
	border := int32(math.Ceil(6 * float64(dpi) / 96))
	if !maximized {
		left, right := x < rect.Left+border, x >= rect.Right-border
		top, bottom := y < rect.Top+border, y >= rect.Bottom-border
		switch {
		case top && left:
			return 13 // HTTOPLEFT
		case top && right:
			return 14
		case bottom && left:
			return 16
		case bottom && right:
			return 17
		case left:
			return 10
		case right:
			return 11
		case top:
			return 12
		case bottom:
			return 15
		}
	}
	if y < rect.Top+int32(math.Ceil(float64(windowDragHeight)*float64(dpi)/96)) {
		return htCaption
	}
	return 0 // Keep the original client/control hit result.
}

func nativeDragFrameProc(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	frameMu.RLock()
	frame, ok := frames[hwnd]
	frameMu.RUnlock()
	if !ok {
		result, _, _ := defWindowProcProc.Call(hwnd, uintptr(message), wparam, lparam)
		return result
	}
	if message == 0x0083 && wparam != 0 { // WM_NCCALCSIZE: no native border painting.
		return 0
	}
	if message == 0x0084 { // WM_NCHITTEST receives signed screen coordinates.
		var rect winRect
		if valid, _, _ := getWindowRectProc.Call(hwnd, uintptr(unsafe.Pointer(&rect))); valid != 0 {
			dpi := uintptr(96)
			if getDpiForWindowProc.Find() == nil {
				dpi, _, _ = getDpiForWindowProc.Call(hwnd)
			}
			zoomed, _, _ := isZoomedProc.Call(hwnd)
			hit := nativeFrameHit(rect, int32(int16(lparam&0xffff)), int32(int16((lparam>>16)&0xffff)), uint32(dpi), zoomed != 0)
			if hit != 0 {
				return hit
			}
		}
	}
	result, _, _ := callWindowProcProc.Call(frame.previous, hwnd, uintptr(message), wparam, lparam)
	if message == 0x0005 && frame.fallback { // WM_SIZE: Windows10 rounded-region fallback.
		if zoomed, _, _ := isZoomedProc.Call(hwnd); zoomed != 0 {
			setWindowRgnProc.Call(hwnd, 0, 1)
			frameMu.Lock()
			current := frames[hwnd]
			current.width, current.height = 0, 0
			frames[hwnd] = current
			frameMu.Unlock()
		} else {
			applyRoundedRegion(hwnd, frame.radius)
		}
	}
	if message == 0x0024 && lparam != 0 { // WM_GETMINMAXINFO: maximize within this monitor's work area.
		type point struct{ X, Y int32 }
		type minMaxInfo struct{ Reserved, MaxSize, MaxPosition, MinTrack, MaxTrack point }
		type monitorInfo struct {
			Size          uint32
			Monitor, Work winRect
			Flags         uint32
		}
		monitor, _, _ := monitorFromWindow.Call(hwnd, 2)
		info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
		if valid, _, _ := getMonitorInfoProc.Call(monitor, uintptr(unsafe.Pointer(&info))); valid != 0 {
			var limits minMaxInfo
			copyNativeMemory.Call(uintptr(unsafe.Pointer(&limits)), lparam, unsafe.Sizeof(limits))
			limits.MaxPosition = point{info.Work.Left - info.Monitor.Left, info.Work.Top - info.Monitor.Top}
			limits.MaxSize = point{info.Work.Right - info.Work.Left, info.Work.Bottom - info.Work.Top}
			copyNativeMemory.Call(lparam, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
		}
	}
	if message == 0x0082 { // WM_NCDESTROY: no stale HWND/callback state after close.
		frameMu.Lock()
		delete(frames, hwnd)
		frameMu.Unlock()
	}
	return result
}

func applyDwmCornerPreference(hwnd uintptr) bool {
	preference := uint32(dwmwcpRound)
	result, _, _ := dwmSetWindowAttributeProc.Call(
		hwnd,
		uintptr(dwmwaWindowCornerPreference),
		uintptr(unsafe.Pointer(&preference)),
		unsafe.Sizeof(preference),
	)
	return result == 0
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

	frameMu.Lock()
	frame, exists := frames[hwnd]
	if !exists || (frame.width == width && frame.height == height) {
		frameMu.Unlock()
		return
	}
	// Publish before SetWindowRgn, which can synchronously re-enter WM_SIZE.
	frame.width, frame.height = width, height
	frames[hwnd] = frame
	frameMu.Unlock()

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

}
