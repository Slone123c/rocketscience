import type * as React from 'react';

export type SpringName = 'snap' | 'glide' | 'heavy' | 'orbit';
export interface Spring { mass: number; stiffness: number; damping: number; }

export type IconName =
  | 'rocket' | 'orbit' | 'globe' | 'route' | 'pulse' | 'terminal' | 'layers' | 'sliders' | 'bolt' | 'shield'
  | 'arrow-up' | 'arrow-down' | 'refresh' | 'chevron' | 'search' | 'check' | 'power' | 'gauge' | 'plus' | 'copy' | 'close'
  | 'help' | 'arrow-right' | 'edit' | 'minus' | 'more' | 'laptop' | 'server' | 'sort' | 'sort-asc' | 'sort-desc' | 'signal' | 'sun' | 'moon' | 'contrast'
  | 'alert-circle' | 'alert-triangle' | 'x-circle';

export interface IconProps {
  name: IconName;
  /** px, default 16. Strokes stay 1.5px at 16. */
  size?: number;
  /** Give a label only when the icon stands alone and means something. */
  label?: string;
  className?: string;
}

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  /** primary = ink fill, one per view; secondary = outlined; ghost = toolbar/inline. Default secondary. */
  variant?: 'primary' | 'secondary' | 'ghost';
  size?: 'sm' | 'md';
  icon?: IconName;
  iconAfter?: IconName;
  /** Add 'is-spinning' while an async action runs to spin the icon. */
  className?: string;
}

export interface SwitchProps {
  checked?: boolean;
  defaultChecked?: boolean;
  onChange?: (checked: boolean) => void;
  /** With a label the switch renders as a full settings row. */
  label?: string;
  description?: string;
  icon?: IconName;
  disabled?: boolean;
  'aria-label'?: string;
  className?: string;
}

export interface SegmentedOption { value: string; /** Omit for an icon-only segment, then give a title. */ label?: string; icon?: IconName; /** Tooltip and accessible name; required when label is omitted. */ title?: string; }
export interface SegmentedControlProps {
  options: SegmentedOption[];
  disabled?: boolean;
  value?: string;
  defaultValue?: string;
  onChange?: (value: string) => void;
  size?: 'md' | 'sm';
  'aria-label'?: string;
  className?: string;
}

export interface IgnitionButtonProps {
  /** idle = core stopped; igniting = starting; orbit = running with system proxy. */
  state?: 'idle' | 'igniting' | 'orbit';
  onToggle?: (action: 'launch' | 'land') => void;
  /** Epoch ms when orbit began; drives the T+ clock. */
  since?: number;
  /** e.g. "规则 · 自动选择 → 东京 02". */
  subtitle?: string;
  /** One-line failure reason shown in ignition, e.g. "7890 端口被占用". */
  error?: string;
  /** Defaults are Chinese (待命/发射/在轨, 启动/停止). Override per state, e.g. { orbit: { title: 'In orbit', action: 'Stop' } }. */
  copy?: Partial<Record<'idle' | 'igniting' | 'orbit', { title?: string; action?: string }>>;
  className?: string;
}

export interface TelemetryHistoryProps {
  /** Up to 24 recent download samples in bytes per second, oldest first. */
  down?: number[];
  /** Up to 24 recent upload samples in bytes per second, oldest first. */
  up?: number[];
  className?: string;
}

export interface TelemetryStatProps {
  direction: 'up' | 'down';
  /** bytes per second; null shows an em dash. */
  value: number | null;
  /** Up to 24 recent samples, oldest first. The highest one sets the peak hand. */
  history?: number[];
  /** Cumulative bytes this session. */
  total?: number;
  label?: string;
  className?: string;
}

export interface LatencyBadgeProps {
  /** ms; 0 or 'timeout' = lost signal; undefined = not tested. */
  ms?: number | 'timeout' | null;
  testing?: boolean;
  className?: string;
}

