import type { Connection } from "./mygo";

export interface RouteNode { name:string; count:number; active:boolean; weight:number }
export interface RouteState { counters:Record<string,{node:string;upload:number;download:number}>; lastActivity:Record<string,number> }
export const emptyRouteState = ():RouteState => ({counters:{},lastActivity:{}});

/** mihomo chains are ordered from actual outbound to the outer policy group. */
export function aggregateRoutes(connections:Connection[],previous:RouteState,now:number): {nodes:RouteNode[];state:RouteState;unknown:number} {
  const counters:RouteState["counters"]={},lastActivity:RouteState["lastActivity"]={},counts:Record<string,number>={};
  let unknown=0;
  for(const connection of connections){
    const node=connection.chains?.[0]?.trim();if(!node){unknown++;continue}
    counts[node]=(counts[node]||0)+1;
    const prior=previous.counters[connection.id];
    if(prior && prior.node===node && (connection.upload>prior.upload || connection.download>prior.download))lastActivity[node]=now;
    counters[connection.id]={node,upload:connection.upload,download:connection.download};
  }
  const nodes=Object.entries(counts).map(([name,count])=>{
    const last=lastActivity[name]??previous.lastActivity[name];
    if(last!==undefined && now-last<3000)lastActivity[name]=last;
    return {name,count,active:last!==undefined && now-last<3000,weight:Math.min(4,count)};
  }).sort((a,b)=>b.count-a.count||a.name.localeCompare(b.name));
  return {nodes,state:{counters,lastActivity},unknown};
}
