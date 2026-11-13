import {expect,test} from "bun:test";
import React from "react";
import {renderToStaticMarkup} from "react-dom/server";
import {readFileSync} from "node:fs";
import {runInNewContext} from "node:vm";
import {startupCopy} from "../src/StartupSettings";
import {OverviewDeck} from "../src/OverviewDeck";

const window:any={React,matchMedia:()=>({matches:true})};
runInNewContext(readFileSync(new URL("../src/ui/bundle.js",import.meta.url),"utf8"),{window});
const DS=window.RocketScience;

test("startupCopy uses OS-appropriate login wording for windows and linux",()=>{
  const win=startupCopy("windows",false);
  expect(win.loginDescription).toContain("Windows");
  expect(win.loginDescription).not.toContain("macOS");
  expect(win.autoConnectDescription).not.toContain("TUN");

  const linux=startupCopy("linux",false);
  expect(linux.loginDescription).not.toContain("macOS");
  expect(linux.autoConnectDescription).not.toContain("TUN");

  const mac=startupCopy("darwin",true);
  expect(mac.loginDescription).toContain("macOS");
  expect(mac.autoConnectDescription).toContain("TUN");
});

test("OverviewDeck marks TUN unavailable when tunAvailable is false",()=>{
  const html=renderToStaticMarkup(
    <OverviewDeck
      design={DS}
      status={{starting:false,startupStage:"",running:false,version:"",mixedPort:7890,mode:"rule",error:"",corePath:"embedded",systemProxy:true,tun:false,recoveryPending:false,warning:"",geoWarning:"",controller:"",startedAt:""}}
      groups={[]}
      delays={{}}
      busy={false}
      starting={false}
      initialized={true}
      portText="127.0.0.1:7890"
      streamsMemory={null}
      history={null}
      telemetry={null}
      connections={null}
      showFirstRun={false}
      onStartTour={()=>{}}
      onDismissFirstRun={()=>{}}
      onToggle={()=>{}}
      onMode={()=>{}}
      onSystemProxy={()=>{}}
      onTun={()=>{}}
      onCopyCommand={()=>{}}
      onSpeedTest={()=>{}}
      tunAvailable={false}
      systemProxyAutomatic={true}
      manualProxyNote=""
    />
  );
  expect(html).toContain("首版仅在 macOS 提供");
  expect(html).toContain('aria-label="TUN 增强模式" disabled=""');
});
