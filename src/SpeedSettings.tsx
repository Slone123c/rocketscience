import React, { useEffect, useState } from "react";
import { Profiles, type SpeedSettings as Values } from "./mygo";
import type * as Design from "../design-system/components/index";

type Field = "url" | "timeoutMs";
export function SpeedSettings({design:DS}:{design:typeof Design}) {
  const [saved, setSaved] = useState<Values | null>(null);
  const [url, setUrl] = useState("");
  const [timeout, setTimeoutValue] = useState("");
  const [error, setError] = useState("");
  const [saving, setSaving] = useState(false);
  const [applied, setApplied] = useState(false);
  useEffect(() => { void Profiles.speedSettings().then(v => { setSaved(v); setUrl(v.url); setTimeoutValue(String(v.timeoutMs)); }).catch(e => setError(String(e))); }, []);
  async function commit(field: Field, raw: string) {
    if (!saved || saving) return;
    const value = field === "url" ? { ...saved, url: raw } : { ...saved, timeoutMs: Number(raw) };
    if (value.url === saved.url && value.timeoutMs === saved.timeoutMs) return;
    setSaving(true); setError(""); setApplied(false);
    try { const result = await Profiles.setSpeedSettings(value); setSaved(result); setUrl(result.url); setTimeoutValue(String(result.timeoutMs)); setApplied(true); }
    catch (e) { setError(String(e)); }
    finally { setSaving(false); }
  }
  return <DS.SettingsSection title="测速" description="主窗口和菜单栏共用；仅影响手动测速。" status={applied?"已应用":null}>
    <DS.SettingRow label="测速地址" description="输入完整的 HTTP 或 HTTPS 地址。" configKey="url"><DS.SettingField aria-label="测速地址" type="text" width={280} value={url} disabled={!saved||saving} onCommit={value=>void commit("url",String(value))}/></DS.SettingRow>
    <DS.SettingRow label="测速超时" description="100 到 30000 毫秒。" configKey="timeout"><DS.SettingField aria-label="测速超时（毫秒）" type="number" width={120} suffix="毫秒" value={timeout} disabled={!saved||saving} validate={text=>/^\d+$/.test(text.trim())&&Number(text)>=100&&Number(text)<=30000?null:"超时请输入 100 到 30000 之间的整数（毫秒）。"} onCommit={value=>void commit("timeoutMs",String(value))}/></DS.SettingRow>
    {error && <p className="profile-error" role="alert">{error}</p>}
  </DS.SettingsSection>;
}
