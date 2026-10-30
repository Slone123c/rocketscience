import React, { useEffect, useRef, useState } from "react";
import type * as Design from "../design-system/components/index";
import type {ConnectionSnapshot, Status} from "./mygo";
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
  if (!on) return "不接管";
  if (phase === "orbit") return "已接管" + (detail ? " · " + detail : "");
  if (phase === "igniting") return "正在接管…";
  return "启动后接管";
}

export function OverviewDeck({design: DS, status, busy, starting = false, initialized, portText, streamsMemory, history, telemetry, connections, showFirstRun, onStartTour, onDismissFirstRun, onToggle, onMode, onSystemProxy, onTun, onCopyCommand, onSpeedTest}: {
  design: typeof Design;
  status: Status | null;
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
  const subtitle = stopping
    ? "正在停止并恢复网络"
    : buttonState === "igniting"
      ? `正在启动 mihomo · 接管 ${summary}`
      : running
        ? `${modeLabel(status?.mode)} · ${summary}`
        : summary === "未接管" ? "内核未运行" : `内核未运行 · 启动后接管 ${summary}`;
  const memory = running && streamsMemory?.connected ? `${(streamsMemory.inuse / 1048576).toFixed(1)} MB` : "—";
  const stats: Array<[string, string]> = [
    ["连接数", running && connections ? String(connections.connections.length) : "—"],
    ["规则数", "—"],
    ["内存", memory],
    ["内核", status?.version || "—"],
  ];
  return <>
    {showFirstRun && initialized && <DS.FirstRunCard description="了解导入、启动、选节点和恢复网络。开始前会先停止代理并恢复原有设置。" onStart={onStartTour} onDismiss={onDismissFirstRun}/>}
    <div className="hero">
      <div className="panel">
        <div id="tour-launch">
          <DS.IgnitionButton state={buttonState} since={buttonState === "orbit" && status?.startedAt ? Date.parse(status.startedAt) : undefined} subtitle={subtitle} copy={stopping ? {igniting: {title: "停止中", action: "停止中"}} : undefined} onToggle={onToggle}/>
        </div>
        <div className="tele">
          <DS.TelemetryStat direction="down" value={telemetry?.connected ? telemetry.down : null} total={connections?.downloadTotal} history={telemetry?.connected ? history?.down : []}/>
          <DS.TelemetryStat direction="up" value={telemetry?.connected ? telemetry.up : null} total={connections?.uploadTotal} history={telemetry?.connected ? history?.up : []}/>
        </div>
      </div>
      <div className="panel quick">
        <div className="cap" style={{paddingBottom: 12}}>飞行控制</div>
        <div style={{padding: "12px 0"}}>
          <DS.SegmentedControl aria-label="出站模式" options={MODES} value={status?.mode || ""} onChange={onMode}/>
        </div>
        <div className="quick">
          <div className="cap" style={{display: "flex", justifyContent: "space-between", paddingTop: 12}}>接管方式{phase !== "orbit" && <span style={{textTransform: "none", letterSpacing: 0}}>启动后生效</span>}</div>
          <DS.Switch label="系统代理" icon="globe" description={"浏览器等大多数应用 · " + takeoverNote(phase, !!status?.systemProxy, portText)} aria-label="系统代理" checked={!!status?.systemProxy} disabled={busy || !initialized} onChange={onSystemProxy}/>
          <DS.Switch label="TUN 增强模式" icon="shield" description={"所有应用，包括命令行 · " + takeoverNote(phase, !!status?.tun)} aria-label="TUN 增强模式" checked={!!status?.tun} disabled={busy || !initialized} onChange={onTun}/>
          {!status?.systemProxy && !status?.tun && (phase === "orbit"
            ? <p className="lf-takeover__note">没有接管任何流量：内核在运行，但只有手动填了 {portText} 的应用会走代理。</p>
            : <p className="lf-takeover__warn">至少选择一种接管方式</p>)}
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
