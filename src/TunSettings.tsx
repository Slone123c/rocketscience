import React,{useEffect,useState} from "react";
import {Profiles,type TunOptions} from "./mygo";
const fields:Array<{key:"autoRoute"|"autoDetectInterface"|"strictRoute";label:string;hint:string}>=[
  {key:"autoRoute",label:"自动路由",hint:"auto-route · 关闭后需自行配置流量进入虚拟网卡的路由。"},
  {key:"autoDetectInterface",label:"自动检测出口网卡",hint:"auto-detect-interface"},
  {key:"strictRoute",label:"严格路由",hint:"strict-route · 效果取决于内核和系统支持。"}
];
export function TunSettings({running}:{running:boolean}){
  const [saved,setSaved]=useState<TunOptions|null>(null),[draft,setDraft]=useState<TunOptions|null>(null),[mtu,setMTU]=useState(""),[dns,setDNS]=useState("");
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  function load(v:TunOptions){setSaved(v);setDraft(v);setMTU(v.mtu==null?"":String(v.mtu));setDNS(v.dnsHijack?.join("\n")||"")}
  useEffect(()=>{void Profiles.tunSettings().then(load).catch(e=>setMessage(String(e)))},[]);
  async function apply(){if(!draft)return;const raw=mtu.trim();if(raw&&(!/^\d+$/.test(raw)||Number(raw)<576||Number(raw)>65535)){setMessage("MTU 请输入 576 到 65535 之间的整数，留空使用默认。");return}setBusy(true);setMessage("");try{load(await Profiles.setTunSettings({...draft,mtu:raw?Number(raw):null,dnsHijack:draft.dnsHijack===null?null:dns.split("\n").map(s=>s.trim()).filter(Boolean)}));setMessage("已保存，下次启动 TUN 时生效。")}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  const disabled=running||busy||!draft;
  return <section className="panel settings"><div className="eyebrow">TUN / ADVANCED</div><h2>TUN 高级参数</h2><p>{running?"请先停止内核，再修改虚拟网卡参数。":"留空或选择默认时使用应用默认值；TUN 启用状态由接管开关决定。"}</p>
    <div className="setting"><div><h3>网络栈</h3><p>stack</p></div><select aria-label="TUN 网络栈" disabled={disabled} value={draft?.stack??"default"} onChange={e=>draft&&setDraft({...draft,stack:e.target.value==="default"?null:e.target.value})}><option value="default">默认（mixed）</option><option value="system">system</option><option value="gvisor">gvisor</option><option value="mixed">mixed</option></select></div>
    {fields.map(field=><div className="setting" key={field.key}><div><h3>{field.label}</h3><p>{field.hint}</p></div><select aria-label={field.label} disabled={disabled} value={draft?.[field.key]==null?"default":draft[field.key]?"on":"off"} onChange={e=>draft&&setDraft({...draft,[field.key]:e.target.value==="default"?null:e.target.value==="on"})}><option value="default">默认</option><option value="on">开启</option><option value="off">关闭</option></select></div>)}
    <div className="setting"><div><h3>MTU</h3><p>mtu · 留空使用内核默认值。</p></div><input aria-label="TUN MTU" disabled={disabled} inputMode="numeric" value={mtu} onChange={e=>setMTU(e.target.value)}/></div>
    <div className="setting dns-list"><div><h3>DNS 劫持</h3><p>默认拦截 UDP 和 TCP 的 53 端口。</p><label><input type="checkbox" disabled={disabled} checked={!!draft&&draft.dnsHijack!==null} onChange={e=>draft&&setDraft({...draft,dnsHijack:e.target.checked?[]:null})}/> 自定义；空列表表示关闭劫持</label></div>{draft&&draft.dnsHijack!==null&&<textarea aria-label="TUN DNS 劫持列表" disabled={disabled} value={dns} onChange={e=>setDNS(e.target.value)} placeholder={'any:53\ntcp://any:53'}/>}</div>
    <div className="actions"><button disabled={disabled} onClick={()=>void apply()}>保存 TUN 参数</button><button disabled={disabled||!saved} onClick={()=>{if(saved)load(saved);setMessage("")}}>撤回修改</button></div>{message&&<p role="status">{message}</p>}
  </section>;
}
