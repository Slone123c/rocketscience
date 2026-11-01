//go:build darwin

package network

import (
	"encoding/json"
	"errors"
	"github.com/ebitengine/purego"
	"os"
	"path/filepath"
	"sync"
	"unsafe"
)

var once sync.Once
var loadErr error
var snapshotFn func() unsafe.Pointer
var freeFn func(unsafe.Pointer)
var authorizeFn func()
var appIconFn func(string) unsafe.Pointer
var traySpeedsFn func(string, string) unsafe.Pointer

func load() error {
	once.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			loadErr = err
			return
		}
		candidates := []string{filepath.Join(filepath.Dir(exe), "..", "Resources", "bin", "libRocketNetwork.dylib"), "resources/bin/libRocketNetwork.dylib", "../../resources/bin/libRocketNetwork.dylib"}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err != nil {
				continue
			}
			library, err := purego.Dlopen(candidate, purego.RTLD_NOW|purego.RTLD_LOCAL)
			if err != nil {
				loadErr = err
				return
			}
			purego.RegisterLibFunc(&snapshotFn, library, "rs_wifi_snapshot")
			purego.RegisterLibFunc(&freeFn, library, "rs_wifi_free")
			purego.RegisterLibFunc(&authorizeFn, library, "rs_wifi_request_access")
			purego.RegisterLibFunc(&appIconFn, library, "rs_app_icon")
			purego.RegisterLibFunc(&traySpeedsFn, library, "rs_tray_speeds")
			return
		}
		loadErr = errors.New("找不到 Wi-Fi 辅助模块，请重新构建应用")
	})
	return loadErr
}

// Read and RequestAccess must be dispatched to the host application's main thread.
func Read() (Snapshot, error) {
	if err := load(); err != nil {
		return Snapshot{}, err
	}
	pointer := snapshotFn()
	if pointer == nil {
		return Snapshot{}, errors.New("无法读取网络状态")
	}
	var value Snapshot
	if err := decode(pointer, 65536, &value); err != nil {
		return Snapshot{}, errors.New("网络状态响应过大")
	}
	return value, nil
}

// decode reads a JSON C string the native module allocated, then frees it.
func decode(pointer unsafe.Pointer, limit int, value any) error {
	defer freeFn(pointer)
	data := []byte{}
	for i := 0; i < limit; i++ {
		b := *(*byte)(unsafe.Add(pointer, i))
		if b == 0 {
			return json.Unmarshal(data, value)
		}
		data = append(data, b)
	}
	return errors.New("native response too large")
}
func RequestAccess() error {
	if err := load(); err != nil {
		return err
	}
	authorizeFn()
	return nil
}
