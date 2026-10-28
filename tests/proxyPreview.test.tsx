import {expect,test} from "bun:test";
import React from "react";
import {renderToStaticMarkup} from "react-dom/server";
import {readFileSync} from "node:fs";
import {runInNewContext} from "node:vm";
import {ProxyList} from "../src/ProxyList";
import type * as Design from "../design-system/components/index";

test("stopped configuration preview shows protocol metadata with unavailable controls and no selected node",()=>{
  const window:any={React,matchMedia:()=>({matches:true})};
  runInNewContext(readFileSync(new URL("../design-system/components/bundle.js",import.meta.url),"utf8"),{window,console});
  const markup=renderToStaticMarkup(<ProxyList design={window.RocketScience as typeof Design} running={false} groups={[{name:"选择",type:"Selector",now:"东京",all:["东京"],nodes:[{name:"东京",type:"ss",udp:true,delay:null,delayAt:""}]}]} delays={{}} testing={Object.create(null)} onTest={()=>{throw Error("preview test")}} onSelect={()=>{throw Error("preview select")}}/>);
  expect(markup).toContain("配置预览");
  expect(markup).toContain("东京");
  expect(markup).toContain("ss · UDP");
  expect(markup).toContain('aria-disabled="true"');
  expect(markup).toContain('aria-label="测速：选择" disabled=""');
  expect(markup).not.toContain('aria-checked="true"');
});

test("running groups show explicit selected labels while the summary names observed usage separately",()=>{
  const window:any={React,matchMedia:()=>({matches:true})};
  runInNewContext(readFileSync(new URL("../design-system/components/bundle.js",import.meta.url),"utf8"),{window,console});
  const markup=renderToStaticMarkup(<ProxyList design={window.RocketScience as typeof Design} running mode="global" groups={[{name:"GLOBAL",type:"Selector",now:"Tokyo",all:["Tokyo"],nodes:[]}]} snapshot={{connections:[{chains:["London","GLOBAL"]} as any],uploadTotal:0,downloadTotal:0}} delays={{}} testing={Object.create(null)} onTest={()=>{}} onSelect={()=>{}} onConnections={()=>{}}/>);
  expect(markup).toContain("全局出站：Tokyo");
  expect(markup).toContain("GLOBAL → Tokyo");
  expect(markup).toContain("活动连接实际出站");
  expect(markup).toContain("London");
  expect(markup).toContain("当前选择：");
  expect(markup).toContain("已选");
  expect(markup).toContain("1 条连接");
  expect(markup).toContain("查看连接");
});
