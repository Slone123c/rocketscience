//go:build darwin

package core

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"rocketscience/internal/tunhelper"
)

// tunDevice is a utun device rs-tun-helper created as root. The app keeps the
// original descriptor and gives mihomo duplicates, so reloading mihomo's TUN
// listener never destroys the device. Closing the connection lets the helper
// clean up and exit; closing the last descriptor removes the device and its
// routes. Both also happen when the app exits or crashes.
type tunDevice struct {
	file   *os.File
	name   string
	conn   *net.UnixConn
	cmd    *exec.Cmd
	exited chan struct{}
	dir    string
}

func tunHelperPath() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	for _, path := range []string{filepath.Join(filepath.Dir(executable), "..", "Resources", "bin", "rs-tun-helper"), filepath.Join("resources", "bin", "rs-tun-helper"), filepath.Join("..", "..", "resources", "bin", "rs-tun-helper")} {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return filepath.Abs(path)
		}
	}
	return "", errors.New("未找到 TUN 辅助程序，请重新构建应用")
}

func shellArgument(value string) (string, error) {
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", errors.New("路径包含不支持的控制字符")
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'", nil
}

// tunHelperError is the helper's own answer, such as an invalid route. It is
// returned as is; other failures mean the service could not be reached.
type tunHelperError struct{ message string }

func (e tunHelperError) Error() string { return "TUN 启动失败：" + e.message }

// openTunDevice gets a device from the installed service. When the service is
// missing, outdated or refuses this user, it installs it with one
// administrator prompt; if the service still cannot be reached, it falls back
// to a one-shot helper, which asks again.
func openTunDevice(ctx context.Context, request tunhelper.Request) (*tunDevice, error) {
	helper, err := tunHelperPath()
	if err != nil {
		return nil, err
	}
	if tunServiceCurrent(helper) {
		device, err := dialTunService(ctx, request, 0)
		if err == nil || errors.As(err, new(tunHelperError)) || ctx.Err() != nil {
			return device, err
		}
	}
	if err := runPrivileged(ctx, helper, "install", strconv.Itoa(os.Getuid())); err != nil {
		if ctx.Err() != nil {
			return nil, errors.New("内核启动已取消")
		}
		return nil, fmt.Errorf("TUN 服务安装失败或授权已取消：%w", err)
	}
	// launchd starts the daemon right away; give it a moment to listen.
	device, err := dialTunService(ctx, request, 3*time.Second)
	if err == nil || errors.As(err, new(tunHelperError)) || ctx.Err() != nil {
		return device, err
	}
	return openTunOnce(ctx, helper, request)
}

// tunServiceCurrent reports whether the installed service is this app's helper.
func tunServiceCurrent(helper string) bool {
	if _, err := os.Stat(tunhelper.ServicePlist); err != nil {
		return false
	}
	return sameFile(helper, tunhelper.ServicePath)
}

func sameFile(a, b string) bool {
	first, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	second, err := os.ReadFile(b)
	return err == nil && bytes.Equal(first, second)
}

func dialTunService(ctx context.Context, request tunhelper.Request, wait time.Duration) (*tunDevice, error) {
	deadline := time.Now().Add(wait)
	for {
		conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: tunhelper.ServiceSocket, Net: "unix"})
		if err == nil {
			device := &tunDevice{conn: conn}
			if err = device.handshake(ctx, request); err != nil {
				device.Close()
				return nil, err
			}
			return device, nil
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, errors.New("内核启动已取消")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// runPrivileged runs the helper as root through the system password dialog.
func runPrivileged(ctx context.Context, helper string, args ...string) error {
	arguments := append([]string{helper}, args...)
	for i, argument := range arguments {
		quoted, err := shellArgument(argument)
		if err != nil {
			return err
		}
		arguments[i] = quoted
	}
	script := "do shell script " + strconv.Quote(strings.Join(arguments, " ")) + " with administrator privileges"
	output, err := exec.CommandContext(ctx, "/usr/bin/osascript", "-e", script).CombinedOutput()
	if err != nil {
		if message := strings.TrimSpace(string(output)); message != "" {
			return errors.New(message)
		}
		return err
	}
	return nil
}

// openTunOnce asks for administrator privileges for this session only and
// returns once the helper has handed over the device, the user cancels, or ctx
// ends.
func openTunOnce(ctx context.Context, helper string, request tunhelper.Request) (*tunDevice, error) {
	var err error
	// Unix socket paths are limited to 104 bytes, so keep it short under TMPDIR.
	dir, err := os.MkdirTemp("", "rs-tun")
	if err != nil {
		return nil, err
	}
	socket := filepath.Join(dir, "s")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	defer listener.Close()
	listener.SetUnlinkOnClose(true)
	arguments := []string{helper, socket, strconv.Itoa(os.Getuid())}
	for i, argument := range arguments {
		if arguments[i], err = shellArgument(argument); err != nil {
			os.RemoveAll(dir)
			return nil, err
		}
	}
	script := "do shell script " + strconv.Quote(strings.Join(arguments, " ")) + " with administrator privileges"
	cmd := exec.Command("/usr/bin/osascript", "-e", script)
	if err = cmd.Start(); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("无法请求 TUN 授权：%w", err)
	}
	device := &tunDevice{cmd: cmd, exited: make(chan struct{}), dir: dir}
	var waitErr error
	go func() { waitErr = cmd.Wait(); close(device.exited) }()
	accepted := make(chan *net.UnixConn, 1)
	go func() {
		if conn, err := listener.AcceptUnix(); err == nil {
			accepted <- conn
		}
	}()
	select {
	case device.conn = <-accepted:
	case <-device.exited:
		device.Close()
		return nil, fmt.Errorf("TUN 授权已取消或辅助程序未能启动：%v", waitErr)
	case <-ctx.Done():
		device.Close()
		return nil, errors.New("内核启动已取消")
	}
	if err = device.handshake(ctx, request); err != nil {
		device.Close()
		return nil, err
	}
	return device, nil
}

