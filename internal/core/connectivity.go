package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/tunnel"
)

type Connectivity struct {
	Local           bool                 `json:"local"`
	LocalStatus     string               `json:"localStatus"`
	LocalDetail     string               `json:"localDetail"`
	Core            bool                 `json:"core"`
	TakeoverStatus  string               `json:"takeoverStatus"`
	TakeoverDetail  string               `json:"takeoverDetail"`
	AutoExited      bool                 `json:"autoExited"`
	Request         bool                 `json:"request"`
	Message         string               `json:"message"`
	Step            string               `json:"step,omitempty"`
	NodeStatus      string               `json:"nodeStatus"`
	NodeDetail      string               `json:"nodeDetail"`
	VerifiedNode    string               `json:"verifiedNode,omitempty"`
	Switched        bool                 `json:"switched,omitempty"`
	Recommendations []NodeRecommendation `json:"recommendations"`
}

func reportCheckStep(ctx context.Context, result *Connectivity, step string) {
	result.Step = step
	if win := mygo.CallerWindow(ctx); win != nil {
		_ = ConnectivityProgress.Emit(win, *result)
	}
}

// ConnectivityProgress is emitted after each finished step of a check.
var ConnectivityProgress = mygo.NewEvent[Connectivity]("connectivity-progress")

// NodeRecommendation is an observation or applied node switch.
type NodeRecommendation struct {
	Group   string `json:"group"`
	Current string `json:"current"`
	Node    string `json:"node"`
	Delay   int    `json:"delay"`
}

var domesticProbeURLs = []string{
	"http://connectivitycheck.platform.hicloud.com/generate_204",
	"http://captive.apple.com/hotspot-detect.html",
	"https://www.baidu.com",
}

