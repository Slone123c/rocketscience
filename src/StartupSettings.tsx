import React, {useEffect,useState} from "react";
import { Desktop, Profiles } from "./mygo";
import type * as Design from "../design-system/components/index";

export function StartupSettings({design:DS}:{design:typeof Design}) {
  const [login,setLogin]=useState<boolean|null>(null),[connect,setConnect]=useState<boolean|null>(null);
  const [tray,setTray]=useState<string|null>(null);
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  async function read(){try{const [l,c,t,state]=await Promise.all([Desktop.openAtLogin(),Profiles.autoConnect(),Desktop.trayDisplay(),Desktop.state()]);setLogin(l);setConnect(c);setTray(t);if(state.trayError)setMessage(state.trayError)}catch(e){setMessage(String(e))}}
  useEffect(()=>{void read();const focus=()=>void read();window.addEventListener("focus",focus);return ()=>window.removeEventListener("focus",focus)},[]);
  async function changeLogin(enabled:boolean){setBusy(true);setMessage("");try{await Desktop.setOpenAtLogin(enabled);setLogin(await Desktop.openAtLogin())}catch(e){setMessage(String(e));setLogin(await Desktop.openAtLogin().catch(()=>false))}finally{setBusy(false)}}
  async function changeConnect(enabled:boolean){setBusy(true);setMessage("");try{await Profiles.setAutoConnect(enabled);setConnect(enabled)}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  async function changeTray(value:string){setBusy(true);setMessage("");try{await Desktop.setTrayDisplay(value);setTray(value)}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  return <section className="panel settings"><div className="eyebrow">GENERAL / STARTUP</div><h2>启动</h2>
    <div className="setting"><div><h3>登录时打开</h3><p>在 macOS 登录后启动 RocketScience；可在系统设置中管理。</p></div><DS.Switch aria-label="登录时打开" checked={!!login} disabled={login===null||busy} onChange={enabled=>void changeLogin(enabled)}/></div>
    <div className="setting"><div><h3>启动后自动连接</h3><p>使用已选配置及接管方式。TUN 仍需系统管理员授权；网络恢复未完成时不会自动接管。</p></div><DS.Switch aria-label="启动后自动连接" checked={!!connect} disabled={connect===null||busy} onChange={enabled=>void changeConnect(enabled)}/></div>
    <div className="setting"><div><h3>菜单栏显示</h3><p>网速来自内核的实时流量；无法读取时显示“—”。</p></div><select aria-label="菜单栏显示" disabled={tray===null||busy} value={tray||"both"} onChange={e=>void changeTray(e.target.value)}><option value="icon">图标</option><option value="speed">网速</option><option value="both">图标和网速</option></select></div>
    {message && <p role="status">{message}</p>}
  </section>;
}
