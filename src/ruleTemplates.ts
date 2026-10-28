import type {OverrideGroup,OverrideRule} from "./mygo";

// The four entries are the original ControlCenter prototype's LAN_TEMPLATE.
export const lanDirectRules:readonly OverrideRule[] = [
  {type:"IP-CIDR",payload:"192.168.0.0/16",policy:"DIRECT"},
  {type:"IP-CIDR",payload:"10.0.0.0/8",policy:"DIRECT"},
  {type:"IP-CIDR",payload:"172.16.0.0/12",policy:"DIRECT"},
  {type:"DOMAIN-SUFFIX",payload:"local",policy:"DIRECT"}
];

export function addLANPreset(groups:OverrideGroup[],makeID:()=>string):{groups:OverrideGroup[];id:string;existing:boolean} {
  const existing=groups.find(group=>lanDirectRules.every(rule=>group.rules.some(saved=>saved.type===rule.type&&saved.payload===rule.payload&&saved.policy===rule.policy)));
  if(existing)return {groups:groups.map(group=>group.id===existing.id?{...group,enabled:true}:group),id:existing.id,existing:true};
  const id=makeID();
  return {groups:[...groups,{id,name:"局域网直连",enabled:true,rules:lanDirectRules.map(rule=>({...rule}))}],id,existing:false};
}
