import React, {useEffect,useRef,useState} from "react";
import type * as Design from "./ui/index";
import {Service,type ConnectionSnapshot,type ForwardRoute,type IPLocation,type NodeLocation} from "./mygo";
import {aggregateRoutes,emptyRouteState,type RouteNode} from "./trajectoryData";
import {localOrigin} from "./localOrigin";

const LOCAL=["DIRECT","REJECT","REJECT-DROP","PASS"];
// Only the busiest nodes get a route: each exit check sends a request through the node.
const CORE_NODES=5;

// The backend caches the lookup for ten minutes; ask again at that pace.
function useIPLocation():IPLocation|null {
  const [ip,setIP]=useState<IPLocation|null>(null);
  useEffect(()=>{
    let stopped=false,timer:ReturnType<typeof setTimeout>|undefined;
    const read=async()=>{
      let next:IPLocation|null=null;
      try{next=await Service.localIPLocation()}catch{next=null}
      if(stopped)return;
      setIP(next);
      timer=setTimeout(read,next?.known?600000:60000);
    };
    void read();
    return ()=>{stopped=true;clearTimeout(timer)};
  },[]);
  return ip;
}

// "Los Angeles, US": city and country code, whichever are known.
const placeOf=(location:{city?:string;country?:string})=>[location.city,location.country].filter(Boolean).join(", ");

// Hand each segment its own hops (address, latency, -1 = no answer), so the strip replays the real trace.
// Segments cover the hops in order; a count that does not add up leaves the segment without them.
function withHops(forward:ForwardRoute):Design.RouteSegment[] {
  const hops=forward.hops||[];
  let at=0;
  return (forward.segments||[]).map(segment=>{
    const slice=hops.slice(at,at+segment.hops).map(hop=>({ip:hop.ip,ms:hop.ms,place:hop.country}));
    at+=segment.hops;
    const kind=segment.kind as Design.RouteSegment["kind"];
    return slice.length===segment.hops?{...segment,kind,hopList:slice}:{...segment,kind};
  });
}

// Lookups run in the background; ask again soon while any is pending.
function useNodeLocations(names:string[]):Record<string,NodeLocation> {
  const [locations,setLocations]=useState<Record<string,NodeLocation>>({});
  const key=names.join("\n");
  useEffect(()=>{
    if(!key){setLocations({});return}
    let stopped=false,timer:ReturnType<typeof setTimeout>|undefined;
    const read=async()=>{
      let next:Record<string,NodeLocation>={};
      try{next=await Service.nodeLocations(key.split("\n"))||{}}catch{next={}}
      if(stopped)return;
      setLocations(next);
      timer=setTimeout(read,Object.values(next).some(location=>location.pending)?3000:120000);
    };
    void read();
    return ()=>{stopped=true;clearTimeout(timer)};
  },[key]);
  return locations;
}

export function TrajectoryPanel({design:DS,running,connections}:{design:typeof Design;running:boolean;connections:ConnectionSnapshot|null}) {
  const prior=useRef(emptyRouteState());
  const [routes,setRoutes]=useState<RouteNode[]>([]),[unknownCount,setUnknownCount]=useState(0);
  const ip=useIPLocation();
  const origin=localOrigin(ip,Intl.DateTimeFormat().resolvedOptions().timeZone||"",DS.geoOf);
  const originPlace=ip?.known?placeOf(ip):"";
  useEffect(()=>{
    if(!running || !connections){prior.current=emptyRouteState();setRoutes([]);setUnknownCount(0);return}
    const result=aggregateRoutes(connections.connections,prior.current,Date.now());prior.current=result.state;setRoutes(result.nodes);setUnknownCount(result.unknown);
    const timer=setTimeout(()=>setRoutes(current=>current.map(node=>({...node,active:false}))),3000);
    return ()=>clearTimeout(timer);
  },[running,connections]);
  const local=(node:RouteNode)=>LOCAL.includes(node.name.toUpperCase());
  const core=routes.filter(node=>!local(node)).sort((a,b)=>b.count-a.count).slice(0,CORE_NODES);
  const located=useNodeLocations(running?core.map(node=>node.name):[]);
  const placed=core.map(node=>{
    const {entry,exit,forward,pending}=located[node.name]||{};
    const geo:[number,number]|null=exit?.known?[exit.lon,exit.lat]:DS.geoOf(node.name);
    const entryAt:[number,number]|undefined=entry?.known?[entry.lon,entry.lat]:undefined;
    const exitPlace=exit?.known?placeOf(exit):"",entryPlace=entry?.known?placeOf(entry):"";
    // An anycast entry names its network, with the site when it could be asked.
    const entryLabel=[entry?.network,entryPlace].filter(Boolean).join(" · ");
    // Only measured ends go on the map: routers register to a carrier's head office, not where they stand.
    const hops=entryAt?[{at:entryAt,place:entryPlace,country:entry?.country}]:[];
    const relay=!!(entry?.known&&exit?.known&&entry.country&&exit.country&&entry.country!==exit.country);
    // A trace that has not finished yet shows as pending; a failed one shows its reason.
    const route=forward?.known||forward?.error?{segments:withHops(forward),error:forward.known?"":forward.error}:undefined;
    return {node,geo,measured:!!exit?.known,entry:entryAt,hops:hops.length?hops:undefined,place:exitPlace,country:exit?.known?exit.country:undefined,entryPlace:entryLabel,relay,route,tracing:!!pending&&!forward?.known};
  }).filter(item=>item.geo);
  return <section className="panel"><div className="cap">连接轨迹</div>
    <DS.Trajectory origin={originPlace} originCountry={ip?.known?ip.country:undefined} originGeo={origin.geo} nodes={placed.map(({node,geo,measured,entry,hops,place,country,entryPlace,relay,route,tracing})=>({name:node.name,active:node.active,weight:node.weight,lon:geo![0],lat:geo![1],entry,hops,place,country,estimated:!measured,entryPlace:entryPlace||undefined,relay,route:route as Design.TrajectoryRoute|undefined,tracing}))} caption={running?undefined:"启动内核后显示活动连接"}/>
    {running && routes.length===0 && unknownCount===0 && <p className="empty">当前没有活动连接。</p>}
  </section>;
}
