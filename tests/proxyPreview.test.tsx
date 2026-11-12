import {expect,test} from "bun:test";
import React from "react";
import {renderToStaticMarkup} from "react-dom/server";
import {readFileSync} from "node:fs";
import {runInNewContext} from "node:vm";
import {ProxyList} from "../src/ProxyList";
import type * as Design from "./ui/index";

test("stopped preview lets people choose and test the node in use",()=>{
  const window:any={React,matchMedia:()=>({matches:true})};
  runInNewContext(readFileSync(new URL("../src/ui/bundle.js",import.meta.url),"utf8"),{window,console});
  const markup=renderToStaticMarkup(<ProxyList design={window.RocketScience as typeof Design} running={false} groups={[{name:"选择",type:"Selector",now:"东京",all:["东京"],nodes:[{name:"东京",type:"ss",udp:true,delay:null,delayAt:""}]}]} delays={{}} testing={Object.create(null)} onTest={()=>{throw Error("preview test")}} onSelect={()=>{throw Error("preview select")}}/>);
  expect(markup).toContain("下次启动时生效");
  expect(markup).toContain("东京");
  expect(markup).toContain("ss · UDP");
  expect(markup).toContain('aria-label="测速：选择"');
  expect(markup).not.toContain('aria-label="测速：选择" disabled=""');
  expect(markup).toContain(">测速<");
  expect(markup).toContain('aria-checked="true"');
  expect(markup).not.toContain('aria-disabled="true"');
  expect(markup).toContain("当前节点");
  expect(markup).not.toContain("下次启动：");
  // Untested nodes show no placeholder dash.
  expect(markup).not.toContain("—");
});

test("running groups show explicit selected labels while the summary names observed usage separately",()=>{
  const window:any={React,matchMedia:()=>({matches:true})};
  runInNewContext(readFileSync(new URL("../src/ui/bundle.js",import.meta.url),"utf8"),{window,console});
  const markup=renderToStaticMarkup(<ProxyList design={window.RocketScience as typeof Design} running mode="global" groups={[{name:"GLOBAL",type:"Selector",now:"Tokyo",all:["Tokyo"],nodes:[]}]} snapshot={{connections:[{chains:["London","GLOBAL"]} as any],uploadTotal:0,downloadTotal:0}} delays={{}} testing={Object.create(null)} onTest={()=>{}} onSelect={()=>{}} onConnections={()=>{}}/>);
  expect(markup).toContain("全局出站：Tokyo");
  expect(markup).toContain("GLOBAL → Tokyo");
  expect(markup).toContain("活动连接实际出站");
  expect(markup).toContain("London");
  expect(markup).toContain("当前节点");
  expect(markup).toContain("已选");
  expect(markup).toContain("1 条连接");
  expect(markup).toContain("查看连接");
});

test("rule mode shows one main exit and collapses branch groups to their final node",()=>{
  const window:any={React,matchMedia:()=>({matches:true})};
  runInNewContext(readFileSync(new URL("../src/ui/bundle.js",import.meta.url),"utf8"),{window,console});
  const g=(name:string,now:string,all:string[],rules=0,fallback=false)=>({name,type:"Selector",now,all,nodes:[],rules,fallback});
  const groups=[g("节点选择","香港 01",["香港 01","日本 01"],40),g("国外媒体","节点选择",["节点选择","日本 01"],12),g("漏网之鱼","节点选择",["节点选择","DIRECT"],1,true),g("GLOBAL","DIRECT",["节点选择","DIRECT"])];
  const markup=renderToStaticMarkup(<ProxyList design={window.RocketScience as typeof Design} running mode="rule" groups={groups} delays={{}} testing={Object.create(null)} onTest={()=>{}} onSelect={()=>{}}/>);
  expect(markup).toContain('aria-label="主出口"');
  expect(markup).toContain("主出口：节点选择 → 香港 01");
  expect(markup).toContain("跟随主出口");
  expect(markup).toContain("12 条规则");
  expect(markup).toContain("1 条规则 · 兜底");
  expect(markup).not.toContain(">GLOBAL<");
  // Only the main exit starts expanded.
  expect(markup.match(/aria-expanded="true"/g)?.length).toBe(1);
});
