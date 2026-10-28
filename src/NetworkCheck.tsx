import React, { useEffect, useRef, useState } from "react";
import { Service, type Connectivity } from "./mygo";
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
  const [result, setResult] = useState<Connectivity | null>(null);
  const [working, setWorking] = useState(false);
  const [message, setMessage] = useState("");
  const request = useRef(0);
  async function run() {
    const id = ++request.current;
    setWorking(true); setMessage("");
    try { const next = await Service.checkConnectivity(target); if (id === request.current) setResult(next); }
    catch (e) { if (id === request.current) { setResult(null); setMessage(String(e)); } }
    finally { if (id === request.current) setWorking(false); }
  }
  useEffect(() => { if (open) void run(); }, [open]);
  async function recover() {
    ++request.current; setWorking(true); setMessage("");
    try { await Service.recoverNetwork(); await onRefresh(); setResult(null); setMessage("已停止代理并恢复原有网络设置。"); }
    catch (e) { setMessage("恢复网络失败：" + String(e)); }
    finally { setWorking(false); }
  }
  async function launch() {
    ++request.current; setWorking(true); setMessage("");
    try { await onLaunch(); await run(); } catch (e) { setWorking(false); setMessage(String(e)); }
  }
  async function acceptRecommendation() {
    const rec = result?.recommendations?.[0];
    if (!rec || !window.confirm(`将“${rec.group}”从“${rec.current}”切换到“${rec.node}”（${rec.delay} ms）？`)) return;
    ++request.current; setWorking(true); setMessage("");
    try { await Service.selectRecommendation(rec.group, rec.current, rec.node); await onRefresh(); await run(); }
    catch (e) { setWorking(false); setMessage(String(e)); }
  }
  const rec = result?.recommendations?.[0];
  const nodeStatus: Design.ConnectivityCheckItem["status"] = working ? "running" : !result ? "idle" : result.nodeStatus === "ok" ? "ok" : result.nodeStatus === "fail" ? "fail" : "skip";
  const checks: Design.ConnectivityCheckItem[] = [
    { id:"core", label:"代理内核", status:working ? "running" : !result ? "idle" : result.core ? "ok" : "fail", detail:result && !result.core ? result.message : undefined, action:result && !result.core ? { label:"启动", onClick:()=>void launch() } : undefined },
    { id:"node", label:"已选节点", status:nodeStatus, detail:result?.nodeDetail || undefined, action:result?.nodeStatus === "fail" ? rec ? { label:`切换到 ${rec.node}（${rec.delay} ms）`, onClick:()=>void acceptRecommendation() } : { label:"检查配置或更新订阅", onClick:()=>{onOpenChange(false);onView("配置");} } : undefined },
    { id:"request", label:"访问外网", status:working ? "running" : !result ? "idle" : !result.core ? "skip" : result.request ? "ok" : "fail", detail:result?.core ? result.message : undefined, action:result?.core && !result.request ? {label:"查看节点",onClick:()=>{onOpenChange(false);onView("代理");}} : undefined }
  ];
  return <div id="tour-check"><DS.CheckPopover open={open} onOpenChange={onOpenChange} checks={checks} onRun={()=>void run()} onRecover={working ? undefined : ()=>void recover()}/>{message && <p className="network-check-message" role="status">{message}</p>}</div>;
}
