import React,{useEffect,useRef,useState} from "react";
import {WiFi,Desktop,type WiFiSettings as Values,type WiFiStatus} from "./mygo";
import type * as Design from "./ui/index";
export function WiFiSettings({design:DS}:{design:typeof Design}){
  const [saved,setSaved]=useState<Values|null>(null),[state,setState]=useState<WiFiStatus|null>(null),[names,setNames]=useState("");
  const namesRef=useRef(names);
  function remember(next:string){namesRef.current=next;setNames(next)}
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  async function stateOnly(){try{setState(await WiFi.state())}catch(e){setMessage(String(e))}}
  useEffect(()=>{void WiFi.settings().then(v=>{setSaved(v);remember(v.trusted.join("\n"))}).catch(e=>setMessage(String(e)));void stateOnly();const visible=()=>{if(!document.hidden)void stateOnly()};const timer=setInterval(()=>{if(!document.hidden)void stateOnly()},5000);document.addEventListener("visibilitychange",visible);return ()=>{clearInterval(timer);document.removeEventListener("visibilitychange",visible)}},[]);
  async function apply(enabled=saved?.enabled||false){setBusy(true);setMessage("");try{const v=await WiFi.setSettings({enabled,trusted:namesRef.current.split("\n").filter(s=>s!=="")});setSaved(v);remember(v.trusted.join("\n"))}catch(e){setMessage(String(e));setSaved(await WiFi.settings().catch(()=>saved))}finally{await stateOnly();setBusy(false)}}
  async function access(){setBusy(true);setMessage("");try{await Desktop.requestWiFiAccess();setMessage("请完成 macOS 定位授权；随后会重新识别 Wi-Fi。")}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  async function check(){setBusy(true);try{await WiFi.check();await stateOnly()}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  return <DS.SettingsSection title="可信 Wi-Fi 直连" description="在指定 Wi-Fi 上自动使用直连模式；离开后恢复之前的模式。手动选择模式时会保留你的选择。">
    <DS.SettingRow label="自动直连" description="不会自动启动已停止的内核；无法确认网络名称时退出自动直连。"><DS.Switch aria-label="可信 Wi-Fi 自动直连" checked={!!saved?.enabled} disabled={busy||!saved} onChange={enabled=>void apply(enabled)}/></DS.SettingRow>
    <DS.SettingRow label="当前网络"><code className="lf-setrow__value">{state?.known?(state.ssid||"非 Wi-Fi 网络"):"未知"}</code></DS.SettingRow>
    {state?.detail&&<p>{state.detail}</p>}
    <DS.SettingRow label="可信 Wi-Fi 名称" description="每行一个，区分大小写。点保存列表后生效。" stack><DS.ListField aria-label="可信 Wi-Fi 名称" value={names.split("\n").filter(Boolean)} disabled={busy||!saved} onCommit={lines=>remember(lines.join("\n"))}/></DS.SettingRow>
    <DS.SettingRow label="列表操作">
      <DS.Button size="sm" disabled={busy||!saved} onClick={()=>void apply()}>保存列表</DS.Button>
      <DS.Button size="sm" variant="ghost" disabled={busy||!state?.known||!state.ssid} onClick={()=>{if(state?.ssid)remember([...new Set([...namesRef.current.split("\n").filter(Boolean),state.ssid])].join("\n"))}}>加入当前 Wi-Fi</DS.Button>
      <DS.Button size="sm" variant="ghost" disabled={busy} onClick={()=>void check()}>重新检查</DS.Button>
      <DS.Button size="sm" variant="ghost" disabled={busy} onClick={()=>void access()}>授权读取 Wi-Fi 名称</DS.Button>
    </DS.SettingRow>
    {message&&<p role="status">{message}</p>}
  </DS.SettingsSection>;
}
