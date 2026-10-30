import {expect,test} from "bun:test";
import {activeProxyFeatures,confirmProxyCloseMessage,createNetworkCheckFlow,type CheckState} from "../src/networkCheckFlow";
import type {Connectivity} from "../src/mygo";

function deferred<T>() {let resolve!:(value:T)=>void,reject!:(error:unknown)=>void;const promise=new Promise<T>((yes,no)=>{resolve=yes;reject=no});return {promise,resolve,reject}}
const tick=async()=>{for(let i=0;i<8;i++)await Promise.resolve()};
const good=():Connectivity=>({core:true,request:true,message:"passed",nodeStatus:"ok",nodeDetail:"",recommendations:[]});
const recommendation=()=>({...good(),nodeStatus:"fail",recommendations:[{group:"policy",current:"old",node:"new",delay:20}]});
function harness(overrides:Partial<Parameters<typeof createNetworkCheckFlow>[0]>={}) {
  const events:string[]=[],states:CheckState[]=[];
  const flow=createNetworkCheckFlow({check:async()=>{events.push("check");return good()},launch:async()=>{events.push("launch")},recover:async()=>{events.push("recover")},refresh:async()=>{events.push("refresh")},confirm:()=>{events.push("confirm");return true},select:async()=>{events.push("select")},...overrides},s=>states.push(s));
  return {flow,events,states};
}

test("repeated checks share one request and failure does not poison retry",async()=>{
  const wait=deferred<Connectivity>();let calls=0;
  const {flow}=harness({check:()=>++calls===1?wait.promise:Promise.resolve(good())});
  const a=flow.run(),b=flow.run();expect(a).toBe(b);await tick();expect(calls).toBe(1);
  wait.reject(new Error("offline"));await a;
  expect(flow.snapshot().phase).toBe("idle");expect(flow.snapshot().message).toContain("offline");
  await flow.run();expect(calls).toBe(2);expect(flow.snapshot().phase).toBe("idle");expect(flow.snapshot().result?.request).toBe(true);
});

test("recovery preempts a pending read and late success cannot undo restored state",async()=>{
  const wait=deferred<Connectivity>();
  const {flow,events}=harness({check:()=>wait.promise});
  const check=flow.run();await tick();await flow.recover();
  expect(events).toEqual(["recover","refresh"]);expect(flow.snapshot().recovered).toBe(true);
  wait.resolve(good());await check;
  expect(flow.snapshot().result).toBeNull();expect(flow.snapshot().recovered).toBe(true);
});

test("late check errors cannot replace recovery or its message",async()=>{
  const wait=deferred<Connectivity>();const {flow}=harness({check:()=>wait.promise});
  const check=flow.run();await tick();await flow.recover();wait.reject(new Error("old request failed"));await check;
  expect(flow.snapshot().message).toBe("已停止代理并恢复原有网络设置。");
});

test("repeated recovery is exclusive and another check cannot supersede it",async()=>{
  const wait=deferred<void>();let recoveries=0,checks=0;
  const {flow}=harness({recover:()=>{recoveries++;return wait.promise},check:async()=>{checks++;return good()}});
  const a=flow.recover();expect(flow.recover()).toBe(a);expect(flow.run()).toBe(a);await tick();
  expect(recoveries).toBe(1);expect(checks).toBe(0);wait.resolve();await a;
});

test("recovery waits for a submitted launch, preventing a late restart",async()=>{
  const wait=deferred<void>(),events:string[]=[];
  const {flow}=harness({launch:async()=>{events.push("launch");await wait.promise;events.push("launched")},recover:async()=>{events.push("recover")},check:async()=>{events.push("check");return good()}});
  const launch=flow.launch();await tick();const recovery=flow.recover();await tick();
  expect(events).toEqual(["launch"]);expect(flow.snapshot().phase).toBe("recovering");
  wait.resolve();await Promise.all([launch,recovery]);
  expect(events).toEqual(["launch","launched","recover"]);expect(flow.snapshot().recovered).toBe(true);
});

test("same-turn recovery cancels a launch that has not yet been submitted",async()=>{
  const {flow,events}=harness();await Promise.all([flow.launch(),flow.recover()]);
  expect(events).toEqual(["recover","refresh"]);
});

test("launch success refreshes and checks, while launch failure stays retryable",async()=>{
  let fail=true;const {flow,events}=harness({launch:async()=>{events.push("launch");if(fail)throw new Error("launch denied")}});
  await flow.launch();expect(flow.snapshot().message).toContain("launch denied");expect(events).toEqual(["launch"]);
  fail=false;await flow.launch();expect(events).toEqual(["launch","launch","refresh","check"]);expect(flow.snapshot().phase).toBe("idle");
});

