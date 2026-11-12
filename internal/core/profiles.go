package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Profile is a locally stored configuration. Subscription URLs can contain credentials.
type Profile struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Source      string             `json:"source"`
	URL         string             `json:"url"`
	Interval    int                `json:"interval"`
	Content     string             `json:"content"`
	Path        string             `json:"path"`
	UpdatedAt   time.Time          `json:"updatedAt"`
	LastError   string             `json:"lastError"`
	LastAttempt time.Time          `json:"lastAttempt"`
	Usage       *SubscriptionUsage `json:"usage"`
}
type SubscriptionUsage struct {
	Upload   uint64 `json:"upload"`
	Download uint64 `json:"download"`
	Total    uint64 `json:"total"`
	Expire   int64  `json:"expire"`
}
type profileData struct {
	Profiles           []Profile       `json:"profiles"`
	ActiveID           string          `json:"activeId"`
	MixedPort          int             `json:"mixedPort"`
	SystemProxy        bool            `json:"systemProxy"`
	Overrides          []OverrideGroup `json:"overrides"`
	OverrideRevision   uint64          `json:"overrideRevision"`
	Speed              *SpeedSettings  `json:"speed,omitempty"`
	ProxyBypass        *[]string       `json:"proxyBypass,omitempty"`
	LAN                *LANSettings    `json:"lan,omitempty"`
	Tun                bool            `json:"tun"`
	AutoConnect        bool            `json:"autoConnect"`
	TrayDisplay        string          `json:"trayDisplay,omitempty"`
	HideDockWhenClosed bool            `json:"hideDockWhenClosed,omitempty"`
	WiFi               *WiFiSettings   `json:"wifi,omitempty"`
	CoreOptions        *CoreOptions    `json:"coreOptions,omitempty"`
}
type ProfileState struct {
	Profiles    []Profile `json:"profiles"`
	ActiveID    string    `json:"activeId"`
	MixedPort   int       `json:"mixedPort"`
	SystemProxy bool      `json:"systemProxy"`
}

// Profiles keeps the original configurations separate from runtime overrides.
type Profiles struct {
	mu       sync.Mutex
	dir      string
	updateMu sync.Mutex
	service  *Service
	draft    string
	// selections is created on first use; see selectionsLocked.
	selections *selectionStore
}

