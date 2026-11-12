import {expect,test} from "bun:test";
import {readFileSync} from "node:fs";
import {runInNewContext} from "node:vm";

const source=readFileSync(new URL("../src/ui/bundle.js",import.meta.url),"utf8");
type Node={type:unknown;props:Record<string,any>;children:unknown[]};

// Exercise the shipped component's callbacks/effect lifetimes without a browser.
// Child components and animation frames are deliberately not rendered here.
function popover() {
  const slots:any[]=[],pending:Array<()=>void>=[],listeners=new Set<(e:any)=>void>();
  const timers=new Map<number,()=>void>();
  let cursor=0,timerID=0,tree:Node;
  function effect(run:()=>void|(()=>void),deps:unknown[]) {
    const i=cursor++,old=slots[i];
    if(!old||deps.some((value,j)=>value!==old.deps[j])||deps.length!==old.deps.length){
      pending.push(()=>{old?.cleanup?.();slots[i]={deps,cleanup:run()}});
    }
  }
  const React={
    memo:(component:unknown)=>component,
    createElement:(type:unknown,props:Record<string,any>|null,...children:unknown[]):Node=>({type,props:props||{},children:children.flat(Infinity)}),
    useState:(initial:unknown)=>{const i=cursor++;if(!(i in slots))slots[i]=initial;return [slots[i],(value:any)=>{slots[i]=typeof value==="function"?value(slots[i]):value}]},
    useRef:(initial:unknown)=>{const i=cursor++;if(!(i in slots))slots[i]={current:initial};return slots[i]},
    useEffect:effect,useLayoutEffect:effect,
    useMemo:(run:()=>unknown)=>run()
  };
  const window:any={React,matchMedia:()=>({matches:true}),
    addEventListener:(name:string,fn:(e:any)=>void)=>{if(name==="keydown")listeners.add(fn)},
    removeEventListener:(name:string,fn:(e:any)=>void)=>{if(name==="keydown")listeners.delete(fn)}
  };
  runInNewContext(source,{window,Date,Promise,console,
    setTimeout:(fn:()=>void)=>{timers.set(++timerID,fn);return timerID},clearTimeout:(id:number)=>timers.delete(id),
    setInterval:()=>0,clearInterval:()=>{},cancelAnimationFrame:()=>{}});
  return {
    render(props:Record<string,unknown>={}) {cursor=0;tree=window.RocketScience.MenuBarPopover(props);pending.splice(0).forEach(fn=>fn());return tree},
    key(key:string) {let prevented=false;for(const fn of [...listeners])fn({key,metaKey:true,preventDefault:()=>{prevented=true}});return prevented},
    find(predicate:(node:Node)=>boolean) {
      function walk(value:unknown):Node|undefined {if(!value||typeof value!=="object")return;const node=value as Node;if(predicate(node))return node;for(const child of node.children||[]){const hit=walk(child);if(hit)return hit}}
      return walk(tree);
    },
    unmount(){for(const slot of slots)slot?.cleanup?.()},
    listenerCount:()=>listeners.size,timerCount:()=>timers.size
  };
}
const flush=async()=>{for(let i=0;i<8;i++)await Promise.resolve()};

test("menu shortcuts use callbacks installed after startup and after busy state",()=>{
  const ui=popover();
  const calls:string[]=[];
  ui.render({systemProxy:false,tun:false});
  ui.render({systemProxy:false,tun:false,onSystemProxyChange:(value:boolean)=>calls.push(`proxy:${value}`),onTunChange:(value:boolean)=>calls.push(`tun:${value}`),onOpenDashboard:()=>calls.push("dashboard")});
  ui.key("s");ui.key("e");ui.key("d");
  expect(calls).toEqual(["proxy:true","tun:true","dashboard"]);
  ui.render({systemProxy:false,tun:false}); // busy: callbacks removed
  ui.key("s");ui.key("e");
  expect(calls).toHaveLength(3);
  ui.render({systemProxy:false,tun:false,onSystemProxyChange:()=>calls.push("latest")});
  ui.key("s");
  expect(calls.at(-1)).toBe("latest");
  expect(ui.listenerCount()).toBe(1);
  ui.render({open:false});
  expect(ui.listenerCount()).toBe(0);
  ui.unmount();
});

