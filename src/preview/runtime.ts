// Browser preview stand-in for `mygo-runtime`. Vite aliases that package to
// this file only when started with `--mode preview`. The desktop `dev` script
// and the default Vite server keep the real runtime.

import type {
  Connection,
  ConnectionSnapshot,
  Connectivity,
  ControllerOptions,
  CoreOptions,
  DNSOptions,
  DesktopState,
  GeoOptions,
  ImportedProfileFile,
  InboundOptions,
  KernelInfo,
  LANInterface,
  LANSettings,
  LogEntry,
  Monitoring,
  OverrideGroup,
  OverrideState,
  Profile,
  ProfileState,
  Proxy,
  ProxyBypassSettings,
  ProxyNode,
  Rule,
  SpeedSettings,
  Status,
  Streams,
  SubscriptionDownload,
  TunOptions,
  WiFiSettings,
  WiFiStatus,
} from "../mygo";

function fail(message: string): never {
  const error = new Error(message);
  error.name = "CallError";
  throw error;
}

function clone<T>(value: T): T {
  return structuredClone(value);
}

const sampleYaml = `proxies: []
proxy-groups:
  - { name: 🚀 节点选择, type: select, proxies: [美国 02, 日本 01, 美国 01, DIRECT] }
rules:
  - DOMAIN-SUFFIX,local,DIRECT
  - MATCH,🚀 节点选择
`;

function node(name: string, type: string, delay: number): ProxyNode {
  return { name, type, udp: true, delay, delayAt: new Date().toISOString() };
}

const previewNodes = [node("美国 02", "ss", 86), node("日本 01", "vmess", 124), node("美国 01", "trojan", 210)];
const nodeNames = previewNodes.map(item => item.name);

function group(name: string, type: string, now: string, members: string[]): Proxy {
  const known = new Map<string, ProxyNode>([...previewNodes.map(item => [item.name, item] as const), ["DIRECT", node("DIRECT", "Direct", 12)], ["REJECT", { name: "REJECT", type: "Reject", udp: true, delay: null, delayAt: "" }]]);
  return { name, type, now, all: members, nodes: members.map(member => known.get(member) ?? { name: member, type: "Selector", udp: true, delay: null, delayAt: "" }), rules: 0, fallback: false };
}

// A common subscription shape: one main selector and branch groups that follow it.
function makeGroups(): Proxy[] {
  return [
    group("🚀 节点选择", "Selector", "美国 02", ["自动选择", ...nodeNames, "DIRECT"]),
    group("自动选择", "URLTest", "日本 01", nodeNames),
    group("国外媒体", "Selector", "🚀 节点选择", ["🚀 节点选择", "自动选择", ...nodeNames]),
    group("电报消息", "Selector", "日本 01", ["🚀 节点选择", ...nodeNames]),
    group("国内服务", "Selector", "DIRECT", ["DIRECT", "🚀 节点选择"]),
    group("广告拦截", "Selector", "REJECT", ["REJECT", "DIRECT"]),
    group("漏网之鱼", "Selector", "🚀 节点选择", ["🚀 节点选择", "DIRECT"]),
    group("GLOBAL", "Selector", "🚀 节点选择", ["🚀 节点选择", "自动选择", "国外媒体", "电报消息", "国内服务", "广告拦截", "漏网之鱼", ...nodeNames, "DIRECT", "REJECT"]),
  ];
}

const subscriptionRules: Rule[] = [
  { type: "DOMAIN-SUFFIX", payload: "local", proxy: "DIRECT" },
  { type: "DOMAIN-SUFFIX", payload: "doubleclick.net", proxy: "广告拦截" },
  { type: "DOMAIN-SUFFIX", payload: "youtube.com", proxy: "国外媒体" },
  { type: "DOMAIN-SUFFIX", payload: "netflix.com", proxy: "国外媒体" },
  { type: "DOMAIN-SUFFIX", payload: "telegram.org", proxy: "电报消息" },
  { type: "DOMAIN-SUFFIX", payload: "github.com", proxy: "🚀 节点选择" },
  { type: "DOMAIN-SUFFIX", payload: "google.com", proxy: "🚀 节点选择" },
  { type: "GEOIP", payload: "CN", proxy: "国内服务" },
  { type: "MATCH", payload: "", proxy: "漏网之鱼" },
];