func NewProfiles(dir string) *Profiles { return &Profiles{dir: dir} }
func NewProfilesWithService(dir string, service *Service) *Profiles {
	p := &Profiles{dir: dir, service: service}
	if service != nil {
		service.startActive = p.StartActive
	}
	return p
}
func (p *Profiles) directory() (string, error) {
	if p.dir != "" {
		return p.dir, nil
	}
	path, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(path, "RocketScience"), nil
}
func (p *Profiles) load() (profileData, error) {
	// A new install uses the system proxy, so the first start needs no takeover choice.
	// A saved file keeps its explicit value.
	d := profileData{Profiles: []Profile{}, MixedPort: 7890, SystemProxy: true, Overrides: defaultOverrides()}
	dir, err := p.directory()
	if err != nil {
		return d, err
	}
	b, err := os.ReadFile(filepath.Join(dir, "profiles.json"))
	if errors.Is(err, os.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	if err = json.Unmarshal(b, &d); err != nil {
		return d, fmt.Errorf("读取配置索引失败：%w", err)
	}
	if d.Profiles == nil {
		d.Profiles = []Profile{}
	}
	if len(d.Overrides) == 0 {
		d.Overrides = defaultOverrides()
	}
	for i := range d.Overrides {
		if d.Overrides[i].Rules == nil {
			d.Overrides[i].Rules = []OverrideRule{}
		}
	}
	if d.MixedPort < 1 || d.MixedPort > 65535 {
		d.MixedPort = 7890
	}
	if len(d.Profiles) == 0 {
		d.ActiveID = ""
	} else if d.ActiveID != "" {
		activeFound := false
		for _, v := range d.Profiles {
			if v.ID == d.ActiveID {
				activeFound = true
				break
			}
		}
		if !activeFound {
			d.ActiveID = d.Profiles[0].ID
		}
	}
	return d, nil
}
func (p *Profiles) write(d profileData) error {
	dir, err := p.directory()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".profiles-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil {
		return err
	}
	return os.Rename(name, filepath.Join(dir, "profiles.json"))
}
func (p *Profiles) State() (ProfileState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	return ProfileState{d.Profiles, d.ActiveID, d.MixedPort, d.SystemProxy}, err
}
func validateProfile(content string) error {
	if len(content) == 0 || len(content) > 10*1024*1024 {
		return errors.New("配置为空或超过 10 MB")
	}
	if cfg, err := unmarshalConfigMap([]byte(content)); err == nil && cfg != nil {
		if err := validateProviderPaths(cfg); err != nil {
			return err
		}
	}
	dir, err := os.MkdirTemp("", "rocket-validate-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err = (&geoCache{}).seed(dir); err != nil {
		return fmt.Errorf("无法准备 GEO 数据：%w", err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err = os.WriteFile(path, []byte(content), 0600); err != nil {
		return err
	}
	if err := parseConfigInDir(dir, path, []byte(content)); err != nil {
		return fmt.Errorf("mihomo：%v", err)
	}
	return nil
}
func validateMeta(v Profile) error {
	if strings.TrimSpace(v.Name) == "" {
		return errors.New("请输入配置名称")
	}
	if v.Source != "file" && v.Source != "url" {
		return errors.New("配置来源无效")
	}
	if v.Source == "url" {
		u, err := url.Parse(v.URL)
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			return errors.New("订阅链接必须是 HTTP 或 HTTPS 地址")
		}
	}
	if v.Interval != 0 && v.Interval != 6 && v.Interval != 12 && v.Interval != 24 {
		return errors.New("更新间隔无效")
	}
	return nil
}

// Save validates before replacing any saved configuration.
func (p *Profiles) Save(v Profile, expectedUpdatedAt string) (ProfileState, error) {
	if err := validateMeta(v); err != nil {
		return ProfileState{}, err
	}
	if err := validateProfile(v.Content); err != nil {
		return ProfileState{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return ProfileState{}, err
	}
	v.Name = strings.TrimSpace(v.Name)
	v.LastError = ""
	v.UpdatedAt = time.Now().UTC()
	if v.ID == "" {
		id := make([]byte, 16)
		if _, err = rand.Read(id); err != nil {
			return ProfileState{}, err
		}
		v.ID = hex.EncodeToString(id)
		d.Profiles = append(d.Profiles, v)
		if d.ActiveID == "" {
			d.ActiveID = v.ID
		}
	} else {
		found := false
		for i := range d.Profiles {
			if d.Profiles[i].ID == v.ID {
				version, versionErr := time.Parse(time.RFC3339Nano, expectedUpdatedAt)
				if versionErr != nil || !d.Profiles[i].UpdatedAt.Equal(version) {
					return ProfileState{}, errors.New("配置已更新，请重新打开后编辑")
				}
				d.Profiles[i] = v
				found = true
				break
			}
		}
		if !found {
			return ProfileState{}, errors.New("配置不存在")
		}
	}
	if v.ID == d.ActiveID {
		err = p.commitProfile(d, v.Content)
	} else {
		err = p.write(d)
	}
	if err != nil {
		return ProfileState{}, err
	}
	return ProfileState{d.Profiles, d.ActiveID, d.MixedPort, d.SystemProxy}, nil
}
func (p *Profiles) Use(id string) (ProfileState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return ProfileState{}, err
	}
	found := false
	var content string
	for _, v := range d.Profiles {
		if v.ID == id {
			found = true
			content = v.Content
			break
		}
	}
	if !found {
		return ProfileState{}, errors.New("配置不存在")
	}
	d.ActiveID = id
	if err = p.commitProfile(d, content); err != nil {
		return ProfileState{}, err
	}
	return ProfileState{d.Profiles, d.ActiveID, d.MixedPort, d.SystemProxy}, nil
}
func (p *Profiles) Delete(id string) (ProfileState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return ProfileState{}, err
	}
	if d.ActiveID == id {
		return ProfileState{}, errors.New("正在使用的配置不能删除")
	}
	found := false
	for i, v := range d.Profiles {
		if v.ID == id {
			d.Profiles = append(d.Profiles[:i], d.Profiles[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return ProfileState{}, errors.New("配置不存在")
	}
	if err = p.write(d); err != nil {
		return ProfileState{}, err
	}
	return ProfileState{d.Profiles, d.ActiveID, d.MixedPort, d.SystemProxy}, nil
}
func (p *Profiles) SetPort(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("端口必须在 1 到 65535 之间")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.service != nil {
		p.service.opMu.Lock()
		defer p.service.opMu.Unlock()
		p.service.mu.Lock()
		defer p.service.mu.Unlock()
		if p.service.cmd != nil {
			return errors.New("请先停止内核，再修改混合代理端口")
		}
	}
	d, err := p.load()
	if err != nil {
		return err
	}
	if options := coreOptionsValue(d.CoreOptions); options.Inbound != nil {
		if err = validateInboundOptions(*options.Inbound, port); err != nil {
			return err
		}
	}
	if options := coreOptionsValue(d.CoreOptions); options.Controller != nil {
		if err = validateControllerPorts(*options.Controller, port, options.Inbound); err != nil {
			return err
		}
	}
	d.MixedPort = port
	return p.write(d)
}
func (p *Profiles) SetSystemProxy(enabled bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, err := p.load()
	if err != nil {
		return err
	}
	d.SystemProxy = enabled
	return p.write(d)
}
func downloadProfile(raw string) (string, *SubscriptionUsage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", raw, nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("User-Agent", "clash.meta")
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return errors.New("重定向次数过多")
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return errors.New("不支持的重定向协议")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", nil, errors.New("下载订阅超时（30s），请检查订阅地址或网络")
		}
		var uErr *url.Error
		if errors.As(err, &uErr) && uErr.Err != nil {
			return "", nil, fmt.Errorf("下载失败：%w", uErr.Err)
		}
		return "", nil, fmt.Errorf("下载失败：%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", nil, fmt.Errorf("下载失败：HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024+1))
	if err != nil {
		return "", nil, errors.New("读取订阅失败")
	}
	if len(b) > 10*1024*1024 {
		return "", nil, errors.New("订阅超过 10 MB")
	}
	return string(b), parseUsage(resp.Header.Get("subscription-userinfo")), nil
}

func parseUsageNumber(raw string) (uint64, bool) {
	v := strings.TrimSpace(raw)
	if n, err := strconv.ParseUint(v, 10, 63); err == nil {
		return n, true
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > float64(math.MaxInt64) {
		return 0, false
	}
	return uint64(f), true
}

func parseUsage(raw string) *SubscriptionUsage {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	usage := &SubscriptionUsage{}
	found := false
	for _, field := range strings.Split(raw, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(field), "=")
		if !ok {
			continue
		}
		n, ok := parseUsageNumber(value)
		if !ok {
			continue
		}
		switch strings.TrimSpace(strings.ToLower(key)) {
		case "upload":
			usage.Upload = n
			found = true
		case "download":
			usage.Download = n
			found = true
		case "total":
			usage.Total = n
			found = true
		case "expire":
			usage.Expire = int64(n)
			found = true
		}
	}
	if !found {
		return nil
	}
	return usage
}

type SubscriptionDownload struct {
	Content   string             `json:"content"`
	Usage     *SubscriptionUsage `json:"usage"`
	UpdatedAt time.Time          `json:"updatedAt"`
}

// Fetch downloads and validates a subscription before returning it to the editor.
func (p *Profiles) Fetch(raw string) (SubscriptionDownload, error) {
	v := Profile{Name: "subscription", Source: "url", URL: raw}
	if err := validateMeta(v); err != nil {
		return SubscriptionDownload{}, err
	}
	content, usage, err := downloadProfile(raw)
	if err != nil {
		return SubscriptionDownload{}, err
	}
	if err = validateProfile(content); err != nil {
		return SubscriptionDownload{}, err
	}
	return SubscriptionDownload{Content: content, Usage: usage, UpdatedAt: time.Now().UTC()}, nil
}
