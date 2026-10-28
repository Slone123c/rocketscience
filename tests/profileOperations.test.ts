import {expect,test} from "bun:test";
import {commitProfileChange,subscriptionFailureMessage} from "../src/profileOperations";
import type {ProfileState} from "../src/mygo";

const next={profiles:[],activeId:"selected",mixedPort:7890,systemProxy:false} as unknown as ProfileState;

test("a committed profile is accepted before refresh and reports a failed read without rejecting the save",async()=>{
  const events:string[]=[],warnings:string[]=[];let closed=false,accepted:ProfileState|undefined;
  const result=await commitProfileChange(async()=>{events.push("write");return next},state=>{events.push("accept");accepted=state;closed=true},async()=>{events.push("refresh");throw new Error("read unavailable")},message=>warnings.push(message),"配置已保存");
  expect(result).toBe(next);expect(accepted).toBe(next);expect(closed).toBe(true);
  expect(events).toEqual(["write","accept","refresh"]);
  expect(warnings).toEqual(["配置已保存，但界面刷新失败：Error: read unavailable"]);
});

test("a failed write neither closes the editor nor reads back or accepts an uncommitted change",async()=>{
  const failure=new Error("editor version is stale"),events:string[]=[];
  await expect(commitProfileChange(async()=>{throw failure},()=>{events.push("accept")},async()=>{events.push("refresh")},()=>events.push("warn"),"配置已保存")).rejects.toBe(failure);
  expect(events).toEqual([]);
});

test("successful readback does not produce a warning or repeat the write",async()=>{
  let writes=0,reads=0;const warnings:string[]=[];
  await commitProfileChange(async()=>{writes++;return next},()=>{},async()=>{reads++},message=>warnings.push(message),"订阅已更新");
  expect(writes).toBe(1);expect(reads).toBe(1);expect(warnings).toEqual([]);
});

test("completed switch, update and delete operations remain completed if refresh fails",async()=>{
  for(const label of ["配置已切换","订阅已更新","配置已删除"]){
    const warnings:string[]=[];let accepted=0;
    await commitProfileChange(async()=>next,()=>{accepted++},async()=>{throw new Error("refresh failed")},message=>warnings.push(message),label);
    expect(accepted).toBe(1);expect(warnings[0]).toStartWith(label+"，但界面刷新失败");
  }
});

test("an update failure reads persisted error metadata without losing the original failure",async()=>{
  let read=false;
  const message=await subscriptionFailureMessage(new Error("HTTP 503"),async()=>{read=true});
  expect(read).toBe(true);expect(message).toBe("Error: HTTP 503");
});

test("a failed metadata read preserves both errors and does not escape as an unhandled rejection",async()=>{
  const message=await subscriptionFailureMessage(new Error("invalid YAML"),async()=>{throw new Error("controller unavailable")});
  expect(message).toContain("invalid YAML");expect(message).toContain("无法读取更新状态");expect(message).toContain("controller unavailable");
});
