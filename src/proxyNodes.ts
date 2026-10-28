import type {Proxy} from "./mygo";
import type {ProxyGroupProps} from "../design-system/components/index";

export interface DelayMeasurement {values:Record<string,number>;completedAt:number}
const DELAY_STORAGE_KEY="rs-shared-delays",SYNC_STORAGE_KEY="rs-sync-tick";
export function saveSharedDelays(startedAt:string,delays:Record<string,DelayMeasurement>):void {
  try{if(typeof localStorage!=="undefined"&&Object.keys(delays).length>0)localStorage.setItem(DELAY_STORAGE_KEY,JSON.stringify({startedAt,delays}))}catch{}
}
export function loadSharedDelays(startedAt:string):Record<string,DelayMeasurement> {
  try{
    if(typeof localStorage==="undefined")return {};
    const raw=localStorage.getItem(DELAY_STORAGE_KEY);
    if(!raw)return {};
    const parsed=JSON.parse(raw) as {startedAt?:string;delays?:Record<string,DelayMeasurement>};
    if(startedAt&&parsed.startedAt&&parsed.startedAt!==startedAt)return {};
    return parsed.delays||{};
  }catch{return {}}
}
export function notifyStateSync():void {
  try{if(typeof localStorage!=="undefined")localStorage.setItem(SYNC_STORAGE_KEY,String(Date.now()))}catch{}
}
export function proxyNodes(group:Proxy,manual?:DelayMeasurement):ProxyGroupProps["nodes"] {
  const details=new Map((group.nodes||[]).map(node=>[node.name,node]));
  return (group.all||[]).map(name=>{
    const node=details.get(name);
    const recorded=node?.delayAt?Date.parse(node.delayAt):NaN;
    const measured=manual&&(!Number.isFinite(recorded)||manual.completedAt>=recorded);
    const delay=measured?(manual.values[name]??0):node?.delay;
    return {name,type:node?.type||undefined,udp:node?.udp??undefined,delay:delay===0?"timeout":delay==null||delay<0?undefined:delay};
  });
}
