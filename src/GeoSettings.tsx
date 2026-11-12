import React, {useEffect,useState} from "react";
import {Profiles,Service,type GeoOptions} from "./mygo";
import type * as Design from "./ui/index";

const selectValue=(v:boolean|null)=>v==null?"inherit":v?"on":"off";
const boolValue=(v:string)=>v==="inherit"?null:v==="on";
export function GeoSettings({design:DS,running,warning}:{design:typeof Design;running:boolean;warning:string}) {
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
  return <DS.SettingsSection title="GEO 数据库" description="跟随配置，或单独设置数据格式和自动更新；下载地址由所选配置决定。">
    <DS.SettingRow label="数据格式" configKey="geodata-mode"><DS.Select aria-label="GEO 数据格式" disabled={!draft||busy} value={selectValue(draft?.dataMode??null)} onChange={value=>draft&&setDraft({...draft,dataMode:boolValue(value)})} options={[{value:"inherit",label:"跟随配置"},{value:"off",label:"MMDB"},{value:"on",label:"DAT"}]}/></DS.SettingRow>
    <DS.SettingRow label="自动更新" configKey="geo-auto-update"><DS.Select aria-label="GEO 自动更新" disabled={!draft||busy} value={selectValue(draft?.autoUpdate??null)} onChange={value=>draft&&setDraft({...draft,autoUpdate:boolValue(value)})} options={[{value:"inherit",label:"跟随配置"},{value:"on",label:"开启"},{value:"off",label:"关闭"}]}/></DS.SettingRow>
    <DS.SettingRow label="更新间隔" description="小时；留空跟随配置。" configKey="geo-update-interval"><DS.TextField aria-label="GEO 更新间隔" mono inputMode="numeric" value={interval} disabled={!draft||busy} onChange={e=>setIntervalValue(e.target.value)}/></DS.SettingRow>
    <DS.SettingRow label="保存">
      <DS.Button size="sm" disabled={!draft||busy} onClick={()=>void apply()}>应用 GEO 设置</DS.Button>
      <DS.Button size="sm" variant="ghost" disabled={!saved||busy} onClick={()=>{if(saved)load(saved);setMessage("")}}>撤回修改</DS.Button>
      <DS.Button size="sm" variant="ghost" icon="refresh" disabled={!running||busy} onClick={()=>void update()}>立即更新</DS.Button>
    </DS.SettingRow>
    {warning && <p className="profile-error" role="alert">{warning} <DS.Button size="sm" variant="ghost" disabled={busy} onClick={()=>void recover()}>重新检查恢复记录</DS.Button></p>}
    {message && <p role="status">{message}</p>}
  </DS.SettingsSection>;
}
