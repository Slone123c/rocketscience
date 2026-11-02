import {expect,test} from "bun:test";
import {readFileSync} from "node:fs";
import {runInNewContext} from "node:vm";
import {readNameSort} from "../src/nameSort";

const noop=()=>{};
const window:any={React:{createElement:noop,useState:noop,useRef:noop,useEffect:noop,useLayoutEffect:noop,useMemo:noop,memo:(c:unknown)=>c},matchMedia:()=>({matches:false}),addEventListener:noop,removeEventListener:noop};
runInNewContext(readFileSync(new URL("../design-system/components/bundle.js",import.meta.url),"utf8"),{window,Intl});
const {compareNames,sortByName,sortNodes}=window.RocketScience;

test("Chinese names file under their pinyin initial among Latin names",()=>{
  const names=["旧金山 3","🇯🇵 东京 02","Dallas 1","节点 10","节点 2","Amsterdam","新加坡","Ébène","乌鲁木齐","济南","Zurich"];
  expect([...names].sort(compareNames)).toEqual(["Amsterdam","🇯🇵 东京 02","Dallas 1","Ébène","济南","节点 2","节点 10","旧金山 3","乌鲁木齐","新加坡","Zurich"]);
});

test("sortByName keeps the config order unless asked, and reverses for Z→A",()=>{
  const nodes=[{name:"b"},{name:"阿"},{name:"C"}];
  expect(sortByName(nodes,"none")).toBe(nodes);
  expect(sortByName(nodes,"asc").map((n:any)=>n.name)).toEqual(["阿","b","C"]);
  expect(sortByName(nodes,"desc").map((n:any)=>n.name)).toEqual(["C","b","阿"]);
  expect(nodes.map(n=>n.name)).toEqual(["b","阿","C"]);
});

test("an unknown stored order falls back to the config order",()=>{
  expect(readNameSort("asc")).toBe("asc");
  expect(readNameSort("desc")).toBe("desc");
  expect(readNameSort(null)).toBe("none");
  expect(readNameSort("latency")).toBe("latency");
  expect(readNameSort("speed")).toBe("none");
});

test("latency order puts the fastest first, then timeouts, then untested, each by name",()=>{
  const nodes=[{name:"未测 b"},{name:"慢",delay:300},{name:"超时 b",delay:"timeout"},{name:"快",delay:40},{name:"A 同速",delay:120},{name:"超时 a",delay:0},{name:"B 同速",delay:120},{name:"a 未测"}];
  expect(sortNodes(nodes,"latency").map((n:any)=>n.name)).toEqual(["快","A 同速","B 同速","慢","超时 a","超时 b","a 未测","未测 b"]);
  expect(nodes[0].name).toBe("未测 b");
});

test("sortNodes passes the other orders through to the name sort",()=>{
  const nodes=[{name:"b"},{name:"a"}];
  expect(sortNodes(nodes,"none")).toBe(nodes);
  expect(sortNodes(nodes,"asc").map((n:any)=>n.name)).toEqual(["a","b"]);
});
