import type {Connectivity,NodeRecommendation} from "./mygo";

export type CheckPhase="idle"|"checking"|"launching"|"selecting"|"recovering";
export interface CheckState {result:Connectivity|null;phase:CheckPhase;message:string;recovered:boolean;recoveryFailed:boolean}
export const initialCheckState=():CheckState=>({result:null,phase:"idle",message:"",recovered:false,recoveryFailed:false});
interface Source {
  check:()=>Promise<Connectivity>;
  launch:()=>Promise<void>;
  recover:()=>Promise<void>;
  refresh:()=>Promise<void>;
  confirm:(recommendation:NodeRecommendation)=>boolean|Promise<boolean>;
  select:(recommendation:NodeRecommendation)=>Promise<void>;
}
interface Task {kind:CheckPhase;promise:Promise<void>;mutationDone:Promise<void>;endMutation:()=>void}

// A recovery supersedes reads immediately, but waits for an already submitted
// mutation to settle so a delayed launch cannot restart the core after recovery.
export function createNetworkCheckFlow(source:Source,changed:(state:CheckState)=>void) {
  let state=initialCheckState(),current:Task|null=null,active=true;
  const live=(task:Task)=>active&&current===task;
  function publish(patch:Partial<CheckState>){state={...state,...patch};if(active)changed(state)}
  async function check(task:Task) {
    if(!live(task))return;
    publish({phase:"checking",recovered:false,message:""});
    const result=await source.check();
    if(live(task))publish({result});
  }
  function start(kind:CheckPhase,work:(task:Task)=>Promise<void>,clear=false) {
    const task:Task={kind,promise:Promise.resolve(),mutationDone:Promise.resolve(),endMutation:()=>{}};
    if(kind!=="checking")task.mutationDone=new Promise(resolve=>{task.endMutation=resolve});
    current=task;
    publish({phase:kind,message:"",recovered:false,recoveryFailed:false,...(clear?{result:null}:{})});
    task.promise=Promise.resolve().then(()=>work(task)).catch(error=>{
      if(live(task))publish({message:(kind==="recovering"?"恢复网络失败：":"")+String(error),recoveryFailed:kind==="recovering"});
    }).finally(()=>{
      task.endMutation();
      if(live(task)){current=null;publish({phase:"idle"})}
    });
    return task.promise;
  }
  return {
    snapshot:()=>state,
    activate(){active=true},
    dispose(){active=false;current=null},
    run(){if(!active)return Promise.resolve();if(current)return current.promise;return start("checking",check,true)},
    launch(){
      if(!active)return Promise.resolve();if(current)return current.promise;
      return start("launching",async task=>{
        try{if(!live(task))return;await source.launch()}finally{task.endMutation()}
        if(!live(task))return;await source.refresh();await check(task);
      },true);
    },
    selectRecommendation(){
      if(!active)return Promise.resolve();if(current)return current.promise;
      const recommendation=state.result?.recommendations?.[0];
      if(!recommendation)return Promise.resolve();
      return start("selecting",async task=>{
        try {
          if(!live(task)||!await source.confirm(recommendation)||!live(task))return;
          await source.select(recommendation);
        } finally {task.endMutation()}
        if(!live(task))return;
        publish({result:null});await source.refresh();await check(task);
      });
    },
    recover(){
      if(!active)return Promise.resolve();if(current?.kind==="recovering")return current.promise;
      const previous=current;
      return start("recovering",async task=>{
        if(previous)await previous.mutationDone;
        // Once requested, restoration must finish even if the component closes.
        await source.recover();
        if(live(task))publish({result:null,recovered:true,message:"已停止代理并恢复原有网络设置。"});
        try{await source.refresh()}catch(error){if(live(task))publish({message:"网络已恢复，但状态刷新失败："+String(error)})}
      },true);
    }
  };
}
