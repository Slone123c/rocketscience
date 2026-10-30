import React from "react";
import type * as Design from "../design-system/components/index";
import type {ProxyRouteSummary as Summary} from "./proxyRouteSummary";

export function ProxyRouteSummary({design:DS,summary,running,onConnections}:{design:typeof Design;summary:Summary;running:boolean;onConnections?:()=>void}) {
  return <section className="proxy-route" aria-label="当前出站">
    <div className="proxy-route__head"><h2>{summary.title}</h2>{running&&onConnections&&<DS.Button variant="ghost" size="sm" onClick={onConnections}>查看连接</DS.Button>}</div>
    <p>{summary.detail}</p>
    {summary.path.length>1&&<p className="proxy-route__path">{summary.pathLabel}：{summary.path.join(" → ")}</p>}
    {running&&<div className="proxy-route__actual"><span>活动连接实际出站</span>{!summary.observed?<span>正在读取活动连接</span>:summary.actual.length===0&&summary.unknown===0?<span>暂无活动连接</span>:<><ul>{summary.actual.slice(0,4).map(node=><li key={node.name}><b>{node.name==="DIRECT"?"直连":node.name==="REJECT"?"拒绝连接":node.name}</b><span>{node.count} 条连接</span></li>)}</ul>{summary.actual.length>4&&<span>另有 {summary.actual.length-4} 个出站，可在连接页查看</span>}{summary.unknown>0&&<span>{summary.unknown} 条连接的出站未知</span>}</>}</div>}
  </section>;
}
