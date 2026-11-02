import React from "react";
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
  onSavePort:(value:string)=>void;
  onSystemProxy:(enabled:boolean)=>void;onTun:(enabled:boolean)=>void;
  onRestart:()=>Promise<void>;onTopic:(topic:SettingsTopic)=>void;
}

export function SettingsPages({design:DS,topic,status,memory,busy,initialized,port,savedPort,onSavePort,onSystemProxy,onTun,onRestart,onTopic}:Props) {
  const running=!!status?.running;
  // Keep panels mounted while moving between topics so drafts and save feedback
  // survive navigation. The hidden attribute also removes inactive controls from focus.
  return <>
    <div className="settings-topic" hidden={topic!=="proxy"}>
      <DS.SettingsSection title="接管方式">
        <DS.SettingRow label="代理端口" description="本机混合代理端口；停止内核后修改。">
          <DS.SettingField aria-label="代理端口" type="number" width={120} value={port} disabled={running||busy} validate={text=>{const n=Number(text);return /^\d+$/.test(text.trim())&&n>=1&&n<=65535?null:"端口必须在 1 到 65535 之间"}} onCommit={value=>onSavePort(String(value))}/>
        </DS.SettingRow>
        <DS.SettingRow label="系统代理" description="启动后接管；关闭最后一种接管方式会恢复网络并停止内核。"><DS.Switch aria-label="系统代理" checked={status?.systemProxy||false} disabled={busy||!initialized} onChange={onSystemProxy}/></DS.SettingRow>
        <DS.SettingRow label="TUN 增强模式" description="首次开启时需要一次管理员授权来安装 TUN 服务；运行中修改会重启内核。关闭最后一种接管方式会停止内核。">
          <DS.Button size="sm" variant="ghost" disabled={busy} onClick={()=>onTopic("net")}>设置虚拟网卡参数</DS.Button>
          <DS.Switch aria-label="TUN 增强模式" checked={status?.tun||false} disabled={busy||!initialized} onChange={onTun}/>
        </DS.SettingRow>
        <DS.SettingRow label="终端代理命令" description="复制后在终端执行；仅影响该终端中遵守代理变量的程序，内核需保持运行。">
          <CopyButton design={DS} label="复制命令" copy={async()=>{const effectivePort=running&&status?.mixedPort?status.mixedPort:(await Profiles.summary()).mixedPort;await Desktop.copyTerminalProxy(effectivePort)}}/>
        </DS.SettingRow>
      </DS.SettingsSection>
      <LANSharing design={DS} running={running} port={running&&status?.mixedPort?status.mixedPort:savedPort}/>
      <ProxyBypass design={DS} running={running}/>
      <WiFiSettings design={DS}/>
    </div>
    <div className="settings-topic" hidden={topic!=="general"}><StartupSettings design={DS}/><SpeedSettings design={DS}/></div>
    <div className="settings-topic" hidden={topic!=="dns"}><DNSSettings design={DS} running={running} tun={!!status?.tun}/></div>
    <div className="settings-topic" hidden={topic!=="net"}><InboundSettings design={DS} running={running}/><TunSettings design={DS} running={running} tun={!!status?.tun}/></div>
    <div className="settings-topic" hidden={topic!=="general"&&topic!=="net"}><CoreSettings design={DS} running={running} section={topic==="general"?"general":"connections"}/></div>
    <div className="settings-topic" hidden={topic!=="core"}>
      <ControllerSettings design={DS} running={running} effective={status?.controller||""}/>
      <GeoSettings design={DS} running={running} warning={status?.geoWarning||""}/>
      <KernelSettings design={DS} running={running} memory={memory} busy={busy} onRestart={onRestart}/>
    </div>
  </>;
}