test("copy confirms only after success, rejects duplicate requests, and clears its timer",async()=>{
  const ui=popover();let resolve!:()=>void,calls=0;
  const props={onCopyCommand:()=>{calls++;return new Promise<void>(r=>{resolve=r})}};
  ui.render(props);ui.key("c");ui.key("c");await flush();
  expect(calls).toBe(1);
  ui.render(props);
  expect(ui.find(n=>n.children.includes("已复制到剪贴板"))).toBeUndefined();
  resolve();await flush();ui.render(props);
  expect(ui.find(n=>n.children.includes("已复制到剪贴板"))).toBeDefined();
  expect(ui.timerCount()).toBe(1);
  ui.unmount();expect(ui.timerCount()).toBe(0);
});

test("copy failure stays unconfirmed and a late success after unmount adds no timer",async()=>{
  const ui=popover();
  const failure={onCopyCommand:()=>Promise.reject(new Error("clipboard unavailable"))};
  ui.render(failure);ui.key("c");await flush();ui.render(failure);
  expect(ui.find(n=>n.children.includes("已复制到剪贴板"))).toBeUndefined();
  let resolve!:()=>void;
  ui.render({onCopyCommand:()=>new Promise<void>(r=>{resolve=r})});
  ui.key("c");await flush();ui.unmount();resolve();await flush();
  expect(ui.timerCount()).toBe(0);
  expect(ui.listenerCount()).toBe(0);
});

test("testing another group disables another test; mutations disable node selection",()=>{
  const ui=popover();
  const groups=[{name:"A",type:"Selector",nodes:[{name:"node",type:"Direct"}]}];
  ui.render({state:"orbit",groups,testing:"B",onTest:()=>{}});
  expect(ui.find(n=>n.props["aria-label"]==="测速：A")?.props.disabled).toBe(true);
  expect(ui.find(n=>n.props.role==="radio")?.props.disabled).toBe(false);
  ui.render({state:"igniting",groups,onTest:()=>{}});
  expect(ui.find(n=>n.props.role==="radio")?.props.disabled).toBe(true);
  expect(ui.find(n=>n.props["aria-label"]==="测速：A")?.props.disabled).toBe(true);
  ui.unmount();
});

test("idle state shows node info, sort toggle, and allows preview selection and testing unless readOnly",()=>{
  const ui=popover();
  const selected:string[]=[],tested:string[]=[];
  const groups=[
    {name:"主出口",type:"Selector",now:"节点 B",nodes:[{name:"节点 B",type:"ss"},{name:"节点 A",type:"vmess"}]},
    {name:"配置中的独立节点",type:"Selector",now:"",readOnly:true,nodes:[{name:"独立 1",type:"trojan"},{name:"独立 2",type:"ss"}]}
  ];
  ui.render({state:"idle",groups,sort:"asc",onSelect:(g:string,n:string)=>selected.push(`${g}:${n}`),onTest:(g:string)=>tested.push(g)});
  expect(ui.find(n=>n.props.className?.includes("lf-pop__sort"))).toBeDefined();
  const testBtn=ui.find(n=>n.props["aria-label"]==="测速：主出口");
  expect(testBtn?.props.disabled).toBe(false);
  testBtn?.props.onClick();
  const firstRadio=ui.find(n=>n.props.role==="radio");
  expect(firstRadio?.props.disabled).toBe(false);
  expect(firstRadio?.props["aria-disabled"]).toBeUndefined();
  firstRadio?.props.onClick();
  expect(tested).toEqual(["主出口"]);
  expect(selected).toEqual(["主出口:节点 A"]);

  ui.render({state:"idle",groups:[groups[1]],onSelect:(g:string,n:string)=>selected.push(`${g}:${n}`)});
  const readOnlyRadio=ui.find(n=>n.props.role==="radio");
  expect(readOnlyRadio?.props["aria-disabled"]).toBe(true);
  readOnlyRadio?.props.onClick();
  expect(selected).toHaveLength(1);
  ui.unmount();
});
