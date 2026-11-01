import {expect,test} from "bun:test";
import {localOrigin} from "../src/localOrigin";

const none=()=>null;
const ip={known:true,city:"Guangzhou",region:"Guangdong",country:"CN",lon:113.25,lat:23.12,source:"ipinfo.io",error:""};

test("the public IP's city comes first",()=>{
  expect(localOrigin(ip,"Asia/Shanghai",none)).toEqual({geo:[113.25,23.12],source:"ip",note:"本机位置按公网 IP 推测：Guangzhou，Guangdong，CN（ipinfo.io）"});
});

test("without the IP the time zone places the origin and says why",()=>{
  const failed=localOrigin({...ip,known:false,error:"timeout"},"Asia/Shanghai",none);
  expect(failed.geo).toEqual([121.5,31.2]);expect(failed.source).toBe("timezone");expect(failed.note).toContain("公网 IP 查询失败");
  expect(localOrigin(null,"America/Los_Angeles",none).note).toContain("正在查询公网 IP");
  expect(localOrigin(null,"Europe/Frankfurt",name=>name==="Frankfurt"?[8.7,50.1]:null).geo).toEqual([8.7,50.1]);
});

test("an unknown zone leaves the origin out instead of inventing one",()=>{
  const result=localOrigin(null,"Pacific/Nowhere",none);
  expect(result.geo).toBeNull();expect(result.note).toContain("本机位置未知");
});
