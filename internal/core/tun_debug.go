package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	mihomoLog "github.com/metacubex/mihomo/log"
)

// The TUN debug log records each step of a TUN session in
// ~/Library/Logs/RocketScience/tun.log: the device requested, how the helper
// was reached, what mihomo made of it and the self-check after start. Lines
// also go to mihomo's log, so the app's log page shows them.

var tunDebugMu sync.Mutex

const tunDebugMaxSize = 1 << 20

func tunDebugPath() string {
	if path := os.Getenv("ROCKETSCIENCE_TUN_LOG"); path != "" {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "rocketscience-tun.log")
	}
	return filepath.Join(home, "Library", "Logs", "RocketScience", "tun.log")
}

func tunDebugf(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	mihomoLog.Infoln("[TUN 调试] %s", message)
	tunDebugMu.Lock()
	defer tunDebugMu.Unlock()
	path := tunDebugPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	if info, err := os.Stat(path); err == nil && info.Size() > tunDebugMaxSize {
		_ = os.Rename(path, path+".1")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, "%s %s\n", time.Now().Format("2006-01-02 15:04:05.000"), message)
}

// TunDebugLog returns the end of the TUN debug log and the helper service's
// log, ready to paste into a bug report.
func (s *Service) TunDebugLog() string {
	var b strings.Builder
	for _, path := range []string{tunDebugPath(), tunServiceLogPath()} {
		if path == "" {
			continue
		}
		fmt.Fprintf(&b, "==> %s <==\n", path)
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(&b, "（无法读取：%v）\n\n", err)
			continue
		}
		lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		if len(lines) > 300 {
			lines = lines[len(lines)-300:]
		}
		b.WriteString(strings.Join(lines, "\n"))
		b.WriteString("\n\n")
	}
	return b.String()
}
