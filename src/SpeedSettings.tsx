import React, { useEffect, useRef, useState } from "react";
import { Profiles, type SpeedSettings as Values } from "./mygo";

type Field = "url" | "timeoutMs";
export function SpeedSettings() {
  const [saved, setSaved] = useState<Values | null>(null);
  const [url, setUrl] = useState("");
  const [timeout, setTimeoutValue] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [applied, setApplied] = useState(false);
  const canceled = useRef(false);
  useEffect(() => { void Profiles.speedSettings().then(v => { setSaved(v); setUrl(v.url); setTimeoutValue(String(v.timeoutMs)); }).catch(e => setError(String(e))); }, []);
  async function commit(field: Field) {
    if (canceled.current) { canceled.current = false; return; }
    if (!saved || saving) return;
    const value = field === "url" ? { ...saved, url } : { ...saved, timeoutMs: Number(timeout) };
    if (value.url === saved.url && value.timeoutMs === saved.timeoutMs) return;
    if (field === "timeoutMs" && (!/^\d+$/.test(timeout) || !Number.isSafeInteger(value.timeoutMs))) { setError("超时请输入 100 到 30000 之间的整数（毫秒）。"); return; }
    setSaving(true); setError(""); setApplied(false);
    try { const result = await Profiles.setSpeedSettings(value); setSaved(result); setUrl(result.url); setTimeoutValue(String(result.timeoutMs)); setApplied(true); }
    catch (e) { setError(String(e)); }
    finally { setSaving(false); }
  }
  function key(e: React.KeyboardEvent<HTMLInputElement>, field: Field) {
    if (e.key === "Enter") { e.preventDefault(); e.currentTarget.blur(); }
    if (e.key === "Escape") { canceled.current = true; if (field === "url") setUrl(saved?.url || ""); else setTimeoutValue(String(saved?.timeoutMs || "")); setError(""); e.currentTarget.blur(); }
  }
  return <section className="panel settings"><div className="section-head"><div><div className="eyebrow">SPEED TEST</div><h2>测速</h2><p>主窗口和菜单栏共用；仅影响手动测速。</p></div>{applied && <span className="badge">已应用</span>}</div>
    <div className="setting"><div><h3>测速地址</h3><p>输入完整的 HTTP 或 HTTPS 地址。</p></div><input aria-label="测速地址" type="url" value={url} disabled={!saved || saving} onChange={e=>setUrl(e.target.value)} onBlur={()=>void commit("url")} onKeyDown={e=>key(e,"url")}/></div>
    <div className="setting"><div><h3>测速超时</h3><p>100 到 30000 毫秒。</p></div><input aria-label="测速超时（毫秒）" inputMode="numeric" value={timeout} disabled={!saved || saving} onChange={e=>setTimeoutValue(e.target.value)} onBlur={()=>void commit("timeoutMs")} onKeyDown={e=>key(e,"timeoutMs")}/></div>
    {error && <p className="profile-error" role="alert">{error}</p>}
  </section>;
}
