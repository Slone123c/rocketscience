import React,{useEffect,useRef,useState} from "react";
import type * as Design from "./ui/index";
import type {App,ConnectionSnapshot} from "./mygo";
import {filterConnections} from "./listFilters";
import {PageTools} from "./pageTools";

interface Props{design:typeof Design;running:boolean;snapshot:ConnectionSnapshot|null;onClose:(id:string)=>void;onCloseAll:()=>Promise<void>;onContextMenu:(connection:Design.Connection,at:{x:number;y:number})=>void;apps?:(paths:string[])=>Promise<Record<string,App>>}
export function ConnectionList({design:DS,running,snapshot,onClose,onCloseAll,onContextMenu,apps}:Props){
  const [query,setQuery]=useState(""),[network,setNetwork]=useState("all"),[busy,setBusy]=useState(false);
  const all=snapshot?.connections||[],visible=filterConnections(all,query,network);
  // Each executable is looked up once; command-line tools come back empty and keep their terminal tile.
  const [known,setKnown]=useState<Record<string,App>>({}),asked=useRef(new Set<string>());
  const pending=[...new Set(all.map(c=>c.metadata.processPath).filter(path=>path&&!asked.current.has(path)))];
  const pendingKey=pending.join("\n");
  useEffect(()=>{if(!apps||!pending.length)return;pending.forEach(path=>asked.current.add(path));void apps(pending).then(found=>setKnown(old=>({...old,...found}))).catch(()=>{})},[apps,pendingKey]);
  async function closeAll(){if(!window.confirm(`关闭全部 ${all.length} 个连接？${query.trim()||network!=="all"?"包括当前筛选之外的连接。":""}应用可能会自动重新连接。`))return;setBusy(true);try{await onCloseAll()}finally{setBusy(false)}}
  return <>
    <PageTools>
      <DS.SearchField aria-label="搜索连接" placeholder="主机、应用、规则或节点" autoCapitalize="off" autoCorrect="off" spellCheck={false} value={query} onValueChange={setQuery}/>
      <DS.Select aria-label="连接协议" value={network} onChange={setNetwork} options={[{value:"all",label:"全部协议"},{value:"TCP",label:"TCP"},{value:"UDP",label:"UDP"}]}/>
      <DS.Button size="sm" variant="ghost" icon="close" disabled={!running||busy||all.length===0} onClick={()=>void closeAll()}>全部关闭</DS.Button>
    </PageTools>
    <p className="ovr-intro"><span>{visible.length} / {all.length} 个连接</span></p>
    <DS.ConnectionTable empty={!running?"启动内核后查看连接。":all.length?"没有匹配的连接。":"当前没有活动连接。"} connections={visible.map(c=>({id:c.id,host:c.metadata.host||c.metadata.destinationIP,port:Number(c.metadata.destinationPort)||undefined,network:c.metadata.network.toUpperCase()==="UDP"?"UDP":"TCP",process:c.metadata.process,processPath:c.metadata.processPath||undefined,app:known[c.metadata.processPath]?.name,icon:known[c.metadata.processPath]?.icon,rule:c.rule+(c.rulePayload?", "+c.rulePayload:""),chain:[...(c.chains||[])].reverse(),up:c.upload,down:c.download}))} onClose={onClose} onContextMenu={onContextMenu}/>
    <p className="ovr-intro"><span>右键某条连接，或点 ⋯，可以直接为这个地址添加规则。</span></p>
  </>;
}
