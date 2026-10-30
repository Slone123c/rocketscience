import React, {useEffect, useState} from "react";
import {Service, type KernelInfo, type MemoryUsage} from "./mygo";
import type * as Design from "../design-system/components/index";

export function KernelSettings({design:DS,running, memory, busy, onRestart}:{design:typeof Design;running:boolean;memory:MemoryUsage|null;busy:boolean;onRestart:()=>Promise<void>}) {
  const [info,setInfo]=useState<KernelInfo|null>(null);
  const [error,setError]=useState("");
  useEffect(()=>{let active=true;void Service.kernelInfo().then(v=>{if(active){setInfo(v);setError("")}}).catch(e=>{if(active)setError(String(e))});return ()=>{active=false}},[running]);
  return <DS.SettingsSection title="代理内核">
    <DS.SettingRow label="内核版本" description={info?.version || "尚未读取到版本"}><code className="lf-setrow__value">{info?.platform||"—"}</code></DS.SettingRow>
    <DS.SettingRow label="内核内存" description="mihomo 当前报告的内存用量。"><code className="lf-setrow__value">{running&&memory?.connected?`${(memory.inuse/1048576).toFixed(1)} MB`:"—"}</code></DS.SettingRow>
    {info?.path && <DS.SettingRow label="内核路径"><code className="lf-setrow__value">{info.path}</code></DS.SettingRow>}
    <DS.SettingRow label="重启内核" description="中断现有连接，保留出站模式、手动选择的节点和接管方式。TUN 可能再次请求授权。"><DS.Button size="sm" icon="refresh" disabled={!running||busy} onClick={()=>void onRestart()}>{busy?"处理中…":"重启内核"}</DS.Button></DS.SettingRow>
    {error && <p className="profile-error" role="alert">{error}</p>}
  </DS.SettingsSection>;
}
