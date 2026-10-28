import {describe,expect,test} from "bun:test";
import {addLANPreset,lanDirectRules} from "../src/ruleTemplates";
import type {OverrideGroup} from "../src/mygo";

describe("LAN direct template",()=>{
  test("appends prototype rules without changing existing priorities or input",()=>{
    const original:OverrideGroup[]=[{id:"custom",name:"我的规则",enabled:true,rules:[{type:"DOMAIN",payload:"private.example",policy:"REJECT"}]}];
    const snapshot=JSON.stringify(original);
    const result=addLANPreset(original,()=>"lan");
    expect(JSON.stringify(original)).toBe(snapshot);
    expect(result.groups[0]).toBe(original[0]);
    expect(result.groups[1]).toEqual({id:"lan",name:"局域网直连",enabled:true,rules:lanDirectRules});
    result.groups[1].rules[0].payload="changed";
    expect(lanDirectRules[0].payload).toBe("192.168.0.0/16");
  });
  test("repeated requests reuse and enable a matching group without dropping extra rules",()=>{
    const first=addLANPreset([],()=>"lan");
    first.groups[0].enabled=false;
    first.groups[0].name="家里";
    first.groups[0].rules.push({type:"DOMAIN",payload:"nas.example",policy:"DIRECT"});
    const result=addLANPreset(first.groups,()=>{throw new Error("duplicate group created")});
    expect(result.existing).toBe(true);
    expect(result.id).toBe("lan");
    expect(result.groups).toHaveLength(1);
    expect(result.groups[0].enabled).toBe(true);
    expect(result.groups[0].rules).toHaveLength(5);
    expect(first.groups[0].enabled).toBe(false);
  });
  test("a different policy is not mistaken for an existing direct template",()=>{
    const first=addLANPreset([],()=>"other");
    first.groups[0].rules[0].policy="REJECT";
    const result=addLANPreset(first.groups,()=>"lan");
    expect(result.existing).toBe(false);
    expect(result.groups).toHaveLength(2);
    expect(result.groups[0].rules[0].policy).toBe("REJECT");
  });
});