// CheckConnectivity checks in order from the no-proxy state to the proxy-enabled state:
// 1. Whether the local machine has internet without proxy.
// 2. Whether enabling proxy causes a disconnect (if so, automatically exits proxy).
// 3. Whether proxy connects via at least one working node (auto-switching if the current node fails).
func (s *Service) CheckConnectivity(ctx context.Context, target string) (result Connectivity, resultErr error) {
	if pending := s.startupStatus.Load(); pending != nil {
		return Connectivity{}, fmt.Errorf("内核正在启动（%s），请等待就绪后再检查网络", pending.StartupStage)
	}
	u, err := url.Parse(target)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return Connectivity{}, errors.New("检查地址必须为 HTTP 或 HTTPS")
	}

	s.mu.Lock()
	wasRunning := s.cmd != nil
	savedPort := s.mixedPort
	savedContent := s.currentContent
	savedBypass := cloneBypass(s.proxyBypass)
	savedLAN := s.lan
	savedOptions := s.coreOptions
	startActive := s.startActive
	var pendingRecovery bool
	if s.proxy != nil {
		pendingRecovery, _ = s.proxy.Pending()
	}
	s.mu.Unlock()

	if wasRunning || pendingRecovery {
		reportCheckStep(ctx, &result, "正在关闭代理并恢复网络")
		if err := s.RecoverNetwork(); err != nil {
			return Connectivity{
				LocalStatus:     "skip",
				TakeoverStatus:  "fail",
				TakeoverDetail:  "关闭代理失败：" + err.Error(),
				NodeStatus:      "skip",
				Message:         "关闭代理失败：" + err.Error(),
				Recommendations: []NodeRecommendation{},
			}, nil
		}
	}

	// Step 1: Check whether the local machine has internet in the no-proxy state.
	reportCheckStep(ctx, &result, "正在直连检查本机网络")
	if !probeLocalDirect(target) {
		return Connectivity{
			Local:           false,
			LocalStatus:     "fail",
			LocalDetail:     "无代理状态下无法连接互联网，请检查本机网络",
			Core:            false,
			TakeoverStatus:  "skip",
			NodeStatus:      "skip",
			Message:         "本机未连接互联网",
			Recommendations: []NodeRecommendation{},
		}, nil
	}

	result = Connectivity{
		Local:           true,
		LocalStatus:     "ok",
		LocalDetail:     "无代理状态下本机网络正常",
		Recommendations: []NodeRecommendation{},
	}
	reportCheckStep(ctx, &result, "本机网络正常，正在启动内核")

	// Step 2 (Part A): Transition from no-proxy state to proxy-enabled state.
	reportCheckStep(ctx, &result, "正在启动内核")
	var startErr error
	switch {
	case wasRunning || (savedContent != "" && savedPort > 0):
		startErr = s.startForCheck(ctx, savedContent, savedPort, savedBypass, savedLAN, savedOptions)
	case startActive != nil:
		startErr = startActive()
	default:
		result.Core = false
		result.TakeoverStatus = "fail"
		result.TakeoverDetail = "内核未启动"
		result.Message = "内核未启动"
		return result, nil
	}
	if startErr != nil {
		_ = s.RecoverNetwork()
		result.Core = false
		result.TakeoverStatus = "fail"
		result.TakeoverDetail = startErr.Error()
		result.Message = startErr.Error()
		return result, nil
	}

	s.mu.Lock()
	if s.cmd == nil {
		s.mu.Unlock()
		result.Core = false
		result.TakeoverStatus = "fail"
		result.TakeoverDetail = "内核未启动"
		result.Message = "内核未启动"
		return result, nil
	}
	port := s.mixedPort
	cmd, startedAt, content, modeRevision := s.cmd, s.startedAt, s.currentContent, s.modeRevision
	options, _ := json.Marshal(s.coreOptions)
	s.mu.Unlock()

	var (
		selections   map[string]string
		sessionArmed bool
		autoExiting  bool
	)
	unchanged := func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		currentOptions, _ := json.Marshal(s.coreOptions)
		return s.cmd == cmd && s.startedAt.Equal(startedAt) && s.currentContent == content && s.modeRevision == modeRevision && bytes.Equal(options, currentOptions)
	}
	defer func() {
		if !sessionArmed || autoExiting {
			return
		}
		valid := unchanged()
		if valid && selections != nil {
			latest, err := s.Groups()
			if err != nil {
				valid = false
			} else {
				count := 0
				for _, group := range latest {
					if group.Type != "Selector" || group.Now == "" {
						continue
					}
					count++
					if previous, ok := selections[group.Name]; !ok || previous != group.Now {
						valid = false
					}
				}
				valid = valid && count == len(selections)
			}
		}
		if !valid || !unchanged() {
			result = Connectivity{}
			resultErr = errors.New("检查期间内核、配置或出站选择已改变，请重新检查")
		}
	}()

	result.Core = true

	// One working node is enough. A dead selected node is replaced from its
	// own group, and that scan stops at the first success.
	reportCheckStep(ctx, &result, "正在测试当前节点")
	s.verifyAndSelectWorkingNode(target, &result)

	if groups, err := s.Groups(); err == nil {
		selections = make(map[string]string)
		for _, group := range groups {
			if group.Type == "Selector" && group.Now != "" {
				selections[group.Name] = group.Now
			}
		}
	}
	sessionArmed = true

	// Step 2 (Part B): Verify whether enabling the proxy caused a network disconnect.
	reportCheckStep(ctx, &result, "正在经代理测试网络")
	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport:     transport,
		Timeout:       8 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	res, err := client.Get(target)
	if err == nil {
		res.Body.Close()
	}
	if !unchanged() {
		return result, nil
	}

	targetOK := err == nil && res.StatusCode == http.StatusNoContent
	if !targetOK {
		// When target is an external URL, check if basic domestic internet still works through the proxy
		// before concluding that the proxy disconnected the machine.
		if !isLoopbackURL(target) && probeURLs(&http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}, domesticProbeURLs) {
			result.TakeoverStatus = "ok"
			result.TakeoverDetail = "开启代理后网络正常，未导致断网"
			if result.NodeStatus == "ok" {
				result.Request = true
				result.Message = "代理连接正常"
			} else {
				result.Request = false
				result.Message = "本机网络正常，但代理节点无法连接"
			}
			return result, nil
		}

		// The machine had internet in Step 1 without proxy, and lost connectivity after enabling proxy:
		// automatically exit the proxy and restore network settings.
		autoExiting = true
		_ = s.RecoverNetwork()
		result.Core = false
		result.AutoExited = true
		result.Request = false
		result.TakeoverStatus = "fail"
		result.TakeoverDetail = "因开启代理导致断网，已自动退出代理并恢复网络"
		result.Message = "检测到代理导致断网，已自动退出代理并恢复网络"
		return result, nil
	}

	result.TakeoverStatus = "ok"
	result.TakeoverDetail = "开启代理后网络正常，未导致断网"
	result.Request = true
	if result.NodeStatus == "fail" {
		result.Message = "本机网络未受影响，但所有代理节点均无法连接"
	} else {
		result.Message = "测试请求通过"
	}
	return result, nil
}

func (s *Service) startForCheck(ctx context.Context, content string, port int, bypass *[]string, lan LANSettings, options CoreOptions) error {
	done := make(chan struct{})
	defer close(done)
	go s.relayStartupStage(ctx, done)
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		lastErr = s.start(content, port, bypass, lan, options)
		if lastErr == nil {
			return nil
		}
		if !strings.Contains(lastErr.Error(), "端口") {
			return lastErr
		}
		time.Sleep(25 * time.Millisecond)
	}
	return lastErr
}