export interface ProxyNodeProps {
  name: string;
  /** Protocol: Shadowsocks, VMess, Trojan, Hysteria2, TUIC, WireGuard, Direct… */
  type?: string;
  delay?: number | 'timeout' | null;
  udp?: boolean;
  selected?: boolean;
  testing?: boolean;
  /** Non-Selector groups: shows state, can't be picked. */
  readOnly?: boolean;
  disabled?: boolean;
  onSelect?: (name: string) => void;
  className?: string;
}

/** Order for node lists: the config's order, by name A→Z or Z→A (Chinese by pinyin initial, interleaved with Latin), or by latency. */
export type NameSort = 'none' | 'asc' | 'desc' | 'latency';
export interface NameSortProps {
  /** Controlled order; leave out to let the component keep its own. */
  sort?: NameSort;
  /** Starting order when uncontrolled; default 'none'. */
  defaultSort?: NameSort;
  onSortChange?: (sort: NameSort) => void;
}
export interface SortMenuProps {
  value: NameSort;
  onChange: (sort: NameSort) => void;
  className?: string;
}
export interface ProxyGroupProps extends NameSortProps {
  /** Observed active connections traversing this group. */
  activity?: number;
  /** Disable node selection (automatic groups, or a stopped preview with nothing to choose). Testing stays on. */
  readOnly?: boolean;
  /** Disable the latency test, e.g. when nothing in the group can be measured. */
  testDisabled?: boolean;
  /** What the readout means, before the via/leaf chip. Defaults to 当前节点 (Selector) or 自动选中. */
  label?: string;
  name: string;
  type?: 'Selector' | 'URLTest' | 'Fallback' | 'LoadBalance' | 'Relay';
  /** Currently selected member (mihomo `now`). */
  now?: string;
  /** Final node after following nested groups; replaces the `now` readout when set. */
  leaf?: string;
  /** Groups passed before `leaf`, shown faint, e.g. "自动选择 →". */
  via?: string;
  /** Faint note after the type, e.g. "356 条规则 · 兜底". */
  meta?: string;
  nodes: Array<Pick<ProxyNodeProps, 'name' | 'type' | 'delay' | 'udp'>>;
  onSelect?: (name: string) => void;
  onTest?: () => void;
  testing?: boolean;
  defaultOpen?: boolean;
  className?: string;
}

export interface TrajectoryNode {
  name: string;
  /** Explicit coordinates (where traffic lands); otherwise resolved from the name by geoOf (city, then flag). */
  lon?: number;
  lat?: number;
  /** [lon, lat] of the server traffic enters. Kept for one relay; `hops` draws every city. */
  entry?: [number, number];
  /** Located cities in order, each drawn as its own marker. `at` is [lon, lat]. */
  hops?: Array<{ at: [number, number]; place?: string; country?: string }>;
  /** Located address of the exit, drawn faint under the node name. */
  place?: string;
  /** ISO code the exit's IP was located in; draws its flag beside `place` in place of the flag guessed from the name. */
  country?: string;
  /** 0–1 fallback position when the name resolves to no place. */
  x?: number;
  y?: number;
  active?: boolean;
  /** Relative bandwidth 1–4: thickens the active route. */
  weight?: number;
  delay?: number;
  /** Position is inferred from the node's name rather than verified. */
  estimated?: boolean;
  /** Located entry ("Hong Kong, HK"), shown with `place` in the row under the map. */
  entryPlace?: string;
  /** Entry and exit are in different countries. */
  relay?: boolean;
  /** Measured outbound line; rows appear under the map for nodes with a route, entry or trace in progress. */
  route?: TrajectoryRoute;
  /** The outbound trace is still running. */
  tracing?: boolean;
}
export interface TrajectoryProps {
  nodes: TrajectoryNode[];
  /** Label at the origin crosshair; default "本机", "" draws the crosshair alone. */
  origin?: string;
  /** ISO code this Mac's public IP was located in; draws its flag before `origin`. */
  originCountry?: string;
  /** Zoom & pan; default true. */
  zoomable?: boolean;
  /** Default 5. */
  maxZoom?: number;
  /** [lon, lat] of this Mac; null hides the unknown origin. */
  originGeo?: [number, number] | null;
  /** Longitude at the map's middle; default 150 (Europe left, Asia centre, Americas right). */
  centerLon?: number;
  caption?: string;
  width?: number;
  height?: number;
  className?: string;
}

