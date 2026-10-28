import React,{useEffect,useState} from "react";
import {WiFi,Desktop,type WiFiSettings as Values,type WiFiStatus} from "./mygo";
import type * as Design from "../design-system/components/index";
export function WiFiSettings({design:DS}:{design:typeof Design}){
  const [saved,setSaved]=useState<Values|null>(null),[state,setState]=useState<WiFiStatus|null>(null),[names,setNames]=useState("");
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  async function stateOnly(){try{setState(await WiFi.state())}catch(e){setMessage(String(e))}}
  useEffect(()=>{void WiFi.settings().then(v=>{setSaved(v);setNames(v.trusted.join("\n"))}).catch(e=>setMessage(String(e)));void stateOnly();const timer=setInterval(()=>void stateOnly(),5000);return ()=>clearInterval(timer)},[]);
  async function apply(enabled=saved?.enabled||false){setBusy(true);setMessage("");try{const v=await WiFi.setSettings({enabled,trusted:names.split("\n").filter(s=>s!=="")});setSaved(v);setNames(v.trusted.join("\n"))}catch(e){setMessage(String(e));setSaved(await WiFi.settings().catch(()=>saved))}finally{await stateOnly();setBusy(false)}}
  async function access(){setBusy(true);setMessage("");try{await Desktop.requestWiFiAccess();setMessage("请完成 macOS 定位授权；随后会重新识别 Wi-Fi。")}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  async function check(){setBusy(true);try{await WiFi.check();await stateOnly()}catch(e){setMessage(String(e))}finally{setBusy(false)}}
  return <section className="panel settings"><div className="eyebrow">TRUSTED WI-FI</div><h2>可信 Wi-Fi 直连</h2><p>在指定 Wi-Fi 上自动使用直连模式；离开后恢复之前的模式。手动选择模式时会保留你的选择。</p>
    <div className="setting"><div><h3>自动直连</h3><p>不会自动启动已停止的内核；无法确认网络名称时退出自动直连。</p></div><DS.Switch aria-label="可信 Wi-Fi 自动直连" checked={!!saved?.enabled} disabled={busy||!saved} onChange={enabled=>void apply(enabled)}/></div>
    <p>当前网络：{state?.known?(state.ssid||"非 Wi-Fi 网络"):"未知"}</p>{state?.detail&&<p>{state.detail}</p>}
    <label className="bypass-list">可信 Wi-Fi 名称（每行一个，区分大小写）<textarea aria-label="可信 Wi-Fi 名称" value={names} disabled={busy||!saved} onChange={e=>setNames(e.target.value)}/></label>
    <div className="actions"><button disabled={busy||!saved} onClick={()=>void apply()}>保存列表</button><button disabled={busy||!state?.known||!state.ssid} onClick={()=>{if(state?.ssid)setNames([...new Set([...names.split("\n").filter(Boolean),state.ssid])].join("\n"))}}>加入当前 Wi-Fi</button><button disabled={busy} onClick={()=>void check()}>重新检查</button><button disabled={busy} onClick={()=>void access()}>授权读取 Wi-Fi 名称</button></div>{message&&<p role="status">{message}</p>}
  </section>;
}
