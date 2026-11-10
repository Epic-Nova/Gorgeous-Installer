//go:build cgo && windows

package ui

import (
	"syscall"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

type contentCaptureWindow struct {
	fyne.Window
	content fyne.CanvasObject
}

func (w *contentCaptureWindow) SetContent(content fyne.CanvasObject) { w.content = content }

func TestWindowDragAreaReservesSpaceOutsidePanelControls(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	window := &contentCaptureWindow{}
	wrapped := withWindowDragArea(window)
	body := canvas.NewRectangle(gtBg1)
	wrapped.SetContent(body)
	window.content.Resize(fyne.NewSize(920, 560))
	if body.Position().Y < windowDragHeight || body.Position().Y+body.Size().Height > 560 {
		t.Fatalf("drag strip overlaps panel controls: body position=%v size=%v", body.Position(), body.Size())
	}
	// Every content replacement must keep the same reserved drag strip.
	replacement := canvas.NewRectangle(gtBg2)
	wrapped.SetContent(replacement)
	window.content.Resize(fyne.NewSize(920, 560))
	if replacement.Position().Y < windowDragHeight {
		t.Fatal("replacing a panel removed the drag strip")
	}
}

func TestNativeFrameDestroysOnlyItsOwnStateAndChainsOriginalProcedure(t *testing.T) {
	const hwnd = uintptr(0x4711)
	const other = uintptr(0x4712)
	calls := 0
	previous := syscall.NewCallback(func(window, message, wparam, lparam uintptr) uintptr {
		calls++
		return 73
	})
	frameMu.Lock()
	frames[hwnd] = nativeDragFrame{previous: previous}
	frames[other] = nativeDragFrame{previous: previous}
	frameMu.Unlock()
	defer func() { frameMu.Lock(); delete(frames, hwnd); delete(frames, other); frameMu.Unlock() }()
	if result := nativeDragFrameProc(hwnd, 0x0082, 0, 0); result != 73 || calls != 1 {
		t.Fatalf("original window procedure was not chained once: result=%d calls=%d", result, calls)
	}
	frameMu.RLock()
	_, retained := frames[hwnd]
	_, otherRetained := frames[other]
	frameMu.RUnlock()
	if retained || !otherRetained {
		t.Fatalf("destroyed window state retained=%v, unrelated retained=%v", retained, otherRetained)
	}
}

func TestNativeDragHitAreaPreservesControlsAndMonitorCoordinates(t *testing.T) {
	for _, dpi := range []uint32{96, 144, 192} {
		rect := winRect{Left: -1600, Top: -200, Right: -600, Bottom: 500}
		captionY := rect.Top + int32(16*dpi/96)
		if hit := nativeFrameHit(rect, -1100, captionY, dpi, false); hit != htCaption {
			t.Fatalf("dpi%d: custom strip hit=%d, want caption", dpi, hit)
		}
		if hit := nativeFrameHit(rect, -1100, rect.Top+int32(100*dpi/96), dpi, false); hit != 0 {
			t.Fatalf("dpi%d: content intercepted by native frame: %d", dpi, hit)
		}
		if hit := nativeFrameHit(rect, rect.Left+1, rect.Top+1, dpi, false); hit != 13 {
			t.Fatalf("dpi%d: top-left resize hit=%d", dpi, hit)
		}
		if hit := nativeFrameHit(rect, rect.Left-1, rect.Top+50, dpi, false); hit != 0 {
			t.Fatalf("dpi%d: unrelated screen point captured", dpi)
		}
		if hit := nativeFrameHit(rect, rect.Left+1, rect.Top+1, dpi, true); hit != htCaption {
			t.Fatalf("dpi%d: maximized caption should drag-restore, got%d", dpi, hit)
		}
	}
}
