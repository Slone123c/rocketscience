import React, { useEffect, useState } from "react";
import { Profiles, type DNSOptions } from "./mygo";

type ListField = "defaultNameservers" | "nameservers" | "fallback";
const lists: Array<{key:ListField;label:string;hint:string}> = [
  {key:"defaultNameservers",label:"默认 DNS",hint:"default-nameserver · 用 IP 地址解析 DNS 服务器的域名。"},
  {key:"nameservers",label:"上游 DNS",hint:"nameserver · 支持 DNS、DoH、DoT 等地址。"},
  {key:"fallback",label:"备用 DNS",hint:"fallback · 可留空以清除备用服务器。"}
];
function tri(value:boolean|null) {return value==null?"inherit":value?"on":"off"}
function bool(value:string) {return value==="inherit"?null:value==="on"}
export function DNSSettings({ running }: {running:boolean}) {
  const [saved,setSaved]=useState<DNSOptions|null>(null),[draft,setDraft]=useState<DNSOptions|null>(null);
  const [texts,setTexts]=useState<Record<ListField,string>>({defaultNameservers:"",nameservers:"",fallback:""});
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  function load(value:DNSOptions) {setSaved(value);setDraft(value);setTexts({defaultNameservers:value.defaultNameservers?.join("\n")||"",nameservers:value.nameservers?.join("\n")||"",fallback:value.fallback?.join("\n")||""})}
  useEffect(()=>{void Profiles.dnsSettings().then(load).catch(e=>setMessage(String(e)))},[]);
  async function apply() {
    if(!draft)return;setBusy(true);setMessage("");
    const next={...draft};for(const field of lists){if(next[field.key]!==null)next[field.key]=texts[field.key].split("\n").map(s=>s.trim()).filter(Boolean)}
    try{load(await Profiles.setDNSSettings(next));setMessage(running?"已校验、应用并重新加载内核。":"已校验并保存，启动时生效。")}
    catch(e){setMessage(String(e))}finally{setBusy(false)}
  }
  return <section className="panel settings"><div className="eyebrow">DNS / ADVANCED</div><h2>DNS</h2><p>默认跟随配置。可一起编辑后应用，未修改的 DNS 规则会保留。</p>
    <div className="setting"><div><h3>DNS 服务</h3><p>dns.enable</p></div><select aria-label="DNS 服务" disabled={!draft||busy} value={tri(draft?.enabled??null)} onChange={e=>draft && setDraft({...draft,enabled:bool(e.target.value)})}><option value="inherit">跟随配置</option><option value="on">开启</option><option value="off">关闭</option></select></div>
    <div className="setting"><div><h3>DNS IPv6</h3><p>dns.ipv6 · 控制 AAAA 解析。</p></div><select aria-label="DNS IPv6" disabled={!draft||busy} value={tri(draft?.ipv6??null)} onChange={e=>draft && setDraft({...draft,ipv6:bool(e.target.value)})}><option value="inherit">跟随配置</option><option value="on">开启</option><option value="off">关闭</option></select></div>
    <div className="setting"><div><h3>DNS 处理模式</h3><p>dns.enhanced-mode</p></div><select aria-label="DNS 处理模式" disabled={!draft||busy} value={draft?.enhancedMode??"inherit"} onChange={e=>draft && setDraft({...draft,enhancedMode:e.target.value==="inherit"?null:e.target.value})}><option value="inherit">跟随配置</option><option value="redir-host">redir-host</option><option value="fake-ip">fake-ip</option></select></div>
    <div className="setting"><div><h3>Fake-IP 网段</h3><p>dns.fake-ip-range · 留空跟随配置。</p></div><input aria-label="Fake-IP 网段" autoCapitalize="off" autoCorrect="off" spellCheck={false} disabled={!draft||busy} value={draft?.fakeIPRange??""} onChange={e=>draft && setDraft({...draft,fakeIPRange:e.target.value||null})} placeholder="198.18.0.1/16"/></div>
    {lists.map(item=><div className="setting dns-list" key={item.key}><div><h3>{item.label}</h3><p>{item.hint}</p><label><input type="checkbox" checked={draft?.[item.key]!==null && !!draft} disabled={!draft||busy} onChange={e=>draft && setDraft({...draft,[item.key]:e.target.checked?[]:null})}/> 自定义</label></div>{draft?.[item.key]!==null && draft && <textarea aria-label={item.label} autoCapitalize="off" autoCorrect="off" spellCheck={false} disabled={busy} value={texts[item.key]} onChange={e=>setTexts({...texts,[item.key]:e.target.value})} placeholder="每行一个地址"/>}</div>)}
    <div className="actions"><button disabled={!draft||busy} onClick={()=>void apply()}>{busy?"校验中…":"应用 DNS 设置"}</button><button disabled={!saved||busy} onClick={()=>{if(saved)load(saved);setMessage("")}}>撤回修改</button></div>{message && <p role="status">{message}</p>}
  </section>;
}
