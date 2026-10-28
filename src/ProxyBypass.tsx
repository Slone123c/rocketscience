import React, { useEffect, useRef, useState } from "react";
import { Profiles, type ProxyBypassSettings } from "./mygo";
import type * as Design from "../design-system/components/index";

export function ProxyBypass({ design: DS, running }: { design: typeof Design; running: boolean }) {
  const [saved, setSaved] = useState<ProxyBypassSettings | null>(null);
  const [text, setText] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [applied, setApplied] = useState(false);
  const canceled = useRef(false);
  useEffect(()=>{ void Profiles.proxyBypass().then(v=>{setSaved(v);setText(v.entries.join("\n"));}).catch(e=>setError(String(e))); },[]);
  async function commit(value: ProxyBypassSettings) {
    if (saving || running) return;
    setSaving(true); setError(""); setApplied(false);
    try { const next = await Profiles.setProxyBypass(value); setSaved(next); setText(next.entries.join("\n")); setApplied(true); }
    catch (e) { setError(String(e)); }
    finally { setSaving(false); }
  }
  return <section className="panel settings"><div className="section-head"><div><div className="eyebrow">SYSTEM PROXY</div><h2>不走代理的地址</h2><p>默认保留系统已有的例外；自定义只在系统代理接管期间生效，停止时恢复原有值。</p></div>{applied && <span className="badge">已保存</span>}</div>
    <div className="setting"><div><h3>自定义例外</h3><p>{running ? "停止内核后可修改，下一次启动生效。" : "开启后使用下方列表；空列表表示不添加例外。"}</p></div><DS.Switch aria-label="自定义代理例外" checked={!!saved?.custom} disabled={!saved || saving || running} onChange={custom=>void commit({custom,entries:custom ? text.split("\n") : []})}/></div>
    {saved?.custom && <label className="bypass-list">每行一个域名、通配符或 IP 地址<textarea aria-label="不走代理的地址" disabled={saving || running} value={text} onChange={e=>setText(e.target.value)} onBlur={()=>{if(canceled.current){canceled.current=false;return} if(text!==saved.entries.join("\n"))void commit({custom:true,entries:text.split("\n")})}} onKeyDown={e=>{if(e.key==="Escape"){canceled.current=true;setText(saved.entries.join("\n"));setError("");e.currentTarget.blur()} else if(e.key==="Enter" && (e.metaKey || e.ctrlKey)){e.preventDefault();e.currentTarget.blur()}}} placeholder={'*.local\n192.168.0.0/16'}/></label>}
    {error && <p className="profile-error" role="alert">{error}</p>}
  </section>;
}