export interface RouteHopMark {
  ip?: string;
  ms: number;
  /** Country code from the local GeoIP database, when it knows the hop. */
  place?: string;
}
export interface RouteSegment {
  /** "电信 163", "CN2 GIA", "本地网络" */
  label: string;
  /** local / entry: the two ends; carrier: a backbone or metro network; premium: a paid line (CN2, 9929, CMI); intl: the international leg; unreached: where an unfinished trace stopped. */
  kind?: 'local' | 'carrier' | 'premium' | 'intl' | 'entry' | 'unreached';
  hops?: number;
  /** Latency where the segment ends. */
  ms?: number;
  /** The segment's hops in order, one per hop: drawn as dots, hover shows the address. ms -1 = never answered (drawn as ---). Paces the replay. */
  hopList?: RouteHopMark[];
}
/** Outbound path from this Mac to the node's entry, measured by the app. */
export interface TrajectoryRoute {
  segments?: RouteSegment[];
  error?: string;
}

export interface Connection {
  id: string;
  host: string;
  port?: number;
  network?: 'TCP' | 'UDP';
  process?: string;
  /** Display name of the owning app (bundle name); falls back to process. */
  app?: string;
  /** Full executable path; a path outside any .app marks a command-line tool. */
  processPath?: string;
  /** The owning app's icon (data URI or URL), read from its bundle by the native side. */
  icon?: string;
  rule: string;
  /** Group → … → node. */
  chain: string[];
  up: number;
  down: number;
}
export interface ConnectionTableProps {
  connections: Connection[];
  onClose?: (id: string) => void;
  /** Right-click / ContextMenu key / ⇧F10 on a row, or its ⋯ button. Rows become focusable when set. */
  onContextMenu?: (connection: Connection, at: { x: number; y: number }) => void;
  empty?: string;
  className?: string;
}

export interface LogLine { time: string; level: 'debug' | 'info' | 'warning' | 'error'; message: string; }
export interface LogStreamProps {
  lines: LogLine[];
  /** Initial minimum level. Default info. */
  level?: LogLine['level'];
  /** Text filter applied to messages. */
  query?: string;
  showFilter?: boolean;
  className?: string;
}

export interface ProfileCardProps {
  name: string;
  source: 'url' | 'file';
  url?: string;
  updated?: string;
  usage?: { used: number; total: number };
  expires?: string;
  active?: boolean;
  updating?: boolean;
  error?: string;
  onUse?: () => void;
  onUpdate?: () => void;
  /** Shows 编辑; open ProfileEditor. */
  onEdit?: () => void;
  className?: string;
}

export interface SearchFieldProps extends React.InputHTMLAttributes<HTMLInputElement> {
  onValueChange?: (value: string) => void;
  /** e.g. "⌘F" */
  shortcut?: string;
}

export interface SidebarChild { id: string; label: string; group?: string; }
export interface SidebarItem { id: string; label: string; icon: IconName; badge?: string | number; children?: SidebarChild[]; }
export interface SidebarProps {
  items: SidebarItem[];
  active: string;
  onSelect?: (id: string) => void;
  activeChild?: string;
  onSelectChild?: (parent: string, child: string) => void;
  title?: string;
  footer?: React.ReactNode;
  className?: string;
}

