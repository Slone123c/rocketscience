import React, { useEffect, useRef, useState } from "react";
import { Profiles, type DNSOptions } from "./mygo";
import type * as Design from "../design-system/components/index";

type ListField = "defaultNameservers" | "nameservers" | "fallback";
const lists: Array<{key:ListField;label:string;keyName:string;hint:string}> = [
  {key:"defaultNameservers",label:"默认 DNS",keyName:"default-nameserver",hint:"用 IP 地址解析 DNS 服务器的域名。"},
  {key:"nameservers",label:"上游 DNS",keyName:"nameserver",hint:"支持 DNS、DoH、DoT 等地址。"},
  {key:"fallback",label:"备用 DNS",keyName:"fallback",hint:"可留空以清除备用服务器。"}
];
const triOpts=[{value:"inherit",label:"跟随配置"},{value:"on",label:"开启"},{value:"off",label:"关闭"}];
function tri(value:boolean|null) {return value==null?"inherit":value?"on":"off"}
function bool(value:string) {return value==="inherit"?null:value==="on"}
export function DNSSettings({ design:DS, running, tun=false }: {design:typeof Design;running:boolean;tun?:boolean}) {
  const [saved,setSaved]=useState<DNSOptions|null>(null),[draft,setDraft]=useState<DNSOptions|null>(null);
  const [texts,setTexts]=useState<Record<ListField,string>>({defaultNameservers:"",nameservers:"",fallback:""});
  const textsRef=useRef(texts);
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  function load(value:DNSOptions) {const next={defaultNameservers:value.defaultNameservers?.join("\n")||"",nameservers:value.nameservers?.join("\n")||"",fallback:value.fallback?.join("\n")||""};textsRef.current=next;setSaved(value);setDraft(value);setTexts(next)}
  useEffect(()=>{void Profiles.dnsSettings().then(load).catch(e=>setMessage(String(e)))},[]);
  async function apply() {
    if(!draft)return;setBusy(true);setMessage("");
    const next={...draft,enabled:tun?true:draft.enabled};for(const field of lists){if(next[field.key]!==null)next[field.key]=textsRef.current[field.key].split("\n").map(s=>s.trim()).filter(Boolean)}
    try{load(await Profiles.setDNSSettings(next));setMessage(running?"已校验、应用并重新加载内核。":"已校验并保存，启动时生效。")}
    catch(e){setMessage(String(e))}finally{setBusy(false)}
  }
  return <DS.SettingsSection title="DNS" description="默认跟随配置。可一起编辑后应用，未修改的 DNS 规则会保留。">
    <DS.SettingRow label="DNS 服务" description={tun?"虚拟网卡已开启，内核 DNS 将保持启用；保留你配置的上游 DNS。":undefined} configKey="dns.enable"><DS.Select aria-label="DNS 服务" disabled={!draft||busy||tun} value={tun?"on":tri(draft?.enabled??null)} onChange={value=>draft && setDraft({...draft,enabled:bool(value)})} options={triOpts}/></DS.SettingRow>
    <DS.SettingRow label="DNS IPv6" description="控制 AAAA 解析。" configKey="dns.ipv6"><DS.Select aria-label="DNS IPv6" disabled={!draft||busy} value={tri(draft?.ipv6??null)} onChange={value=>draft && setDraft({...draft,ipv6:bool(value)})} options={triOpts}/></DS.SettingRow>
    <DS.SettingRow label="DNS 处理模式" configKey="dns.enhanced-mode"><DS.Select aria-label="DNS 处理模式" disabled={!draft||busy} value={draft?.enhancedMode??"inherit"} onChange={value=>draft && setDraft({...draft,enhancedMode:value==="inherit"?null:value})} options={[{value:"inherit",label:"跟随配置"},{value:"redir-host",label:"redir-host"},{value:"fake-ip",label:"fake-ip"}]}/></DS.SettingRow>
    <DS.SettingRow label="Fake-IP 网段" description="留空跟随配置。" configKey="dns.fake-ip-range"><DS.TextField aria-label="Fake-IP 网段" mono value={draft?.fakeIPRange??""} disabled={!draft||busy} placeholder="198.18.0.1/16" onChange={e=>draft && setDraft({...draft,fakeIPRange:e.target.value||null})}/></DS.SettingRow>
    {lists.map(item=><DS.SettingRow key={item.key} label={item.label} description={item.hint} configKey={item.keyName} stack>
      <DS.Switch aria-label={`自定义${item.label}`} checked={!!draft && draft[item.key]!==null} disabled={!draft||busy} onChange={checked=>draft && setDraft({...draft,[item.key]:checked?[]:null})}/>
      {draft && draft[item.key]!==null && <DS.ListField aria-label={item.label} disabled={busy} value={texts[item.key].split("\n").filter(Boolean)} placeholder="每行一个地址" onCommit={lines=>{const next={...textsRef.current,[item.key]:lines.join("\n")};textsRef.current=next;setTexts(next)}}/>}
    </DS.SettingRow>)}
    <DS.SettingRow label="保存">
      <DS.Button size="sm" disabled={!draft||busy} onClick={()=>void apply()}>{busy?"校验中…":"应用 DNS 设置"}</DS.Button>
      <DS.Button size="sm" variant="ghost" disabled={!saved||busy} onClick={()=>{if(saved)load(saved);setMessage("")}}>撤回修改</DS.Button>
    </DS.SettingRow>
    {message && <p role="status">{message}</p>}
  </DS.SettingsSection>;
}
