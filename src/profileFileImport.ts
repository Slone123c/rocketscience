export async function readDroppedProfile(file:Pick<File,"name"|"size"|"arrayBuffer">):Promise<{name:string;path:string;content:string}> {
  if(!/\.ya?ml$/i.test(file.name))throw new Error("请选择 .yaml 或 .yml 配置文件");
  if(file.size>10*1024*1024)throw new Error("配置文件不能超过 10 MB");
  const bytes=await file.arrayBuffer();
  let content:string;
  try{content=new TextDecoder("utf-8",{fatal:true,ignoreBOM:true}).decode(bytes)}catch{throw new Error("配置文件必须使用 UTF-8 编码")}
  return {name:file.name.replace(/\.ya?ml$/i,""),path:file.name,content};
}
