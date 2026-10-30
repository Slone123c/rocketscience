import React,{useEffect,useState} from "react";
import type * as Design from "../design-system/components/index";

export function RuleWalkthrough({design:DS,open,onClose,onStep}:{design:typeof Design;open:boolean;onClose:()=>void;onStep?:(index:number)=>void}) {
  const [step,setStep]=useState(0);
  useEffect(()=>{if(open){setStep(0);onStep?.(0)}},[open]);
  const steps:Design.TourStep[]=[
    {target:"#rule-new-group",title:"按场景建一个分组",kicker:"规则指引",body:<>例如“公司内网”或“家里 NAS”。也可以使用已有分组；这个指引不会自动创建或修改规则。</>,placement:"top"},
    {target:"#rule-first-group .lf-ovr__add",title:"添加一条规则",kicker:"规则指引",body:<>选择匹配类型，填写域名或 IP，再选择走向。访问内网可以选择“直连”。</>,hint:"粘贴网址时会提取域名；单个 IP 会补齐网段。",placement:"top"},
    {target:"#rule-first-group .lf-switch",title:"打开分组开关",kicker:"规则指引",body:<>只有启用的分组会参与匹配。保存成功后会立即应用；失败时保留先前的运行规则。</>,placement:"left"},
    {target:"#rule-effective",title:"核对生效顺序",kicker:"规则指引",body:<>启动内核后可查看生效规则。覆盖规则排在订阅规则之前，从上到下匹配；具体连接命中了哪条规则，可在“连接”页查看。</>,placement:"top",doneLabel:"知道了"}
  ];
  return <DS.Tour open={open} steps={steps} step={step} onStepChange={index=>{setStep(index);onStep?.(index)}} onClose={onClose}/>;
}
