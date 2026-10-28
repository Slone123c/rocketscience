import React,{useEffect,useState} from "react";
import {Profiles,type InboundOptions} from "./mygo";
export function InboundSettings({running}:{running:boolean}){
  const [saved,setSaved]=useState<InboundOptions|null>(null),[http,setHTTP]=useState("0"),[socks,setSOCKS]=useState("0"),[custom,setCustom]=useState(false);
  const [accounts,setAccounts]=useState<Array<{username:string;password:string}>>([]),[busy,setBusy]=useState(false),[message,setMessage]=useState("");
  function load(v:InboundOptions){setSaved(v);setHTTP(String(v.httpPort));setSOCKS(String(v.socksPort));setCustom(v.authentication!==null);setAccounts((v.authentication||[]).map(entry=>{const at=entry.indexOf(":");return {username:entry.slice(0,at),password:entry.slice(at+1)}}))}
  useEffect(()=>{void Profiles.inboundSettings().then(load).catch(e=>setMessage(String(e)))},[]);
  async function apply(){if(!/^\d+$/.test(http)||!/^\d+$/.test(socks)){setMessage("端口请输入 0 到 65535 之间的整数。");return}setBusy(true);setMessage("");try{load(await Profiles.setInboundSettings({httpPort:Number(http),socksPort:Number(socks),authentication:custom?accounts.map(a=>a.username+":"+a.password):null}));setMessage("已保存，下次启动内核时生效。")}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  const disabled=running||busy||!saved;
  return <section className="panel settings"><div className="eyebrow">INBOUND / ADVANCED</div><h2>入站端口与认证</h2><p>{running?"请先停止内核，再修改入站设置。":"0 表示关闭独立端口；混合端口继续提供 HTTP 和 SOCKS。"}</p>
    <div className="setting"><div><h3>HTTP 端口</h3><p>port</p></div><input aria-label="HTTP 入站端口" inputMode="numeric" value={http} disabled={disabled} onChange={e=>setHTTP(e.target.value)}/></div>
    <div className="setting"><div><h3>SOCKS 端口</h3><p>socks-port</p></div><input aria-label="SOCKS 入站端口" inputMode="numeric" value={socks} disabled={disabled} onChange={e=>setSOCKS(e.target.value)}/></div>
    <div className="setting"><div><h3>代理认证</h3><p>默认跟随配置。回环地址免认证，供本机系统代理使用；所有网卡共享时，额外端口也可从局域网访问。</p></div><label><input type="checkbox" checked={custom} disabled={disabled} onChange={e=>setCustom(e.target.checked)}/> 自定义账号</label></div>
    {custom&&<div className="proxy-accounts"><p>账号为空表示关闭认证。</p>{accounts.map((account,index)=><div className="actions" key={index}><input aria-label={`代理用户名 ${index+1}`} autoComplete="off" value={account.username} disabled={disabled} onChange={e=>setAccounts(accounts.map((a,i)=>i===index?{...a,username:e.target.value}:a))}/><input aria-label={`代理密码 ${index+1}`} type="password" autoComplete="new-password" value={account.password} disabled={disabled} onChange={e=>setAccounts(accounts.map((a,i)=>i===index?{...a,password:e.target.value}:a))}/><button disabled={disabled} onClick={()=>setAccounts(accounts.filter((_,i)=>i!==index))}>移除</button></div>)}<button className="text-button" disabled={disabled} onClick={()=>setAccounts([...accounts,{username:"",password:""}])}>添加账号</button></div>}
    <div className="actions"><button disabled={disabled} onClick={()=>void apply()}>保存入站设置</button><button disabled={disabled} onClick={()=>{if(saved)load(saved);setMessage("")}}>撤回修改</button></div>{message&&<p role="status">{message}</p>}
  </section>;
}