export interface TrayIndicatorProps {
  /** The three menu-bar display modes. Default both. */
  display?: 'icon' | 'speed' | 'both';
  /** bytes per second */
  up?: number;
  down?: number;
  state?: 'idle' | 'igniting' | 'orbit';
  /** True while the popover is open. */
  active?: boolean;
  onClick?: () => void;
  className?: string;
}

export interface MenuBarPopoverProps extends NameSortProps {
  /** Drives the enter/exit spring. Default true. */
  open?: boolean;
  state?: 'idle' | 'igniting' | 'orbit';
  since?: number;
  subtitle?: string;
  onToggle?: (action: 'launch' | 'land') => void;
  copy?: IgnitionButtonProps['copy'];
  speed?: { up: number | null; down: number | null; history?: { up: number[]; down: number[] } };
  mode?: 'rule' | 'global' | 'direct';
  onModeChange?: (mode: string) => void;
  groups?: Array<Pick<ProxyGroupProps, 'name' | 'type' | 'now' | 'nodes' | 'readOnly'>>;
  defaultGroup?: string;
  onSelect?: (group: string, node: string) => void;
  onTest?: (group: string) => void;
  /** Name of the group whose latency test is running. */
  testing?: string | null;
  systemProxy?: boolean;
  onSystemProxyChange?: (on: boolean) => void;
  tun?: boolean;
  onTunChange?: (on: boolean) => void;
  /** Current Wi-Fi name; the bypass row shows only when onSsidBypassChange is given. */
  ssid?: string;
  ssidBypass?: boolean;
  onSsidBypassChange?: (on: boolean) => void;
  onCopyCommand?: () => void | Promise<void>;
  onOpenDashboard?: () => void;
  onQuit?: () => void;
  className?: string;
}

export interface FlagProps {
  /** Node name as the subscription gives it. */
  name: string;
  /** Show a faint globe when no country/region matches. */
  placeholder?: boolean;
}

export interface TourStep {
  title: string;
  body: React.ReactNode;
  /** CSS selector of the element to spotlight; omit for a centred card. */
  target?: string;
  placement?: 'right' | 'bottom' | 'left' | 'top';
  /** Small caption after the step count, e.g. "第 2 步 · 启动". */
  kicker?: string;
  media?: React.ReactNode;
  /** One-line invitation to try something. */
  hint?: string;
  /** Host view to show for this step; switch to it in onStepChange. */
  view?: string;
  /** Space around the spotlit element, px. Default 6. */
  padding?: number;
  /** Clicking the spotlit control moves to the next step. */
  advanceOnClick?: boolean;
  /** ms after that click before moving on. Default 900. */
  advanceDelay?: number;
  /** Label of the last step's button. Default 开始使用. */
  doneLabel?: string;
}
export interface TourProps {
  steps: TourStep[];
  open?: boolean;
  step?: number;
  onStepChange?: (index: number, step: TourStep) => void;
  /** completed = true when they reached the end. */
  onClose?: (completed: boolean) => void;
  skipLabel?: string;
  doneLabel?: string;
}

export interface FirstRunCardProps {
  onStart?: () => void;
  onDismiss?: () => void;
  title?: string;
  description?: string;
  /** What the tour teaches, shown as numbered chips. */
  topics?: string[];
  startLabel?: string;
  className?: string;
}

export interface ConnectivityCheckItem {
  id: string;
  label: string;
  status?: 'idle' | 'running' | 'ok' | 'fail' | 'skip';
  detail?: string;
  /** The one fix offered on a failed row, e.g. 换到可用节点. */
  action?: { label: string; onClick: () => void };
}
export interface ConnectivityCheckProps {
  /** Narrow layout (used inside CheckPopover): icon-only run button, details under labels, full-width fixes. */
  compact?: boolean;
  checks: ConnectivityCheckItem[];
  onRun?: () => void;
  /** Stop the core and turn off system proxy / TUN. Shown whenever a check fails. */
  onRecover?: () => void;
  /** True right after onRecover ran. */
  recovered?: boolean;
  title?: string;
  idleText?: string;
  className?: string;
}

