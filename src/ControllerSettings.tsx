import React,{useEffect,useState} from "react";
import {Profiles,type ControllerOptions} from "./mygo";
import type * as Design from "./ui/index";
export function ControllerSettings({design:DS,running,effective}:{design:typeof Design;running:boolean;effective:string}){
  const [saved,setSaved]=useState<ControllerOptions|null>(null),[address,setAddress]=useState("127.0.0.1"),[port,setPort]=useState("0"),[secret,setSecret]=useState("");
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  function load(v:ControllerOptions){setSaved(v);setAddress(v.address);setPort(String(v.port));setSecret(v.secret)}
  useEffect(()=>{void Profiles.controllerSettings().then(load).catch(e=>setMessage(String(e)))},[]);
  async function apply(){if(!/^\d+$/.test(port)){setMessage("控制接口端口请输入 0 到 65535 之间的整数。");return}setBusy(true);setMessage("");try{load(await Profiles.setControllerSettings({address:address.trim(),port:Number(port),secret}));setMessage("已保存，下次启动内核时生效。")}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  const disabled=!saved||busy||running;
  return <DS.SettingsSection title="控制接口" description={running?"停止内核后可修改控制接口。":"默认仅允许本机连接。向其他地址开放时必须设置密钥。"}>
    {effective&&<DS.SettingRow label="当前接口"><code className="lf-setrow__value">{effective}</code></DS.SettingRow>}
    <DS.SettingRow label="监听地址" description="本机 IP 或 *。" configKey="external-controller"><DS.TextField aria-label="控制接口监听地址" mono value={address} disabled={disabled} onChange={e=>setAddress(e.target.value)}/></DS.SettingRow>
    <DS.SettingRow label="监听端口" description="0 表示自动选择。"><DS.TextField aria-label="控制接口端口" mono inputMode="numeric" value={port} disabled={disabled} onChange={e=>setPort(e.target.value)}/></DS.SettingRow>
    <DS.SettingRow label="密钥" description="回环地址下留空则在每次启动时自动生成。" configKey="secret"><DS.TextField aria-label="控制接口密钥" type="password" autoComplete="new-password" mono value={secret} disabled={disabled} onChange={e=>setSecret(e.target.value)}/></DS.SettingRow>
    <DS.SettingRow label="保存">
      <DS.Button size="sm" disabled={disabled} onClick={()=>void apply()}>保存控制接口</DS.Button>
      <DS.Button size="sm" variant="ghost" disabled={disabled} onClick={()=>{if(saved)load(saved);setMessage("")}}>撤回修改</DS.Button>
    </DS.SettingRow>
    {message&&<p role="status">{message}</p>}
  </DS.SettingsSection>;
}