function profile(id: string, name: string, source: "url" | "file"): Profile {
  return {
    id,
    name,
    source,
    url: source === "url" ? "https://example.invalid/subscription" : "",
    interval: source === "url" ? 24 : 0,
    content: sampleYaml,
    path: source === "file" ? "示例配置.yaml" : "",
    updatedAt: "2026-10-28T08:00:00Z",
    lastError: "",
    lastAttempt: "2026-10-28T08:00:00Z",
    usage: source === "url" ? { upload: 1_200_000_000, download: 8_400_000_000, total: 50_000_000_000, expire: 1_796_000_000 } : null,
  };
}

interface PreviewState {
  running: boolean;
  mode: string;
  systemProxy: boolean;
  tun: boolean;
  mixedPort: number;
  startedAt: string;
  warning: string;
  recoveryPending: boolean;
  openAtLogin: boolean;
  autoConnect: boolean;
  tray: string;
  desktop: DesktopState;
  profiles: Profile[];
  activeId: string;
  nextId: number;
  groups: Proxy[];
  connections: Connection[];
  overrides: OverrideState;
  logs: LogEntry[];
  tick: number;
  dns: DNSOptions;
  core: CoreOptions;
  tunOptions: TunOptions;
  tunService: boolean;
  geo: GeoOptions;
  inbound: InboundOptions;
  controller: ControllerOptions;
  lan: LANSettings;
  interfaces: LANInterface[];
  bypass: ProxyBypassSettings;
  speed: SpeedSettings;
  wifi: WiFiSettings;
  wifiState: WiFiStatus;
}

const state: PreviewState = {
  running: true,
  mode: "rule",
  systemProxy: true,
  tun: false,
  mixedPort: 7890,
  startedAt: "2026-10-28T09:00:00Z",
  warning: "",
  recoveryPending: false,
  openAtLogin: false,
  autoConnect: false,
  tray: "both",
  desktop: { quitError: "", startupReady: true, startupError: "", trayError: "" },
  profiles: [profile("demo", "示例订阅", "url")] as Profile[],
  activeId: "demo",
  nextId: 2,
  groups: makeGroups(),
  connections: [
    conn("c1", "www.example.com", "443", "tcp", "Safari", ["美国 02", "🚀 节点选择"], "MATCH", "", 180_000, 2_400_000),
    conn("c2", "api.example.com", "443", "tcp", "Mail", ["日本 01", "自动选择"], "MATCH", "", 40_000, 610_000),
    conn("c4", "very-long-subdomain.cdn.streaming-provider.example.com", "443", "tcp", "Safari", ["美国 02", "IEPL 专线中转", "自动选择", "国外媒体", "🚀 节点选择"], "RuleSet", "https://raw.githubusercontent.com/example/rules/main/streaming-global.yaml", 90_000, 52_000_000),
    conn("c3", "10.0.0.8", "53", "udp", "mDNSResponder", ["DIRECT"], "GEOIP", "CN", 2_000, 1_200),
  ] as Connection[],
  overrides: {
    groups: [{
      id: "lan",
      name: "局域网",
      enabled: true,
      rules: [{ type: "IP-CIDR", payload: "192.168.0.0/16", policy: "DIRECT" }],
    }],
    revision: 1,
    policies: ["🚀 节点选择", "自动选择", "DIRECT", "REJECT"],
    skipped: [],
    origins: [],
  },
  logs: [
    { time: "2026-10-28T09:00:01Z", type: "info", payload: "内核已启动" },
    { time: "2026-10-28T09:00:04Z", type: "info", payload: "规则已加载" },
    { time: "2026-10-28T09:01:12Z", type: "warning", payload: "节点美国 01 握手较慢" },
  ],
  tick: 0,
  dns: emptyDns(),
  core: emptyCore(),
  tunOptions: emptyTun(),
  tunService: true,
  geo: { dataMode: null, autoUpdate: null, updateInterval: null },
  inbound: { httpPort: 0, socksPort: 0, authentication: null },
  controller: { address: "127.0.0.1", port: 0, secret: "" },
  lan: { enabled: false, address: "*" },
  interfaces: [{ name: "en0", address: "192.168.1.20" }],
  bypass: { custom: false, entries: [] },
  speed: { url: "https://www.gstatic.com/generate_204", timeoutMs: 5000 },
  wifi: { enabled: false, trusted: ["Office"] },
  wifiState: { known: true, ssid: "Office", detail: "", enabled: false, trusted: true, applied: false, manualOverride: false },
};

