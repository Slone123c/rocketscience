import React, {useEffect,useState} from "react";
import {Profiles,Service,type GeoOptions} from "./mygo";

const selectValue=(v:boolean|null)=>v==null?"inherit":v?"on":"off";
const boolValue=(v:string)=>v==="inherit"?null:v==="on";
export function GeoSettings({running,warning}:{running:boolean;warning:string}) {
  const [saved,setSaved]=useState<GeoOptions|null>(null),[draft,setDraft]=useState<GeoOptions|null>(null),[interval,setIntervalValue]=useState("");
  const [busy,setBusy]=useState(false),[message,setMessage]=useState("");
  function load(value:GeoOptions){setSaved(value);setDraft(value);setIntervalValue(value.updateInterval==null?"":String(value.updateInterval))}
  useEffect(()=>{void Profiles.geoSettings().then(load).catch(e=>setMessage(String(e)))},[]);
  async function apply(){
    if(!draft)return;const raw=interval.trim();if(raw&&(!/^\d+$/.test(raw)||Number(raw)<1||Number(raw)>8760)){setMessage("更新间隔请输入 1 到 8760 小时，留空则跟随配置。");return}
    setBusy(true);setMessage("");try{load(await Profiles.setGeoSettings({...draft,updateInterval:raw?Number(raw):null}));setMessage(running?"已应用并重新加载内核。":"已保存，启动时生效。") }catch(e){setMessage(String(e))}finally{setBusy(false)}
  }
  async function update(){setBusy(true);setMessage("正在更新 GEO 数据库…");try{await Service.updateGeo();setMessage("更新完成，数据库已保存。") }catch(e){setMessage(String(e))}finally{setBusy(false)}}
  async function recover(){setBusy(true);setMessage("");try{await Service.recoverGeoCache();setMessage("GEO 恢复记录已检查。") }catch(e){setMessage(String(e))}finally{setBusy(false)}}
  return <section className="panel settings"><div className="eyebrow">GEO DATABASE</div><h2>GEO 数据库</h2><p>跟随配置，或单独设置数据格式和自动更新；下载地址由所选配置决定。</p>
    <div className="setting"><div><h3>数据格式</h3><p>geodata-mode</p></div><select aria-label="GEO 数据格式" value={selectValue(draft?.dataMode??null)} disabled={!draft||busy} onChange={e=>draft&&setDraft({...draft,dataMode:boolValue(e.target.value)})}><option value="inherit">跟随配置</option><option value="off">MMDB</option><option value="on">DAT</option></select></div>
    <div className="setting"><div><h3>自动更新</h3><p>geo-auto-update</p></div><select aria-label="GEO 自动更新" value={selectValue(draft?.autoUpdate??null)} disabled={!draft||busy} onChange={e=>draft&&setDraft({...draft,autoUpdate:boolValue(e.target.value)})}><option value="inherit">跟随配置</option><option value="on">开启</option><option value="off">关闭</option></select></div>
    <div className="setting"><div><h3>更新间隔</h3><p>geo-update-interval · 小时；留空跟随配置。</p></div><input aria-label="GEO 更新间隔" disabled={!draft||busy} inputMode="numeric" value={interval} onChange={e=>setIntervalValue(e.target.value)}/></div>
    <div className="actions"><button disabled={!draft||busy} onClick={()=>void apply()}>应用 GEO 设置</button><button disabled={!saved||busy} onClick={()=>{if(saved)load(saved);setMessage("")}}>撤回修改</button><button disabled={!running||busy} onClick={()=>void update()}>立即更新</button></div>
    {warning && <div><p className="profile-error" role="alert">{warning}</p><button className="text-button" disabled={busy} onClick={()=>void recover()}>重新检查恢复记录</button></div>}{message && <p role="status">{message}</p>}
  </section>;
}
