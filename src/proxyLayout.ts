import type {Proxy} from "./mygo";

export interface GroupRoute {
  group:Proxy;
  /** Members followed from `group.now` down to the final node; empty while unknown. */
  path:string[];
  /** Final node, or "" when the group has no single selection (preview, LoadBalance, Relay). */
  leaf:string;
  /** The path passes through the main exit, so changing the main exit moves this group too. */
  follows:boolean;
  /** Groups that list this one as a member. */
  parents:string[];
}

export interface ProxyLayout {main:GroupRoute|null;branches:GroupRoute[]}

const blocked=new Set(["REJECT","REJECT-DROP"]);
const terminal=(name:string)=>name==="DIRECT"||blocked.has(name);

function routeOf(group:Proxy,byName:Map<string,Proxy>):string[] {
  const path:string[]=[];
  const seen=new Set([group.name]);
  let current:Proxy|undefined=group;
  while(current&&current.now&&current.type!=="LoadBalance"&&current.type!=="Relay"&&!seen.has(current.now)) {
    path.push(current.now);seen.add(current.now);
    current=byName.get(current.now);
  }
  return path;
}

/**
 * Splits groups into one main exit and the branch groups rules send traffic to.
 * Global mode exits through GLOBAL. Otherwise GLOBAL is hidden, and the main exit
 * is the group most rules target, skipping groups that end in DIRECT or REJECT.
 */
export function proxyLayout(groups:Proxy[],mode:string):ProxyLayout {
  const byName=new Map(groups.map(group=>[group.name,group]));
  const parents=new Map<string,string[]>();
  // GLOBAL lists every group, so it is never a meaningful parent.
  for(const group of groups)if(group.name!=="GLOBAL")for(const member of group.all||[])if(byName.has(member)&&member!==group.name)parents.set(member,[...(parents.get(member)||[]),group.name]);
  const global=mode.toLowerCase()==="global";
  const routes=groups.filter(group=>global||group.name!=="GLOBAL").map(group=>{
    const path=routeOf(group,byName);
    const last=path[path.length-1]||"";
    return {group,path,leaf:byName.has(last)?"":last,follows:false,parents:parents.get(group.name)||[]};
  });
  let main:GroupRoute|null=null;
  if(global)main=routes.find(route=>route.group.name==="GLOBAL")||null;
  if(!main) {
    for(const route of routes) {
      if(terminal(route.leaf)||route.group.all.length>0&&route.group.all.every(terminal))continue;
      if(!main||(route.group.rules||0)>(main.group.rules||0))main=route;
    }
    main??=routes[0]||null;
  }
  const branches=routes.filter(route=>route!==main);
  if(main)for(const route of branches)route.follows=route.path.includes(main.group.name);
  return {main,branches};
}

export function exitName(name:string):string {
  if(name==="DIRECT")return "直连";
  if(blocked.has(name))return "拒绝";
  return name;
}

/**
 * Where a group sends traffic, split so the final node can carry its flag:
 * `via` names how it gets there ("跟随主出口 → 自动选择 →"), `leaf` the node.
 */
export function routeLabel(route:GroupRoute,main:GroupRoute|null,running:boolean):{via:string;leaf:string} {
  const {group,leaf,follows,path}=route;
  const shared=group.type==="LoadBalance"||group.type==="Relay";
  if(shared)return {via:"",leaf:"按策略分配"};
  const auto=group.type==="URLTest"||group.type==="Fallback";
  if(!group.now)return {via:"",leaf:auto?"启动后自动选择":"启动后显示去向"};
  const isMain=route===main;
  if(terminal(leaf))return {via:isMain&&!running?"下次启动：":"",leaf:exitName(leaf)};
  // The path stops at an automatic group that has not tested yet (stopped preview).
  const pending=!leaf&&path.length>0?path[path.length-1]:"";
  const via=(pending?path:path.slice(0,-1)).filter(name=>isMain||name!==main?.group.name).map(exitName);
  const prefix=isMain?(running?(auto?"自动选中：":"当前选择："):"下次启动："):follows?"跟随主出口":auto?"自动":"单独指定";
  const steps=isMain?via:[prefix,...via];
  return {via:isMain?prefix+steps.map(name=>name+" →").join(" "):steps.join(" → ")+" →",leaf:leaf?exitName(leaf):pending?"启动后自动选择":"待定"};
}

/** Why the group matters: rule count, MATCH fallback, or the groups that contain it. */
export function branchMeta(route:GroupRoute):string {
  const parts:string[]=[];
  if(route.group.rules>0)parts.push(`${route.group.rules} 条规则`);
  if(route.group.fallback)parts.push("兜底");
  if(!parts.length&&route.parents.length)parts.push(`被 ${route.parents.join("、")} 引用`);
  return parts.join(" · ")||"规则未使用";
}
