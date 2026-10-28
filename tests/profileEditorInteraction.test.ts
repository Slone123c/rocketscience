import {expect,test} from "bun:test";
import {readFileSync} from "node:fs";
import {runInNewContext} from "node:vm";

const source=readFileSync(new URL("../design-system/components/bundle.js",import.meta.url),"utf8");
type Node={type:unknown;props:Record<string,any>;children:unknown[]};
function editor() {
  const slots:any[]=[],effects:Array<()=>void>=[];let cursor=0,tree:Node;
  const React={memo:(c:unknown)=>c,
    createElement:(type:unknown,props:any,...children:unknown[]):Node=>({type,props:props||{},children:children.flat(Infinity)}),
    useState:(initial:unknown)=>{const i=cursor++;if(!(i in slots))slots[i]=initial;return [slots[i],(v:any)=>{slots[i]=typeof v==="function"?v(slots[i]):v}]},
    useEffect:(run:()=>void,deps:unknown[])=>{const i=cursor++,old=slots[i];if(!old||deps.some((v,j)=>v!==old[j])){slots[i]=deps;effects.push(run)}},
    useRef:(initial:unknown)=>({current:initial})};
  const window:any={React,confirm:()=>true};runInNewContext(source,{window,Promise});
  function walk(value:unknown,predicate:(n:Node)=>boolean):Node|undefined {if(!value||typeof value!=="object")return;const n=value as Node;if(predicate(n))return n;for(const child of n.children||[]){const found=walk(child,predicate);if(found)return found}}
  return {
    render(props:Record<string,unknown>){cursor=0;tree=window.RocketScience.ProfileEditor({open:true,profile:{id:"saved",name:"Config",source:"file",content:"rules: []",interval:"0"},onClose:()=>{},...props});effects.splice(0).forEach(fn=>fn());return tree},
    save:()=>tree.props.footer.find((n:Node|undefined)=>n?.props?.key==="ok") as Node,
    find:(predicate:(n:Node)=>boolean)=>walk(tree,predicate),
    content:()=>walk(tree,n=>n.type===window.RocketScience.CodeEditor),
    tree:()=>tree
  };
}
const tick=async()=>{for(let i=0;i<8;i++)await Promise.resolve()};

test("pending saves block repeated saves and closing; rejection retains YAML and error",async()=>{
  const ui=editor();let reject!:(e:unknown)=>void,saves=0,closed=0;
  const props={defaultTab:"file",onSave:()=>{saves++;return new Promise<void>((_,no)=>{reject=no})},onClose:()=>{closed++}};
  ui.render(props);ui.render(props);ui.save().props.onClick();await tick();ui.render(props);
  expect(ui.save().props.disabled).toBe(true);
  ui.save().props.onClick();ui.tree().props.onClose();expect(saves).toBe(1);expect(closed).toBe(0);
  reject(new Error("mihomo rejected this YAML"));await tick();ui.render(props);
  expect(ui.save().props.disabled).toBe(false);
  expect(ui.content()?.props.value).toBe("rules: []");
  expect(ui.find(n=>n.children.includes("Error: mihomo rejected this YAML"))).toBeDefined();expect(closed).toBe(0);
});

test("native file picker rejection stays in the editor and preserves existing content",async()=>{
  const ui=editor();let closed=0;
  const props={onSave:()=>{},onClose:()=>{closed++},onPickFile:()=>Promise.reject(new Error("file unreadable"))};
  ui.render(props);ui.render(props);
  ui.find(n=>n.children.includes("选择 .yaml"))!.props.onClick();await tick();ui.render(props);
  expect(ui.content()?.props.value).toBe("rules: []");expect(ui.find(n=>n.children.includes("Error: file unreadable"))).toBeDefined();expect(closed).toBe(0);
});

test("active profile edits describe a reload only while the core is running",()=>{
  const ui=editor();const props={onSave:()=>{},active:true,running:false,defaultTab:"file"};
  ui.render(props);ui.render(props);ui.content()!.props.onChange("rules: [MATCH,DIRECT]");ui.render(props);
  expect(ui.save().children).toContain("保存");
  ui.render({...props,running:true});expect(ui.save().children).toContain("保存并重新加载");
});
