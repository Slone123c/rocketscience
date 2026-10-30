import React,{useState} from "react";
import type * as Design from "../design-system/components/index";
import type {Proxy,ConnectionSnapshot} from "./mygo";
import {proxyNodes,type DelayMeasurement} from "./proxyNodes";
import {filterProxyGroups} from "./listFilters";
import {PageTools} from "./pageTools";

import {proxyRouteSummary} from "./proxyRouteSummary";
import {ProxyRouteSummary} from "./ProxyRouteCard";
import {branchMeta,proxyLayout,routeLabel,type GroupRoute} from "./proxyLayout";

const GROUP_TYPES=["Selector","URLTest","Fallback","LoadBalance","Relay"];

interface Props{mode?:string;snapshot?:ConnectionSnapshot|null;onConnections?:()=>void;design:typeof Design;groups:Proxy[];running:boolean;delays:Record<string,DelayMeasurement>;testing:Record<string,boolean>;onSelect:(group:string,node:string)=>void;onTest:(group:string)=>void}
export function ProxyList({design:DS,groups,running,delays,testing,onSelect,onTest,mode="rule",snapshot=null,onConnections}:Props){
  const [query,setQuery]=useState("");
  const layout=proxyLayout(groups,mode);
  const main=layout.main;
  const summary=proxyRouteSummary(running,mode,groups,snapshot?.connections??null,main?[main.group.name,...main.path]:[]);
  const searching=!!query.trim();
  const visible=new Map(filterProxyGroups(groups,query).map(group=>[group.name,group]));
  const shown=(route:GroupRoute|null)=>route&&visible.has(route.group.name)?route:null;
  const branches=layout.branches.filter(shown);
  const branchNote=mode.toLowerCase()==="global"?"全局模式下只使用主出口经过的组。":mode.toLowerCase()==="direct"?"直连模式下这些组暂不使用。":"规则命中时才使用。标有“跟随主出口”的组会随主出口一起切换，改选其中节点会让该组单独指定。";
  // Remount on search so matching branch groups open, and close again afterwards.
  const card=(route:GroupRoute,open:boolean)=>{
    const group=visible.get(route.group.name)!;
    const {via,leaf}=routeLabel(route,main,running);
    return <DS.ProxyGroup key={group.name+(searching?":search":"")} defaultOpen={open||searching} name={group.name} via={via} leaf={leaf} meta={branchMeta(route)} activity={summary.groupUsage[group.name]} readOnly={!running&&!group.now} testDisabled={!running} type={(GROUP_TYPES.includes(group.type)?group.type:undefined) as Design.ProxyGroupProps["type"]} now={group.now||undefined} nodes={proxyNodes(group,running?delays[group.name]:undefined)} testing={!!testing[group.name]} onTest={()=>onTest(group.name)} onSelect={name=>onSelect(group.name,name)}/>;
  };
  return <><PageTools><DS.SearchField aria-label="搜索节点或策略组" placeholder="搜索节点或策略组" autoCapitalize="off" autoCorrect="off" spellCheck={false} value={query} onValueChange={setQuery}/></PageTools><ProxyRouteSummary design={DS} summary={summary} running={running} onConnections={onConnections}/>{searching&&<p className="ovr-intro"><span>测速仍覆盖整个策略组。</span></p>}{!running&&groups.length>0&&<p className="ovr-intro" role="status">未启动：可以先选好节点，下次启动时生效；如果节点已不在配置中，启动后会改用实际使用的节点。测速需启动后进行，外部提供者与自动包含的节点也需启动后加载。</p>}<div id="tour-nodes">{groups.length===0&&<p className="empty">{running?"没有可测速的策略组，请检查所选配置。":"当前配置未声明节点或策略组。请先导入并使用配置；外部提供者的节点需启动后加载。"}</p>}{groups.length>0&&visible.size===0&&<p className="empty">没有匹配的节点或策略组。</p>}
    {shown(main)&&<section className="proxy-section" aria-label="主出口"><h2 className="cap">主出口</h2>{card(main!,true)}</section>}
    {branches.length>0&&<section className="proxy-section" aria-label="分流组"><div className="proxy-section__head"><h2 className="cap">分流组</h2><p>{branchNote}</p></div>{branches.map(route=>card(route,false))}</section>}
  </div></>;
}
