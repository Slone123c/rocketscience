import {expect,test} from "bun:test";
import {readFileSync} from "node:fs";
import {runInNewContext} from "node:vm";

const source=readFileSync(new URL("../src/ui/bundle.js",import.meta.url),"utf8");
type Tree={props:Record<string,any>;children:unknown[]};
function tourHarness() {
  const slots:any[]=[],effects:Array<()=>void>=[],events=new Map<string,Set<(e:any)=>void>>();
  const frames=new Map<number,()=>void>(),observers:Array<{callback:()=>void;targets:unknown[];disconnected:boolean}>=[];
  let cursor=0,frame=0,rect={left:20,top:30,width:250,height:100};
  const document:any={activeElement:null};
  function element(name:string,tabIndex=0) {return {name,tabIndex,style:{} as Record<string,string>,hidden:false,
    getClientRects(){return this.hidden?[]:[{}]},closest(){return null},
    focus(){document.activeElement=this},contains(other:unknown){return other===this},
    matches(){return false},querySelectorAll(){return [] as any[]}}}
  const input=element("input"),control=element("switch"),hidden=element("hidden"),negative=element("negative",-1);
  hidden.hidden=true;
  const back=element("back"),next=element("next"),outside=element("outside"),option=element("portal-option");
  const portal={...element("portal"),contains:(el:unknown)=>el===option,querySelectorAll:()=>[option]};
  const trigger={getAttribute:()=>"controlled-list"};
  const target={...element("target"),scrollIntoView(){},getBoundingClientRect:()=>rect,
    contains:(el:unknown)=>[input,control,hidden,negative].includes(el as any),
    querySelectorAll:(selector:string)=>selector==="[aria-controls]"?[trigger]:[input,control,hidden,negative]};
  const card={...element("card"),offsetWidth:300,offsetHeight:180,
    contains:(el:unknown)=>el===back||el===next,
    querySelectorAll:(selector:string)=>selector==="[aria-controls]"?[]:[back,next],querySelector:()=>next};
  const hole=element("hole");
  const add=(name:string,fn:(e:any)=>void)=>{if(!events.has(name))events.set(name,new Set());events.get(name)!.add(fn)};
  const remove=(name:string,fn:(e:any)=>void)=>events.get(name)?.delete(fn);
  Object.assign(document,{querySelector:()=>target,getElementById:(id:string)=>id==="controlled-list"?portal:null,addEventListener:add,removeEventListener:remove});
  function effect(run:()=>unknown,deps:unknown[]) {const i=cursor++,old=slots[i];if(!old||deps.length!==old.deps.length||deps.some((x,j)=>x!==old.deps[j]))effects.push(()=>{old?.cleanup?.();slots[i]={deps,cleanup:run()}})}
  const React={memo:(c:unknown)=>c,
    createElement:(type:unknown,props:any,...children:unknown[])=>({type,props:props||{},children:children.flat(Infinity)}),
    useState:(initial:any)=>{const i=cursor++;if(!(i in slots))slots[i]=initial;return [slots[i],(v:any)=>{slots[i]=typeof v==="function"?v(slots[i]):v}]},
    useRef:(initial:unknown)=>{const i=cursor++;if(!(i in slots))slots[i]={current:initial};return slots[i]},
    useEffect:effect,useLayoutEffect:effect,useMemo:(run:()=>unknown)=>run()};
  const window:any={React,innerWidth:1200,innerHeight:780,matchMedia:()=>({matches:true}),addEventListener:add,removeEventListener:remove,
    ResizeObserver:class {record:any;constructor(callback:()=>void){this.record={callback,targets:[],disconnected:false};observers.push(this.record)}observe(t:unknown){this.record.targets.push(t)}disconnect(){this.record.disconnected=true}}};
  runInNewContext(source,{window,document,Date,Promise,console,setTimeout,clearTimeout,setInterval,clearInterval,
    requestAnimationFrame:(fn:()=>void)=>{frames.set(++frame,fn);return frame},cancelAnimationFrame:(id:number)=>frames.delete(id)});
  function bind(tree:unknown) {if(!tree||typeof tree!=="object")return;const node=tree as Tree;
    const ref=node.props?.ref,name=node.props?.className||"";
    if(ref){const el=name.includes("lf-tour__card")?card:name.includes("lf-tour__hole")?hole:element("block");if(typeof ref==="function")ref(el);else ref.current=el}
    node.children?.forEach(bind);
  }
  return {
    input,control,back,next,outside,option,hole,observers,document,
    render(props:Record<string,unknown>={}) {cursor=0;const tree=window.RocketScience.Tour({steps:[{title:"one",target:"#target"},{title:"two"}],step:0,...props});bind(tree);effects.splice(0).forEach(fn=>fn());while(frames.size){const batch=[...frames.values()];frames.clear();batch.forEach(fn=>fn())}return tree},
    emit(name:string,data:Record<string,unknown>={}) {const event={key:"",target:next,defaultPrevented:false,preventDefault(){this.defaultPrevented=true},...data};for(const fn of [...events.get(name)||[]])fn(event);return event},
    resize(nextRect:typeof rect){rect=nextRect;observers.filter(x=>!x.disconnected).forEach(x=>x.callback())},
    listenerCount:()=>[...events.values()].reduce((n,set)=>n+set.size,0),
    unmount(){slots.forEach(slot=>slot?.cleanup?.())},
    firstRunTopics(){const tree=window.RocketScience.FirstRunCard({});return JSON.stringify(tree)}
  };
}

