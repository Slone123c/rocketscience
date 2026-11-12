import React,{useEffect,useRef,useState} from "react";
import {Desktop,Profiles,Service,type TunOptions} from "./mygo";
import type * as Design from "./ui/index";
const fields:Array<{key:"autoRoute"|"autoDetectInterface"|"strictRoute";label:string;hint:string}>=[
  {key:"autoRoute",label:"自动路由",hint:"auto-route · 关闭后需自行配置流量进入虚拟网卡的路由。"},
  {key:"autoDetectInterface",label:"自动检测出口网卡",hint:"auto-detect-interface"},
  {key:"strictRoute",label:"严格路由",hint:"strict-route · 效果取决于内核和系统支持。"}
];
const tri=[{value:"default",label:"默认"},{value:"on",label:"开启"},{value:"off",label:"关闭"}];
export function TunSettings({design:DS,running,tun=false}:{design:typeof Design;running:boolean;tun?:boolean}){
  const tunInUse=running&&tun;
  const [saved,setSaved]=useState<TunOptions|null>(null),[draft,setDraft]=useState<TunOptions|null>(null),[mtu,setMTU]=useState(""),[dns,setDNS]=useState("");
  const dnsRef=useRef(dns);
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  function load(v:TunOptions){const next=v.dnsHijack?.join("\n")||"";dnsRef.current=next;setSaved(v);setDraft(v);setMTU(v.mtu==null?"":String(v.mtu));setDNS(next)}
  const [service,setService]=useState<boolean|null>(null),[serviceBusy,setServiceBusy]=useState(false),[serviceMessage,setServiceMessage]=useState("");
  useEffect(()=>{void Profiles.tunSettings().then(load).catch(e=>setMessage(String(e)))},[]);
  // Re-check after each start or stop: the first TUN start installs the service.
  useEffect(()=>{void Service.tunServiceInstalled().then(setService).catch(()=>setService(null))},[running]);
  async function copyDebugLog(){setServiceMessage("");try{await Desktop.copyText(await Service.tunDebugLog());setServiceMessage("已复制 TUN 调试日志，可以直接粘贴给开发者。")}catch(e){setServiceMessage(String(e))}}
  async function uninstall(){setServiceBusy(true);setServiceMessage("");try{await Service.uninstallTunService();setService(await Service.tunServiceInstalled());setServiceMessage("已卸载。下次开启 TUN 时会再次请求管理员密码并重新安装。")}catch(e){setServiceMessage(String(e))}finally{setServiceBusy(false)}}
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
    <DS.SettingRow label="TUN 服务" description={service?tunInUse?"已安装。TUN 使用中，关闭 TUN 或停止内核后可以卸载。":"已安装：开启 TUN 时不再询问管理员密码。应用更新后如服务版本不同，会再请求一次密码。":"未安装：首次开启 TUN 时请求一次管理员密码，安装后不再询问。"}>
      {service&&<DS.Button size="sm" variant="ghost" disabled={serviceBusy||tunInUse} onClick={()=>void uninstall()}>{serviceBusy?"处理中…":"卸载服务"}</DS.Button>}
    </DS.SettingRow>
    <DS.SettingRow label="调试日志" description="记录每次 TUN 启动的步骤和启动后的自检（路由出口、地址冲突、DNS、连接测试）。日志位于 ~/Library/Logs/RocketScience/tun.log。">
      <DS.Button size="sm" variant="ghost" onClick={()=>void copyDebugLog()}>复制调试日志</DS.Button>
    </DS.SettingRow>
    {serviceMessage&&<p role="status">{serviceMessage}</p>}
  </DS.SettingsSection>;
}
