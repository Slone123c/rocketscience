import React, { useEffect, useRef, useState } from "react";
import { shouldAdvanceAfterLaunch } from "./onboardingFlow";
import { LAUNCH_VISUAL_MS } from "./OverviewDeck";
import type * as Design from "./ui/index";

interface Props {
  design: typeof Design;
  running: boolean;
  open: boolean;
  onView: (view: string) => void;
  onClose: (completed: boolean) => void;
  onCheck: (open: boolean) => void;
  profileEditing?: boolean;
}

function RouteStop({ design: DS, icon, label, cls, sub }: { design: typeof Design; icon: Design.IconName; label: string; cls: string; sub?: string }) {
  return <div className={`route__stop ${cls}`}><span className="route__dot"><DS.Icon name={icon} size={18}/></span><span>{label}</span>{sub && <small>{sub}</small>}</div>;
}

function RouteDiagram({ design: DS }: { design: typeof Design }) {
  return <div className="route" role="img" aria-label="你的电脑 → 节点 → 网站">
    <RouteStop design={DS} icon="laptop" label="你的电脑" cls="is-me"/>
    <span className="route__hop" aria-hidden="true"><DS.Icon name="arrow-right" size={12}/></span>
    <RouteStop design={DS} icon="server" label="节点" cls="is-node" sub="海外服务器"/>
    <span className="route__hop" aria-hidden="true"><DS.Icon name="arrow-right" size={12}/></span>
    <RouteStop design={DS} icon="globe" label="网站" cls="is-web"/>
  </div>;
}

// The tour only describes and highlights existing controls; it never starts a core or changes a setting.
export function Onboarding({ design: DS, running, open, onView, onClose, onCheck,profileEditing=false }: Props) {
  const [step, setStep] = useState(0);
  const previousRunning = useRef(running);
  const steps: Design.TourStep[] = [
    { title: "先认识代理", body: <>已停止 Rocket Science 的代理并恢复原有网络设置。连接后，流量会按照配置选择的路径访问网站。</>, media: <RouteDiagram design={DS}/>, view: "总览" },
    { title: "导入并保存配置", body: <>导入本地配置或填写订阅链接，保存后选择要使用的配置。编辑窗口关闭后可继续引导。</>, target: "#page-tools .lf-btn--primary", placement: "bottom", view: "配置" },
    { title: "启动", body: <>点击启动，大多数应用就会走代理。遇到网络问题时，再点停止即可恢复之前的网络设置。</>, target: "#tour-launch .lf-ignite__core", padding: 12, view: "总览", hint: "启动成功后会自动进入下一步" },
    { title: "测速并选择节点", body: <>在策略组中测速，再选择可用的节点；如果连接有问题，可返回总览停止代理。</>, target: "#tour-nodes .lf-group", placement: "bottom", view: "代理" },
    { title: "检查网络", body: <>打开网络检查，确认连接是否正常。失败时可以更换节点，或选择停止代理、恢复网络。</>, target: "#tour-check .lf-chkpop__trigger", placement: "right", view: "代理" },
    { title: "选择模式", body: <>日常推荐规则模式；也可以选择全局或直连。配置中的规则决定流量走向。</>, target: "#tour-mode .lf-seg", view: "代理" },
    { title: "引导完成", body: <>先导入配置并启动，再测速、选择节点和检查网络。遇到断网时，停止代理以恢复之前的网络设置。</>, view: "总览" }
  ];
  useEffect(() => { onCheck(false); if (open) { setStep(0); onView("总览"); } }, [open]);
  useEffect(() => {
    const advance = shouldAdvanceAfterLaunch(open, step, previousRunning.current, running);
    previousRunning.current = running;
    if (!advance) return;
    const timer = window.setTimeout(() => { setStep(3); onView("代理"); }, LAUNCH_VISUAL_MS);
    return () => window.clearTimeout(timer);
  }, [open, step, running]);
  return <DS.Tour open={open&&!profileEditing} steps={steps} step={step} onStepChange={(index, next) => {if(step===1&&profileEditing)return;setStep(index); if (next.view) onView(next.view); onCheck(index === 4); }} onClose={onClose}/>;
}

// Shown when start is pressed with no takeover on. Turning a switch on continues that start.
export function TakeoverGuide({ design: DS, open, onClose }: { design: typeof Design; open: boolean; onClose: () => void }) {
  return <DS.Tour open={open} doneLabel="稍后再选" onClose={onClose} steps={[{
    title: "选择代理方式",
    body: <>打开一个开关后继续启动。不确定就选系统代理；TUN 首次需要管理员密码。</>,
    target: "#tour-takeover",
    padding: 0,
  }]}/>;
}

// Shown when start is pressed with no profile. Opening the editor closes it.
export function ImportGuide({ design: DS, open, onClose }: { design: typeof Design; open: boolean; onClose: () => void }) {
  return <DS.Tour open={open} doneLabel="稍后再说" onClose={onClose} steps={[{
    title: "先导入配置",
    body: <>导入配置文件或填写订阅链接，保存并选用后就能启动。</>,
    target: "#page-tools",
    placement: "bottom",
  }]}/>;
}
