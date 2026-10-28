import React, {useEffect,useRef,useState} from "react";
import type * as Design from "../design-system/components/index";
import type {ConnectionSnapshot} from "./mygo";
import {aggregateRoutes,emptyRouteState,type RouteNode} from "./trajectoryData";

export function TrajectoryPanel({design:DS,running,connections}:{design:typeof Design;running:boolean;connections:ConnectionSnapshot|null}) {
  const prior=useRef(emptyRouteState());
  const [routes,setRoutes]=useState<RouteNode[]>([]),[unknownCount,setUnknownCount]=useState(0);
  useEffect(()=>{
    if(!running || !connections){prior.current=emptyRouteState();setRoutes([]);setUnknownCount(0);return}
    const result=aggregateRoutes(connections.connections,prior.current,Date.now());prior.current=result.state;setRoutes(result.nodes);setUnknownCount(result.unknown);
    const timer=setTimeout(()=>setRoutes(current=>current.map(node=>({...node,active:false}))),3000);
    return ()=>clearTimeout(timer);
  },[running,connections]);
  const inferable=routes.filter(node=>!['DIRECT','REJECT','REJECT-DROP','PASS'].includes(node.name.toUpperCase()) && !!DS.geoOf(node.name));
  const unknown=routes.filter(node=>!inferable.includes(node));
  return <section className="panel"><div className="eyebrow">CONNECTION TRAJECTORY</div><h2>连接轨迹</h2><p>本机位置未知；地图上的节点位置按名称推测，并非实测。</p>
    <DS.Trajectory originGeo={null} nodes={inferable.map(node=>({name:node.name,active:node.active,weight:node.weight,estimated:true}))} caption={running ? "节点位置由名称推测；未显示虚构的本机起点" : "启动内核后显示活动连接"}/>
    {running && routes.length===0 && unknownCount===0 && <p className="empty">当前没有活动连接。</p>}
    {unknown.length>0 && <p>位置未知：{unknown.map(node=>`${node.name}（${node.count} 个连接）`).join("、")}</p>}
    {unknownCount>0 && <p>{unknownCount} 个连接没有可识别的出口信息。</p>}
  </section>;
}
