import React,{useState} from "react";
import type * as Design from "../design-system/components/index";
import type {Proxy} from "./mygo";
import {proxyNodes,type DelayMeasurement} from "./proxyNodes";
import {filterProxyGroups} from "./listFilters";

interface Props{design:typeof Design;groups:Proxy[];running:boolean;delays:Record<string,DelayMeasurement>;testing:string;onSelect:(group:string,node:string)=>void;onTest:(group:string)=>void}
export function ProxyList({design:DS,groups,running,delays,testing,onSelect,onTest}:Props){
  const [query,setQuery]=useState("");const visible=filterProxyGroups(groups,query);
  return <><DS.SearchField aria-label="搜索节点或策略组" placeholder="搜索节点或策略组" value={query} onValueChange={setQuery}/>{query.trim()&&<p>测速仍覆盖整个策略组。</p>}<div id="tour-nodes">{groups.length===0&&<p className="empty">{running?"没有可测速的策略组，请检查所选配置。":"启动后查看策略组。"}</p>}{groups.length>0&&visible.length===0&&<p className="empty">没有匹配的节点或策略组。</p>}{visible.map(group=><DS.ProxyGroup key={group.name} name={group.name} type={( ["Selector","URLTest","Fallback","LoadBalance","Relay"].includes(group.type)?group.type:undefined) as Design.ProxyGroupProps["type"]} now={group.now} nodes={proxyNodes(group,delays[group.name])} testing={testing===group.name} onTest={()=>onTest(group.name)} onSelect={name=>onSelect(group.name,name)}/>)}</div></>;
}
