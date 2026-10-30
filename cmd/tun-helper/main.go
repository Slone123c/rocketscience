//go:build darwin

// rs-tun-helper creates utun devices as root for the app. It is normally
// installed once as a launchd daemon ("serve"); "install" and "uninstall" run
// with administrator privileges from the app, and the one-shot form serves a
// single session when the daemon is unavailable. See internal/tunhelper.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	tun "github.com/metacubex/sing-tun"
	"golang.org/x/sys/unix"
	"rocketscience/internal/tunhelper"
)

func main() {
	var err error
	switch {
	case len(os.Args) == 3 && os.Args[1] == "serve":
		err = serve(os.Args[2])
	case len(os.Args) == 3 && os.Args[1] == "install":
		err = install(os.Args[2])
	case len(os.Args) == 2 && os.Args[1] == "uninstall":
		err = uninstall()
	case len(os.Args) == 3:
		err = once(os.Args[1], os.Args[2])
	default:
		fmt.Fprintln(os.Stderr, "usage: rs-tun-helper serve|install <uid> | uninstall | <socket> <uid>")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseUID(value string) (uint32, error) {
	uid, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, errors.New("invalid uid")
	}
	return uint32(uid), nil
}

// once serves one session over a socket the app created.
func once(socket, owner string) error {
	uid, err := parseUID(owner)
	if err != nil {
		return err
	}
	// Only talk to a socket the requesting user created, so another local user
	// cannot have root build a device to their specification.
	var info unix.Stat_t
	if err := unix.Lstat(socket, &info); err != nil {
		return err
	}
	if info.Mode&unix.S_IFMT != unix.S_IFSOCK || info.Uid != uid {
		return errors.New("socket is not owned by the requesting user")
	}
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return err
	}
	return session(conn, uid)
}

// opening serializes device creation, so two sessions never pick the same
// utun unit or find each other's descriptor.
var opening sync.Mutex

// session creates one device for the app on conn and keeps it until the app
// hangs up: when it stops TUN, quits or crashes.
func session(conn *net.UnixConn, uid uint32) error {
	defer conn.Close()
	if err := checkPeer(conn, uid); err != nil {
		return err
	}
	line, err := bufio.NewReader(io.LimitReader(conn, 1<<20)).ReadBytes('\n')
	if err != nil {
		return err
	}
	var request tunhelper.Request
	if err := json.Unmarshal(line, &request); err != nil {
		return reply(conn, tunhelper.Response{Error: "无法读取虚拟网卡参数"}, -1)
	}
	opening.Lock()
	device, name, err := open(request)
	fd := -1
	if err == nil {
		fd, err = descriptorOf(name)
		if err != nil {
			device.Close()
		} else if err = prepare(fd, request); err != nil {
			device.Close()
			fd = -1
		}
	}
	opening.Unlock()
	if err != nil {
		return reply(conn, tunhelper.Response{Error: err.Error()}, -1)
	}
	defer device.Close()
	if err := reply(conn, tunhelper.Response{Name: name}, fd); err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, conn)
	return nil
}

func checkPeer(conn *net.UnixConn, uid uint32) error {
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
	if cred.Uid != uid {
		return errors.New("socket peer is not the requesting user")
	}
	return nil
}

func open(request tunhelper.Request) (tun.Tun, string, error) {
	if err := request.Validate(); err != nil {
		return nil, "", err
	}
	name := request.Name
	if name == "" {
		name = freeName()
	}
	inet4, _ := tunhelper.ParsePrefixes(request.Inet4Address)
	inet6, _ := tunhelper.ParsePrefixes(request.Inet6Address)
	routes, _ := tunhelper.ParsePrefixes(request.RouteAddress)
	excludes, _ := tunhelper.ParsePrefixes(request.RouteExclude)
	options := tun.Options{
		Name:         name,
		MTU:          request.MTU,
		Inet4Address: inet4,
		Inet6Address: inet6,
		AutoRoute:    request.AutoRoute,
		DNSMode:      request.DNSMode,
		EXP_RecvMsgX: request.RecvMsgX,
	}
	for _, prefix := range routes {
		if prefix.Addr().Is4() {
			options.Inet4RouteAddress = append(options.Inet4RouteAddress, prefix)
		} else {
			options.Inet6RouteAddress = append(options.Inet6RouteAddress, prefix)
		}
	}
	for _, prefix := range excludes {
		if prefix.Addr().Is4() {
			options.Inet4RouteExcludeAddress = append(options.Inet4RouteExcludeAddress, prefix)
		} else {
			options.Inet6RouteExcludeAddress = append(options.Inet6RouteExcludeAddress, prefix)
		}
	}
	device, err := tun.New(options)
	if errors.Is(err, unix.EEXIST) {
		return nil, "", fmt.Errorf("路由已被其他 VPN 或代理应用的 TUN 占用，请先关闭它：%w", err)
	}
	if err != nil {
		return nil, "", fmt.Errorf("虚拟网卡或路由无法创建：%w", err)
	}
	return device, name, nil
}

// prepare finishes the control socket before handing it over. sing-tun applies
// the same options again when it is given a descriptor, and that second
// setsockopt fails on a socket this process created, which makes mihomo drop
// the listener. Doing it here, in the creating process, leaves the descriptor
// ready.
func prepare(fd int, request tunhelper.Request) error {
	const utunOptMaxPendingPackets = 16
	if err := unix.SetNonblock(fd, true); err != nil {
		return fmt.Errorf("虚拟网卡无法设为非阻塞：%w", err)
	}
	if !request.RecvMsgX {
		return nil
	}
	receive := 8 << 20
	for receive >= 1<<20 {
		if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF, receive); err == nil {
			break
		}
		receive /= 2
	}
	current, err := unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_RCVBUF)
	if err != nil || current <= 0 {
		current = 512 << 10
	}
	pending := current / 8 * 7 / (int(request.MTU) + 4)
	if pending > 64 {
		pending = 64
	}
	if pending < 1 {
		pending = 1
	}
	if err := unix.SetsockoptInt(fd, 2 /* SYSPROTO_CONTROL */, utunOptMaxPendingPackets, pending); err != nil {
		return fmt.Errorf("虚拟网卡无法开启批量接收：%w", err)
	}
	return nil
}

// descriptorOf finds the control socket sing-tun opened for the device, by
// asking each open descriptor for its utun name (UTUN_OPT_IFNAME), the same
// query mihomo uses to name a descriptor it is given.
func descriptorOf(name string) (int, error) {
	for fd := 3; fd < 1024; fd++ {
		if got, err := unix.GetsockoptString(fd, 2 /* SYSPROTO_CONTROL */, 2 /* UTUN_OPT_IFNAME */); err == nil && got == name {
			return fd, nil
		}
	}
	return -1, errors.New("找不到虚拟网卡的描述符")
}

// freeName picks the utun unit after the highest one in use, as mihomo does.
func freeName() string {
	next := 0
	interfaces, _ := net.Interfaces()
	for _, item := range interfaces {
		if index, err := strconv.Atoi(strings.TrimPrefix(item.Name, "utun")); err == nil && strings.HasPrefix(item.Name, "utun") && index >= next {
			next = index + 1
		}
	}
	return "utun" + strconv.Itoa(next)
}

func reply(conn *net.UnixConn, response tunhelper.Response, fd int) error {
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	var rights []byte
	if fd >= 0 {
		rights = unix.UnixRights(fd)
	}
	if _, _, err := conn.WriteMsgUnix(data, rights, nil); err != nil {
		return err
	}
	if response.Error != "" {
		return errors.New(response.Error)
	}
	return nil
}
