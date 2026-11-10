//go:build cgo && windows

package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
	"unicode/utf16"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/driver/desktop"
)

// The real GLFW driver must run on the main goroutine. This explicit opt-in
// integration process creates only its own window and uses no desktop input.
// Run: GORGEOUS_NATIVE_DROP_TEST=1 go test ./internal/ui -run '^$' -count=1.
func TestMain(m *testing.M) {
	if os.Getenv("GORGEOUS_NATIVE_DROP_TEST") == "1" {
		if err := runNativeWindowDropProbe(); err != nil {
			fmt.Fprintln(os.Stderr, "NATIVE_WINDOW_DROP_TEST failed:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runNativeWindowDropProbe() error {
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-finished:
		case <-time.After(8 * time.Second):
			fmt.Fprintln(os.Stderr, "NATIVE_WINDOW_DROP_TEST timed out after8seconds")
			os.Exit(2) // Bounded failure even when the native UI loop is blocked.
		}
	}()
	folder, err := os.MkdirTemp(os.Getenv("GORGEOUS_NATIVE_DROP_TESTROOT"), "gorgeous-window-drop-")
	if err != nil {
		return err
	}
	defer os.Remove(folder)
	path := filepath.Join(folder, "Native Drop Ω%.uproject")
	if err = os.WriteFile(path, []byte(`{"EngineAssociation":"5.8"}`), 0600); err != nil {
		return err
	}
	defer os.Remove(path)
	application := app.NewWithID("com.gorgeouscore.native.window.drop.test")
	desktopDriver, ok := application.Driver().(desktop.Driver)
	if !ok {
		return fmt.Errorf("real desktop driver unavailable")
	}
	window := desktopDriver.CreateSplashWindow()
	window.SetTitle("Owned Gorgeous native drop regression")
	window.Resize(fyne.NewSize(400, 160))
	nativeWindow := window
	native := window.(driver.NativeWindow)
	window = withWindowDragArea(window)
	window.SetContent(canvas.NewText("Owned native file-drop regression", gtTextPrimary))
	var selected string
	var rejected error
	configureProjectDrop(window, "content", func() bool { return false }, func(value string) { selected = value }, func(err error) { rejected = err })
	window.Show()
	startRoundedWindowStyling(nativeWindow, 32)
	go func() {
		time.Sleep(350 * time.Millisecond)
		fyne.Do(func() {
			native.RunNative(func(context any) {
				hwnd := context.(driver.WindowsWindowContext).HWND
				frameMu.RLock()
				_, installed := frames[hwnd]
				frameMu.RUnlock()
				if !installed {
					err = fmt.Errorf("native frameless window procedure was not installed")
					return
				}
				err = sendOwnedNativeFileDrop(hwnd, path)
			})
			window.Close()
			application.Quit()
		})
	}()
	application.Run()
	if err != nil {
		return err
	}
	if rejected != nil || selected != path {
		return fmt.Errorf("real WM_DROPFILES did not select exact fixture: selected=%q rejected=%v", selected, rejected)
	}
	result, _ := json.Marshal(map[string]any{"passed": true, "native_driver": "Windows GLFW", "transport": "WM_DROPFILES with receiver-owned HDROP", "selected": selected, "desktop_mouse_or_keyboard_input": false})
	fmt.Println("NATIVE_WINDOW_DROP_TEST", string(result))
	return nil
}

func sendOwnedNativeFileDrop(hwnd uintptr, path string) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	allocate, lock, unlock := kernel32.NewProc("GlobalAlloc"), kernel32.NewProc("GlobalLock"), kernel32.NewProc("GlobalUnlock")
	name := utf16.Encode([]rune(path + "\x00\x00"))
	// DROPFILES uses a20-byte header followed by UTF16 file names.
	header := struct {
		Offset                uint32
		X, Y, NonClient, Wide int32
	}{Offset: 20, Wide: 1}
	handle, _, _ := allocate.Call(0x0042, 20+uintptr(len(name)*2))
	if handle == 0 {
		return fmt.Errorf("GlobalAlloc failed for owned native fixture")
	}
	buffer, _, _ := lock.Call(handle)
	if buffer == 0 {
		kernel32.NewProc("GlobalFree").Call(handle)
		return fmt.Errorf("GlobalLock failed for owned native fixture")
	}
	copyNativeMemory.Call(buffer, uintptr(unsafe.Pointer(&header)), unsafe.Sizeof(header))
	copyNativeMemory.Call(buffer+20, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)*2))
	unlock.Call(handle)
	// The real GLFW WM_DROPFILES handler calls DragFinish, owning the handle.
	user32ProcDLL.NewProc("SendMessageW").Call(hwnd, 0x0233, handle, 0)
	return nil
}