test("guide arrows preserve highlighted controls, handled keys and modifier shortcuts",()=>{
  const ui=tourHarness(),steps:number[]=[];
  ui.render({onStepChange:(i:number)=>steps.push(i)});
  expect(ui.emit("keydown",{key:"ArrowRight",target:ui.input}).defaultPrevented).toBe(false);
  expect(ui.emit("keydown",{key:"ArrowRight",target:ui.control}).defaultPrevented).toBe(false);
  ui.emit("keydown",{key:"ArrowRight",defaultPrevented:true});
  ui.emit("keydown",{key:"ArrowRight",metaKey:true});
  expect(steps).toEqual([]);
  expect(ui.emit("keydown",{key:"ArrowRight"}).defaultPrevented).toBe(true);
  expect(steps).toEqual([1]);ui.unmount();
});

test("Tab wraps inside the highlighted controls, guide and associated popup",()=>{
  const ui=tourHarness();ui.render();
  ui.document.activeElement=ui.next;
  ui.emit("keydown",{key:"Tab"});expect(ui.document.activeElement).toBe(ui.option);
  ui.emit("keydown",{key:"Tab"});expect(ui.document.activeElement).toBe(ui.input);
  ui.emit("keydown",{key:"Tab",shiftKey:true});expect(ui.document.activeElement).toBe(ui.option);
  ui.emit("focusin",{target:ui.outside});expect(ui.document.activeElement).toBe(ui.input);
  ui.document.activeElement=ui.option;
  ui.emit("focusin",{target:ui.option});expect(ui.document.activeElement).toBe(ui.option);
  ui.unmount();
});

test("guide tracks changed target size and removes observers/listeners on close",()=>{
  const ui=tourHarness();ui.render();
  expect(ui.hole.style.height).toBe("112px");
  ui.resize({left:20,top:30,width:250,height:190});
  expect(ui.hole.style.height).toBe("202px");
  expect(ui.observers[0].targets).toHaveLength(2);
  ui.render({open:false});
  expect(ui.listenerCount()).toBe(0);
  expect(ui.observers.every(x=>x.disconnected)).toBe(true);ui.unmount();
});

test("guide refreshes callbacks without changing the step, and Escape never completes it",()=>{
  const ui=tourHarness(),closed:string[]=[];
  ui.render({onClose:(done:boolean)=>closed.push(`old:${done}`)});
  ui.render({onClose:(done:boolean)=>closed.push(`new:${done}`)});
  ui.emit("keydown",{key:"Escape"});expect(closed).toEqual(["new:false"]);ui.unmount();
});

test("first-run topics reflect the approved manual launch order",()=>{
  const ui=tourHarness(),text=ui.firstRunTopics();
  expect(text.indexOf("导入配置")).toBeLessThan(text.indexOf("选择接管并启动"));
  expect(text.indexOf("选择接管并启动")).toBeLessThan(text.indexOf("测速选节点"));
  expect(text.indexOf("测速选节点")).toBeLessThan(text.indexOf("检查网络与模式"));
});
