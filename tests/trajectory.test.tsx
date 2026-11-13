import { describe, expect, test } from "bun:test";
import React from "react";
import {renderToStaticMarkup} from "react-dom/server";
import {aggregateRoutes,emptyRouteState} from "../src/trajectoryData";
import type {Connection} from "../src/mygo";

function connection(id:string,node:string,upload=0,download=0):Connection {
  return {id,chains:node?[node,"policy"]:[],upload,download,start:"",rule:"",rulePayload:"",metadata:{host:"",destinationIP:"",destinationPort:"",network:"",process:""}};
}
describe("trajectory data",()=>{
  test("uses actual outbound, aggregates connections, and counts unknown chains",()=>{
    const result=aggregateRoutes([connection("a","日本 01"),connection("b","日本 01"),connection("c","DIRECT"),connection("d","")],emptyRouteState(),1000);
    expect(result.nodes).toEqual([{name:"日本 01",count:2,active:false,weight:2},{name:"DIRECT",count:1,active:false,weight:1}]);
    expect(result.unknown).toBe(1);
  });
  test("marks only observed traffic as active and expires it",()=>{
    const a=aggregateRoutes([connection("a","Tokyo",10,0)],emptyRouteState(),1000);
    const b=aggregateRoutes([connection("a","Tokyo",20,0)],a.state,2000);
    expect(b.nodes[0].active).toBe(true);
    const c=aggregateRoutes([connection("a","Tokyo",20,0)],b.state,4999);
    expect(c.nodes[0].active).toBe(true);
    const d=aggregateRoutes([connection("a","Tokyo",20,0)],c.state,5000);
    expect(d.nodes[0].active).toBe(false);
    const moved=aggregateRoutes([connection("a","London",25,0)],b.state,2500);
    expect(moved.nodes[0].active).toBe(false);
  });
});
describe("prototype trajectory safety",()=>{
  test("unknown origin is hidden and unlocated nodes do not get invented coordinates",async()=>{
    const target=globalThis as unknown as {window:Record<string,unknown>};
    target.window={React,matchMedia:()=>({matches:true})};
    await import("../src/ui/bundle.js");
    const DS=target.window.RocketScience as {Trajectory:React.ComponentType<Record<string,unknown>>};
    const markup=renderToStaticMarkup(<DS.Trajectory originGeo={null} zoomable={false} nodes={[{name:"东京 01",estimated:true},{name:"no-location-xyz"}]}/>);
    // Labels show only the node; the caption says positions are guessed.
    expect(markup).toContain("东京 01</text>");
    expect(markup).not.toContain("推测");
    expect(markup).not.toContain("lf-traj__origin");
    expect(markup).not.toContain("lf-traj__path");
    expect(markup).not.toContain("NaN");
    expect(markup).not.toContain("no-location-xyz</text>");
  });
  test("draws each city hop as its own labelled marker",async()=>{
    const host=globalThis as unknown as {window:Record<string,unknown>};
    host.window={React,matchMedia:()=>({matches:true})};
    // bundle.js is a browser IIFE, not an ES module, so a static import cannot load it.
    await import("../src/ui/bundle.js?hop");
    const DS=host.window.RocketScience as {Trajectory:React.ComponentType<Record<string,unknown>>};
    const markup=renderToStaticMarkup(<DS.Trajectory origin="" originGeo={[113.3,23.1]} zoomable={false} nodes={[{
      name:"CN2-1-备",active:true,lon:-118.24,lat:34.05,place:"Los Angeles, US",
      hops:[{at:[-122.42,37.77],place:"San Francisco, US"}],
    }]}/>);
    expect(markup).toContain("San Francisco, US");
    expect(markup).toContain("Los Angeles, US");
    expect(markup.match(/class="lf-traj__hop"/g)).toHaveLength(1);
    expect(markup.match(/ Q/g)).toHaveLength(2);
  });
  test("the CN2 line name is not read as China",async()=>{
    const host=globalThis as unknown as {window:Record<string,unknown>};
    host.window={React,matchMedia:()=>({matches:true})};
    await import("../src/ui/bundle.js?cn2");
    const DS=host.window.RocketScience as {flagOf:(name:string)=>{code:string}|null;geoOf:(name:string)=>unknown};
    for(const name of ["CN2-1-备-2","CN2 GIA 01","CN2GIA"]){
      expect(DS.flagOf(name)).toBeNull();
      expect(DS.geoOf(name)).toBeNull();
    }
    // Another place in the name still counts, and so does the country code itself.
    expect(DS.flagOf("JP-CN2-01")?.code).toBe("JP");
    expect(DS.flagOf("中国 CN2")?.code).toBe("CN");
    expect(DS.flagOf("CN-01")?.code).toBe("CN");
    expect(DS.flagOf("CN01")?.code).toBe("CN");
    expect(DS.flagOf("HK01")?.code).toBe("HK");
  });
  test("draws the flag of the country each IP was located in",async()=>{
    const host=globalThis as unknown as {window:Record<string,unknown>};
    host.window={React,matchMedia:()=>({matches:true})};
    await import("../src/ui/bundle.js?flags");
    const DS=host.window.RocketScience as {Trajectory:React.ComponentType<Record<string,unknown>>};
    const markup=renderToStaticMarkup(<DS.Trajectory origin="Guangzhou, CN" originCountry="CN" originGeo={[113.3,23.1]} zoomable={false} nodes={[{
      name:"日本 01",lon:-118.24,lat:34.05,place:"Los Angeles, US",country:"US",
      hops:[{at:[103.82,1.35],place:"Singapore, SG",country:"SG"},{at:[-122.42,37.77],place:"San Francisco"}],
    }]}/>);
    expect(markup).toContain("🇨🇳 Guangzhou, CN");
    expect(markup).toContain("🇸🇬 Singapore, SG");
    expect(markup).toContain("🇺🇸 Los Angeles, US");
    // The measured country replaces the flag guessed from the name; no code draws no flag.
    expect(markup).not.toContain("🇯🇵");
    expect(markup).toContain(">San Francisco<");
    const originGroup = markup.match(/<g class="lf-traj__origin">.*?<\/g>/)?.[0] ?? "";
    expect(originGroup).toContain("lf-traj__origin-dot");
    expect(originGroup).toContain("lf-traj__origin-glow");
    expect(originGroup).not.toContain("<path");
  });
  test("an unfinished trace ends in a segment saying where it stopped",async()=>{
    const host=globalThis as unknown as {window:Record<string,unknown>};
    host.window={React,matchMedia:()=>({matches:true})};
    await import("../src/ui/bundle.js?unreached");
    const DS=host.window.RocketScience as {Trajectory:React.ComponentType<Record<string,unknown>>};
    const render=(tail:Record<string,unknown>)=>renderToStaticMarkup(<DS.Trajectory zoomable={false} nodes={[{name:"东京 01",lon:139.69,lat:35.69,route:{segments:[{label:"公网",kind:"carrier",hops:1,ms:10,hopList:[{ip:"203.0.113.1",ms:10}]},tail]}}]}/>);
    const silent=render({label:"第 1 跳后无回应",kind:"unreached",hops:2,hopList:[{ip:"",ms:-1},{ip:"",ms:-1}]});
    expect(silent).toContain("is-unreached");
    expect(silent).toContain("第 1 跳后无回应");
    expect(silent).toContain("2 跳沉默");
    expect(silent.match(/lf-traj__hop is-silent/g)).toHaveLength(2);
    // The hop limit ran out on an answering hop: no invented dot for the closing segment.
    const capped=render({label:"第 1 跳仍未到入口",kind:"unreached",hops:0,hopList:[]});
    expect(capped).toContain("第 1 跳仍未到入口");
    expect(capped).not.toContain("is-silent");
  });
  test("route rows carry no summary sentence, only a trace error or progress",async()=>{
    const host=globalThis as unknown as {window:Record<string,unknown>};
    host.window={React,matchMedia:()=>({matches:true})};
    await import("../src/ui/bundle.js?note");
    const DS=host.window.RocketScience as {Trajectory:React.ComponentType<Record<string,unknown>>};
    const render=(node:Record<string,unknown>)=>renderToStaticMarkup(<DS.Trajectory zoomable={false} nodes={[{name:"东京 01",lon:139.69,lat:35.69,...node}]}/>);
    const segments=[{label:"电信 163",kind:"carrier",hops:1,ms:9}];
    expect(render({route:{segments}})).not.toContain("lf-traj__note");
    expect(render({route:{error:"探测失败"}})).toContain("探测失败");
    expect(render({tracing:true})).toContain("正在测量去程…");
  });
});
