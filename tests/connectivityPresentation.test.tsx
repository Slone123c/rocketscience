import {expect,test} from "bun:test";
import React from "react";
import {renderToStaticMarkup} from "react-dom/server";
import {readFileSync} from "node:fs";
import {runInNewContext} from "node:vm";
const window:any={React,matchMedia:()=>({matches:true})};
runInNewContext(readFileSync(new URL("../src/ui/bundle.js",import.meta.url),"utf8"),{window});
const DS=window.RocketScience;

test("recovery remains available while diagnostics are still running",()=>{
  const html=renderToStaticMarkup(<DS.ConnectivityCheck checks={[{id:"core",label:"core",status:"running"}]} onRecover={()=>{}}/>);
  expect(html).toContain("停止代理，恢复网络");expect(html).toContain("正在检查");
});
test("legitimately skipped node checks can pass, while unknown checks cannot",()=>{
  const checks=[{id:"core",status:"ok"},{id:"node",status:"skip"},{id:"request",status:"ok"}];
  expect(renderToStaticMarkup(<DS.ConnectivityCheck checks={checks}/>)).toContain("一切正常");
  checks[1].status="idle";
  expect(renderToStaticMarkup(<DS.ConnectivityCheck checks={checks}/>)).not.toContain("一切正常");
});
test("restoration does not claim the previous network was direct",()=>{
  const html=renderToStaticMarkup(<DS.ConnectivityCheck recovered/>);
  expect(html).toContain("恢复原有网络设置");expect(html).not.toContain("直连");
});
