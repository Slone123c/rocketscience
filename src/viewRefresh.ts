import type {Status,ProfileState,Proxy,ConnectionSnapshot,Rule,Streams,Monitoring,OverrideState} from "./mygo";

export interface ViewRoute {page:string;topic:string}
export interface ViewSource {
  status:()=>Promise<Status>;profiles:()=>Promise<ProfileState>;overrides:()=>Promise<OverrideState>;
  groups:()=>Promise<Proxy[]>;connections:()=>Promise<ConnectionSnapshot>;rules:()=>Promise<Rule[]>;
  streams:()=>Promise<Streams>;monitoring:()=>Promise<Monitoring>;
}
export interface ViewData {
  status:Status;profiles?:ProfileState;overrides?:OverrideState;groups?:Proxy[];
  connections?:ConnectionSnapshot;rules?:Rule[];streams?:Streams;monitoring?:Monitoring;
}
export async function loadViewData(route:ViewRoute,source:ViewSource):Promise<ViewData> {
  const status=await source.status();
  const data:ViewData={status};
  const tasks:Promise<void>[]=[];
  if(route.page==="配置")tasks.push(source.profiles().then(v=>{data.profiles=v}));
  if(route.page==="规则"||route.page==="连接")tasks.push(source.overrides().then(v=>{data.overrides=v}));
  if(status.running) {
    if(route.page==="代理")tasks.push(source.groups().then(v=>{data.groups=v}));
    if(route.page==="总览"||route.page==="连接")tasks.push(source.connections().then(v=>{data.connections=v}));
    if(route.page==="规则")tasks.push(source.rules().then(v=>{data.rules=v}));
    if(route.page==="日志")tasks.push(source.streams().then(v=>{data.streams=v}));
    if(route.page==="总览"||route.page==="设置"&&route.topic==="core")tasks.push(source.monitoring().then(v=>{data.monitoring=v}));
  }
  // Wait for every read to finish even when one fails, so a retry never overlaps
  // outstanding reads from a previous batch.
  const results=await Promise.allSettled(tasks);
  const failed=results.find(result=>result.status==="rejected");
  if(failed?.status==="rejected")throw failed.reason;
  return data;
}

// At most one batch runs at a time. Concurrent refreshes request one follow-up
// batch, which reads the latest route/state after an operation has completed.
export function shouldStopInputKeyPropagation(event:{key?:string;isComposing?:boolean;target?:unknown}):boolean {
  const el=event.target as {tagName?:string;isContentEditable?:boolean}|null;
  if(!el)return false;
  const tag=(el.tagName||"").toUpperCase();
  if(tag!=="INPUT"&&tag!=="TEXTAREA"&&tag!=="SELECT"&&!el.isContentEditable)return false;
  if(event.isComposing)return true;
  return event.key===" "||event.key==="Enter"||event.key==="Backspace"||event.key==="Escape"||event.key==="ArrowLeft"||event.key==="ArrowRight";
}

export function createRefreshQueue(run:()=>Promise<void>) {
  let running=false;
  let waiters:Array<{resolve:()=>void;reject:(error:unknown)=>void}>=[];
  async function drain() {
    running=true;
    while(waiters.length) {
      const batch=waiters;waiters=[];
      try{await run();for(const waiter of batch)waiter.resolve()}
      catch(error){for(const waiter of batch)waiter.reject(error)}
    }
    running=false;
  }
  const request=()=>new Promise<void>((resolve,reject)=>{waiters.push({resolve,reject});if(!running)void drain()});
  if(typeof window!=="undefined"&&typeof window.addEventListener==="function"){
    const onWake=()=>{if(typeof document==="undefined"||!document.hidden)void request().catch(()=>{})};
    const onKey=(e:KeyboardEvent)=>{if(shouldStopInputKeyPropagation(e))e.stopPropagation()};
    window.addEventListener("focus",onWake);
    window.addEventListener("storage",onWake);
    if(typeof document!=="undefined"&&typeof document.addEventListener==="function"){
      document.addEventListener("keydown",onKey);
    }
  }
  return {request};
}