func (s *Service) relayStartupStage(ctx context.Context, done <-chan struct{}) {
	var last string
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-time.After(200 * time.Millisecond):
		}
		pending := s.startupStatus.Load()
		if pending == nil || pending.StartupStage == "" || pending.StartupStage == last {
			continue
		}
		last = pending.StartupStage
		reportCheckStep(ctx, &Connectivity{Local: true, LocalStatus: "ok", LocalDetail: "无代理状态下本机网络正常"}, checkStepOf(last))
	}
}

func checkStepOf(stage string) string {
	switch stage {
	case "准备配置":
		return "正在准备配置"
	case "启动 mihomo":
		return "正在启动内核，缺少 GEO 数据时会下载"
	case "等待虚拟网卡授权":
		return "正在等待虚拟网卡授权"
	case "应用系统代理设置":
		return "正在应用系统代理"
	default:
		return stage
	}
}

func (s *Service) selectNodeInCheck(group, node string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd == nil {
		return errors.New("内核未启动")
	}
	proxies, err := inProcessProxiesMap()
	before, hadBefore := proxies[group]
	if err != nil {
		hadBefore = false
	}
	if err := selectInProcessProxy(group, node); err != nil {
		return err
	}
	if !hadBefore || before.Now != node {
		s.closeGroupConnectionsLocked(group)
	}
	s.rememberSelectionLocked(group, node)
	return nil
}

func (s *Service) verifyAndSelectWorkingNode(target string, result *Connectivity) {
	nodes := collectActualProxyNodes()
	if len(nodes) == 0 {
		result.NodeStatus = "skip"
		result.NodeDetail = "当前配置没有代理节点"
		return
	}
	byName := make(map[string]C.Proxy, len(nodes))
	for _, node := range nodes {
		byName[node.Name()] = node
	}
	groups, groupErr := s.Groups()
	proxiesMap, mapErr := inProcessProxiesMap()
	if groupErr != nil || mapErr != nil {
		s.verifyAnyNode(target, nodes, result)
		return
	}
	if strings.EqualFold(currentInProcessMode(), "global") {
		if globalGroup, ok := proxiesMap["GLOBAL"]; ok && globalGroup.Type == "Selector" {
			groups = append([]Proxy{globalGroup.Proxy}, groups...)
		}
	}
	var verifiedNode string
	verifiedDelay := 0
	switchedNode, switchedDelay := "", 0
	checkedSelector := false
	seen := map[string]bool{}
	for _, group := range groups {
		if seen[group.Name] || group.Type != "Selector" || group.Now == "" || isBuiltInPolicyName(group.Now) {
			continue
		}
		seen[group.Name] = true
		leaf := resolveLeafProxy(group.Now, proxiesMap)
		proxy := byName[leaf]
		if proxy == nil {
			continue
		}
		checkedSelector = true
		if delay, ok := probeProxyDelay(proxy, target); ok {
			if verifiedNode == "" {
				verifiedNode, verifiedDelay = leaf, delay
			}
			continue
		}
		candidate, leafName, delay, ok := s.firstWorkingMember(group, proxiesMap, byName, target)
		if !ok || candidate == group.Now {
			continue
		}
		if err := s.selectNodeInCheck(group.Name, candidate); err != nil {
			continue
		}
		if g, ok := proxiesMap[group.Name]; ok {
			g.Now = candidate
			proxiesMap[group.Name] = g
		}
		result.Recommendations = append(result.Recommendations, NodeRecommendation{
			Group: group.Name, Current: group.Now, Node: candidate, Delay: delay,
		})
		if switchedNode == "" {
			switchedNode, switchedDelay = leafName, delay
		}
	}
	if !checkedSelector {
		s.verifyAnyNode(target, nodes, result)
		return
	}
	if switchedNode != "" {
		result.NodeStatus = "ok"
		result.VerifiedNode = switchedNode
		result.Switched = true
		result.NodeDetail = fmt.Sprintf("已自动切换到可用节点「%s」（%d ms）", switchedNode, switchedDelay)
		return
	}
	if verifiedNode != "" {
		result.NodeStatus = "ok"
		result.VerifiedNode = verifiedNode
		result.NodeDetail = fmt.Sprintf("节点「%s」连接成功（%d ms）", verifiedNode, verifiedDelay)
		return
	}
	result.NodeStatus = "fail"
	result.NodeDetail = "当前节点无法连接，同组内也没有可用节点"
}

func (s *Service) verifyAnyNode(target string, nodes []C.Proxy, result *Connectivity) {
	if name, delay, ok := firstWorkingProxy(nodes, target); ok {
		result.NodeStatus = "ok"
		result.VerifiedNode = name
		result.NodeDetail = fmt.Sprintf("节点「%s」连接成功（%d ms）", name, delay)
		return
	}
	result.NodeStatus = "fail"
	result.NodeDetail = "所有代理节点均无法连接"
}

