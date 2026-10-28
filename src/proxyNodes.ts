import type {Proxy} from "./mygo";
import type {ProxyGroupProps} from "../design-system/components/index";

export interface DelayMeasurement {values:Record<string,number>;completedAt:number}
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
