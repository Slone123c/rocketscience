//go:build !darwin && !windows && !linux

package systemproxy

func automaticAvailablePlatform() bool { return false }

func manualFallbackNote() string {
	return "当前平台不支持自动系统代理。请启动内核后使用应用提供的终端代理命令。"
}