export declare const ConnectivityCheck: React.FC<ConnectivityCheckProps>;
export declare const Flag: React.FC<FlagProps>;
export declare const Tour: React.FC<TourProps>;
export declare const FirstRunCard: React.FC<FirstRunCardProps>;
/** Country/region from a node name: { code: 'JP', emoji: '🇯🇵' } or null. */
export declare function flagOf(name: string): { code: string; emoji: string } | null;
/** A–Z toggle cycling 默认顺序 → A→Z → Z→A. */
export declare const SortMenu: React.FC<SortMenuProps>;
/** Compare two names: flags skipped, Han by pinyin initial among Latin letters, numbers as numbers. */
export declare function compareNames(a: string, b: string): number;
export declare function sortByName<T>(list: T[], sort: Exclude<NameSort, 'latency'>, nameOf?: (item: T) => string): T[];
/** Fastest first, then timeouts, then untested, each by name; other orders as sortByName. */
export declare function sortNodes<T extends { name: string; delay?: number | 'timeout' | null }>(list: T[], sort: NameSort): T[];
export declare const TrayIndicator: React.FC<TrayIndicatorProps>;
export declare const MenuBarPopover: React.FC<MenuBarPopoverProps>;
export declare const Button: React.FC<ButtonProps>;
export declare const Switch: React.FC<SwitchProps>;
export declare const SegmentedControl: React.FC<SegmentedControlProps>;
export declare const IgnitionButton: React.FC<IgnitionButtonProps>;
export declare const TextField: React.FC<TextFieldProps>;
export declare const Select: React.FC<SelectProps>;
export declare const CodeEditor: React.FC<CodeEditorProps>;
export declare const Sheet: React.FC<SheetProps>;
export declare const ProfileEditor: React.FC<ProfileEditorProps>;
export declare const OverrideGroup: React.FC<OverrideGroupProps>;
export declare const RuleGuide: React.FC<RuleGuideProps>;
/** Line number named in a mihomo error, e.g. 'yaml: line 9: …' → 9. */
export declare function errorLineOf(message: string | null | undefined): number | null;
export declare function normalizeRule(type: RuleType, raw: string): { type: RuleType | 'IP-CIDR6'; payload: string } | { error: string };
export declare function geoOf(name: string): [number, number] | null;
export declare const RULE_TYPES: Array<{ value: RuleType; label: string; hint: string }>;
export declare const TelemetryStat: React.FC<TelemetryStatProps>;
export declare const TelemetryHistory: React.FC<TelemetryHistoryProps>;
export declare const LatencyBadge: React.FC<LatencyBadgeProps>;
export declare const ProxyNode: React.FC<ProxyNodeProps>;
export declare const ProxyGroup: React.FC<ProxyGroupProps>;
export declare const Trajectory: React.FC<TrajectoryProps>;
export declare const ConnectionTable: React.FC<ConnectionTableProps>;
export declare const LogStream: React.FC<LogStreamProps>;
export declare const ProfileCard: React.FC<ProfileCardProps>;
export declare const SearchField: React.FC<SearchFieldProps>;
export declare const Sidebar: React.FC<SidebarProps>;
export declare const Icon: React.FC<IconProps>;

/** Spring presets: mass / stiffness / damping. */
export declare const springs: Record<SpringName, Spring>;
/** A 1-D body on a spring; `to(target, {velocity, spring})`, `set(x)`, `onFrame(x, v)`. */
export declare class Body {
  constructor(x: number, spring?: Spring, eps?: number);
  x: number; v: number; target: number; spring: Spring;
  onFrame: ((x: number, v: number) => void) | null;
  to(target: number, opts?: { velocity?: number; spring?: Spring }): void;
  set(x: number): void;
  stop(): void;
}
export declare function useBody(initial: number, spring: Spring, onFrame: (x: number, v: number) => void, eps?: number): Body;
/** Resting point of a released body: x + v·m/c. */
export declare function project(x: number, v: number, spring: Spring): number;
/** Bake a spring into a CSS linear() easing plus the duration it needs. */
export declare function handoff(spring: Spring, samples?: number): { easing: string; duration: number };
export declare const fmt: {
  speed(bytesPerSecond: number | null): { value: string; unit: string };
  bytes(bytes: number): string;
  clock(seconds: number): string;
};

