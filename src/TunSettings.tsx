import React,{useEffect,useRef,useState} from "react";
import {Profiles,type TunOptions} from "./mygo";
import type * as Design from "../design-system/components/index";
const fields:Array<{key:"autoRoute"|"autoDetectInterface"|"strictRoute";label:string;hint:string}>=[
  {key:"autoRoute",label:"自动路由",hint:"auto-route · 关闭后需自行配置流量进入虚拟网卡的路由。"},
  {key:"autoDetectInterface",label:"自动检测出口网卡",hint:"auto-detect-interface"},
  {key:"strictRoute",label:"严格路由",hint:"strict-route · 效果取决于内核和系统支持。"}
];
const tri=[{value:"default",label:"默认"},{value:"on",label:"开启"},{value:"off",label:"关闭"}];
export function TunSettings({design:DS,running}:{design:typeof Design;running:boolean}){
  const [saved,setSaved]=useState<TunOptions|null>(null),[draft,setDraft]=useState<TunOptions|null>(null),[mtu,setMTU]=useState(""),[dns,setDNS]=useState("");
  const dnsRef=useRef(dns);
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  function load(v:TunOptions){const next=v.dnsHijack?.join("\n")||"";dnsRef.current=next;setSaved(v);setDraft(v);setMTU(v.mtu==null?"":String(v.mtu));setDNS(next)}
  useEffect(()=>{void Profiles.tunSettings().then(load).catch(e=>setMessage(String(e)))},[]);
  async function apply(){if(!draft)return;const raw=mtu.trim();if(raw&&(!/^\d+$/.test(raw)||Number(raw)<576||Number(raw)>65535)){setMessage("MTU 请输入 576 到 65535 之间的整数，留空使用默认。");return}setBusy(true);setMessage("");try{load(await Profiles.setTunSettings({...draft,mtu:raw?Number(raw):null,dnsHijack:draft.dnsHijack===null?null:dnsRef.current.split("\n").map(s=>s.trim()).filter(Boolean)}));setMessage("已保存，下次启动 TUN 时生效。")}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  const disabled=running||busy||!draft;
  return <DS.SettingsSection title="TUN 高级参数" description={running?"请先停止内核，再修改虚拟网卡参数。":"留空或选择默认时使用应用默认值；TUN 启用状态由接管开关决定。"}>
    <DS.SettingRow label="网络栈" configKey="stack"><DS.Select aria-label="TUN 网络栈" disabled={disabled} value={draft?.stack??"default"} onChange={value=>draft&&setDraft({...draft,stack:value==="default"?null:value})} options={[{value:"default",label:"默认（mixed）"},{value:"system",label:"system"},{value:"gvisor",label:"gvisor"},{value:"mixed",label:"mixed"}]}/></DS.SettingRow>
    {fields.map(field=>{const keyName=field.hint.split(" · ")[0];const hint=field.hint.includes(" · ")?field.hint.slice(keyName.length+3):undefined;return <DS.SettingRow key={field.key} label={field.label} description={hint} configKey={keyName}><DS.Select aria-label={field.label} disabled={disabled} value={draft?.[field.key]==null?"default":draft[field.key]?"on":"off"} onChange={value=>draft&&setDraft({...draft,[field.key]:value==="default"?null:value==="on"})} options={tri}/></DS.SettingRow>})}
    <DS.SettingRow label="MTU" description="留空使用内核默认值。" configKey="mtu"><DS.TextField aria-label="TUN MTU" mono inputMode="numeric" value={mtu} disabled={disabled} onChange={e=>setMTU(e.target.value)}/></DS.SettingRow>
    <DS.SettingRow label="DNS 劫持" description="默认拦截 UDP 和 TCP 的 53 端口。自定义且列表为空表示关闭劫持。" configKey="dns-hijack" stack>
      <DS.Switch aria-label="自定义 DNS 劫持" disabled={disabled} checked={!!draft&&draft.dnsHijack!==null} onChange={checked=>draft&&setDraft({...draft,dnsHijack:checked?[]:null})}/>
      {draft&&draft.dnsHijack!==null&&<DS.ListField aria-label="TUN DNS 劫持列表" disabled={disabled} value={dns.split("\n").filter(Boolean)} placeholder={'any:53\ntcp://any:53'} onCommit={lines=>{const next=lines.join("\n");dnsRef.current=next;setDNS(next)}}/>}
    </DS.SettingRow>
    <DS.SettingRow label="保存">
      <DS.Button size="sm" disabled={disabled} onClick={()=>void apply()}>保存 TUN 参数</DS.Button>
      <DS.Button size="sm" variant="ghost" disabled={disabled||!saved} onClick={()=>{if(saved)load(saved);setMessage("")}}>撤回修改</DS.Button>
    </DS.SettingRow>
    {message&&<p role="status">{message}</p>}
  </DS.SettingsSection>;
}
