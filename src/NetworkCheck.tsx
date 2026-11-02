import React, { useEffect, useRef, useState } from "react";
import { Service, events, type Connectivity, type Status } from "./mygo";
import {confirmProxyCloseMessage,createNetworkCheckFlow,initialCheckState} from "./networkCheckFlow";
import type * as Design from "../design-system/components/index";

interface Props {
  design: typeof Design;
  open: boolean;
  status?: Status | null;
  onOpenChange: (open: boolean) => void;
  onLaunch: () => Promise<void>;
  onRefresh: () => Promise<void>;
  onView: (page: string) => void;
}
const target = "https://www.gstatic.com/generate_204";

export function NetworkCheck({ design: DS, open, status, onOpenChange, onLaunch, onRefresh, onView }: Props) {
  const [state,setState]=useState(initialCheckState);
  const [progress,setProgress]=useState<Connectivity|null>(null);
  const callbacks=useRef({onLaunch,onRefresh,status});callbacks.current={onLaunch,onRefresh,status};
  const confirmedOpen=useRef(false);
  function confirmIfNeeded() {
    if(confirmedOpen.current){confirmedOpen.current=false;return true;}
    const prompt=confirmProxyCloseMessage(callbacks.current.status);
    return !prompt || window.confirm(prompt);
  }
  const flow=useRef<ReturnType<typeof createNetworkCheckFlow>|null>(null);
  if(!flow.current)flow.current=createNetworkCheckFlow({
    confirmCheck:confirmIfNeeded,
    check:async()=>{const res=await Service.checkConnectivity(target);try{await callbacks.current.onRefresh()}catch{}return res},
    launch:()=>callbacks.current.onLaunch(),
    recover:()=>Service.recoverNetwork(),refresh:()=>callbacks.current.onRefresh(),
    confirm:rec=>window.confirm(`将“${rec.group}”从“${rec.current}”切换到“${rec.node}”（${rec.delay} ms）？`),
    select:rec=>Service.selectRecommendation(rec.group,rec.current,rec.node)
  },setState);
  useEffect(()=>{flow.current!.activate();return()=>flow.current!.dispose()},[]);
  useEffect(()=>events.connectivityProgress.on(next=>{if(flow.current?.snapshot().phase==="checking")setProgress(next)}),[]);
  useEffect(()=>{if(open){setProgress(null);void flow.current!.run()}},[open]);
  function handleOpenChange(nextOpen: boolean) {
    if(!nextOpen){onOpenChange(false);return;}
    if(!confirmIfNeeded())return;
    confirmedOpen.current=true;
    onOpenChange(true);
  }
  const {result,message,phase,recovered,recoveryFailed}=state,working=phase!=="idle";
  const shown=working&&progress?{...result,...progress}:result;
  const step=working?shown?.step:undefined;
  const localStatus: Design.ConnectivityCheckItem["status"] = recoveryFailed ? "fail" : !shown ? (working ? "running" : "idle") : shown.localStatus === "ok" || shown.local ? "ok" : shown.localStatus === "fail" ? "fail" : (working ? "running" : "idle");
  const takeoverStatus: Design.ConnectivityCheckItem["status"] = !shown ? (working && localStatus === "ok" ? "running" : "idle") : localStatus === "fail" ? "skip" : shown.takeoverStatus === "ok" ? "ok" : shown.takeoverStatus === "fail" ? "fail" : (working ? "running" : shown.takeoverStatus === "skip" ? "skip" : "fail");
  const nodeStatus: Design.ConnectivityCheckItem["status"] = !shown ? "idle" : (localStatus === "fail" || (takeoverStatus === "fail" && shown.nodeStatus !== "fail")) ? "skip" : shown.nodeStatus === "ok" ? "ok" : shown.nodeStatus === "fail" ? "fail" : (working ? "running" : shown.nodeStatus === "skip" ? "skip" : "idle");
  const checks: Design.ConnectivityCheckItem[] = [
    { id:"local", label:"本机网络", status:localStatus, detail:recoveryFailed ? message : localStatus==="running" ? step : shown?.localDetail || (shown ? (shown.local ? "无代理状态下本机网络正常" : shown.message) : undefined) },
    { id:"core", label:"代理状态", status:takeoverStatus, detail:takeoverStatus==="running" ? step : shown?.takeoverDetail || (shown && !shown.core ? shown.message : undefined), action:takeoverStatus === "fail" ? (shown?.autoExited ? { label:"检查配置或更新订阅", onClick:()=>{onOpenChange(false);onView("配置");} } : { label:"启动", onClick:()=>void flow.current!.launch() }) : undefined },
    { id:"node", label:"代理连接", status:nodeStatus, detail:nodeStatus==="running" ? step : shown?.nodeDetail || undefined, action:nodeStatus === "fail" ? { label:"检查配置或更新订阅", onClick:()=>{onOpenChange(false);onView("配置");} } : undefined }
  ];
  return <div id="tour-check"><DS.CheckPopover open={open} onOpenChange={handleOpenChange} recovered={recovered} checks={checks} onRun={()=>void flow.current!.run()} onRecover={phase==="recovering" ? undefined : ()=>void flow.current!.recover()}/>{message && <p className="network-check-message" role="status">{message}</p>}</div>;
}
