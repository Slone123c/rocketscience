import React, {useEffect, useState} from "react";
import type * as Design from "./ui/index";
import {Desktop, Profiles, type PlatformCapabilities, type Status, type MemoryUsage} from "./mygo";
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
  const [caps,setCaps]=useState<PlatformCapabilities|null>(null);
  useEffect(()=>{void Desktop.capabilities().then(setCaps).catch(()=>{})},[]);
  const proxyAuto=caps?.systemProxyAutomatic!==false;
  const tunCap=!!caps?.tun;
  // Keep panels mounted while moving between topics so drafts and save feedback
  // survive navigation. The hidden attribute also removes inactive controls from focus.
  return <>
    <div className="settings-topic" hidden={topic!=="proxy"}>
      <DS.SettingsSection title="接管方式">
        <DS.SettingRow label="代理端口" description="本机混合代理端口；停止内核后修改。">
          <DS.SettingField aria-label="代理端口" type="number" width={120} value={port} disabled={running||busy} validate={text=>{const n=Number(text);return /^\d+$/.test(text.trim())&&n>=1&&n<=65535?null:"端口必须在 1 到 65535 之间"}} onCommit={value=>onSavePort(String(value))}/>
        </DS.SettingRow>
        <DS.SettingRow label="系统代理" description={proxyAuto?"启动后写入系统代理设置；关闭最后一种接管方式会恢复网络并停止内核。":(caps?.manualProxyNote||"当前环境不支持自动系统代理。请启动内核后使用下方终端代理命令，或在系统设置中手动填写本机混合端口。")}><DS.Switch aria-label="系统代理" checked={status?.systemProxy||false} disabled={busy||!initialized||!proxyAuto} onChange={onSystemProxy}/></DS.SettingRow>
        {tunCap ? <DS.SettingRow label="TUN 增强模式" description="仅 macOS 首版提供。首次开启时需要一次管理员授权来安装 TUN 服务；运行中修改会重启内核。关闭最后一种接管方式会停止内核。">
          <DS.Button size="sm" variant="ghost" disabled={busy} onClick={()=>onTopic("net")}>设置虚拟网卡参数</DS.Button>
          <DS.Switch aria-label="TUN 增强模式" checked={status?.tun||false} disabled={busy||!initialized} onChange={onTun}/>
        </DS.SettingRow> : <DS.SettingRow label="TUN 增强模式" description={`首版仅在 ${caps?.tunFirstReleaseScope||"macOS"} 提供。Windows/Linux 请使用系统代理或终端代理命令。`}><DS.Switch aria-label="TUN 增强模式（不可用）" checked={false} disabled onChange={()=>undefined}/></DS.SettingRow>}
        <DS.SettingRow label="终端代理命令" description="复制后在终端执行；仅影响该终端中遵守代理变量的程序，内核需保持运行。无法自动写入系统代理时，这是推荐方式。">
          <CopyButton design={DS} label="复制命令" copy={async()=>{const effectivePort=running&&status?.mixedPort?status.mixedPort:(await Profiles.summary()).mixedPort;await Desktop.copyTerminalProxy(effectivePort)}}/>
        </DS.SettingRow>
      </DS.SettingsSection>
      <LANSharing design={DS} running={running} port={running&&status?.mixedPort?status.mixedPort:savedPort}/>
      <ProxyBypass design={DS} running={running}/>
      {caps?.trustedWiFi ? <WiFiSettings design={DS}/> : null}
    </div>
    <div className="settings-topic" hidden={topic!=="general"}><StartupSettings design={DS}/><SpeedSettings design={DS}/></div>
    <div className="settings-topic" hidden={topic!=="dns"}><DNSSettings design={DS} running={running} tun={!!status?.tun}/></div>
    <div className="settings-topic" hidden={topic!=="net"}><InboundSettings design={DS} running={running}/>{tunCap ? <TunSettings design={DS} running={running} tun={!!status?.tun}/> : null}</div>
    <div className="settings-topic" hidden={topic!=="general"&&topic!=="net"}><CoreSettings design={DS} running={running} section={topic==="general"?"general":"connections"}/></div>
    <div className="settings-topic" hidden={topic!=="core"}>
      <ControllerSettings design={DS} running={running} effective={status?.controller||""}/>
      <GeoSettings design={DS} running={running} warning={status?.geoWarning||""}/>
      <KernelSettings design={DS} running={running} memory={memory} busy={busy} onRestart={onRestart}/>
    </div>
  </>;
}
