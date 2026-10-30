import React, { useEffect, useRef, useState } from "react";
import { Service, type Status } from "./mygo";
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
  useEffect(()=>{if(open)void flow.current!.run()},[open]);
  function handleOpenChange(nextOpen: boolean) {
    if(!nextOpen){onOpenChange(false);return;}
    if(!confirmIfNeeded())return;
    confirmedOpen.current=true;
    onOpenChange(true);
  }
  const {result,message,phase,recovered,recoveryFailed}=state,working=phase!=="idle";
  const localStatus: Design.ConnectivityCheckItem["status"] = working ? "running" : recoveryFailed ? "fail" : !result ? "idle" : result.localStatus === "ok" || result.local ? "ok" : "fail";
  const takeoverStatus: Design.ConnectivityCheckItem["status"] = working ? "running" : !result ? "idle" : localStatus === "fail" ? "skip" : result.takeoverStatus === "ok" ? "ok" : result.takeoverStatus === "fail" ? "fail" : result.takeoverStatus === "skip" ? "skip" : result.core ? "ok" : "fail";
  const nodeStatus: Design.ConnectivityCheckItem["status"] = working ? "running" : !result ? "idle" : (localStatus === "fail" || (takeoverStatus === "fail" && result.nodeStatus !== "fail")) ? "skip" : result.nodeStatus === "ok" ? "ok" : result.nodeStatus === "fail" ? "fail" : result.nodeStatus === "skip" ? "skip" : "idle";
  const checks: Design.ConnectivityCheckItem[] = [
    { id:"local", label:"本机网络", status:localStatus, detail:recoveryFailed ? message : result?.localDetail || (result ? (result.local ? "无代理状态下本机网络正常" : result.message) : undefined) },
    { id:"core", label:"代理状态", status:takeoverStatus, detail:result?.takeoverDetail || (result && !result.core ? result.message : undefined), action:takeoverStatus === "fail" ? (result?.autoExited ? { label:"检查配置或更新订阅", onClick:()=>{onOpenChange(false);onView("配置");} } : { label:"启动", onClick:()=>void flow.current!.launch() }) : undefined },
    { id:"node", label:"代理连接", status:nodeStatus, detail:result?.nodeDetail || undefined, action:nodeStatus === "fail" ? { label:"检查配置或更新订阅", onClick:()=>{onOpenChange(false);onView("配置");} } : undefined }
  ];
  return <div id="tour-check"><DS.CheckPopover open={open} onOpenChange={handleOpenChange} recovered={recovered} checks={checks} onRun={()=>void flow.current!.run()} onRecover={phase==="recovering" ? undefined : ()=>void flow.current!.recover()}/>{message && <p className="network-check-message" role="status">{message}</p>}</div>;
}
