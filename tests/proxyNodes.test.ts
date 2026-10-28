import {describe,expect,test} from "bun:test";
import {proxyNodes} from "../src/proxyNodes";
import type {Proxy} from "../src/mygo";

const group={name:"choose",type:"Selector",now:"fast",all:["fast","failed","untested","missing"],nodes:[
  {name:"fast",type:"Shadowsocks",udp:true,delay:25,delayAt:"2026-01-01T00:01:00Z"},
  {name:"failed",type:"Trojan",udp:false,delay:0,delayAt:"2026-01-01T00:01:00Z"},
  {name:"untested",type:"Direct",udp:null,delay:null,delayAt:""}
]} as Proxy;

describe("proxy node display mapping",()=>{
  test("uses core history with explicit timeout and untested states",()=>{
    const nodes=proxyNodes(group);
    expect(nodes[0]).toEqual({name:"fast",type:"Shadowsocks",udp:true,delay:25});
    expect(nodes[1].delay).toBe("timeout");expect(nodes[2].delay).toBeUndefined();expect(nodes[3].delay).toBeUndefined();
  });
  test("fresh manual measurements win, including missing nodes as timeout",()=>{
    const nodes=proxyNodes(group,{values:{fast:10},completedAt:Date.parse("2026-01-01T00:02:00Z")});
    expect(nodes[0].delay).toBe(10);expect(nodes[1].delay).toBe("timeout");expect(nodes[3].delay).toBe("timeout");
  });
  test("later health checks supersede old manual results and filters retain metadata",()=>{
    expect(proxyNodes(group,{values:{fast:999},completedAt:Date.parse("2026-01-01T00:00:00Z")})[0].delay).toBe(25);
    expect(proxyNodes({...group,all:["failed"]})).toHaveLength(1);
    expect(proxyNodes({...group,all:["failed"]})[0].type).toBe("Trojan");
    expect(group.all).toHaveLength(4);
  });
});
