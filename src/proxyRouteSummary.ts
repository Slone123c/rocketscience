import type {Connection,Proxy} from "./mygo";

export interface ProxyRouteSummary {
  title:string; detail:string; path:string[]; pathLabel:string;
  actual:Array<{name:string;count:number}>; unknown:number; observed:boolean;
  groupUsage:Record<string,number>;
}

/** Selection is a routing preference; connection chains are observed usage. */
/** `main` is the main exit's chain from proxyLayout, shown in rule mode. */
export function proxyRouteSummary(running:boolean,mode:string,groups:Proxy[],connections:Connection[]|null,main:string[]=[]):ProxyRouteSummary {
  const result:ProxyRouteSummary={title:"当前未使用任何节点",detail:"内核未启动，下方仅为配置预览。",path:[],pathLabel:"选择路径",actual:[],unknown:0,observed:connections!==null,groupUsage:Object.create(null)};
  if(!running) {
    if(main.length>1){result.detail="内核未启动。下方是下次启动将使用的节点，可以直接修改。";result.path=main.map(name=>name==="DIRECT"?"直连":name==="REJECT"?"拒绝连接":name);result.pathLabel="下次启动"}
    return result;
  }
  const byName=new Map(groups.map(group=>[group.name,group]));
  const counts=new Map<string,number>();
  for(const connection of connections||[]) {
    const outbound=connection.chains?.[0]?.trim();
    if(outbound)counts.set(outbound,(counts.get(outbound)||0)+1);else result.unknown++;
    for(const name of new Set(connection.chains||[]))if(byName.has(name))result.groupUsage[name]=(result.groupUsage[name]||0)+1;
  }
  result.actual=[...counts].map(([name,count])=>({name,count})).sort((a,b)=>b.count-a.count||a.name.localeCompare(b.name));
  if(mode.toLowerCase()==="direct") {
    result.title="当前出站：直连";
    result.detail="直连模式下，新连接直接访问目标；下方策略组的选择暂不用于出站。";
  } else if(mode.toLowerCase()==="global") {
    result.title="全局出站待确认";
    result.detail="新连接通过 GLOBAL 策略组出站；实际使用情况见活动连接。";
    let name="GLOBAL";
    const visited=new Set<string>();
    while(!visited.has(name)) {
      visited.add(name);result.path.push(name);
      const group=byName.get(name);
      if(!group) {
        if(result.path.length>1)result.title=`全局出站：${name==="DIRECT"?"直连":name==="REJECT"?"拒绝连接":name}`;
        break;
      }
      if(group.type==="LoadBalance"||group.type==="Relay") {result.title=`全局出站：${group.name}（按策略分配）`;break}
      if(!group.now||!group.all.includes(group.now))break;
      name=group.now;
    }
  } else if(mode.toLowerCase()==="rule") {
    result.title="当前出站：按规则分流";
    result.detail="大部分流量走主出口；分流组只在规则命中时生效，标有“跟随主出口”的组会随主出口一起切换。";
    if(main.length>1){result.path=main.map(name=>name==="DIRECT"?"直连":name==="REJECT"?"拒绝连接":name);result.pathLabel="主出口"}
  } else {
    result.title="当前出站模式待确认";result.detail="以活动连接中记录的实际出站为准。";
  }
  if(connections===null)result.detail+=" 正在读取活动连接。";
  return result;
}
