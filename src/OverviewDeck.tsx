import React, { useEffect, useRef, useState } from "react";
import type * as Design from "../design-system/components/index";
import type {ConnectionSnapshot, Proxy, Status} from "./mygo";
import type {DelayMeasurement} from "./proxyNodes";
import {resolveOverviewExit} from "./overviewExit";
import {CopyButton} from "./CopyButton";
import {TrajectoryPanel} from "./TrajectoryPanel";

type Phase = "idle" | "igniting" | "orbit";

const MODES = [
  {value: "rule", label: "规则"},
  {value: "global", label: "全局"},
  {value: "direct", label: "直连"},
];

export function modeLabel(mode: string | undefined) {
  return MODES.find(item => item.value === mode)?.label || "—";
}

export function takeoverSummary(systemProxy: boolean, tun: boolean) {
  const ways = [systemProxy && "系统代理", tun && "TUN"].filter(Boolean);
  return ways.length ? ways.join(" + ") : "未接管";
}

export const LAUNCH_VISUAL_MS = 1200;

export function ignitionButtonState(input: { initialized: boolean; running: boolean; starting: boolean; stopping: boolean; hold: boolean }): Phase {
  if (!input.initialized || input.starting || input.stopping || (input.hold && input.running)) return "igniting";
  return input.running ? "orbit" : "idle";
}

function useTimedHold(active: boolean, ms: number) {
  const [hold, setHold] = useState(false);
  const timer = useRef(0);
  useEffect(() => {
    if (!active) return;
    setHold(true);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setHold(false), ms);
  }, [active, ms]);
  useEffect(() => () => window.clearTimeout(timer.current), []);
  return hold;
}

export function takeoverNote(phase: Phase, on: boolean, detail?: string) {
  if (!on) return "";
  if (phase === "orbit") return "已接管" + (detail ? " · " + detail : "");
  if (phase === "igniting") return "正在接管…";
  return "";
}

const switchDescription = (scope: string, note: string) => note ? `${scope} · ${note}` : scope;

export function OverviewDeck({design: DS, status, groups = [], delays, busy, starting = false, initialized, portText, streamsMemory, history, telemetry, connections, showFirstRun, onStartTour, onDismissFirstRun, onToggle, onMode, onSystemProxy, onTun, onCopyCommand, onSpeedTest}: {
  design: typeof Design;
  status: Status | null;
  groups?: Proxy[];
  delays?: Record<string, DelayMeasurement>;
  busy: boolean;
  starting?: boolean;
  initialized: boolean;
  portText: string;
  streamsMemory: {connected: boolean; inuse: number} | null;
  history: {up: number[]; down: number[]} | null;
  telemetry: {connected: boolean; up: number; down: number} | null;
  connections: ConnectionSnapshot | null;
  showFirstRun: boolean;
  onStartTour: () => void;
  onDismissFirstRun: () => void;
  onToggle: () => void;
  onMode: (mode: string) => void;
  onSystemProxy: (enabled: boolean) => void;
  onTun: (enabled: boolean) => void;
  onCopyCommand: () => Promise<void>;
  onSpeedTest: () => void;
}) {
  const running = !!status?.running;
  const stopping = busy && running && !starting;
  const hold = useTimedHold(starting, LAUNCH_VISUAL_MS);
  const buttonState = ignitionButtonState({ initialized, running, starting, stopping, hold });
  const phase: Phase = buttonState === "igniting" && !stopping ? "igniting" : running ? "orbit" : "idle";
  const summary = takeoverSummary(!!status?.systemProxy, !!status?.tun);
  const exit = running && groups.length ? resolveOverviewExit({ groups, mode: status?.mode, delays, flagOf: DS.flagOf }) : null;
  const subtitle = stopping
    ? "正在停止并恢复网络"
    : buttonState === "igniting"
      ? `正在启动 · 接管 ${summary}`
      : running
        ? exit
          ? `${exit.text} · ${modeLabel(status?.mode)} · ${summary}`
          : `${modeLabel(status?.mode)} · ${summary}`
        : summary === "未接管" ? "" : `启动后使用${summary}`;
  const memory = running && streamsMemory?.connected ? `${(streamsMemory.inuse / 1048576).toFixed(1)} MB` : "—";
  const stats: Array<[string, string]> = [
    ["连接数", running && connections ? String(connections.connections.length) : "—"],
    ["规则数", "—"],
    ["应用内存", memory],
    ["内核", status?.version || "—"],
  ];
  return <>
    {showFirstRun && initialized && <DS.FirstRunCard topics={["导入配置", "启动", "测速选节点", "检查网络与模式"]} description="了解导入、启动、选节点和恢复网络。开始前会先停止代理并恢复原有设置。" onStart={onStartTour} onDismiss={onDismissFirstRun}/>}
    <div className="hero">
      <div className="panel">
        <div id="tour-launch">
          <DS.IgnitionButton state={buttonState} since={undefined} subtitle={subtitle} copy={stopping ? {igniting: {title: "停止中", action: "停止中"}} : undefined} onToggle={onToggle}/>
        </div>
        <div className="tele">
          <DS.TelemetryStat direction="down" value={telemetry?.connected ? telemetry.down : null} total={connections?.downloadTotal} history={telemetry?.connected ? history?.down : []}/>
          <DS.TelemetryStat direction="up" value={telemetry?.connected ? telemetry.up : null} total={connections?.uploadTotal} history={telemetry?.connected ? history?.up : []}/>
        </div>
        <DS.TelemetryHistory down={telemetry?.connected ? history?.down : []} up={telemetry?.connected ? history?.up : []}/>
      </div>
      <div className="panel quick">
        <div className="cap" style={{paddingBottom: 12}}>飞行控制</div>
        <div style={{padding: "12px 0"}}>
          <DS.SegmentedControl aria-label="出站模式" options={MODES} value={status?.mode || ""} onChange={onMode}/>
        </div>
        <div className="quick">
          <div className="cap" style={{display: "flex", justifyContent: "space-between", paddingTop: 12}}>接管方式{phase !== "orbit" && <span style={{textTransform: "none", letterSpacing: 0}}>启动后生效</span>}</div>
          <div className="quick__switches" id="tour-takeover">
          <DS.Switch label="系统代理" icon="globe" description={switchDescription("浏览器等大多数应用", takeoverNote(phase, !!status?.systemProxy, portText))} aria-label="系统代理" checked={!!status?.systemProxy} disabled={busy || !initialized} onChange={onSystemProxy}/>
          <DS.Switch label="TUN 增强模式" icon="shield" description={switchDescription("所有应用，包括命令行", takeoverNote(phase, !!status?.tun))} aria-label="TUN 增强模式" checked={!!status?.tun} disabled={busy || !initialized} onChange={onTun}/>
          </div>
        </div>
        <div className="flight-actions">
          <CopyButton design={DS} variant="secondary" label="复制终端代理命令" copy={onCopyCommand}/>
          <DS.Button size="sm" variant="ghost" icon="refresh" onClick={onSpeedTest}>节点测速</DS.Button>
        </div>
      </div>
    </div>
    <TrajectoryPanel design={DS} running={running} connections={connections}/>
    <div className="stats panel">
      {stats.map(([label, value]) => <div className="stat" key={label}><div className="cap">{label}</div><b>{value}</b></div>)}
    </div>
  </>;
}
