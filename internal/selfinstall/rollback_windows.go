//go:build windows

package selfinstall

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type registryValueState struct {
	exists bool
	kind   uint32
	data   []byte
}

func (a registryValueState) equal(b registryValueState) bool {
	return a.exists == b.exists && (!a.exists || (a.kind == b.kind && bytes.Equal(a.data, b.data)))
}
func readRegistryValue(path, name string) (registryValueState, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err == registry.ErrNotExist {
		return registryValueState{}, nil
	}
	if err != nil {
		return registryValueState{}, err
	}
	defer key.Close()
	for attempt := 0; attempt < 4; attempt++ {
		size, kind, err := key.GetValue(name, nil)
		if err == registry.ErrNotExist {
			return registryValueState{}, nil
		}
		if err != nil && err != registry.ErrShortBuffer {
			return registryValueState{}, err
		}
		data := make([]byte, size)
		size, kind, err = key.GetValue(name, data)
		if err == registry.ErrShortBuffer {
			continue
		}
		if err == registry.ErrNotExist {
			return registryValueState{}, nil
		}
		if err != nil {
			return registryValueState{}, err
		}
		return registryValueState{true, kind, data[:size]}, nil
	}
	return registryValueState{}, fmt.Errorf("registry value kept changing during capture")
}

var setRegistryValueProc = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegSetValueExW")

func restoreRegistryValue(key registry.Key, name string, state registryValueState) error {
	if !state.exists {
		err := key.DeleteValue(name)
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	pointer, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	var data *byte
	if len(state.data) > 0 {
		data = &state.data[0]
	}
	code, _, _ := setRegistryValueProc.Call(uintptr(key), uintptr(unsafe.Pointer(pointer)), 0, uintptr(state.kind), uintptr(unsafe.Pointer(data)), uintptr(len(state.data)))
	runtime.KeepAlive(pointer)
	runtime.KeepAlive(state.data)
	if code != 0 {
		return syscall.Errno(code)
	}
	return nil
}

type shellTransaction struct {
	actions             []func() error
	created             map[string]bool
	stamps              map[string]time.Time
	externalKeys        map[string]bool
	anchorPath          string
	anchor              registryValueState
	externalOwner, done bool
}

func newShellTransaction(uninstallPath string) (*shellTransaction, error) {
	anchor, err := readRegistryValue(uninstallPath, "InstallLocation")
	if err != nil {
		return nil, err
	}
	return &shellTransaction{created: map[string]bool{}, stamps: map[string]time.Time{}, externalKeys: map[string]bool{}, anchorPath: uninstallPath, anchor: anchor}, nil
}
func (t *shellTransaction) mutateFile(path string, global bool, write func() error) error {
	undo, err := mutateOwnedFile(path, write)
	if undo != nil {
		t.actions = append(t.actions, func() error {
			if global && t.externalOwner {
				return nil
			}
			return undo()
		})
	}
	return err
}
func (t *shellTransaction) writeFile(path string, data []byte, mode os.FileMode) error {
	return t.mutateFile(path, false, func() error { return os.WriteFile(path, data, mode) })
}
func (t *shellTransaction) createKey(path string) (registry.Key, error) {
	parts := strings.Split(path, "\\")
	for n := 1; n <= len(parts); n++ {
		current := strings.Join(parts[:n], "\\")
		key, err := registry.OpenKey(registry.CURRENT_USER, current, registry.QUERY_VALUE)
		if err == registry.ErrNotExist {
			t.created[current] = true
			continue
		}
		if err != nil {
			return 0, err
		}
		key.Close()
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	return key, err
}
func (t *shellTransaction) writeRegistry(key registry.Key, path, name string, expected registryValueState, write func() error) error {
	before, err := readRegistryValue(path, name)
	if err != nil {
		return err
	}
	writeErr := write()
	after, readErr := readRegistryValue(path, name)
	if readErr != nil {
		return errors.Join(writeErr, readErr)
	}
	stampKey, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
	if err != nil {
		return errors.Join(writeErr, err)
	}
	info, err := stampKey.Stat()
	stampKey.Close()
	if err != nil {
		return errors.Join(writeErr, err)
	}
	t.stamps[path] = info.ModTime()
	observedNewer := !after.equal(expected) && !after.equal(before)
	if observedNewer {
		t.externalKeys[path] = true
	}
	if path == t.anchorPath && name == "InstallLocation" && after.equal(expected) {
		t.anchor = expected
	}
	t.actions = append(t.actions, func() error {
		if t.externalOwner || observedNewer || before.equal(expected) {
			return nil
		}
		current, err := readRegistryValue(path, name)
		if err != nil {
			return err
		}
		if !current.equal(expected) {
			return nil
		}
		target, err := registry.OpenKey(registry.CURRENT_USER, path, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer target.Close()
		return restoreRegistryValue(target, name, before)
	})
	return writeErr
}
func (t *shellTransaction) setString(key registry.Key, path, name, value string) error {
	units, err := windows.UTF16FromString(value)
	if err != nil {
		return err
	}
	data := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(data[i*2:], u)
	}
	return t.writeRegistry(key, path, name, registryValueState{true, registry.SZ, data}, func() error { return key.SetStringValue(name, value) })
}
func (t *shellTransaction) setDWord(key registry.Key, path, name string, value uint32) error {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, value)
	return t.writeRegistry(key, path, name, registryValueState{true, registry.DWORD, data}, func() error { return key.SetDWordValue(name, value) })
}
func (t *shellTransaction) rollback() error {
	if t.done {
		return nil
	}
	t.done = true
	var failures []error
	anchor, err := readRegistryValue(t.anchorPath, "InstallLocation")
	if err != nil {
		failures = append(failures, err)
		t.externalOwner = true
	} else {
		t.externalOwner = !anchor.equal(t.anchor)
	}
	// Freeze key ownership before any restoration changes its timestamps.
	for path, stamp := range t.stamps {
		key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
		if err != nil {
			t.externalKeys[path] = true
			continue
		}
		info, err := key.Stat()
		key.Close()
		if err != nil || !info.ModTime().Equal(stamp) {
			t.externalKeys[path] = true
		}
	}
	for index := len(t.actions) - 1; index >= 0; index-- {
		if err := t.actions[index](); err != nil {
			failures = append(failures, err)
		}
	}
	paths := make([]string, 0, len(t.created))
	for path := range t.created {
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool { return len(paths[i]) > len(paths[j]) })
	if !t.externalOwner {
		for _, path := range paths {
			if t.externalKeys[path] {
				continue
			}
			key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
			if err == registry.ErrNotExist {
				continue
			}
			if err != nil {
				failures = append(failures, err)
				continue
			}
			info, err := key.Stat()
			key.Close()
			if err != nil {
				failures = append(failures, err)
				continue
			}
			if info.ValueCount == 0 && info.SubKeyCount == 0 {
				if err := removeKey(path); err != nil {
					failures = append(failures, err)
				}
			}
		}
	}
	return errors.Join(failures...)
}
