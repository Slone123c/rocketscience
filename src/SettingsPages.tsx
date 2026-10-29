import React, {useRef} from "react";
import type * as Design from "../design-system/components/index";
import {Desktop, Profiles, type Status, type MemoryUsage} from "./mygo";
import {CopyButton} from "./CopyButton";
import {StartupSettings} from "./StartupSettings";
import {WiFiSettings} from "./WiFiSettings";
import {LANSharing} from "./LANSharing";
import {ProxyBypass} from "./ProxyBypass";
import {SpeedSettings} from "./SpeedSettings";
import {CoreSettings} from "./CoreSettings";
import {DNSSettings} from "./DNSSettings";
import {TunSettings} from "./TunSettings";
import {InboundSettings} from "./InboundSettings";
import {ControllerSettings} from "./ControllerSettings";
import {GeoSettings} from "./GeoSettings";
import {KernelSettings} from "./KernelSettings";
import type {SettingsTopic} from "./settingsNavigation";

interface Props {
  design:typeof Design;topic:SettingsTopic;status:Status|null;memory:MemoryUsage|null;
  busy:boolean;initialized:boolean;port:string;savedPort:number;
  onPortChange:(value:string)=>void;onSavePort:()=>void;
  onSystemProxy:(enabled:boolean)=>void;onTun:(enabled:boolean)=>void;
  onRestart:()=>Promise<void>;onTopic:(topic:SettingsTopic)=>void;
}

export function SettingsPages({design:DS,topic,status,memory,busy,initialized,port,savedPort,onPortChange,onSavePort,onSystemProxy,onTun,onRestart,onTopic}:Props) {
  const running=!!status?.running;
  const portCanceled=useRef(false);
  // Keep panels mounted while moving between topics so drafts and save feedback
  // survive navigation. The hidden attribute also removes inactive controls from focus.
  return <>
    <div className="settings-topic" hidden={topic!=="proxy"}>
      <section className="panel settings"><div className="eyebrow">PROXY / SHARING</div><h2>接管方式</h2>
        <div className="setting"><div><h3>代理端口</h3><p>本机混合代理端口；停止内核后修改。</p></div><input aria-label="代理端口" type="number" min="1" max="65535" value={port} disabled={running||busy} onChange={e=>onPortChange(e.target.value)} onBlur={()=>{if(portCanceled.current){portCanceled.current=false;return}onSavePort()}} onKeyDown={e=>{if(e.key==="Enter"){e.preventDefault();e.currentTarget.blur()}else if(e.key==="Escape"){portCanceled.current=true;onPortChange(String(savedPort));e.currentTarget.blur()}}}/></div>
        <div className="setting" id="tour-takeover"><div><h3>系统代理</h3><p>启动后接管；关闭最后一种接管方式会恢复网络并停止内核。</p></div><DS.Switch aria-label="系统代理" checked={status?.systemProxy||false} disabled={busy||!initialized} onChange={onSystemProxy}/></div>
        <div className="setting"><div><h3>TUN 增强模式</h3><p>启动时需要管理员授权；运行中修改会重启内核。关闭最后一种接管方式会停止内核。</p><button className="text-button" disabled={busy} onClick={()=>onTopic("net")}>设置虚拟网卡参数 →</button></div><DS.Switch aria-label="TUN 增强模式" checked={status?.tun||false} disabled={busy||!initialized} onChange={onTun}/></div>
        <div className="setting"><div><h3>终端代理命令</h3><p>复制后在终端执行；仅影响该终端中遵守代理变量的程序，内核需保持运行。</p></div><CopyButton design={DS} label="复制命令" copy={async()=>{const effectivePort=running&&status?.mixedPort?status.mixedPort:(await Profiles.summary()).mixedPort;await Desktop.copyTerminalProxy(effectivePort)}}/></div>
      </section>
      <LANSharing design={DS} running={running} port={running&&status?.mixedPort?status.mixedPort:savedPort}/>
      <ProxyBypass design={DS} running={running}/>
      <WiFiSettings design={DS}/>
    </div>
    <div className="settings-topic" hidden={topic!=="general"}><StartupSettings design={DS}/><SpeedSettings/></div>
    <div className="settings-topic" hidden={topic!=="dns"}><DNSSettings running={running} tun={!!status?.tun}/></div>
    <div className="settings-topic" hidden={topic!=="net"}><InboundSettings running={running}/><TunSettings running={running}/></div>
    <div className="settings-topic" hidden={topic!=="general"&&topic!=="net"}><CoreSettings running={running} section={topic==="general"?"general":"connections"}/></div>
    <div className="settings-topic" hidden={topic!=="core"}>
      <ControllerSettings running={running} effective={status?.controller||""}/>
      <GeoSettings running={running} warning={status?.geoWarning||""}/>
      <KernelSettings running={running} memory={memory} busy={busy} onRestart={onRestart}/>
    </div>
  </>;
}
