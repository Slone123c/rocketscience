import React,{useEffect,useState} from "react";
import {Profiles,type InboundOptions} from "./mygo";
import type * as Design from "../design-system/components/index";
export function InboundSettings({design:DS,running}:{design:typeof Design;running:boolean}){
  const [saved,setSaved]=useState<InboundOptions|null>(null),[http,setHTTP]=useState("0"),[socks,setSOCKS]=useState("0"),[custom,setCustom]=useState(false);
  const [accounts,setAccounts]=useState<Array<{username:string;password:string}>>([]),[busy,setBusy]=useState(false),[message,setMessage]=useState("");
  function load(v:InboundOptions){setSaved(v);setHTTP(String(v.httpPort));setSOCKS(String(v.socksPort));setCustom(v.authentication!==null);setAccounts((v.authentication||[]).map(entry=>{const at=entry.indexOf(":");return {username:entry.slice(0,at),password:entry.slice(at+1)}}))}
  useEffect(()=>{void Profiles.inboundSettings().then(load).catch(e=>setMessage(String(e)))},[]);
  async function apply(){if(!/^\d+$/.test(http)||!/^\d+$/.test(socks)){setMessage("端口请输入 0 到 65535 之间的整数。");return}setBusy(true);setMessage("");try{load(await Profiles.setInboundSettings({httpPort:Number(http),socksPort:Number(socks),authentication:custom?accounts.map(a=>a.username+":"+a.password):null}));setMessage("已保存，下次启动内核时生效。")}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  const disabled=running||busy||!saved;
  return <DS.SettingsSection title="入站端口与认证" description={running?"请先停止内核，再修改入站设置。":"0 表示关闭独立端口；混合端口继续提供 HTTP 和 SOCKS。"}>
    <DS.SettingRow label="HTTP 端口" configKey="port"><DS.TextField aria-label="HTTP 入站端口" mono inputMode="numeric" value={http} disabled={disabled} onChange={e=>setHTTP(e.target.value)}/></DS.SettingRow>
    <DS.SettingRow label="SOCKS 端口" configKey="socks-port"><DS.TextField aria-label="SOCKS 入站端口" mono inputMode="numeric" value={socks} disabled={disabled} onChange={e=>setSOCKS(e.target.value)}/></DS.SettingRow>
    <DS.SettingRow label="代理认证" description="默认跟随配置。回环地址免认证，供本机系统代理使用；所有网卡共享时，额外端口也可从局域网访问。"><DS.Switch aria-label="自定义代理认证" checked={custom} disabled={disabled} onChange={setCustom}/></DS.SettingRow>
    {custom&&<DS.SettingRow label="账号" description="账号为空表示关闭认证。" stack>
      {accounts.map((account,index)=><div className="actions" key={index}><DS.TextField aria-label={`代理用户名 ${index+1}`} autoComplete="off" value={account.username} disabled={disabled} onChange={e=>setAccounts(accounts.map((a,i)=>i===index?{...a,username:e.target.value}:a))}/><DS.TextField aria-label={`代理密码 ${index+1}`} type="password" autoComplete="new-password" value={account.password} disabled={disabled} onChange={e=>setAccounts(accounts.map((a,i)=>i===index?{...a,password:e.target.value}:a))}/><DS.Button size="sm" variant="ghost" disabled={disabled} onClick={()=>setAccounts(accounts.filter((_,i)=>i!==index))}>移除</DS.Button></div>)}
      <DS.Button size="sm" variant="ghost" disabled={disabled} onClick={()=>setAccounts([...accounts,{username:"",password:""}])}>添加账号</DS.Button>
    </DS.SettingRow>}
    <DS.SettingRow label="保存">
      <DS.Button size="sm" disabled={disabled} onClick={()=>void apply()}>保存入站设置</DS.Button>
      <DS.Button size="sm" variant="ghost" disabled={disabled} onClick={()=>{if(saved)load(saved);setMessage("")}}>撤回修改</DS.Button>
    </DS.SettingRow>
    {message&&<p role="status">{message}</p>}
  </DS.SettingsSection>;
}
