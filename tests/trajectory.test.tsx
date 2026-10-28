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
    await import("../design-system/components/bundle.js");
    const DS=target.window.RocketScience as {Trajectory:React.ComponentType<Record<string,unknown>>};
    const markup=renderToStaticMarkup(<DS.Trajectory originGeo={null} zoomable={false} nodes={[{name:"东京 01",estimated:true},{name:"no-location-xyz"}]}/>);
    expect(markup).toContain("东京 01 · 推测");
    expect(markup).not.toContain("lf-traj__origin");
    expect(markup).not.toContain("lf-traj__path");
    expect(markup).not.toContain("NaN");
    expect(markup).not.toContain("no-location-xyz</text>");
  });
});
