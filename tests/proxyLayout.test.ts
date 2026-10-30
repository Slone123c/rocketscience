import {expect,test} from "bun:test";
import {branchMeta,proxyLayout,routeLabel} from "../src/proxyLayout";
import type {Proxy} from "../src/mygo";

const group=(name:string,type:string,now:string,all:string[],rules=0,fallback=false):Proxy=>({name,type,now,all,nodes:[],rules,fallback});

// Shape of a common subscription: one main selector, branch groups that follow it.
const groups=[
  group("节点选择","Selector","自动选择",["自动选择","香港 01","日本 01"],6883),
  group("自动选择","URLTest","日本 01",["香港 01","日本 01"]),
  group("国外媒体","Selector","节点选择",["节点选择","香港 01","DIRECT"],356),
  group("电报消息","Selector","香港 01",["节点选择","香港 01"],13),
  group("国内服务","Selector","DIRECT",["DIRECT","节点选择"],301),
  group("广告拦截","Selector","REJECT",["REJECT","DIRECT"],1596),
  group("漏网之鱼","Selector","节点选择",["节点选择","DIRECT"],1,true),
  group("GLOBAL","Selector","DIRECT",["节点选择","自动选择","国外媒体","DIRECT"]),
];

test("rule mode picks the most-used group that reaches a node and hides GLOBAL",()=>{
  const {main,branches}=proxyLayout(groups,"rule");
  expect(main?.group.name).toBe("节点选择");
  expect(main?.path).toEqual(["自动选择","日本 01"]);expect(main?.leaf).toBe("日本 01");
  expect(branches.map(route=>route.group.name)).toEqual(["自动选择","国外媒体","电报消息","国内服务","广告拦截","漏网之鱼"]);
});

test("branch summaries say whether a group follows the main exit or was set on its own",()=>{
  const {main,branches}=proxyLayout(groups,"rule");
  const text=(route:Parameters<typeof routeLabel>[0])=>{const {via,leaf}=routeLabel(route,main,true);return via?`${via} ${leaf}`:leaf};
  const summary=Object.fromEntries(branches.map(route=>[route.group.name,text(route)]));
  expect(text(main!)).toBe("当前选择：自动选择 → 日本 01");
  expect(summary["国外媒体"]).toBe("跟随主出口 → 自动选择 → 日本 01");
  expect(summary["电报消息"]).toBe("单独指定 → 香港 01");
  expect(summary["国内服务"]).toBe("直连");
  expect(summary["广告拦截"]).toBe("拒绝");
  expect(summary["自动选择"]).toBe("自动 → 日本 01");
  const meta=Object.fromEntries(branches.map(route=>[route.group.name,branchMeta(route)]));
  expect(meta["漏网之鱼"]).toBe("1 条规则 · 兜底");
  expect(meta["自动选择"]).toBe("被 节点选择 引用");
});

test("global mode exits through GLOBAL",()=>{
  const {main,branches}=proxyLayout([...groups.slice(0,7),group("GLOBAL","Selector","节点选择",["节点选择","DIRECT"])],"global");
  expect(main?.group.name).toBe("GLOBAL");
  expect(main?.path).toEqual(["节点选择","自动选择","日本 01"]);
  expect(branches.some(route=>route.group.name==="GLOBAL")).toBe(false);
});

test("the stopped preview picks the main exit by rule counts and order, and shows what the next start uses",()=>{
  const preview=groups.filter(g=>g.name!=="GLOBAL").map(g=>({...g,now:""}));
  const {main,branches}=proxyLayout(preview,"rule");
  expect(main?.group.name).toBe("节点选择");expect(main?.leaf).toBe("");
  expect(routeLabel(branches[0],main,false)).toEqual({via:"",leaf:"启动后自动选择"});
  // Selectors show their next-start choice; a path into an untested automatic group stays open.
  const next=proxyLayout(groups.filter(g=>g.name!=="GLOBAL").map(g=>g.type==="URLTest"?{...g,now:""}:g),"rule");
  expect(routeLabel(next.main!,next.main,false)).toEqual({via:"下次启动：自动选择 →",leaf:"启动后自动选择"});
  const telegram=next.branches.find(route=>route.group.name==="电报消息")!;
  expect(routeLabel(telegram,next.main,false)).toEqual({via:"单独指定 →",leaf:"香港 01"});
  const unranked=proxyLayout(preview.map(g=>({...g,rules:0})),"rule");
  expect(unranked.main?.group.name).toBe("节点选择");
});

test("cyclic selections stop instead of looping",()=>{
  const {main}=proxyLayout([group("A","Selector","B",["B"],2),group("B","Selector","A",["A"])],"rule");
  expect(main?.group.name).toBe("A");expect(main?.path).toEqual(["B"]);expect(main?.leaf).toBe("");
});