func (s *Service) firstWorkingMember(group Proxy, proxies map[string]proxyDetail, byName map[string]C.Proxy, target string) (string, string, int, bool) {
	var members []C.Proxy
	leaves := map[string]string{}
	for _, candidate := range group.All {
		if candidate == group.Now || isBuiltInPolicyName(candidate) {
			continue
		}
		leaf := resolveLeafProxy(candidate, proxies)
		proxy := byName[leaf]
		if proxy == nil {
			continue
		}
		members = append(members, proxy)
		if _, seen := leaves[leaf]; !seen {
			leaves[leaf] = candidate
		}
	}
	name, delay, ok := firstWorkingProxy(members, target)
	if !ok {
		return "", "", 0, false
	}
	return leaves[name], name, delay, true
}

const checkProbeParallel = 16

func firstWorkingProxy(nodes []C.Proxy, target string) (string, int, bool) {
	if len(nodes) == 0 {
		return "", 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type hit struct {
		name  string
		delay int
	}
	found := make(chan hit, 1)
	var wg sync.WaitGroup
	sem := make(chan struct{}, checkProbeParallel)
	for _, node := range nodes {
		wg.Add(1)
		go func(node C.Proxy) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()
			delay, err := node.URLTest(ctx, target, nil)
			if err != nil || ctx.Err() != nil {
				return
			}
			ms := int(delay)
			if ms <= 0 {
				ms = 1
			}
			select {
			case found <- hit{name: node.Name(), delay: ms}:
				cancel()
			default:
			}
		}(node)
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case hit := <-found:
		<-done
		return hit.name, hit.delay, true
	case <-done:
		return "", 0, false
	}
}

func probeProxyDelay(proxy C.Proxy, target string) (int, bool) {
	name, delay, ok := firstWorkingProxy([]C.Proxy{proxy}, target)
	if !ok || name == "" {
		return 0, false
	}
	return delay, true
}

func collectActualProxyNodes() []C.Proxy {
	raw := tunnel.Proxies()
	seen := make(map[string]bool, len(raw))
	var out []C.Proxy
	add := func(p C.Proxy) {
		if p == nil || !isActualProxyNode(p) || seen[p.Name()] {
			return
		}
		seen[p.Name()] = true
		out = append(out, p)
	}
	names := make([]string, 0, len(raw))
	for name := range raw {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		add(raw[name])
	}
	for _, provider := range tunnel.Providers() {
		for _, p := range provider.Proxies() {
			add(p)
		}
	}
	return out
}

func isActualProxyNode(p C.Proxy) bool {
	if isBuiltInPolicyName(p.Name()) {
		return false
	}
	switch p.Type() {
	case C.Direct, C.Reject, C.RejectDrop, C.Pass, C.Compatible,
		C.Selector, C.URLTest, C.Fallback, C.LoadBalance, C.Relay:
		return false
	default:
		return true
	}
}

func isBuiltInPolicyName(name string) bool {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "", "DIRECT", "REJECT", "REJECT-DROP", "PASS", "COMPATIBLE", "GLOBAL":
		return true
	default:
		return false
	}
}

func resolveLeafProxy(name string, proxies map[string]proxyDetail) string {
	curr := name
	visited := make(map[string]bool, 8)
	for curr != "" && !visited[curr] {
		visited[curr] = true
		detail, ok := proxies[curr]
		if !ok || detail.Now == "" {
			return curr
		}
		curr = detail.Now
	}
	return curr
}

func isLoopbackURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func probeLocalDirect(target string) bool {
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport:     transport,
		Timeout:       5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
	}
	if isLoopbackURL(target) {
		return probeSingleURL(client, target)
	}
	urls := make([]string, 0, len(domesticProbeURLs)+1)
	urls = append(urls, domesticProbeURLs...)
	urls = append(urls, target)
	return probeURLs(client, urls)
}

func probeURLs(client *http.Client, urls []string) bool {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	results := make(chan bool, len(urls))
	for _, u := range urls {
		go func(targetURL string) {
			results <- probeSingleURLWithContext(ctx, client, targetURL)
		}(u)
	}
	for range urls {
		if <-results {
			cancel()
			return true
		}
	}
	return false
}

func probeSingleURL(client *http.Client, targetURL string) bool {
	return probeSingleURLWithContext(context.Background(), client, targetURL)
}

func probeSingleURLWithContext(ctx context.Context, client *http.Client, targetURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, targetURL, nil)
	if err == nil {
		if res, err := client.Do(req); err == nil {
			res.Body.Close()
			if res.StatusCode >= 200 && res.StatusCode < 400 {
				return true
			}
			if res.StatusCode != http.StatusMethodNotAllowed && res.StatusCode != http.StatusNotImplemented {
				return false
			}
		}
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return false
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	res.Body.Close()
	return res.StatusCode >= 200 && res.StatusCode < 400
}