export interface TextFieldProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'className'> {
  label?: string;
  /** Help under the field; replaced by error. */
  description?: string;
  /** Shown in ignition and announced; set only after a save attempt. */
  error?: string | false | null;
  /** Monospace, for URLs, domains, IPs. */
  mono?: boolean;
  onValueChange?: (value: string) => void;
  className?: string;
}
export interface SelectProps {
  options: Array<{ value: string; label?: string }>;
  value: string;
  onChange?: (value: string) => void;
  /** With a label it renders as a field; without, pass aria-label. */
  label?: string;
  'aria-label'?: string;
  description?: string;
  mono?: boolean;
  disabled?: boolean;
  className?: string;
}
export interface CodeEditorProps {
  value: string;
  onChange?: (text: string) => void;
  /** 1-based line to mark, e.g. errorLineOf(mihomo's error). */
  errorLine?: number | null;
  height?: number | string;
  readOnly?: boolean;
  'aria-label'?: string;
  className?: string;
}
export interface SheetProps {
  open: boolean;
  onClose: () => void;
  title: string;
  kicker?: string;
  tabs?: React.ReactNode;
  footer?: React.ReactNode;
  width?: number | string;
  className?: string;
  children?: React.ReactNode;
}
export interface Profile {
  /** Absent while creating. */
  id?: string;
  name: string;
  source: 'url' | 'file';
  url?: string;
  /** Hours between automatic updates; '0' = off. */
  interval?: '0' | '6' | '12' | '24';
  path?: string;
  /** The YAML text. */
  content: string;
  /** mihomo's error from the last failed update or load, verbatim. */
  error?: string | null;
}
export interface ProfileEditorProps {
  open: boolean;
  /** Omit (or no id) to create a new profile. */
  profile?: Partial<Profile> | null;
  /** Run `mihomo -t` on the text; resolve to mihomo's error message, or null when it loads. */
  validate?: (content: string) => Promise<string | null> | string | null;
  onSave: (draft: Profile) => void | Promise<void>;
  onClose: () => void;
  onDelete?: () => void | Promise<void>;
  busy?: boolean;
  /** The profile in use: cannot be deleted; a changed file saves as 「保存并重新加载」. */
  active?: boolean;
  /** Whether saving the active profile will reload a running core. */
  running?: boolean;
  /** Open a file dialog, then call done with what was read. */
  onPickFile?: (done: (file: { path: string; name?: string; content: string }) => void) => void | Promise<void>;
  /** Shown on a subscription's file tab: jump to 覆盖规则. */
  onGoOverrides?: () => void;
  defaultTab?: 'info' | 'file';
  width?: number;
}
export type RuleType = 'DOMAIN-SUFFIX' | 'DOMAIN' | 'DOMAIN-KEYWORD' | 'IP-CIDR' | 'PROCESS-NAME';
export interface OverrideRule {
  type: RuleType | 'IP-CIDR6';
  payload: string;
  /** DIRECT, REJECT or a policy-group name. */
  policy: string;
}
export interface OverrideGroupProps {
  name: string;
  rules: OverrideRule[];
  enabled?: boolean;
  onToggle?: (on: boolean) => void;
  onChange: (rules: OverrideRule[]) => void;
  onRename?: (name: string) => void;
  onDelete?: () => void;
  /** Reorder from the ⋯ menu. Pass null at the top/bottom to show the item disabled; leave undefined to hide it. */
  onMoveUp?: (() => void) | null;
  onMoveDown?: (() => void) | null;
  /** The profile's group names; DIRECT and REJECT are always offered. */
  policies?: string[];
  defaultOpen?: boolean;
  className?: string;
}
export interface RuleGuideProps {
  onDismiss?: () => void;
  /** 「跟着做一遍」: start the rules walkthrough. */
  onStartTour?: () => void;
  /** Add the 局域网直连 template group. */
  onTemplate?: () => void;
  title?: string;
  description?: string;
  className?: string;
}

