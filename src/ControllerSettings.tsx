import React,{useEffect,useState} from "react";
import {Profiles,type ControllerOptions} from "./mygo";
export function ControllerSettings({running,effective}:{running:boolean;effective:string}){
  const [saved,setSaved]=useState<ControllerOptions|null>(null),[address,setAddress]=useState("127.0.0.1"),[port,setPort]=useState("0"),[secret,setSecret]=useState("");
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  function load(v:ControllerOptions){setSaved(v);setAddress(v.address);setPort(String(v.port));setSecret(v.secret)}
  useEffect(()=>{void Profiles.controllerSettings().then(load).catch(e=>setMessage(String(e)))},[]);
  async function apply(){if(!/^\d+$/.test(port)){setMessage("控制接口端口请输入 0 到 65535 之间的整数。");return}setBusy(true);setMessage("");try{load(await Profiles.setControllerSettings({address:address.trim(),port:Number(port),secret}));setMessage("已保存，下次启动内核时生效。")}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  const disabled=!saved||busy||running;
  return <section className="panel settings"><div className="eyebrow">CONTROLLER / ADVANCED</div><h2>控制接口</h2><p>{running?"停止内核后可修改控制接口。":"默认仅允许本机连接。向其他地址开放时必须设置密钥。"}</p>{effective&&<p>当前接口：{effective}</p>}
    <div className="setting"><div><h3>监听地址</h3><p>external-controller · 本机 IP 或 *。</p></div><input aria-label="控制接口监听地址" value={address} disabled={disabled} onChange={e=>setAddress(e.target.value)}/></div>
    <div className="setting"><div><h3>监听端口</h3><p>0 表示自动选择。</p></div><input aria-label="控制接口端口" inputMode="numeric" value={port} disabled={disabled} onChange={e=>setPort(e.target.value)}/></div>
    <div className="setting"><div><h3>密钥</h3><p>secret · 回环地址下留空则在每次启动时自动生成。</p></div><input aria-label="控制接口密钥" type="password" autoComplete="new-password" value={secret} disabled={disabled} onChange={e=>setSecret(e.target.value)}/></div>
    <div className="actions"><button disabled={disabled} onClick={()=>void apply()}>保存控制接口</button><button disabled={disabled} onClick={()=>{if(saved)load(saved);setMessage("")}}>撤回修改</button></div>{message&&<p role="status">{message}</p>}
  </section>;
}
