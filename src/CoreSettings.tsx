import React, { useEffect, useState } from "react";
import { Profiles, type CoreOptions } from "./mygo";
import type * as Design from "./ui/index";

type BoolField = "ipv6" | "tcpConcurrent" | "unifiedDelay" | "sniffer";
const switches: Array<{ field:BoolField; title:string; keyName:string; description:string }> = [
  {field:"ipv6",title:"IPv6",keyName:"ipv6",description:"允许使用 IPv6 连接。"},
  {field:"tcpConcurrent",title:"并发连接",keyName:"tcp-concurrent",description:"尝试多个解析地址，使用先连接成功的地址。"},
  {field:"unifiedDelay",title:"统一延迟",keyName:"unified-delay",description:"减少不同协议握手过程对延迟比较的影响。"},
  {field:"sniffer",title:"域名嗅探",keyName:"sniffer.enable",description:"尝试从连接中识别域名；其他嗅探参数仍使用配置文件。"}
];
const tri=[{value:"inherit",label:"跟随配置"},{value:"on",label:"开启"},{value:"off",label:"关闭"}];
export function CoreSettings({ design:DS, running, section="connections" }: { design:typeof Design; running:boolean; section?:"general"|"connections" }) {
  const [saved,setSaved]=useState<CoreOptions|null>(null);
  const [busy,setBusy]=useState(false);
  const [message,setMessage]=useState("");
  useEffect(()=>{void Profiles.coreSettings().then(setSaved).catch(e=>setMessage(String(e)))},[]);
  async function commit(next:CoreOptions) {
    setBusy(true);setMessage("");
    try{const result=await Profiles.setCoreSettings(next);setSaved(result);setMessage(running?"已应用并重新加载内核。":"已保存，启动后生效。")}
    catch(e){setMessage(String(e))}finally{setBusy(false)}
  }
  return <DS.SettingsSection title={section==="general"?"日志":"连接参数"} description="默认跟随所选配置；手动设置不会修改原始订阅。">
    {section==="general" && <DS.SettingRow label="日志级别" configKey="log-level"><DS.Select aria-label="日志级别" disabled={!saved||busy} value={saved?.logLevel ?? "inherit"} onChange={value=>saved && void commit({...saved,logLevel:value==="inherit"?null:value})} options={[{value:"inherit",label:"跟随配置"},{value:"silent",label:"静默"},{value:"error",label:"错误"},{value:"warning",label:"警告"},{value:"info",label:"信息"},{value:"debug",label:"调试"}]}/></DS.SettingRow>}
    {section==="connections" && switches.map(item=><DS.SettingRow key={item.field} label={item.title} description={item.description} configKey={item.keyName}><DS.Select aria-label={item.title} disabled={!saved||busy} value={saved?.[item.field] == null?"inherit":saved[item.field]?"on":"off"} onChange={value=>saved && void commit({...saved,[item.field]:value==="inherit"?null:value==="on"})} options={tri}/></DS.SettingRow>)}
    {section==="connections" && <DS.SettingRow label="进程匹配" description="用于按应用匹配规则。" configKey="find-process-mode"><DS.Select aria-label="进程匹配" disabled={!saved||busy} value={saved?.findProcessMode??"inherit"} onChange={value=>saved && void commit({...saved,findProcessMode:value==="inherit"?null:value})} options={[{value:"inherit",label:"跟随配置"},{value:"off",label:"关闭"},{value:"strict",label:"按需"},{value:"always",label:"始终"}]}/></DS.SettingRow>}
    {section==="connections" && <DS.SettingRow label="连接保活间隔" description="秒；留空跟随配置。" configKey="keep-alive-interval"><DS.SettingField aria-label="连接保活间隔" type="text" width={120} suffix="秒" value={saved?.keepAliveInterval==null?"":String(saved.keepAliveInterval)} disabled={!saved||busy} validate={text=>{const raw=text.trim();if(!raw)return null;return /^\d+$/.test(raw)&&Number(raw)>=1&&Number(raw)<=86400?null:"保活间隔请输入 1 到 86400 秒之间的整数，留空则跟随配置。"}} onCommit={value=>{if(!saved)return;const raw=String(value).trim();const next=raw?Number(raw):null;if(next!==saved.keepAliveInterval)void commit({...saved,keepAliveInterval:next})}}/></DS.SettingRow>}
    {message && <p role="status">{message}</p>}
  </DS.SettingsSection>;
}
