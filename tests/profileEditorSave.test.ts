import {describe,expect,test} from "bun:test";
import {saveEditorProfile} from "../src/profileEditorSave";
import type {Profile,ProfileState} from "../src/mygo";
const original:Profile={id:"p",name:"saved",source:"url",url:"https://old.example/sub",interval:6,content:"old yaml",path:"",updatedAt:"2026-01-01T00:00:00Z",lastAttempt:"2026-01-01T00:00:00Z",usage:{upload:1,download:2,total:100,expire:0},lastError:""};
const state:ProfileState={profiles:[],activeId:"",mixedPort:7890,systemProxy:false};

describe("prototype profile editor save flow",()=>{
  test("new subscriptions download before saving with traffic metadata",async()=>{
    const calls:string[]=[];
    await saveEditorProfile({name:" new ",source:"url",url:" https://new.example/sub ",interval:"12",content:""},null,{
      fetch:async url=>{calls.push(url);return {content:"downloaded",usage:original.usage,updatedAt:original.updatedAt}},
      save:async(profile,expected)=>{calls.push("save");expect(profile.name).toBe("new");expect(profile.content).toBe("downloaded");expect(profile.usage).toEqual(original.usage);expect(profile.interval).toBe(12);expect(expected).toBe("");return state}
    });expect(calls).toEqual(["https://new.example/sub","save"]);
  });
  test("manual edits to an existing subscription are preserved instead of downloaded over",async()=>{
    const before=JSON.stringify(original);
    await saveEditorProfile({...original,source:"url",interval:"6",content:"edited"},original,{
      fetch:async()=>{throw new Error("unexpected download")},
      save:async(profile,expected)=>{expect(profile.content).toBe("edited");expect(expected).toBe(original.updatedAt);expect(profile.usage).toEqual(original.usage);return state}
    });expect(JSON.stringify(original)).toBe(before);
  });
  test("changing a subscription URL downloads the new source and keeps optimistic version checking",async()=>{
    await saveEditorProfile({...original,source:"url",interval:"6",url:"https://new.example/sub"},original,{
      fetch:async()=>({content:"new source",usage:null,updatedAt:"2026-01-02T00:00:00Z"}),
      save:async(profile,expected)=>{expect(profile.content).toBe("new source");expect(profile.lastAttempt).toBe("2026-01-02T00:00:00Z");expect(expected).toBe(original.updatedAt);throw new Error("stale editor")}
    }).then(()=>{throw new Error("save should fail")},error=>expect(String(error)).toContain("stale editor"));
  });
  test("download failures never save a partial subscription",async()=>{
    let saved=false;
    await expect(saveEditorProfile({name:"new",source:"url",url:"https://new.example/sub",content:""},null,{fetch:async()=>{throw new Error("offline")},save:async()=>{saved=true;return state}})).rejects.toThrow("offline");
    expect(saved).toBe(false);
  });
  test("local imports retain source path and never schedule subscription updates",async()=>{
    await saveEditorProfile({name:"file",source:"file",path:"/chosen/config.yaml",content:"local",interval:"24"},null,{
      fetch:async()=>{throw new Error("local file downloaded")},
      save:async(profile)=>{expect(profile.path).toBe("/chosen/config.yaml");expect(profile.content).toBe("local");expect(profile.interval).toBe(0);expect(profile.url).toBe("");expect(profile.usage).toBeNull();return state}
    });
  });
});