func (d *tunDevice) handshake(ctx context.Context, request tunhelper.Request) error {
	if err := checkHelperPeer(d.conn); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = d.conn.SetDeadline(time.Now()) })
	defer stop()
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	if _, err = d.conn.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("无法发送虚拟网卡参数：%w", err)
	}
	buffer := make([]byte, 4096)
	oob := make([]byte, unix.CmsgSpace(4))
	n, oobn, _, _, err := d.conn.ReadMsgUnix(buffer, oob)
	if err != nil {
		if ctx.Err() != nil {
			return errors.New("内核启动已取消")
		}
		return fmt.Errorf("TUN 辅助程序没有响应：%w", err)
	}
	var fds []int
	if messages, err := unix.ParseSocketControlMessage(oob[:oobn]); err == nil {
		for _, message := range messages {
			if rights, err := unix.ParseUnixRights(&message); err == nil {
				fds = append(fds, rights...)
			}
		}
	}
	var response tunhelper.Response
	line, _, _ := bufio.NewReader(strings.NewReader(string(buffer[:n]))).ReadLine()
	if err := json.Unmarshal(line, &response); err != nil || response.Error != "" || len(fds) != 1 {
		for _, fd := range fds {
			unix.Close(fd)
		}
		if response.Error != "" {
			return tunHelperError{response.Error}
		}
		return errors.New("TUN 辅助程序返回了无效的响应")
	}
	d.file, d.name = os.NewFile(uintptr(fds[0]), response.Name), response.Name
	return nil
}

// checkHelperPeer makes sure the descriptor really comes from a root process.
func checkHelperPeer(conn *net.UnixConn) error {
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var cred *unix.Xucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
	}); err != nil {
		return err
	}
	if credErr != nil {
		return credErr
	}
	if cred.Uid != 0 {
		return errors.New("TUN 辅助程序未以管理员权限运行")
	}
	return nil
}

// dup returns a descriptor mihomo may own and close.
func (d *tunDevice) dup() (int, error) {
	fd, err := unix.Dup(int(d.file.Fd()))
	if err != nil {
		return -1, fmt.Errorf("无法复制虚拟网卡描述符：%w", err)
	}
	return fd, nil
}

// Close releases the device and lets the helper exit. Call it after mihomo's
// TUN listener has stopped.
func (d *tunDevice) Close() {
	if d == nil {
		return
	}
	if d.file != nil {
		_ = d.file.Close()
	}
	if d.conn != nil {
		_ = d.conn.Close()
	}
	if d.cmd == nil {
		return
	}
	if d.conn == nil {
		// Still waiting for the password: stop the authorization dialog.
		_ = d.cmd.Process.Kill()
	}
	select {
	case <-d.exited:
	case <-time.After(3 * time.Second):
		_ = d.cmd.Process.Kill()
		<-d.exited
	}
	_ = os.RemoveAll(d.dir)
}

func tunServiceInstalled() bool {
	_, err := os.Stat(tunhelper.ServicePlist)
	return err == nil
}

// uninstallTunService runs the installed copy, which removes itself.
func uninstallTunService() error {
	if err := runPrivileged(context.Background(), tunhelper.ServicePath, "uninstall"); err != nil {
		return fmt.Errorf("卸载 TUN 服务失败或授权已取消：%w", err)
	}
	return nil
}
