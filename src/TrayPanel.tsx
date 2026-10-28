import React, { useEffect, useRef, useState } from "react";
import { Desktop, Profiles, Service, WiFi, type WiFiStatus, type ProfileSummary, type Proxy, type Status, type Monitoring } from "./mygo";
import {createRefreshQueue} from "./viewRefresh";
import type * as Design from "../design-system/components/index";

export function TrayPanel({ design: DS }: { design: typeof Design }) {
  const [wifi,setWifi]=useState<WiFiStatus|null>(null);
  const [ready, setReady] = useState(false);
  const [status, setStatus] = useState<Status | null>(null);
  const [profiles, setProfiles] = useState<ProfileSummary | null>(null);
  const [groups, setGroups] = useState<Proxy[]>([]);
  const [streams, setStreams] = useState<Monitoring | null>(null);
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState<string | null>(null);
  const [delays, setDelays] = useState<Record<string, Record<string, number>>>({});
  const [error, setError] = useState("");
  async function read() {
    const desktop = await Desktop.state();
    setReady(desktop.startupReady);
    if (!desktop.startupReady) return;
    const [next, saved] = await Promise.all([Service.status(), Profiles.summary()]);
    setStatus(next); setProfiles(saved); setWifi(await WiFi.state());
    if (next.running) {
      const [g, s] = await Promise.all([Service.groups(), Service.monitoring()]);
      setGroups(g); setStreams(s);
    } else { setGroups([]); setStreams(null); }
  }
  const refreshQueue=useRef<ReturnType<typeof createRefreshQueue>|null>(null);
  if(!refreshQueue.current)refreshQueue.current=createRefreshQueue(read);
  async function refresh(){await refreshQueue.current!.request()}
  useEffect(()=>{let stopped=false,timer:ReturnType<typeof setTimeout>|undefined;async function tick(){if(!document.hidden)try{await refresh()}catch(e){setError(String(e))}if(!stopped)timer=setTimeout(()=>void tick(),3000)}const visible=()=>{if(!document.hidden)void refresh().catch(e=>setError(String(e)))};void tick();document.addEventListener("visibilitychange",visible);return ()=>{stopped=true;if(timer)clearTimeout(timer);document.removeEventListener("visibilitychange",visible)}},[]);
  async function perform(fn: () => Promise<unknown>) {
    if (busy || !ready) return;
    setBusy(true); setError("");
    try { await fn(); await WiFi.check(); } catch (e) { setError(String(e)); }
    finally { try { await refresh(); } catch (e) { setError(String(e)); } setBusy(false); }
  }
  async function setProxy(enabled: boolean) {
    await perform(async () => {
      const previous = (await Profiles.summary()).systemProxy;
      await Profiles.setSystemProxy(enabled);
      try { await Service.setSystemProxy(enabled); }
      catch (e) { await Profiles.setSystemProxy(previous); throw e; }
    });
  }
  const proxyGroups: NonNullable<Design.MenuBarPopoverProps["groups"]> = groups.map(g => ({
    name: g.name, type: g.type as Design.ProxyGroupProps["type"], now: g.now,
    nodes: (g.all || []).map(name => ({ name, delay: delays[g.name]?.[name] }))
  }));
  const mode = status?.mode?.toLowerCase();
  return <div className="tray-page"><DS.MenuBarPopover
    state={busy || !ready ? "igniting" : status?.running ? "orbit" : "idle"}
    since={status?.startedAt ? Date.parse(status.startedAt) : undefined}
    subtitle={status?.warning || undefined}
    speed={{ up: streams?.telemetry.connected ? streams.telemetry.up : null, down: streams?.telemetry.connected ? streams.telemetry.down : null, history: streams?.telemetry.connected ? streams.history : {up:[],down:[]} }}
    mode={mode === "global" || mode === "direct" ? mode : "rule"}
    groups={proxyGroups} testing={testing} systemProxy={!!status?.systemProxy} tun={!!status?.tun}
    onToggle={() => void perform(async () => { if (status?.running) await Service.stop(); else if (profiles?.activeId) await Profiles.startActive(); else throw new Error("请先在控制台保存并选择配置。"); })}
    onSystemProxyChange={busy || !ready || !status ? undefined : value => void setProxy(value)}
    onTunChange={busy || !ready || !status ? undefined : value => void perform(() => Profiles.changeTun(value))}
    ssid={wifi?.ssid || undefined} ssidBypass={wifi?.enabled || false}
    onSsidBypassChange={!ready || busy ? undefined : enabled => void perform(async()=>{const settings=await WiFi.settings();await WiFi.setSettings({...settings,enabled})})}
    onModeChange={value => void perform(() => Service.setMode(value))}
    onSelect={(group, node) => void perform(() => Service.select(group, node))}
    onTest={group => { setTesting(group); void Profiles.speedSettings().then(speed => Service.testGroup(group, speed.url, speed.timeoutMs)).then(result => setDelays(old => ({ ...old, [group]: result }))).catch(e => setError(String(e))).finally(() => setTesting(null)); }}
    onCopyCommand={() => void Desktop.copyTerminalProxy(profiles?.mixedPort || 7890).catch(e => setError(String(e)))}
    onOpenDashboard={() => void Desktop.openDashboard().catch(e => setError(String(e)))}
    onQuit={() => void Desktop.quit().catch(e => setError(String(e)))}
  />{error && <div className="tray-error" role="alert">{error}<button onClick={() => setError("")} aria-label="关闭">×</button></div>}</div>;
}
