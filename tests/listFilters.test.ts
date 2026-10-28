import {describe,test,expect} from "bun:test";
import {filterConnections,filterProxyGroups,filterRules} from "../src/listFilters";
import type {Connection,Proxy,Rule} from "../src/mygo";
const connections:Connection[]=[
  {id:"one",metadata:{host:"example.com",destinationIP:"1.2.3.4",destinationPort:"443",network:"tcp",process:"Browser"},chains:["Tokyo","Proxy"],rule:"DOMAIN",rulePayload:"example.com",upload:0,download:0,start:""},
  {id:"two",metadata:{host:"dns.test",destinationIP:"1.1.1.1",destinationPort:"53",network:"udp",process:"Resolver"},chains:["DIRECT"],rule:"MATCH",rulePayload:"",upload:0,download:0,start:""}
];
describe("connection and proxy filters",()=>{
  test("matches metadata and routes case-insensitively with all terms",()=>{
    expect(filterConnections(connections,"BROWSER tokyo").map(c=>c.id)).toEqual(["one"]);
    expect(filterConnections(connections,"1.1.1.1 53","UDP").map(c=>c.id)).toEqual(["two"]);
    expect(filterConnections(connections,"example","UDP")).toHaveLength(0);
    expect(filterConnections(connections,"  ")).toHaveLength(2);
  });
  test("group names show all members and node search preserves selection and originals",()=>{
    const groups:Proxy[]=[{name:"Auto",type:"Selector",now:"Tokyo",all:["Tokyo","London"]},{name:"Direct",type:"Selector",now:"DIRECT",all:["DIRECT"]}];
    expect(filterProxyGroups(groups,"auto")[0].all).toEqual(["Tokyo","London"]);
    expect(filterProxyGroups(groups,"london")[0]).toEqual({...groups[0],all:["London"]});
    expect(groups[0].all).toEqual(["Tokyo","London"]);
    expect(filterProxyGroups(groups,"missing")).toEqual([]);
    expect(filterProxyGroups(groups,"")).toBe(groups);
  });
  test("windows and filters tens of thousands of rules without allocating full results",()=>{
    const rules:Rule[]=Array.from({length:12000},(_,i)=>({
      type:i%2===0?"DOMAIN-SUFFIX":"IP-CIDR",
      payload:i===9999?"special.internal.corp":`host-${i}.example.com`,
      proxy:i%3===0?"DIRECT":"Proxy"
    }));
    const windowed=filterRules(rules,"",500);
    expect(windowed).toHaveLength(500);
    expect(windowed[0].payload).toBe("host-0.example.com");
    const matched=filterRules(rules,"special.internal",500);
    expect(matched).toEqual([{type:"IP-CIDR",payload:"special.internal.corp",proxy:"DIRECT"}]);
  });
});
