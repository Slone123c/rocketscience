import React, {useEffect,useState} from "react";
import { Desktop, Profiles } from "./mygo";
import type * as Design from "./ui/index";

export function StartupSettings({design:DS}:{design:typeof Design}) {
  const [login,setLogin]=useState<boolean|null>(null),[connect,setConnect]=useState<boolean|null>(null);
  const [tray,setTray]=useState<string|null>(null),[hideDock,setHideDock]=useState<boolean|null>(null);
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  async function read(){try{const [l,c,t,dock,state]=await Promise.all([Desktop.openAtLogin(),Profiles.autoConnect(),Desktop.trayDisplay(),Desktop.hideDockWhenClosed(),Desktop.state()]);setLogin(l);setConnect(c);setTray(t);setHideDock(dock);if(state.trayError)setMessage(state.trayError)}catch(e){setMessage(String(e))}}
  useEffect(()=>{void read();const focus=()=>void read();window.addEventListener("focus",focus);return ()=>window.removeEventListener("focus",focus)},[]);
  async function changeLogin(enabled:boolean){setBusy(true);setMessage("");try{await Desktop.setOpenAtLogin(enabled);setLogin(await Desktop.openAtLogin())}catch(e){setMessage(String(e));setLogin(await Desktop.openAtLogin().catch(()=>false))}finally{setBusy(false)}}
  async function changeConnect(enabled:boolean){setBusy(true);setMessage("");try{await Profiles.setAutoConnect(enabled);setConnect(enabled)}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  async function changeTray(value:string){setBusy(true);setMessage("");try{await Desktop.setTrayDisplay(value);setTray(value)}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  async function changeHideDock(enabled:boolean){setBusy(true);setMessage("");try{await Desktop.setHideDockWhenClosed(enabled);setHideDock(enabled)}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  return <DS.SettingsSection title="启动">
    <DS.SettingRow label="登录时打开" description="在 macOS 登录后启动 Rocket Science；可在系统设置中管理。"><DS.Switch aria-label="登录时打开" checked={!!login} disabled={login===null||busy} onChange={enabled=>void changeLogin(enabled)}/></DS.SettingRow>
    <DS.SettingRow label="启动后自动连接" description="使用已选配置及接管方式。TUN 仍需系统管理员授权；网络恢复未完成时不会自动接管。"><DS.Switch aria-label="启动后自动连接" checked={!!connect} disabled={connect===null||busy} onChange={enabled=>void changeConnect(enabled)}/></DS.SettingRow>
    <DS.SettingRow label="菜单栏显示" description="图标和网速在内核未运行时只显示图标。只显示网速时，读不到流量显示“—”。"><DS.Select aria-label="菜单栏显示" disabled={tray===null||busy} value={tray||"both"} onChange={value=>void changeTray(value)} options={[{value:"icon",label:"图标"},{value:"speed",label:"网速"},{value:"both",label:"图标和网速"}]}/></DS.SettingRow>
    <DS.SettingRow label="关闭控制台时隐藏 Dock 图标" description="关闭控制台后从 Dock 移除图标，仅保留菜单栏入口；重新打开控制台时恢复。最小化时仍保留 Dock 图标。"><DS.Switch aria-label="关闭控制台时隐藏 Dock 图标" checked={!!hideDock} disabled={hideDock===null||busy} onChange={enabled=>void changeHideDock(enabled)}/></DS.SettingRow>
    {message && <p className="profile-error" role="alert">{message}</p>}
  </DS.SettingsSection>;
}
