import React, { useEffect, useState } from "react";
import {CopyButton} from "./CopyButton";
import { Desktop, Profiles, type LANInterface, type LANSettings } from "./mygo";
import type * as Design from "./ui/index";

export function LANSharing({ design: DS, running, port }: { design: typeof Design; running: boolean; port: number }) {
  const [settings, setSettings] = useState<LANSettings | null>(null);
  const [interfaces, setInterfaces] = useState<LANInterface[]>([]);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  async function refreshInterfaces() { try { setInterfaces(await Profiles.lanInterfaces()); } catch(e) { setMessage(String(e)); } }
  useEffect(()=>{void Profiles.lanSettings().then(setSettings).catch(e=>setMessage(String(e)));void refreshInterfaces();},[]);
  async function save(next: LANSettings) {
    setBusy(true);setMessage("");
    try { setSettings(await Profiles.setLANSettings(next)); setMessage("已保存，启动时生效。"); }
    catch(e) { setMessage(String(e)); }
    finally { setBusy(false); }
  }
  const addresses = interfaces.filter(i=>settings?.address === "*" || i.address === settings?.address);
  const nicOptions=[{value:"*",label:"所有网卡"},...interfaces.map(i=>({value:i.address,label:`${i.name} · ${i.address}`}))];
  if(settings?.address && settings.address!=="*" && !interfaces.some(i=>i.address===settings.address)) nicOptions.push({value:settings.address,label:`${settings.address}（不可用）`});
  return <DS.SettingsSection title="局域网共享">
    <DS.SettingRow label="允许局域网设备连接" description={running ? "停止内核后可修改。" : "开启后，其他设备可以使用本机代理；身份验证取决于配置文件。配置中的独立监听器不受此开关控制。"}><DS.Switch aria-label="局域网共享" checked={!!settings?.enabled} disabled={!settings || busy || running} onChange={enabled=>settings && void save({...settings,enabled})}/></DS.SettingRow>
    <DS.SettingRow label="监听网卡" description="本机代理始终可用。指定网卡不可用时，启动会提示重新选择。"><DS.Select aria-label="局域网监听网卡" disabled={!settings || busy || running} value={settings?.address || "*"} onChange={address=>settings && void save({...settings,address})} options={nicOptions}/></DS.SettingRow>
    {settings?.enabled && <DS.SettingRow label="其他设备的代理地址" stack>
      {addresses.length ? addresses.map(i=><div key={i.name+i.address}>{i.name}：{i.address}:{port} <CopyButton design={DS} label="复制地址" copy={()=>Desktop.copyText(`${i.address}:${port}`)}/></div>) : <p>未找到可用的 IPv4 地址。</p>}
      <DS.Button size="sm" variant="ghost" onClick={()=>void refreshInterfaces()}>刷新网卡</DS.Button>
    </DS.SettingRow>}
    {message && <p role="status">{message}</p>}
  </DS.SettingsSection>;
}
