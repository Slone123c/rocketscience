import {expect,test} from "bun:test";
import {proxyRouteSummary} from "../src/proxyRouteSummary";
import type {Connection,Proxy} from "../src/mygo";

const group=(name:string,now:string,type="Selector"):Proxy=>({name,now,type,all:now?[now]:[],nodes:[]});
const connection=(...chains:string[])=>({chains} as Connection);

test("stopped preview never treats stale selectors or connections as current usage",()=>{
  const result=proxyRouteSummary(false,"global",[group("GLOBAL","Tokyo")],[connection("Tokyo","GLOBAL")]);
  expect(result.title).toBe("当前未使用任何节点");expect(result.path).toEqual([]);expect(result.actual).toEqual([]);
});
test("global selection resolves nested groups but keeps actual connection usage separate",()=>{
  const result=proxyRouteSummary(true,"global",[group("GLOBAL","自动"),group("自动","Tokyo","URLTest")],[connection("London","自动","GLOBAL")]);
  expect(result.title).toBe("全局出站：Tokyo");expect(result.path).toEqual(["GLOBAL","自动","Tokyo"]);
  expect(result.actual).toEqual([{name:"London",count:1}]);expect(result.groupUsage["自动"]).toBe(1);
});
test("rule routing aggregates real outbounds and never chooses an arbitrary main group",()=>{
  const result=proxyRouteSummary(true,"rule",[group("主选择","Tokyo"),group("AI","London")],[connection("London","AI","AI"),connection("DIRECT"),connection("London","AI"),connection()]);
  expect(result.title).toBe("当前出站：按规则分流");expect(result.actual).toEqual([{name:"London",count:2},{name:"DIRECT",count:1}]);expect(result.unknown).toBe(1);expect(result.groupUsage.AI).toBe(2);expect(result.groupUsage["主选择"]).toBeUndefined();
});
test("direct, empty and unobserved states do not invent a used proxy",()=>{
  expect(proxyRouteSummary(true,"direct",[group("GLOBAL","Tokyo")],[]).title).toBe("当前出站：直连");
  expect(proxyRouteSummary(true,"rule",[],[]).actual).toEqual([]);
  expect(proxyRouteSummary(true,"rule",[],null).observed).toBe(false);
});
test("missing, invalid, cyclic and load-balanced selections do not claim a final node",()=>{
  for(const groups of [[],[group("GLOBAL","")],[group("GLOBAL","A"),group("A","GLOBAL")],[{...group("GLOBAL","missing"),all:[]}]]){
    expect(proxyRouteSummary(true,"global",groups,[]).title).toBe("全局出站待确认");
  }
  expect(proxyRouteSummary(true,"global",[group("GLOBAL","均衡"),group("均衡","Tokyo","LoadBalance")],[]).title).toContain("按策略分配");
});
