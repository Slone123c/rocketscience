import React, { useEffect, useRef, useState } from "react";
import { Service } from "./mygo";
import {createNetworkCheckFlow,initialCheckState} from "./networkCheckFlow";
import type * as Design from "../design-system/components/index";

interface Props {
  design: typeof Design;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onLaunch: () => Promise<void>;
  onRefresh: () => Promise<void>;
  onView: (page: string) => void;
}
const target = "https://www.gstatic.com/generate_204";

export function NetworkCheck({ design: DS, open, onOpenChange, onLaunch, onRefresh, onView }: Props) {
  const [state,setState]=useState(initialCheckState);
  const callbacks=useRef({onLaunch,onRefresh});callbacks.current={onLaunch,onRefresh};
  const flow=useRef<ReturnType<typeof createNetworkCheckFlow>|null>(null);
  if(!flow.current)flow.current=createNetworkCheckFlow({
    check:()=>Service.checkConnectivity(target),launch:()=>callbacks.current.onLaunch(),
    recover:()=>Service.recoverNetwork(),refresh:()=>callbacks.current.onRefresh(),
    confirm:rec=>window.confirm(`将“${rec.group}”从“${rec.current}”切换到“${rec.node}”（${rec.delay} ms）？`),
    select:rec=>Service.selectRecommendation(rec.group,rec.current,rec.node)
  },setState);
  useEffect(()=>{flow.current!.activate();return()=>flow.current!.dispose()},[]);
  useEffect(()=>{if(open)void flow.current!.run()},[open]);
  const {result,message,phase,recovered,recoveryFailed}=state,working=phase!=="idle";
  const rec = result?.recommendations?.[0];
  const nodeStatus: Design.ConnectivityCheckItem["status"] = working ? "running" : !result ? "idle" : result.nodeStatus === "ok" ? "ok" : result.nodeStatus === "fail" ? "fail" : result.nodeStatus === "skip" ? "skip" : "idle";
  const checks: Design.ConnectivityCheckItem[] = [
    { id:"core", label:"代理内核", status:working ? "running" : recoveryFailed ? "fail" : !result ? "idle" : result.core ? "ok" : "fail", detail:recoveryFailed ? message : result && !result.core ? result.message : undefined, action:!recoveryFailed&&result&&!result.core ? { label:"启动", onClick:()=>void flow.current!.launch() } : undefined },
    { id:"node", label:"已选节点", status:nodeStatus, detail:result?.nodeDetail || undefined, action:result?.nodeStatus === "fail" ? rec ? { label:`切换到 ${rec.node}（${rec.delay} ms）`, onClick:()=>void flow.current!.selectRecommendation() } : { label:"检查配置或更新订阅", onClick:()=>{onOpenChange(false);onView("配置");} } : undefined },
    { id:"request", label:"访问外网", status:working ? "running" : !result ? "idle" : !result.core ? "skip" : result.request ? "ok" : "fail", detail:result?.core ? result.message : undefined, action:result?.core && !result.request ? {label:"查看节点",onClick:()=>{onOpenChange(false);onView("代理");}} : undefined }
  ];
  return <div id="tour-check"><DS.CheckPopover open={open} onOpenChange={onOpenChange} recovered={recovered} checks={checks} onRun={()=>void flow.current!.run()} onRecover={phase==="recovering" ? undefined : ()=>void flow.current!.recover()}/>{message && <p className="network-check-message" role="status">{message}</p>}</div>;
}
