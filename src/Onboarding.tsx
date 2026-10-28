import React, { useEffect, useRef, useState } from "react";
import { shouldAdvanceAfterLaunch } from "./onboardingFlow";
import type * as Design from "../design-system/components/index";

interface Props {
  design: typeof Design;
  running: boolean;
  open: boolean;
  onView: (view: string) => void;
  onClose: (completed: boolean) => void;
  onCheck: () => void;
  profileEditing?: boolean;
}

// The tour only describes and highlights existing controls; it never starts a core or changes a setting.
export function Onboarding({ design: DS, running, open, onView, onClose, onCheck,profileEditing=false }: Props) {
  const [step, setStep] = useState(0);
  const previousRunning = useRef(running);
  const steps: Design.TourStep[] = [
    { title: "先认识代理", body: <>已停止 RocketScience 的代理并恢复原有网络设置。连接后，流量会按照配置选择的路径访问网站。</>, media: <>你的电脑　→　节点　→　网站</>, view: "总览" },
    { title: "导入并保存配置", body: <>导入本地配置或填写订阅链接，保存后选择要使用的配置。编辑窗口关闭后可继续引导。</>, target:"#tour-profile", view: "配置" },
    { title: "选择接管方式", body: <>打开系统代理，启动后才会接管。停止代理会恢复之前的设置。</>, target: "#tour-takeover", view: "设置" },
    { title: "手动启动", body: <>点击启动。遇到网络问题时，再点停止即可恢复之前的网络设置。</>, target: "#tour-launch", view: "总览", hint: "启动成功后会自动进入下一步" },
    { title: "测速并选择节点", body: <>在策略组中测速，再选择可用的节点；如果连接有问题，可返回总览停止代理。</>, target: "#tour-nodes", view: "代理" },
    { title: "检查网络", body: <>打开网络检查，确认连接是否正常。失败时可以更换节点，或选择停止代理、恢复网络。</>, target: "#tour-check .lf-chkpop__panel", view: "代理" },
    { title: "选择模式", body: <>日常推荐规则模式；也可以选择全局或直连。配置中的规则决定流量走向。</>, target: "#tour-mode", view: "代理" },
    { title: "准备完成", body: <>先导入配置，选择接管方式并启动，再测速、选择节点和检查网络。遇到断网时，停止代理以恢复之前的网络设置。</>, view: "总览" }
  ];
  useEffect(() => { if (open) { setStep(0); onView("总览"); } }, [open]);
  useEffect(() => { if (shouldAdvanceAfterLaunch(open, step, previousRunning.current, running)) { setStep(4); onView("代理"); } previousRunning.current = running; }, [open, step, running]);
  return <DS.Tour open={open&&!profileEditing} steps={steps} step={step} onStepChange={(index, next) => {if(step===1&&profileEditing)return;setStep(index); if (next.view) onView(next.view); if (index === 5) onCheck(); }} onClose={onClose}/>;
}