test("recommendation cancellation preserves the result and approval precedes selection",async()=>{
  let approve=false,checks=0;const {flow,events}=harness({check:async()=>{checks++;return checks===1?recommendation():good()},confirm:()=>{events.push("confirm");return approve}});
  await flow.run();await flow.selectRecommendation();
  expect(events).toEqual(["confirm"]);expect(flow.snapshot().result?.recommendations).toHaveLength(1);
  approve=true;await flow.selectRecommendation();
  expect(events).toEqual(["confirm","confirm","select","refresh"]);expect(checks).toBe(2);expect(flow.snapshot().result?.recommendations).toEqual([]);
});

test("duplicate recommendation clicks cannot create two confirmations or switches",async()=>{
  const approval=deferred<boolean>();let confirms=0,selected=0;
  const {flow}=harness({check:async()=>recommendation(),confirm:()=>{confirms++;return approval.promise},select:async()=>{selected++}});
  await flow.run();const a=flow.selectRecommendation();expect(flow.selectRecommendation()).toBe(a);await tick();
  expect(confirms).toBe(1);approval.resolve(true);await a;expect(selected).toBe(1);
});

test("recovery requested during confirmation prevents the proposed switch",async()=>{
  const approval=deferred<boolean>();const {flow,events}=harness({check:async()=>recommendation(),confirm:()=>approval.promise});
  await flow.run();const selection=flow.selectRecommendation();await tick();const recovery=flow.recover();
  approval.resolve(true);await Promise.all([selection,recovery]);expect(events).toEqual(["recover","refresh"]);
});

test("selection failure preserves an actionable error and makes no follow-up check",async()=>{
  let checks=0;const {flow}=harness({check:async()=>{checks++;return recommendation()},select:async()=>{throw new Error("group changed")}});
  await flow.run();await flow.selectRecommendation();expect(checks).toBe(1);expect(flow.snapshot().message).toContain("group changed");expect(flow.snapshot().phase).toBe("idle");
});

test("failed restoration never claims recovery and can be retried",async()=>{
  let fail=true;const {flow}=harness({recover:async()=>{if(fail)throw new Error("permission denied")}});
  await flow.recover();expect(flow.snapshot().recovered).toBe(false);expect(flow.snapshot().recoveryFailed).toBe(true);expect(flow.snapshot().message).toContain("恢复网络失败：");
  fail=false;await flow.recover();expect(flow.snapshot().recovered).toBe(true);expect(flow.snapshot().recoveryFailed).toBe(false);
});

test("refresh failure after restoration does not misreport restoration as failed",async()=>{
  const {flow}=harness({refresh:async()=>{throw new Error("read unavailable")}});
  await flow.recover();expect(flow.snapshot().recovered).toBe(true);expect(flow.snapshot().message).toContain("网络已恢复，但状态刷新失败");
});

test("disposal suppresses late UI updates and later mounting can request a fresh check",async()=>{
  const wait=deferred<Connectivity>();let calls=0;const {flow,states}=harness({check:()=>++calls===1?wait.promise:Promise.resolve(good())});
  const pending=flow.run();await tick();flow.dispose();const count=states.length;wait.resolve(good());await pending;
  expect(states).toHaveLength(count);flow.activate();await flow.run();expect(flow.snapshot().result?.request).toBe(true);expect(calls).toBe(2);
});

test("an already requested restoration finishes when its UI is disposed",async()=>{
  const {flow,events,states}=harness();const pending=flow.recover();flow.dispose();const count=states.length;await pending;
  expect(events).toEqual(["recover","refresh"]);expect(states).toHaveLength(count);
});

test("prompts to close active proxy features only when proxy is running or pending recovery",async()=>{
  expect(activeProxyFeatures(null)).toEqual([]);
  expect(activeProxyFeatures({running:false,systemProxy:true,tun:true,recoveryPending:false})).toEqual([]);
  expect(confirmProxyCloseMessage({running:false,systemProxy:true,tun:false,recoveryPending:false})).toBeNull();
  expect(confirmProxyCloseMessage({running:true,systemProxy:true,tun:true,recoveryPending:false})).toContain("系统代理、TUN 增强模式");
  expect(confirmProxyCloseMessage({running:false,systemProxy:false,tun:false,recoveryPending:true})).toContain("残留系统代理");

  let allow=false;
  const {flow,events}=harness({confirmCheck:()=>{events.push("confirmCheck");return allow}});
  await flow.run();
  expect(events).toEqual(["confirmCheck"]);
  expect(flow.snapshot().result).toBeNull();
  allow=true;
  await flow.run();
  expect(events).toEqual(["confirmCheck","confirmCheck","check"]);
  expect(flow.snapshot().result?.request).toBe(true);
});
