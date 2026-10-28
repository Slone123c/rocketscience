// tun-supervisor keeps a privileged mihomo process tied to the owning app.
// It is launched only after macOS administrator authorization, not installed setuid.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func clearQuarantineAttr(path string) {
	if path == "" {
		return
	}
	p, err := syscall.BytePtrFromString(path)
	if err != nil {
		return
	}
	attr, err := syscall.BytePtrFromString("com.apple.quarantine")
	if err != nil {
		return
	}
	_, _, _ = syscall.Syscall(syscall.SYS_REMOVEXATTR, uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(attr)), 0)
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

// Only the fixed GEO files created by the privileged child are returned to the
// app owner; the descriptor and O_NOFOLLOW prevent following substituted links.
func returnGeoOwnership(dir string, uid int) {
	if os.Geteuid() != 0 {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	allowed := map[string]bool{"country.mmdb": true, "geoip.db": true, "geoip.metadb": true, "geoip.dat": true, "geosite.dat": true, "asn.mmdb": true, "cache.db": true}
	for _, entry := range entries {
		if !allowed[strings.ToLower(entry.Name())] {
			continue
		}
		fd, err := syscall.Open(filepath.Join(dir, entry.Name()), syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
		if err != nil {
			continue
		}
		file := os.NewFile(uintptr(fd), entry.Name())
		info, err := file.Stat()
		if err == nil && info.Mode().IsRegular() {
			if st, ok := info.Sys().(*syscall.Stat_t); ok && (st.Uid == 0 || int(st.Uid) == uid) {
				_ = file.Chown(uid, -1)
				_ = file.Chmod(0600)
			}
		}
		_ = file.Close()
	}
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fail(err)
	}
}

func run(args []string) error {
	if len(args) != 3 {
		return errors.New("invalid supervisor arguments")
	}
	core, dir, parentUID := args[0], args[1], args[2]
	uid, err := strconv.Atoi(parentUID)
	if err != nil || uid <= 0 {
		return errors.New("invalid owner")
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != uid || !info.IsDir() || info.Mode().Perm() != 0700 {
		return errors.New("unsafe runtime directory")
	}
	core, err = filepath.EvalSymlinks(core)
	if err != nil {
		return err
	}
	coreInfo, err := os.Stat(core)
	if err != nil || !coreInfo.Mode().IsRegular() {
		return errors.New("invalid core executable")
	}
	config := filepath.Join(dir, "config.yaml")
	heartbeat := filepath.Join(dir, "heartbeat")
	logPath := filepath.Join(dir, "tun-core.log")
	pidPath := filepath.Join(dir, "core.pid")
	for _, path := range []string{config, heartbeat, logPath, pidPath} {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("invalid runtime file: %s", filepath.Base(path))
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || int(stat.Uid) != uid || info.Mode().Perm()&0077 != 0 {
			return errors.New("unsafe runtime file owner or mode")
		}
	}
	fd, err := syscall.Open(logPath, syscall.O_WRONLY|syscall.O_APPEND|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	output := os.NewFile(uintptr(fd), "tun-log")
	defer output.Close()
	clearQuarantineAttr(core)
	cmd := exec.Command(core, "-d", dir, "-f", config)
	cmd.Stdout = output
	cmd.Stderr = output
	if os.Geteuid() != uid {
		cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + dir}
	}
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("start core: %w", err)
	}
	pidFD, err := syscall.Open(pidPath, syscall.O_WRONLY|syscall.O_TRUNC|syscall.O_NOFOLLOW, 0)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	pidFile := os.NewFile(uintptr(pidFD), "core-pid")
	if _, err = fmt.Fprintln(pidFile, cmd.Process.Pid); err != nil {
		_ = pidFile.Close()
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	if err = pidFile.Close(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigCh)
	stopCore := func() error {
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case <-done:
			returnGeoOwnership(dir, uid)
			return nil
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			returnGeoOwnership(dir, uid)
			return nil
		}
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var lastModTime time.Time
	staleTicks := 0
	for {
		select {
		case err = <-done:
			returnGeoOwnership(dir, uid)
			return err
		case <-sigCh:
			return stopCore()
		case <-ticker.C:
			returnGeoOwnership(dir, uid)
			state, readErr := os.ReadFile(heartbeat)
			info, statErr := os.Lstat(heartbeat)
			stopRequested := readErr != nil || statErr != nil || !info.Mode().IsRegular() || strings.TrimSpace(string(state)) == "stop"
			if !stopRequested {
				if time.Since(info.ModTime()) > 8*time.Second && info.ModTime().Equal(lastModTime) {
					staleTicks++
				} else {
					staleTicks = 0
				}
				lastModTime = info.ModTime()
			}
			if stopRequested || staleTicks >= 4 {
				return stopCore()
			}
		}
	}
}
