import React,{useState} from "react";
import type * as Design from "../design-system/components/index";
import type {ConnectionSnapshot} from "./mygo";
import {filterConnections} from "./listFilters";

interface Props{design:typeof Design;running:boolean;snapshot:ConnectionSnapshot|null;onClose:(id:string)=>void;onCloseAll:()=>Promise<void>;onContextMenu:(connection:Design.Connection,at:{x:number;y:number})=>void}
export function ConnectionList({design:DS,running,snapshot,onClose,onCloseAll,onContextMenu}:Props){
  const [query,setQuery]=useState(""),[network,setNetwork]=useState("all"),[busy,setBusy]=useState(false);
  const all=snapshot?.connections||[],visible=filterConnections(all,query,network);
  async function closeAll(){if(!window.confirm(`关闭全部 ${all.length} 个连接？${query.trim()||network!=="all"?"包括当前筛选之外的连接。":""}应用可能会自动重新连接。`))return;setBusy(true);try{await onCloseAll()}finally{setBusy(false)}}
  return <><div className="connection-tools"><DS.SearchField aria-label="搜索连接" placeholder="主机、应用、规则或节点" value={query} onValueChange={setQuery}/><select aria-label="连接协议" value={network} onChange={e=>setNetwork(e.target.value)}><option value="all">全部协议</option><option value="TCP">TCP</option><option value="UDP">UDP</option></select><button className="secondary" disabled={!running||busy||all.length===0} onClick={()=>void closeAll()}>关闭全部连接</button></div>
    <p>{visible.length} / {all.length} 个连接</p><DS.ConnectionTable empty={!running?"启动内核后查看连接。":all.length?"没有匹配的连接。":"当前没有活动连接。"} connections={visible.map(c=>({id:c.id,host:c.metadata.host||c.metadata.destinationIP,port:Number(c.metadata.destinationPort)||undefined,network:c.metadata.network.toUpperCase()==="UDP"?"UDP":"TCP",process:c.metadata.process,rule:c.rule+(c.rulePayload?", "+c.rulePayload:""),chain:[...(c.chains||[])].reverse(),up:c.upload,down:c.download}))} onClose={onClose} onContextMenu={onContextMenu}/></>;
}
