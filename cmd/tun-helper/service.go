//go:build darwin

package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"rocketscience/internal/tunhelper"
)

// maxSessions bounds the devices one daemon keeps; the app needs one.
const maxSessions = 8

// serve is the launchd daemon: it accepts sessions from the installing user.
func serve(owner string) error {
	uid, err := parseUID(owner)
	if err != nil {
		return err
	}
	// /var/run is writable only by root, so nobody else can plant this socket.
	_ = os.Remove(tunhelper.ServiceSocket)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: tunhelper.ServiceSocket, Net: "unix"})
	if err != nil {
		return err
	}
	defer listener.Close()
	// Anyone may connect; session admits only uid (LOCAL_PEERCRED).
	if err := os.Chmod(tunhelper.ServiceSocket, 0o666); err != nil {
		return err
	}
	slots := make(chan struct{}, maxSessions)
	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			return err
		}
		select {
		case slots <- struct{}{}:
		default:
			conn.Close()
			continue
		}
		go func() {
			defer func() { <-slots }()
			if err := session(conn, uid); err != nil {
				log.Println(err)
			}
		}()
	}
}

func plist(uid uint32) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + tunhelper.ServiceLabel + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + tunhelper.ServicePath + `</string>
		<string>serve</string>
		<string>` + strconv.FormatUint(uint64(uid), 10) + `</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardErrorPath</key>
	<string>` + tunhelper.ServiceLog + `</string>
</dict>
</plist>
`)
}

// writeRootFile replaces path atomically with a root-owned file.
func writeRootFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp := path + ".tmp"
	_ = os.Remove(temp)
	if err := os.WriteFile(temp, data, mode); err != nil {
		return err
	}
	if err := os.Chown(temp, 0, 0); err != nil {
		os.Remove(temp)
		return err
	}
	if err := os.Chmod(temp, mode); err != nil {
		os.Remove(temp)
		return err
	}
	return os.Rename(temp, path)
}

// install copies this binary out of the app bundle and (re)starts the daemon
// for uid. It runs as root through the app's one administrator prompt.
func install(owner string) error {
	uid, err := parseUID(owner)
	if err != nil {
		return err
	}
	if os.Geteuid() != 0 {
		return errors.New("install requires root")
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	binary, err := os.ReadFile(self)
	if err != nil {
		return err
	}
	_ = launchctl("bootout", "system/"+tunhelper.ServiceLabel)
	if err := writeRootFile(tunhelper.ServicePath, binary, 0o755); err != nil {
		return err
	}
	if err := writeRootFile(tunhelper.ServicePlist, plist(uid), 0o644); err != nil {
		return err
	}
	return launchctl("bootstrap", "system", tunhelper.ServicePlist)
}

func uninstall() error {
	if os.Geteuid() != 0 {
		return errors.New("uninstall requires root")
	}
	_ = launchctl("bootout", "system/"+tunhelper.ServiceLabel)
	var failed []error
	for _, path := range []string{tunhelper.ServicePlist, tunhelper.ServicePath, tunhelper.ServiceSocket} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			failed = append(failed, err)
		}
	}
	return errors.Join(failed...)
}

func launchctl(args ...string) error {
	var output bytes.Buffer
	cmd := exec.Command("/bin/launchctl", args...)
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("launchctl %s: %v %s", args[0], err, bytes.TrimSpace(output.Bytes()))
	}
	return nil
}
