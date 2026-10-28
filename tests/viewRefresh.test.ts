import {describe,expect,test} from "bun:test";
import {createRefreshQueue,loadViewData,shouldStopInputKeyPropagation,type ViewSource} from "../src/viewRefresh";
import type {Status} from "../src/mygo";

function source(running=true) {
  const calls:string[]=[];
  const readers=Object.fromEntries(["profiles","overrides","previewGroups","groups","connections","rules","streams","monitoring"].map(name=>[name,async()=>{calls.push(name);return {}}]));
  return {calls,readers:{...readers,status:async()=>{calls.push("status");return {running} as Status}} as unknown as ViewSource};
}
function deferred(){let resolve!:()=>void;const promise=new Promise<void>(done=>{resolve=done});return {promise,resolve}}

describe("view data scope",()=>{
  test("overview and tray-style monitoring omit logs, YAML and rule lists",async()=>{
    const mock=source();
    await loadViewData({page:"总览",topic:"proxy"},mock.readers);
    expect(mock.calls.sort()).toEqual(["connections","monitoring","status"]);
  });
  test("each operation page reads only the data its controls need",async()=>{
    for(const [page,expected] of [["代理",["connections","groups","status"]],["规则",["overrides","rules","status"]],["连接",["connections","overrides","status"]],["日志",["status","streams"]],["配置",["profiles","status"]]] as const){
      const mock=source();await loadViewData({page,topic:"proxy"},mock.readers);expect(mock.calls.sort()).toEqual([...expected].sort());
    }
    const advanced=source();await loadViewData({page:"设置",topic:"core"},advanced.readers);expect(advanced.calls.sort()).toEqual(["monitoring","status"]);
    const common=source();await loadViewData({page:"设置",topic:"proxy"},common.readers);expect(common.calls).toEqual(["status"]);
  });
  test("stopped core still allows editing rules and profiles without runtime calls",async()=>{
    for(const page of ["总览","代理","日志","连接","规则","配置"]){
      const mock=source(false);await loadViewData({page,topic:"core"},mock.readers);
      expect(mock.calls).toEqual(page==="规则"||page==="连接"?["status","overrides"]:page==="配置"?["status","profiles"]:page==="代理"?["status","previewGroups"]:["status"]);
    }
  });
  test("a failed batch waits for its other outstanding reads before retry",async()=>{
    const mock=source(),pending=deferred();let settled=false;
    mock.readers.monitoring=async()=>{await pending.promise;return {} as never};
    mock.readers.connections=async()=>{throw new Error("offline")};
    const result=loadViewData({page:"总览",topic:"proxy"},mock.readers).then(()=>{settled=true},()=>{settled=true});
    await new Promise(resolve=>setTimeout(resolve,0));expect(settled).toBe(false);
    pending.resolve();await result;expect(settled).toBe(true);
  });
});

describe("refresh queue",()=>{
  test("coalesces concurrent requests and reads the latest route in the follow-up",async()=>{
    const first=deferred();const seen:string[]=[];let route="总览",active=0,maxActive=0;
    const queue=createRefreshQueue(async()=>{active++;maxActive=Math.max(maxActive,active);seen.push(route);if(seen.length===1)await first.promise;active--});
    const a=queue.request();route="代理";const b=queue.request();route="规则";const c=queue.request();
    expect(seen).toEqual(["总览"]);first.resolve();await Promise.all([a,b,c]);
    expect(seen).toEqual(["总览","规则"]);expect(maxActive).toBe(1);
    route="日志";await queue.request();expect(seen.at(-1)).toBe("日志");
  });
  test("a rejected request does not poison subsequent operation refreshes",async()=>{
    let attempts=0;const queue=createRefreshQueue(async()=>{if(++attempts===1)throw new Error("offline")});
    await expect(queue.request()).rejects.toThrow("offline");await queue.request();expect(attempts).toBe(2);
  });
  test("stops Space, Enter, Backspace, Escape, and IME composition in inputs from bubbling to outer cards or popovers",()=>{
    for(const key of [" ","Enter","Backspace","Escape","ArrowLeft","ArrowRight"]){
      expect(shouldStopInputKeyPropagation({key,target:{tagName:"INPUT"}})).toBe(true);
      expect(shouldStopInputKeyPropagation({key,target:{tagName:"TEXTAREA"}})).toBe(true);
      expect(shouldStopInputKeyPropagation({key,target:{tagName:"DIV"}})).toBe(false);
    }
    expect(shouldStopInputKeyPropagation({key:"a",isComposing:true,target:{tagName:"INPUT"}})).toBe(true);
  });
});
