import React,{useEffect,useRef,useState} from "react";
import {Desktop,Profiles,type Profile,type ProfileState} from "./mygo";
import {saveEditorProfile} from "./profileEditorSave";
import {readDroppedProfile} from "./profileFileImport";
import type * as Design from "../design-system/components/index";

interface Props {design:typeof Design;state:ProfileState|null;running:boolean;busy:boolean;onState:(state:ProfileState)=>void;onChanged:()=>Promise<void>;onGoOverrides:()=>void;onEditorOpen:(open:boolean)=>void}
function subscriptionLabel(raw:string){try{const u=new URL(raw);return u.origin+u.pathname+(u.search?"?•••":"")}catch{return "订阅"}}
export function ProfilesWorkspace({design:DS,state,running,busy:parentBusy,onState,onChanged,onGoOverrides,onEditorOpen}:Props){
  const [open,setOpen]=useState(false),[editing,setEditing]=useState<Profile|null>(null),[draft,setDraft]=useState<Partial<Design.Profile>|null>(null),[tab,setTab]=useState<"info"|"file">("info"),[busy,setBusy]=useState(false),[error,setError]=useState("");
  const lock=useRef(false);
  useEffect(()=>{onEditorOpen(open);return ()=>onEditorOpen(false)},[open,onEditorOpen]);
  async function operation<T,>(run:()=>Promise<T>):Promise<T>{if(lock.current||parentBusy)throw new Error("正在处理，请稍候");lock.current=true;setBusy(true);setError("");try{return await run()}finally{lock.current=false;setBusy(false)}}
  function edit(profile:Profile){if(busy||parentBusy)return;setEditing(profile);setDraft({id:profile.id,name:profile.name,source:profile.source as "url"|"file",url:profile.url,interval:String(profile.interval) as Design.Profile["interval"],path:profile.path,content:profile.content,error:profile.lastError||null});setTab(profile.lastError?"file":"info");setOpen(true)}
  function create(profile:Partial<Design.Profile>={source:"url",name:"",url:"",interval:"24",content:""}){setEditing(null);setDraft(profile);setTab(profile.source==="file"?"file":"info");setOpen(true)}
  async function pickNewFile(){try{await operation(async()=>{const file=await Desktop.pickProfileFile();if(file)create({...file,source:"file",interval:"0"})})}catch(e){setError(String(e))}}
  async function importDropped(file:File){try{await operation(async()=>{create({...await readDroppedProfile(file),source:"file",interval:"0"})})}catch(e){setError(String(e))}}
  useEffect(()=>{
    const view=document.querySelector("main");
    const over=(event:DragEvent)=>{if(event.dataTransfer?.types.includes("Files"))event.preventDefault()};
    const drop=(event:DragEvent)=>{if(!event.dataTransfer?.types.includes("Files"))return;event.preventDefault();const file=event.dataTransfer.files[0];if(file&&!lock.current&&!parentBusy)void importDropped(file)};
    view?.addEventListener("dragover",over);view?.addEventListener("drop",drop);
    return ()=>{view?.removeEventListener("dragover",over);view?.removeEventListener("drop",drop)};
  },[parentBusy]);
  async function save(value:Design.Profile){await operation(async()=>{const next=await saveEditorProfile(value,editing,{fetch:url=>Profiles.fetch(url),save:(profile,expected)=>Profiles.save(profile,expected)});onState(next);setOpen(false);await onChanged()})}
  async function use(id:string){try{await operation(async()=>{onState(await Profiles.use(id));await onChanged()})}catch(e){setError(String(e))}}
  async function update(id:string){try{await operation(async()=>{onState(await Profiles.update(id));await onChanged()})}catch(e){setError(String(e));await onChanged()}}
  async function remove(id:string){if(!window.confirm("删除这份配置？此操作无法撤销。"))return;try{await operation(async()=>{onState(await Profiles.delete(id));if(editing?.id===id)setOpen(false);await onChanged()})}catch(e){setError(String(e));throw e}}
  return <section className="panel" id="tour-profile">
    <div className="section-head"><div><div className="eyebrow">PROFILES</div><h2>配置</h2><p>导入订阅或本地 YAML；运行中修改正在使用的配置会重新加载。</p></div><div className="actions"><DS.Button size="sm" icon="globe" disabled={busy||parentBusy||!state} onClick={()=>create()}>从链接导入</DS.Button><DS.Button size="sm" icon="layers" disabled={busy||parentBusy||!state} onClick={()=>void pickNewFile()}>从文件导入</DS.Button></div></div>
    <fieldset className="profile-list" disabled={busy||parentBusy}>{state?.profiles.map(profile=><div className="profile-wrap" key={profile.id}><DS.ProfileCard name={profile.name} source={profile.source==="url"?"url":"file"} url={profile.source==="url"?subscriptionLabel(profile.url):undefined} active={profile.id===state.activeId} updated={!profile.updatedAt.startsWith("0001")?new Date(profile.updatedAt).toLocaleString():undefined} usage={profile.usage&&profile.usage.total>0?{used:profile.usage.upload+profile.usage.download,total:profile.usage.total}:undefined} expires={profile.usage&&profile.usage.expire>0?new Date(profile.usage.expire*1000).toLocaleDateString():undefined} error={profile.lastError||undefined} updating={busy} onUse={()=>void use(profile.id)} onUpdate={profile.source==="url"?()=>void update(profile.id):undefined} onEdit={()=>edit(profile)}/>{profile.usage&&profile.usage.total===0&&<p className="footnote">已用 {((profile.usage.upload+profile.usage.download)/1073741824).toFixed(1)} GB</p>}</div>)}</fieldset>
    {state?.profiles.length===0&&<p className="empty">还没有配置。导入链接、选择文件，或把 YAML 拖到这里。</p>}{error&&<p className="profile-error" role="alert">{error}</p>}
    <DS.ProfileEditor open={open} profile={draft} busy={busy||parentBusy} running={running} active={!!editing&&editing.id===state?.activeId} defaultTab={tab} onSave={save} onClose={()=>{if(!busy&&!parentBusy)setOpen(false)}} onDelete={editing?()=>remove(editing.id):undefined} onPickFile={async done=>{await operation(async()=>{const file=await Desktop.pickProfileFile();if(file)done(file)})}} onGoOverrides={()=>{setOpen(false);onGoOverrides()}}/>
  </section>;
}
