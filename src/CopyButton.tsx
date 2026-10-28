import React,{useEffect,useRef,useState} from "react";
import type * as Design from "../design-system/components/index";

export function CopyButton({design:DS,copy,label="复制",disabled=false}:{design:typeof Design;copy:()=>Promise<void>;label?:string;disabled?:boolean}) {
  const [busy,setBusy]=useState(false),[copied,setCopied]=useState(false),[error,setError]=useState("");
  const mounted=useRef(true),timer=useRef<ReturnType<typeof setTimeout>|undefined>(undefined);
  useEffect(()=>{mounted.current=true;return ()=>{mounted.current=false;if(timer.current)clearTimeout(timer.current)}},[]);
  async function perform(){if(busy||disabled)return;setBusy(true);setCopied(false);setError("");if(timer.current)clearTimeout(timer.current);try{await copy();if(mounted.current){setCopied(true);timer.current=setTimeout(()=>setCopied(false),1500)}}catch(e){if(mounted.current)setError(String(e))}finally{if(mounted.current)setBusy(false)}}
  return <div><DS.Button size="sm" variant="ghost" icon={copied?"check":"copy"} disabled={busy||disabled} onClick={()=>void perform()}>{busy?"复制中…":copied?"已复制":label}</DS.Button>{error&&<p role="alert" className="profile-error">{error}</p>}</div>;
}
