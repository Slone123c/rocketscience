import {execFileSync} from "node:child_process";
import {existsSync,readdirSync,statSync} from "node:fs";
import {basename,join,resolve} from "node:path";
import {assertUnderBuildOutput,safeWalk} from "./fs-safe";

const root=resolve(import.meta.dir,"..");
const buildRoot=join(root,"build.noindex");
const args=process.argv.slice(2);
const target=resolve(args.find(arg=>!arg.startsWith("--"))||buildRoot);

type PlatformKind="darwin"|"linux"|"windows";

function kindFromDirName(name:string):PlatformKind|null{
  const lower=name.toLowerCase();
  if(lower.startsWith("darwin-"))return "darwin";
  if(lower.startsWith("linux-"))return "linux";
  if(lower.startsWith("windows-"))return "windows";
  return null;
}

const forbiddenNonDarwin=[
  /\.dylib$/i,
  /(^|\/)rs-tun-helper$/,
  /(^|\/)rs-system-proxy$/,
  /(^|\/)libRocketNetwork\.dylib$/,
];

function hitsForbidden(relativePath:string,kind:PlatformKind):string[]{
  if(kind==="darwin")return [];
  const norm=relativePath.replaceAll("\\","/");
  const out:string[]=[];
  for(const pattern of forbiddenNonDarwin){
    if(pattern.test(norm))out.push(`${relativePath}: forbidden on ${kind} (${pattern})`);
  }
  return out;
}

function listTar(archive:string):string[]{
  const out=execFileSync("tar",["-tzf",archive],{encoding:"utf8",maxBuffer:32*1024*1024});
  return out.split("\n").map(line=>line.trim()).filter(Boolean);
}

function listZip(archive:string):string[]{
  const out=execFileSync("unzip",["-Z1",archive],{encoding:"utf8",maxBuffer:32*1024*1024});
  return out.split("\n").map(line=>line.trim()).filter(Boolean);
}

try{
  assertUnderBuildOutput(target,buildRoot);
  const kind=kindFromDirName(basename(target));
  if(!kind)throw new Error(`无法从目录名推断平台：${basename(target)}（期望 darwin-|linux-|windows-）`);

  const hits:string[]=[];
  safeWalk(target,(file,stat)=>{
    if(!stat.isFile())return;
    const relative=file.slice(target.length+1);
    hits.push(...hitsForbidden(relative,kind));
  });

  for(const name of readdirSync(target)){
  const path=join(target,name);
  if(!statSync(path).isFile())continue;
  let entries:string[]=[];
  if(name.endsWith(".tar.gz"))entries=listTar(path);
  else if(name.endsWith(".zip"))entries=listZip(path);
  else continue;
  for(const entry of entries)hits.push(...hitsForbidden(`${name}::${entry}`,kind));
  }

  if(hits.length){
    console.error(`发布包平台校验失败（${basename(target)}）：\n`+hits.join("\n"));
    process.exitCode=1;
  }else{
    console.log(`发布包平台校验通过：${basename(target)}（${kind} 产物未含 macOS 专用二进制；仅检查路径/归档名单，不验证 PE/ELF 魔数）`);
  }
}catch(error){
  console.error(error instanceof Error?error.message:String(error));
  process.exitCode=1;
}
