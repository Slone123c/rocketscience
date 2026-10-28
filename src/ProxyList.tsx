import React,{useState} from "react";
import type * as Design from "../design-system/components/index";
import type {Proxy,ConnectionSnapshot} from "./mygo";
import {proxyNodes,type DelayMeasurement} from "./proxyNodes";
import {filterProxyGroups} from "./listFilters";

import {proxyRouteSummary} from "./proxyRouteSummary";
import {ProxyRouteSummary} from "./ProxyRouteCard";

interface Props{mode?:string;snapshot?:ConnectionSnapshot|null;onConnections?:()=>void;design:typeof Design;groups:Proxy[];running:boolean;delays:Record<string,DelayMeasurement>;testing:string;onSelect:(group:string,node:string)=>void;onTest:(group:string)=>void}
export function ProxyList({design:DS,groups,running,delays,testing,onSelect,onTest,mode="rule",snapshot=null,onConnections}:Props){
  const [query,setQuery]=useState("");const visible=filterProxyGroups(groups,query);const summary=proxyRouteSummary(running,mode,groups,snapshot?.connections??null);
  return <><DS.SearchField aria-label="搜索节点或策略组" placeholder="搜索节点或策略组" autoCapitalize="off" autoCorrect="off" spellCheck={false} value={query} onValueChange={setQuery}/><ProxyRouteSummary design={DS} summary={summary} running={running} onConnections={onConnections}/>{query.trim()&&<p>测速仍覆盖整个策略组。</p>}{!running&&groups.length>0&&<p className="ovr-intro" role="status">配置预览：可查看节点名称与协议；启动后才能选择节点和测速。外部提供者与自动包含的节点需启动后加载。</p>}<div id="tour-nodes">{groups.length===0&&<p className="empty">{running?"没有可测速的策略组，请检查所选配置。":"当前配置未声明节点或策略组。请先导入并使用配置；外部提供者的节点需启动后加载。"}</p>}{groups.length>0&&visible.length===0&&<p className="empty">没有匹配的节点或策略组。</p>}{visible.map(group=><DS.ProxyGroup key={group.name} name={group.name} activity={summary.groupUsage[group.name]} readOnly={!running} type={( ["Selector","URLTest","Fallback","LoadBalance","Relay"].includes(group.type)?group.type:undefined) as Design.ProxyGroupProps["type"]} now={running?group.now:undefined} nodes={proxyNodes(group,running?delays[group.name]:undefined)} testing={testing===group.name} onTest={()=>onTest(group.name)} onSelect={name=>onSelect(group.name,name)}/>)}</div></>;
}
