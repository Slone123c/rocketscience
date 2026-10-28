import React, { useEffect, useRef, useState } from "react";
import { Profiles, type CoreOptions } from "./mygo";

type BoolField = "ipv6" | "tcpConcurrent" | "unifiedDelay" | "sniffer";
const switches: Array<{ field:BoolField; title:string; keyName:string; description:string }> = [
  {field:"ipv6",title:"IPv6",keyName:"ipv6",description:"允许使用 IPv6 连接。"},
  {field:"tcpConcurrent",title:"并发连接",keyName:"tcp-concurrent",description:"尝试多个解析地址，使用先连接成功的地址。"},
  {field:"unifiedDelay",title:"统一延迟",keyName:"unified-delay",description:"减少不同协议握手过程对延迟比较的影响。"},
  {field:"sniffer",title:"域名嗅探",keyName:"sniffer.enable",description:"尝试从连接中识别域名；其他嗅探参数仍使用配置文件。"}
];
export function CoreSettings({ running, section="connections" }: { running:boolean; section?:"general"|"connections" }) {
  const [saved,setSaved]=useState<CoreOptions|null>(null);
  const [interval,setIntervalValue]=useState("");
  const [busy,setBusy]=useState(false);
  const [message,setMessage]=useState("");
  const canceled=useRef(false);
  useEffect(()=>{void Profiles.coreSettings().then(v=>{setSaved(v);setIntervalValue(v.keepAliveInterval==null?"":String(v.keepAliveInterval))}).catch(e=>setMessage(String(e)))},[]);
  async function commit(next:CoreOptions) {
    setBusy(true);setMessage("");
    try{const result=await Profiles.setCoreSettings(next);setSaved(result);setIntervalValue(result.keepAliveInterval==null?"":String(result.keepAliveInterval));setMessage(running?"已应用并重新加载内核。":"已保存，启动后生效。")}
    catch(e){setMessage(String(e))}finally{setBusy(false)}
  }
  function commitInterval(){
    if(canceled.current){canceled.current=false;return} if(!saved)return;
    const raw=interval.trim();if(raw && (!/^\d+$/.test(raw)||Number(raw)<1||Number(raw)>86400)){setMessage("保活间隔请输入 1 到 86400 秒之间的整数，留空则跟随配置。");return}
    const value=raw ? Number(raw) : null;if(value!==saved.keepAliveInterval)void commit({...saved,keepAliveInterval:value});
  }
  return <section className="panel settings"><div className="eyebrow">{section==="general"?"GENERAL / LOGGING":"CONNECTIONS"}</div><h2>{section==="general"?"日志":"连接参数"}</h2><p>默认跟随所选配置；手动设置不会修改原始订阅。</p>
    {section==="general" && <div className="setting"><div><h3>日志级别</h3><p>log-level</p></div><select aria-label="日志级别" disabled={!saved||busy} value={saved?.logLevel ?? "inherit"} onChange={e=>saved && void commit({...saved,logLevel:e.target.value==="inherit"?null:e.target.value})}><option value="inherit">跟随配置</option><option value="silent">静默</option><option value="error">错误</option><option value="warning">警告</option><option value="info">信息</option><option value="debug">调试</option></select></div>}
    <div hidden={section!=="connections"}>{switches.map(item=><div className="setting" key={item.field}><div><h3>{item.title}</h3><p>{item.keyName} · {item.description}</p></div><select aria-label={item.title} disabled={!saved||busy} value={saved?.[item.field] == null?"inherit":saved[item.field]?"on":"off"} onChange={e=>saved && void commit({...saved,[item.field]:e.target.value==="inherit"?null:e.target.value==="on"})}><option value="inherit">跟随配置</option><option value="on">开启</option><option value="off">关闭</option></select></div>)}
    <div className="setting"><div><h3>进程匹配</h3><p>find-process-mode · 用于按应用匹配规则。</p></div><select aria-label="进程匹配" disabled={!saved||busy} value={saved?.findProcessMode??"inherit"} onChange={e=>saved && void commit({...saved,findProcessMode:e.target.value==="inherit"?null:e.target.value})}><option value="inherit">跟随配置</option><option value="off">关闭</option><option value="strict">按需</option><option value="always">始终</option></select></div>
    <div className="setting"><div><h3>连接保活间隔</h3><p>keep-alive-interval · 秒；留空跟随配置。</p></div><input aria-label="连接保活间隔" inputMode="numeric" disabled={!saved||busy} value={interval} onChange={e=>setIntervalValue(e.target.value)} onBlur={commitInterval} onKeyDown={e=>{if(e.key==="Enter"){e.preventDefault();e.currentTarget.blur()}else if(e.key==="Escape"){canceled.current=true;setIntervalValue(saved?.keepAliveInterval==null?"":String(saved.keepAliveInterval));setMessage("");e.currentTarget.blur()}}}/></div>
    </div>
    {message && <p role="status">{message}</p>}
  </section>;
}