function conn(id: string, host: string, port: string, network: string, process: string, chains: string[], rule: string, rulePayload: string, upload: number, download: number): Connection {
  return { id, metadata: { host, destinationIP: "203.0.113.10", destinationPort: port, network, process, processPath: process === "mDNSResponder" ? "/usr/sbin/mDNSResponder" : `/Applications/${process}.app/Contents/MacOS/${process}` }, upload, download, start: "2026-10-28T09:01:00Z", chains, rule, rulePayload };
}

function emptyDns(): DNSOptions {
  return { enabled: null, ipv6: null, enhancedMode: null, fakeIPRange: null, defaultNameservers: null, nameservers: null, fallback: null };
}
function emptyCore(): CoreOptions {
  return { logLevel: null, ipv6: null, tcpConcurrent: null, unifiedDelay: null, findProcessMode: null, keepAliveInterval: null, sniffer: null, dns: null, geo: null, tun: null, inbound: null, controller: null };
}
function emptyTun(): TunOptions {
  return { stack: null, autoRoute: null, autoDetectInterface: null, strictRoute: null, mtu: null, dnsHijack: null };
}

function profileState(): ProfileState {
  return { profiles: clone(state.profiles), activeId: state.activeId, mixedPort: state.mixedPort, systemProxy: state.systemProxy };
}

function effectiveRules(): { rules: Rule[]; origins: string[] } {
  const rules: Rule[] = [];
  const origins: string[] = [];
  for (const group of state.overrides.groups) {
    if (!group.enabled) continue;
    for (const rule of group.rules) {
      rules.push({ type: rule.type, payload: rule.payload, proxy: rule.policy });
      origins.push(group.id);
    }
  }
  for (const rule of subscriptionRules) {
    rules.push(rule);
    origins.push("");
  }
  return { rules, origins };
}

function withRuleCounts(groups: Proxy[]): Proxy[] {
  const rules = effectiveRules().rules;
  return groups.map(item => ({ ...item, rules: rules.filter(rule => rule.proxy === item.name).length, fallback: rules.some(rule => rule.type === "MATCH" && rule.proxy === item.name) }));
}

function overrideState(): OverrideState {
  const next = clone(state.overrides);
  next.origins = effectiveRules().origins;
  return next;
}

function status(): Status {
  return {
    starting: false,
    startupStage: "",
    running: state.running,
    version: "v1.19.0",
    mixedPort: state.mixedPort,
    mode: state.mode,
    error: "",
    corePath: "/Applications/RocketScience.app/Contents/Resources/mihomo",
    systemProxy: state.systemProxy,
    tun: state.tun,
    recoveryPending: state.recoveryPending,
    warning: state.warning,
    geoWarning: "",
    controller: state.running ? `127.0.0.1:${49000 + (state.mixedPort % 100)}` : "",
    startedAt: state.startedAt,
  };
}

function traffic(): Monitoring {
  state.tick += 1;
  const up = state.running ? 42_000 + (state.tick % 7) * 8_000 : 0;
  const down = state.running ? 310_000 + (state.tick % 5) * 40_000 : 0;
  const historyUp = Array.from({ length: 20 }, (_, i) => (state.running ? 20_000 + ((state.tick + i) % 8) * 6_000 : 0));
  const historyDown = Array.from({ length: 20 }, (_, i) => (state.running ? 180_000 + ((state.tick + i) % 6) * 30_000 : 0));
  return {
    telemetry: { up, down, connected: state.running },
    memory: { inuse: state.running ? 72 * 1048576 : 0, connected: state.running },
    history: { up: historyUp, down: historyDown },
  };
}

function streams(): Streams {
  return { ...traffic(), logs: clone(state.logs) };
}

function connections(): ConnectionSnapshot {
  const list = clone(state.connections);
  return {
    connections: list,
    uploadTotal: list.reduce((sum, item) => sum + item.upload, 0),
    downloadTotal: list.reduce((sum, item) => sum + item.download, 0),
  };
}

// `?preview=stopped|stoppedOff|runningTun|noProfile` opens the preview in that backend state.
const scenario = new URLSearchParams(globalThis.location?.search ?? "").get("preview");
if (scenario && scenario !== "running") {
  if (scenario === "runningTun") state.tun = true;
  else {
    state.running = false;
    if (scenario === "stoppedOff") state.systemProxy = false;
    if (scenario === "noProfile") { state.profiles = []; state.activeId = ""; state.groups = []; }
  }
}

