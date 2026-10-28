import React, { useEffect, useState } from "react";
import {CopyButton} from "./CopyButton";
import { Desktop, Profiles, type LANInterface, type LANSettings } from "./mygo";
import type * as Design from "../design-system/components/index";

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
  return <section className="panel settings"><div className="eyebrow">LAN SHARING</div><h2>局域网共享</h2>
    <div className="setting"><div><h3>允许局域网设备连接</h3><p>{running ? "停止内核后可修改。" : "开启后，其他设备可以使用本机代理；身份验证取决于配置文件。配置中的独立监听器不受此开关控制。"}</p></div><DS.Switch aria-label="局域网共享" checked={!!settings?.enabled} disabled={!settings || busy || running} onChange={enabled=>settings && void save({...settings,enabled})}/></div>
    <div className="setting"><div><h3>监听网卡</h3><p>本机代理始终可用。指定网卡不可用时，启动会提示重新选择。</p></div><select aria-label="局域网监听网卡" disabled={!settings || busy || running} value={settings?.address || "*"} onChange={e=>settings && void save({...settings,address:e.target.value})}><option value="*">所有网卡</option>{interfaces.map(i=><option key={i.name+"-"+i.address} value={i.address}>{i.name} · {i.address}</option>)}{settings?.address && settings.address!=="*" && !interfaces.some(i=>i.address===settings.address) && <option value={settings.address}>{settings.address}（不可用）</option>}</select></div>
    {settings?.enabled && <div><p>其他设备的代理地址：</p>{addresses.length ? addresses.map(i=><div key={i.name+i.address}>{i.name}：{i.address}:{port} <CopyButton design={DS} label="复制地址" copy={()=>Desktop.copyText(`${i.address}:${port}`)}/></div>) : <p>未找到可用的 IPv4 地址。</p>}<button className="text-button" onClick={()=>void refreshInterfaces()}>刷新网卡</button></div>}
    {message && <p role="status">{message}</p>}
  </section>;
}
