import type {Connection,Proxy} from "./mygo";

function terms(query:string){return query.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean)}
function matches(value:string,query:string[]){const text=value.toLocaleLowerCase();return query.every(term=>text.includes(term))}
export function filterConnections(connections:Connection[],query:string,network:string="all") {
  const search=terms(query);
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
