import React, { useEffect, useRef, useState } from "react";
import { useAppearance } from "./appearance";
import { Desktop, Profiles, Service, WiFi, type WiFiStatus, type ProfileSummary, type Proxy, type Status, type Monitoring } from "./mygo";
import {proxyNodes,type DelayMeasurement} from "./proxyNodes";
import { proxyLayout } from "./proxyLayout";
import {createRefreshQueue} from "./viewRefresh";
import {createTrayTasks} from "./trayTasks";
import type * as Design from "../design-system/components/index";

export function TrayPanel({ design: DS }: { design: typeof Design }) {
  useAppearance();
  const [wifi,setWifi]=useState<WiFiStatus|null>(null);
  const [ready, setReady] = useState(false);
  const [status, setStatus] = useState<Status | null>(null);
  const [profiles, setProfiles] = useState<ProfileSummary | null>(null);
  const [groups, setGroups] = useState<Proxy[]>([]);
  const [streams, setStreams] = useState<Monitoring | null>(null);
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);
  const [delays, setDelays] = useState<Record<string,DelayMeasurement>>({});
  const [error, setError] = useState("");
  const tasks=useRef(createTrayTasks());
  const mounted=useRef(true);
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;tasks.current.cancelTest()}},[]);
  async function read() {
    const desktop = await Desktop.state();
    if(!mounted.current)return;
    setReady(desktop.startupReady);
    if (!desktop.startupReady) {const pending=await Service.status();if(mounted.current)setStatus(pending);return;}
    const next = await Service.status();
    if(next.starting){setStatus(next);setGroups([]);setStreams(null);return;}
    const saved = await Profiles.summary();
    if(!mounted.current)return;
    if(tasks.current.setSession(next.running ? JSON.stringify([next.startedAt,saved.activeId]) : "")){setDelays({});setTesting(null)}
    setStatus(next); setProfiles(saved);
    const nextWiFi=await WiFi.state();
    if(!mounted.current)return;
    setWifi(nextWiFi);
    if (next.running) {
      const [g, s] = await Promise.all([Service.groups(), Service.monitoring()]);
      if(!mounted.current)return;
      setGroups(g); setStreams(s);
    } else { setGroups([]); setStreams(null); }
  }
  const refreshQueue=useRef<ReturnType<typeof createRefreshQueue>|null>(null);
  if(!refreshQueue.current)refreshQueue.current=createRefreshQueue(read);
  async function refresh(){await refreshQueue.current!.request()}
  useEffect(()=>{let stopped=false,timer:ReturnType<typeof setTimeout>|undefined;async function tick(){if(!document.hidden)try{await refresh()}catch(e){if(!stopped)setError(String(e))}if(!stopped)timer=setTimeout(()=>void tick(),3000)}const visible=()=>{if(!document.hidden)void refresh().catch(e=>{if(!stopped)setError(String(e))})};void tick();document.addEventListener("visibilitychange",visible);return ()=>{stopped=true;if(timer)clearTimeout(timer);document.removeEventListener("visibilitychange",visible)}},[]);
  async function perform(fn: () => Promise<unknown>) {
    if (!ready || status?.starting || !mounted.current) return;
    const finish=tasks.current.beginAction();
    if(!finish)return;
    setBusy(true); setTesting(null); setError("");
    try { await fn(); await WiFi.check(); } catch (e) { if(mounted.current)setError(String(e)); }
    finally { try { await refresh(); } catch (e) { if(mounted.current)setError(String(e)); } finish(); if(mounted.current)setBusy(false); }
  }
  async function testGroup(group:string) {
    if(!ready || !status?.running || !mounted.current)return;
    const request=tasks.current.beginTest();
    if(!request)return;
    setTesting(group);setError("");
    try {
      const speed=await Profiles.speedSettings();
      if(!request.current())return;
      const result=await Service.testGroup(group,speed.url,speed.timeoutMs);
      const completedAt=Date.now();
      // Refresh the core/profile identity before accepting a delayed IPC result.
      await refresh();
      if(request.current()&&mounted.current)setDelays(old=>({...old,[group]:{values:result,completedAt}}));
    } catch(e) { if(request.current()&&mounted.current)setError(String(e)); }
    finally { if(request.finish()&&mounted.current)setTesting(null); }
  }
  async function setProxy(enabled: boolean) {
    await perform(async () => {
      const previous = (await Profiles.summary()).systemProxy;
      await Profiles.setSystemProxy(enabled);
      try { await Service.setSystemProxy(enabled); }
      catch (e) { await Profiles.setSystemProxy(previous); throw e; }
    });
  }
  const mode = status?.mode?.toLowerCase();
  // Same order as the proxy page: the main exit opens first, GLOBAL only in global mode.
  const layout = proxyLayout(groups, mode || "rule");
  const proxyGroups: NonNullable<Design.MenuBarPopoverProps["groups"]> = [layout.main, ...layout.branches].flatMap(route => route ? [route.group] : []).map(g => ({
    name: g.name, type: g.type as Design.ProxyGroupProps["type"], now: g.now,
    nodes: proxyNodes(g,delays[g.name])
  }));
  const transitioning=busy||!!status?.starting;
  return <div className="tray-page"><DS.MenuBarPopover
    state={transitioning || !ready ? "igniting" : status?.running ? "orbit" : "idle"}
    since={status?.startedAt ? Date.parse(status.startedAt) : undefined}
    subtitle={status?.starting?status.startupStage:status?.warning || undefined}
    speed={{ up: streams?.telemetry.connected ? streams.telemetry.up : null, down: streams?.telemetry.connected ? streams.telemetry.down : null, history: streams?.telemetry.connected ? streams.history : {up:[],down:[]} }}
    mode={mode === "global" || mode === "direct" ? mode : "rule"}
    groups={proxyGroups} testing={testing} systemProxy={!!status?.systemProxy} tun={!!status?.tun}
    onToggle={() => void perform(async () => { if (status?.running) await Service.stop(); else if (profiles?.activeId) await Profiles.startActive(); else throw new Error("请先在控制台保存并选择配置。"); })}
    onSystemProxyChange={transitioning || !ready || !status ? undefined : value => void setProxy(value)}
    onTunChange={transitioning || !ready || !status ? undefined : value => void perform(() => Profiles.changeTun(value))}
    ssid={wifi?.ssid || undefined} ssidBypass={wifi?.enabled || false}
    onSsidBypassChange={!ready || transitioning ? undefined : enabled => void perform(async()=>{const settings=await WiFi.settings();await WiFi.setSettings({...settings,enabled})})}
    onModeChange={value => void perform(() => Service.setMode(value))}
    onSelect={(group, node) => void perform(() => Service.select(group, node))}
    onTest={group => void testGroup(group)}
    onCopyCommand={async()=>{try{const effectivePort=(status?.running||status?.starting)&&status.mixedPort?status.mixedPort:(await Profiles.summary()).mixedPort;await Desktop.copyTerminalProxy(effectivePort)}catch(e){setError(String(e));throw e}}}
    onOpenDashboard={() => void Desktop.openDashboard().catch(e => setError(String(e)))}
    onQuit={() => void Desktop.quit().catch(e => setError(String(e)))}
  />{error && <div className="tray-error" role="alert"><span>{error}</span><DS.Button size="sm" variant="ghost" icon="close" aria-label="关闭" onClick={() => setError("")}/></div>}</div>;
}
