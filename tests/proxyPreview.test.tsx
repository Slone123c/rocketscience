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
