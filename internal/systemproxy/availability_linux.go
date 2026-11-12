//go:build linux

package systemproxy

func automaticAvailable() bool {
	_, err := LinuxBackend{}.store()
	return err == nil
}

func manualFallbackNote() string {
	if automaticAvailable() {
		return ""
	}
	return "当前 Linux 桌面不支持自动写入系统代理（本版本支持 GNOME 的 gsettings）。请先启动内核，再复制「终端代理命令」到终端；或在桌面环境的网络设置中手动填写 127.0.0.1 与本机混合端口。"
}
