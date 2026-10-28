import React, {useEffect, useState} from "react";
import {Service, type KernelInfo, type MemoryUsage} from "./mygo";

export function KernelSettings({running, memory, busy, onRestart}:{running:boolean;memory:MemoryUsage|null;busy:boolean;onRestart:()=>Promise<void>}) {
  const [info,setInfo]=useState<KernelInfo|null>(null);
  const [error,setError]=useState("");
  useEffect(()=>{let active=true;void Service.kernelInfo().then(v=>{if(active){setInfo(v);setError("")}}).catch(e=>{if(active)setError(String(e))});return ()=>{active=false}},[running]);
  return <section className="panel settings"><div className="eyebrow">MIHOMO</div><h2>代理内核</h2>
    <div className="setting"><div><h3>内核版本</h3><p>{info?.version || "尚未读取到版本"}</p></div><span>{info?.platform}</span></div>
    <div className="setting"><div><h3>内核内存</h3><p>mihomo 当前报告的内存用量。</p></div><span>{running&&memory?.connected?`${(memory.inuse/1048576).toFixed(1)} MB`:"—"}</span></div>
    {info?.path && <p className="footnote">{info.path}</p>}
    <div className="setting"><div><h3>重启内核</h3><p>中断现有连接，保留出站模式、手动选择的节点和接管方式。TUN 可能再次请求授权。</p></div><button className="secondary" disabled={!running||busy} onClick={()=>void onRestart()}>{busy?"处理中…":"重启内核"}</button></div>
    {error && <p role="alert">{error}</p>}
  </section>;
}