function requireRunning() {
  if (!state.running) fail("内核未启动");
}

function startCore() {
  if (!state.systemProxy && !state.tun) fail("请先开启系统代理或 TUN 增强模式");
  if (!state.activeId) fail("请先导入并保存配置。");
  state.running = true;
  state.recoveryPending = false;
  state.startedAt = new Date().toISOString();
  state.warning = "";
}

function stopCore() {
  state.running = false;
  state.recoveryPending = false;
  state.startedAt = "";
}

function kernelInfo(): KernelInfo {
  return { available: true, path: status().corePath, version: "v1.19.0", platform: "darwin/arm64" };
}

async function copyText(text: string) {
  try { await navigator.clipboard?.writeText(text); } catch { /* 浏览器预览没有系统剪贴板权限时仍让按钮完成。 */ }
}

const handlers: Record<string, (args: unknown[]) => unknown> = {
  "Desktop.State": () => clone(state.desktop),
  "Desktop.ClearQuitError": () => { state.desktop.quitError = ""; },
  "Desktop.CopyTerminalProxy": (args) => copyText(`export https_proxy=http://127.0.0.1:${Number(args[0]) || state.mixedPort}`),
  "Desktop.CopyText": (args) => copyText(String(args[0] ?? "")),
  "Desktop.OpenAtLogin": () => state.openAtLogin,
  "Desktop.SetOpenAtLogin": (args) => { state.openAtLogin = Boolean(args[0]); },
  "Desktop.TrayDisplay": () => state.tray,
  "Desktop.SetTrayDisplay": (args) => { state.tray = String(args[0] || "both"); },
  "Desktop.OpenDashboard": () => undefined,
  "Desktop.ClosePanel": () => undefined,
  "Desktop.TrayReady": () => undefined,
  "Desktop.SetPageBusy": () => undefined,
  "Desktop.Apps": (args) => Object.fromEntries((args[0] as string[]).filter(path => path.includes(".app/")).map(path => {
    const name = path.split("/").find(part => part.endsWith(".app"))!.slice(0, -4);
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect x="4" y="4" width="56" height="56" rx="13" fill="#2f6fde"/><text x="32" y="42" font-size="28" text-anchor="middle" fill="#fff" font-family="sans-serif">${name[0]}</text></svg>`;
    return [path, { name, icon: "data:image/svg+xml;base64," + btoa(svg) }];
  })),
  "Desktop.Quit": () => undefined,
  "Desktop.RequestWiFiAccess": () => { state.wifiState = { ...state.wifiState, known: true, detail: "" }; },
  "Desktop.ReadProfilePath": (args) => {
    const path = String(args[0]).trim();
    if (!/\.ya?ml["']?$/i.test(path)) fail("请选择 .yaml 或 .yml 配置文件");
    return { path, name: path.split("/").pop()!.replace(/\.ya?ml["']?$/i, ""), content: sampleYaml };
  },
  "Desktop.PickProfileFile": (): ImportedProfileFile => ({ path: "示例配置.yaml", name: "示例配置.yaml", content: sampleYaml }),

  "Profiles.State": () => profileState(),
  "Profiles.Summary": () => ({ activeId: state.activeId, mixedPort: state.mixedPort, systemProxy: state.systemProxy }),
  "Profiles.Initialize": () => undefined,
  "Profiles.SetPort": (args) => {
    const port = Number(args[0]);
    if (!Number.isInteger(port) || port < 1 || port > 65535) fail("端口必须在 1 到 65535 之间");
    if (state.running) fail("请先停止内核，再修改混合代理端口");
    state.mixedPort = port;
  },
  "Profiles.SetSystemProxy": (args) => { state.systemProxy = Boolean(args[0]); },
  "Profiles.ChangeTun": (args) => {
    const enabled = Boolean(args[0]);
    if (state.running && !enabled && !state.systemProxy) stopCore();
    state.tun = enabled;
  },
  "Profiles.TunEnabled": () => state.tun,
  "Profiles.AutoConnect": () => state.autoConnect,
  "Profiles.SetAutoConnect": (args) => { state.autoConnect = Boolean(args[0]); },
  "Profiles.TrayDisplay": () => state.tray,
  "Profiles.SetTrayDisplay": (args) => { state.tray = String(args[0] || "both"); },
  "Profiles.Use": (args) => {
    const id = String(args[0]);
    if (!state.profiles.some(item => item.id === id)) fail("配置不存在");
    state.activeId = id;
    return profileState();
  },
  "Profiles.Delete": (args) => {
    const id = String(args[0]);
    if (state.activeId === id) fail("正在使用的配置不能删除");
    state.profiles = state.profiles.filter(item => item.id !== id);
    return profileState();
  },
  "Profiles.Update": (args) => {
    const found = state.profiles.find(item => item.id === String(args[0]));
    if (!found) fail("配置不存在");
    found.updatedAt = new Date().toISOString();
    found.lastError = "";
    return profileState();
  },
  "Profiles.Fetch": (): SubscriptionDownload => ({ content: sampleYaml, usage: null, updatedAt: new Date().toISOString() }),
  "Profiles.Save": (args) => {
    const incoming = args[0] as Profile;
    const now = new Date().toISOString();
    const existing = state.profiles.find(item => item.id === incoming.id);
    if (existing) Object.assign(existing, incoming, { updatedAt: now, lastError: "" });
    else {
      const id = incoming.id || `preview-${state.nextId++}`;
      state.profiles.push({ ...incoming, id, updatedAt: now, lastError: "", lastAttempt: now });
      if (!state.activeId) state.activeId = id;
    }
    return profileState();
  },
  "Profiles.StartActive": () => startCore(),
  "Profiles.StartDraft": () => startCore(),
  "Profiles.Overrides": () => overrideState(),
  "Profiles.SetOverrides": (args) => {
    const expected = Number(args[1]);
    if (expected !== state.overrides.revision) fail("覆盖规则已被修改，请重新加载");
    state.overrides.groups = clone(args[0] as OverrideGroup[]);
    state.overrides.revision += 1;
    return overrideState();
  },
  "Profiles.DNSSettings": () => clone(state.dns),
  "Profiles.SetDNSSettings": (args) => { state.dns = clone(args[0] as DNSOptions); return clone(state.dns); },
  "Profiles.CoreSettings": () => clone(state.core),
  "Profiles.SetCoreSettings": (args) => { state.core = clone(args[0] as CoreOptions); return clone(state.core); },
  "Profiles.TunSettings": () => clone(state.tunOptions),
  "Profiles.SetTunSettings": (args) => {
    if (state.running) fail("请先停止内核，再修改 TUN 高级参数");
    state.tunOptions = clone(args[0] as TunOptions);
    return clone(state.tunOptions);
  },
  "Profiles.GeoSettings": () => clone(state.geo),
  "Profiles.SetGeoSettings": (args) => { state.geo = clone(args[0] as GeoOptions); return clone(state.geo); },
  "Profiles.InboundSettings": () => clone(state.inbound),
  "Profiles.SetInboundSettings": (args) => {
    if (state.running) fail("请先停止内核，再修改入站端口或代理认证");
    state.inbound = clone(args[0] as InboundOptions);
    return clone(state.inbound);
  },
  "Profiles.ControllerSettings": () => clone(state.controller),
  "Profiles.SetControllerSettings": (args) => {
    if (state.running) fail("请先停止内核，再修改控制接口");
    state.controller = clone(args[0] as ControllerOptions);
    return clone(state.controller);
  },
  "Profiles.LANSettings": () => clone(state.lan),
  "Profiles.SetLANSettings": (args) => {
    if (state.running) fail("请先停止内核，再修改局域网共享");
    state.lan = clone(args[0] as LANSettings);
    return clone(state.lan);
  },
  "Profiles.LANInterfaces": () => clone(state.interfaces),
  "Profiles.ProxyBypass": () => clone(state.bypass),
  "Profiles.SetProxyBypass": (args) => {
    if (state.running) fail("请先停止内核，再修改代理例外");
    state.bypass = clone(args[0] as ProxyBypassSettings);
    return clone(state.bypass);
  },
  "Profiles.SpeedSettings": () => clone(state.speed),
  "Profiles.SetSpeedSettings": (args) => { state.speed = clone(args[0] as SpeedSettings); return clone(state.speed); },
  "Profiles.WiFiSettings": () => clone(state.wifi),
  "Profiles.SetWiFiSettings": (args) => { state.wifi = clone(args[0] as WiFiSettings); return clone(state.wifi); },

  "Service.Status": () => status(),
  "Service.NodeLocations": (args) => Object.fromEntries(((args[0] as string[]) || []).map((name) => [name, {
    // a relay: enters in Shanghai, lands where the name says
    entry: { known: true, city: "Shanghai", region: "Shanghai", country: "CN", lon: 121.47, lat: 31.23, source: "ipinfo.io", error: "" },
    exit: /洛杉矶|美国|US/.test(name) ? { known: true, city: "Los Angeles", region: "California", country: "US", lon: -118.24, lat: 34.05, source: "ipinfo.io", error: "" }
      : { known: true, city: "Tokyo", region: "Tokyo", country: "JP", lon: 139.69, lat: 35.69, source: "ipinfo.io", error: "" },
    // the measured path from here to the entry
    forward: /洛杉矶|美国|US/.test(name) ? {
      known: true, reached: true, silent: 2, error: "",
      hops: [{ ip: "192.168.1.1", ms: 2 }, { ip: "100.64.0.1", ms: 6 }, { ip: "", ms: -1 }, { ip: "202.97.33.1", ms: 9 }, { ip: "202.97.50.2", ms: 14 }, { ip: "59.43.80.1", ms: 21 }, { ip: "59.43.183.5", ms: 156 }, { ip: "203.0.113.9", ms: 162 }],
      segments: [
        { label: "本地网络", kind: "local", hops: 1, ms: 2 },
        { label: "城域网", kind: "carrier", hops: 2, ms: 6 },
        { label: "电信 163", kind: "carrier", hops: 2, ms: 14 },
        { label: "CN2 GT", kind: "premium", hops: 2, ms: 156 },
        { label: "入口", kind: "entry", hops: 1, ms: 162 },
      ],
    } : {
      known: true, reached: true, silent: 1, error: "",
      hops: [{ ip: "192.168.1.1", ms: 2 }, { ip: "100.64.0.1", ms: 5 }, { ip: "59.43.80.1", ms: 9 }, { ip: "203.0.113.20", ms: 12 }],
      segments: [
        { label: "本地网络", kind: "local", hops: 1, ms: 2 },
        { label: "城域网", kind: "carrier", hops: 1, ms: 5 },
        { label: "CN2 GIA", kind: "premium", hops: 1, ms: 9 },
        { label: "入口", kind: "entry", hops: 1, ms: 12 },
      ],
    },
    pending: false,
  }])),
  "Service.LocalIPLocation": () => ({ known: true, city: "Guangzhou", region: "Guangdong", country: "CN", lon: 113.25, lat: 23.12, source: "ipinfo.io", error: "" }),
  "Service.TunServiceInstalled": () => state.tunService,
  "Service.TunDebugLog": () => "==> ~/Library/Logs/RocketScience/tun.log <==\n（预览模式没有日志）\n",
  "Service.UninstallTunService": () => { if (state.running && state.tun) fail("TUN 正在使用，请先关闭 TUN 再卸载服务"); state.tunService = false; },
  "Service.Monitoring": () => traffic(),
  "Service.LiveTelemetry": () => traffic().telemetry,
  "Service.Streams": () => { requireRunning(); return streams(); },
  "Service.Groups": () => { requireRunning(); return withRuleCounts(clone(state.groups)); },
  // Stopped: the configured groups with the choices the next start applies.
  // Automatic groups decide only after the core tests them.
  "Profiles.PreviewGroups": () => withRuleCounts(clone(state.groups).filter(item => item.name !== "GLOBAL").map(item => ({ ...item, now: item.type === "Selector" ? item.now : "", nodes: item.nodes.map(n => ({ ...n, delay: null, delayAt: "" })) }))),
  "Profiles.SelectPreview": (args) => {
    if (state.running) fail("内核运行中，请直接选择节点");
    if (!state.activeId) fail("没有正在使用的配置");
    const group = state.groups.find(item => item.name === String(args[0]) && item.name !== "GLOBAL");
    const nodeName = String(args[1]);
    if (!group) fail("策略组不存在");
    if (group.type !== "Selector") fail("只有手动选择的策略组可以指定节点");
    if (!group.all.includes(nodeName)) fail("节点不在该策略组中");
    group.now = nodeName;
  },
  "Service.Connections": () => { requireRunning(); return connections(); },
  "Service.Rules": () => { requireRunning(); return effectiveRules().rules; },
  "Service.SetMode": (args) => {
    const mode = String(args[0]);
    if (mode !== "rule" && mode !== "global" && mode !== "direct") fail("无效的出站模式");
    state.mode = mode;
  },
  "Service.SetSystemProxy": (args) => {
    const enabled = Boolean(args[0]);
    if (state.running && !enabled && !state.tun) stopCore();
    state.systemProxy = enabled;
  },
  "Service.SetTun": (args) => {
    const enabled = Boolean(args[0]);
    if (state.running && !enabled && !state.systemProxy) stopCore();
    state.tun = enabled;
  },
  "Service.Start": () => startCore(),
  "Service.Stop": () => stopCore(),
  "Service.Restart": () => {
    requireRunning();
    state.startedAt = new Date().toISOString();
  },
  "Service.Shutdown": () => stopCore(),
  "Service.RecoverNetwork": () => { state.recoveryPending = false; state.warning = ""; stopCore(); },
  "Service.Select": (args) => {
    requireRunning();
    const group = state.groups.find(item => item.name === String(args[0]));
    const nodeName = String(args[1]);
    if (!group || !group.all.includes(nodeName)) fail("节点不存在");
    group.now = nodeName;
  },
  "Service.SelectRecommendation": (args) => {
    requireRunning();
    const group = state.groups.find(item => item.name === String(args[0]));
    if (!group || group.now !== String(args[1])) fail("推荐已过期，请重新检查");
    const nodeName = String(args[2]);
    if (!group.all.includes(nodeName)) fail("推荐节点已不存在，请重新检查");
    group.now = nodeName;
  },
  "Service.TestGroup": (args) => {
    requireRunning();
    const group = state.groups.find(item => item.name === String(args[0]));
    if (!group) fail("策略组不存在");
    const values: Record<string, number> = {};
    group.all.forEach((name, index) => { values[name] = 70 + index * 48; });
    return values;
  },
  "Profiles.TestPreview": (args) => {
    if (state.running) fail("内核运行中，请直接测速");
    if (!state.activeId) fail("没有正在使用的配置");
    const group = state.groups.find(item => item.name === String(args[0]));
    if (!group) fail("策略组不存在");
    const values: Record<string, number> = {};
    group.all.forEach((name, index) => { values[name] = state.groups.some(item => item.name === name) ? -1 : 90 + index * 52; });
    return values;
  },
  "Service.CloseConnection": (args) => {
    requireRunning();
    state.connections = state.connections.filter(item => item.id !== String(args[0]));
  },
  "Service.CloseAllConnections": () => { requireRunning(); state.connections = []; },
  "Service.CheckConnectivity": (): Connectivity => {
    state.running = true;
    return {
      local: true,
      localStatus: "ok",
      localDetail: "无代理状态下本机网络正常",
      core: true,
      takeoverStatus: "ok",
      takeoverDetail: "开启代理后网络正常，未导致断网",
      autoExited: false,
      request: true,
      message: "测试请求通过",
      nodeStatus: "ok",
      nodeDetail: "节点「美国 02」连接成功（86 ms）",
      verifiedNode: "美国 02",
      switched: false,
      recommendations: [],
    };
  },
  "Service.KernelInfo": () => kernelInfo(),
  "Service.UpdateGeo": () => { requireRunning(); },
  "Service.RecoverGeoCache": () => undefined,

  "WiFi.Settings": () => clone(state.wifi),
  "WiFi.SetSettings": (args) => {
    state.wifi = clone(args[0] as WiFiSettings);
    state.wifiState = { ...state.wifiState, enabled: state.wifi.enabled, trusted: state.wifi.trusted.includes(state.wifiState.ssid) };
    return clone(state.wifi);
  },
  "WiFi.State": () => clone(state.wifiState),
  "WiFi.Check": () => undefined,
};

/** Go events never fire in the browser preview, so subscribing does nothing. */
export function event<T>(name: string) {
  return {
    name,
    on: (_listener: (payload: T) => void) => () => {},
    once: (_listener: (payload: T) => void) => () => {},
  };
}

export async function call<T = unknown>(method: string, ...args: unknown[]): Promise<T> {
  const handler = handlers[method];
  if (!handler) fail(`预览未实现 ${method}`);
  return await handler(args) as T;
}
