package core

import (
	"io"
	"net"
	"os"
	"strings"
)

func activeTunInterfaces() map[string]bool {
	result := map[string]bool{}
	interfaces, err := net.Interfaces()
	if err != nil {
		return result
	}
	for _, item := range interfaces {
		if strings.HasPrefix(item.Name, "utun") {
			result[item.Name] = true
		}
	}
	return result
}
func hasNewTunInterface(before map[string]bool) bool {
	for name := range activeTunInterfaces() {
		if !before[name] {
			return true
		}
	}
	return false
}

func tunLogFailure(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() > 65536 {
		_, _ = f.Seek(-65536, io.SeekEnd)
	}
	data, err := io.ReadAll(io.LimitReader(f, 65536))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(strings.ToLower(line), "start tun listening error") {
			return "虚拟网卡或路由无法创建，请检查系统授权及网络配置"
		}
	}
	return ""
}

func coreLogSummary(path string) string {
	if failure := tunLogFailure(path); failure != "" {
		return failure
	}
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() > 8192 {
		_, _ = f.Seek(-8192, io.SeekEnd)
	}
	data, err := io.ReadAll(io.LimitReader(f, 8192))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && line != "running" {
			return line
		}
	}
	return ""
}