export interface CheckPopoverProps extends Omit<ConnectivityCheckProps, 'compact'> {
  /** Controlled open state; omit to let it manage itself. */
  open?: boolean;
  /** Start a check here when it opens and nothing has run yet. */
  onOpenChange?: (open: boolean) => void;
}
export declare const CheckPopover: React.FC<CheckPopoverProps>;

export type ContextMenuItem =
  | { label: string; icon?: IconName; onSelect?: () => void; primary?: boolean; disabled?: boolean; hint?: string; /** Set on a pick-one list: true shows a check at the end of the row. */ selected?: boolean }
  | { separator: true };
export interface ContextMenuProps {
  open: boolean;
  /** Viewport coordinates; the menu flips and clamps to stay on screen. */
  x: number;
  y: number;
  items: ContextMenuItem[];
  onClose: () => void;
  /** Mono line naming what was clicked, e.g. "api.github.com:443". */
  title?: string;
  label?: string;
}
export declare const ContextMenu: React.FC<ContextMenuProps>;

export interface OverrideGroupData { id: string; name: string; enabled: boolean; rules: OverrideRule[]; }
export interface RuleChoice { type: string; payload: string; label: string; desc: string; }
export interface AddRuleSheetProps {
  open: boolean;
  connection: Connection | null;
  groups: OverrideGroupData[];
  /** The group in use: last added to, else the first enabled one. */
  defaultGroupId?: string;
  /** Proxy group names; DIRECT and REJECT are always offered. */
  policies?: string[];
  /** groupId is null when the person chose 新建分组…; then newGroupName is set. */
  onAdd: (d: { rule: OverrideRule; groupId: string | null; newGroupName: string }) => void;
  onClose: () => void;
  width?: number;
}
export declare const AddRuleSheet: React.FC<AddRuleSheetProps>;
/** Match options for one connection, most useful first. */
export declare function ruleChoicesFor(connection: Connection): RuleChoice[];

export interface SettingsSectionProps { title: string; description?: React.ReactNode; status?: string | null; action?: React.ReactNode; id?: string; className?: string; children?: React.ReactNode; }
export declare const SettingsSection: React.FC<SettingsSectionProps>;
export interface SettingRowProps {
  label: React.ReactNode;
  description?: React.ReactNode;
  /** mihomo config key, shown on 高级 rows, e.g. "dns.enhanced-mode". */
  configKey?: string;
  /** Control below the text, for lists and long values. */
  stack?: boolean;
  disabled?: boolean;
  className?: string;
  children?: React.ReactNode;
}
export declare const SettingRow: React.FC<SettingRowProps>;
export interface SettingFieldProps {
  value: string | number | null;
  /** Called on Enter or blur when the value changed and validate passed. Numbers arrive as numbers (empty → 0). */
  onCommit?: (value: any) => void;
  validate?: (text: string) => string | null;
  type?: 'text' | 'number' | 'password';
  width?: number;
  suffix?: string;
  placeholder?: string;
  mono?: boolean;
  disabled?: boolean;
  'aria-label'?: string;
}
export declare const SettingField: React.FC<SettingFieldProps>;
export interface ListFieldProps {
  value: string[];
  onCommit?: (lines: string[]) => void;
  validate?: (line: string) => string | null;
  hint?: string;
  placeholder?: string;
  minRows?: number;
  maxRows?: number;
  disabled?: boolean;
  'aria-label'?: string;
}
export declare const ListField: React.FC<ListFieldProps>;
