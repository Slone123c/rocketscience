import {expect,test} from "bun:test";
import {readDroppedProfile} from "../src/profileFileImport";

test("dropped YAML retains name and text and rejects invalid encodings",async()=>{
  const content="\uFEFFrules:\n  - MATCH,DIRECT\n";
  const file=new File([content],"公司配置.YAML");
  expect(await readDroppedProfile(file)).toEqual({name:"公司配置",path:"公司配置.YAML",content});
  await expect(readDroppedProfile(new File([new Uint8Array([255])],"bad.yaml"))).rejects.toThrow("UTF-8");
  await expect(readDroppedProfile(new File([content],"bad.txt"))).rejects.toThrow(".yaml");
});
test("oversized drops are rejected before their data is read",async()=>{
  let read=false;
  await expect(readDroppedProfile({name:"large.yaml",size:10*1024*1024+1,arrayBuffer:async()=>{read=true;return new ArrayBuffer(0)}})).rejects.toThrow("10 MB");
  expect(read).toBe(false);
});
