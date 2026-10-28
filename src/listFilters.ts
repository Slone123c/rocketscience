import type {Connection,Proxy,Rule} from "./mygo";

function terms(query:string){return query.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean)}
function matches(value:string,query:string[]){const text=value.toLocaleLowerCase();return query.every(term=>text.includes(term))}
export function filterConnections(connections:Connection[],query:string,network:string="all") {
  const search=terms(query);
  if(!search.length)return network==="all"?connections:connections.filter(c=>c.metadata.network.toUpperCase()===network);
  return connections.filter(c=>(network==="all"||c.metadata.network.toUpperCase()===network)&&matches([c.metadata.host,c.metadata.destinationIP,c.metadata.destinationPort,c.metadata.network,c.metadata.process,c.rule,c.rulePayload,...(c.chains||[])].join(" "),search));
}
export function filterProxyGroups(groups:Proxy[],query:string):Proxy[] {
  const search=terms(query);if(!search.length)return groups;
  return groups.flatMap(group=>{
    if(matches(group.name,search))return [group];
    const all=(group.all||[]).filter(name=>matches(name,search));
    return all.length?[{...group,all}]:[];
  });
}
export function filterRules(rules:Rule[],query:string="",limit:number=500):Rule[] {
  const search=terms(query);
  if(!search.length)return limit>0&&rules.length>limit?rules.slice(0,limit):rules;
  const out:Rule[]=[];
  for(let i=0;i<rules.length;i++){
    const r=rules[i];
    if(matches(`${r.type} ${r.payload} ${r.proxy}`,search)){
      out.push(r);
      if(limit>0&&out.length>=limit)break;
    }
  }
  return out;
}
