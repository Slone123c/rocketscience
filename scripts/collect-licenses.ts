import {execFileSync} from "node:child_process";
import {copyFileSync,existsSync,mkdirSync,readFileSync,readdirSync,rmSync,statSync,writeFileSync} from "node:fs";
import {basename,join,resolve} from "node:path";

const root=resolve(import.meta.dir,"..");
const outDir=join(root,"resources","licenses");
const supplementsDir=join(outDir,"supplements");
const supplementsManifest=join(outDir,"supplements.json");

const releaseGoTargets=[{goos:"darwin",goarch:"amd64"},{goos:"windows",goarch:"amd64"},{goos:"linux",goarch:"amd64"}] as const;

const licenseFileNames=[
  "LICENSE","LICENSE.md","LICENSE.txt","LICENSE.gpl","LICENSE-MIT","LICENSE.APACHE","COPYING","COPYING.md",
  "NOTICE","NOTICE.txt","UNLICENSE","LICENCE","LICENCE.txt",
];

type Entry={id:string;version:string;licenseHint:string;source:"npm"|"go"|"repo";path:string};
type MissingLicense={module:string;dir:string;searched:string[]};

function header(entry:Entry,body:string):string {
  return [
    `Component: ${entry.id}`,
    `Version: ${entry.version}`,
    `License: ${entry.licenseHint}`,
    `Origin: ${entry.source} (${basename(entry.path)})`,
    "---",
    body.replace(/\r\n/g,"\n").trimEnd(),
    "",
  ].join("\n");
}

function writeEntry(entry:Entry,body:string,filename:string){
  writeFileSync(join(outDir,filename),header(entry,body),"utf8");
}

function readNpm(name:string,folder:string):Entry{
  const pkgPath=join(root,"node_modules",folder,"package.json");
  const pkg=JSON.parse(readFileSync(pkgPath,"utf8"));
  const licenseFile=["LICENSE","LICENSE.md","LICENSE.txt","License"].find(f=>existsSync(join(root,"node_modules",folder,f)));
  if(!licenseFile)throw new Error(`缺少许可证文件：npm ${name}`);
  const body=readFileSync(join(root,"node_modules",folder,licenseFile),"utf8");
  const hint=typeof pkg.license==="string"?pkg.license:pkg.license?.type||"See package.json";
  const safe=name.replace(/[@/]/g,"-");
  writeEntry({id:name,version:pkg.version,licenseHint:hint,source:"npm",path:folder},body,`npm-${safe}-${pkg.version}.txt`);
  return {id:name,version:pkg.version,licenseHint:hint,source:"npm",path:folder};
}

function collectGoLicenseTexts(moduleDir:string):{body:string;files:string[];hint:string}{
  const parts:string[]=[];
  const found:string[]=[];
  for(const name of licenseFileNames){
    const p=join(moduleDir,name);
    if(existsSync(p)&&statSync(p).isFile()){
      parts.push(`# ${name}\n\n${readFileSync(p,"utf8").trimEnd()}`);
      found.push(name);
    }
  }
  if(parts.length===0){
    const err=new Error(`未找到许可证文件：${moduleDir}`) as Error&MissingLicense;
    err.module=moduleDir;
    err.dir=moduleDir;
    err.searched=[...licenseFileNames];
    throw err;
  }
  const body=parts.join("\n\n---\n\n");
  let hint="See file";
  if(body.includes("GNU GENERAL PUBLIC LICENSE"))hint="GPL-3.0";
  else if(/MIT License/i.test(body))hint="MIT";
  else if(/Apache License/i.test(body))hint="Apache-2.0";
  return {body,files:found,hint};
}

function moduleDir(modPath:string,version:string):string{
  const out=execFileSync("go",["list","-m","-json",`${modPath}@${version}`],{cwd:root,encoding:"utf8"});
  const info=JSON.parse(out) as {Dir?:string};
  if(!info.Dir||!existsSync(info.Dir))throw new Error(`未下载 Go 模块：${modPath} ${version}`);
  return info.Dir;
}

function loadSupplements():Record<string,string>{
  if(!existsSync(supplementsManifest))return {};
  const raw=JSON.parse(readFileSync(supplementsManifest,"utf8")) as Record<string,string>;
  return raw;
}

function readGo(modPath:string,version:string,filename:string,supplements:Record<string,string>){
  execFileSync("go",["mod","download",`${modPath}@${version}`],{cwd:root,stdio:"ignore"});
  const dir=moduleDir(modPath,version);
  const key=`${modPath}@${version}`;
  let body:string;
  let hint:string;
  try{
    ({body,hint}=collectGoLicenseTexts(dir));
  }catch(err){
    const rel=supplements[key];
    if(!rel){
      throw err;
    }
    const supPath=join(supplementsDir,rel);
    if(!existsSync(supPath)){
      throw new Error(`许可证补充文件缺失：${supPath}（${key}）`);
    }
    body=readFileSync(supPath,"utf8");
    hint="See supplement (GPL-3.0 via mihomo)";
  }
  writeEntry({id:modPath,version,licenseHint:hint,source:"go",path:modPath},body,filename);
}

function listGoModulesForTarget(goos:string,goarch:string):Array<[string,string]>{
  const lines=execFileSync(
    "go",
    ["list","-deps","-f","{{if .Module}}{{.Module.Path}} {{.Module.Version}}{{end}}","./..."],
    {cwd:root,encoding:"utf8",env:{...process.env,GOOS:goos,GOARCH:goarch}},
  )
    .split("\n")
    .map(line=>line.trim())
    .filter(Boolean);
  const out:Array<[string,string]>=[];
  for(const line of lines){
    const space=line.indexOf(" ");
    if(space<0)continue;
    out.push([line.slice(0,space),line.slice(space+1)]);
  }
  return out;
}

function main(){
  mkdirSync(outDir,{recursive:true});
  writeEntry(
    {id:"rocketscience",version:"0.1.0",licenseHint:"GPL-3.0",source:"repo",path:"LICENSE"},
    readFileSync(join(root,"LICENSE"),"utf8"),
    "rocketscience-GPL-3.0.txt",
  );
  copyFileSync(join(root,"THIRD_PARTY_NOTICES.md"),join(outDir,"THIRD_PARTY_NOTICES.md"));

  const existing=readFileSync(join(outDir,"mihomo-GPL-3.0.txt"),"utf8");
  if(!existing.includes("GNU GENERAL PUBLIC LICENSE"))throw new Error("mihomo 许可证文件无效");

  const npm:Entry[]=[
    readNpm("react","react"),
    readNpm("react-dom","react-dom"),
    readNpm("scheduler","scheduler"),
    readNpm("mygo-runtime","mygo-runtime"),
  ];

  const singTun=readFileSync(join(root,"third_party","sing-tun","LICENSE"),"utf8");
  writeEntry(
    {id:"github.com/metacubex/sing-tun",version:"v0.4.28 (vendored)",licenseHint:"GPL-3.0",source:"repo",path:"third_party/sing-tun"},
    singTun,
    "go-metacubex-sing-tun-v0.4.28.txt",
  );
  const pinnedSingTun="github.com/metacubex/sing-tun@v0.4.28";

  const mainModule=execFileSync("go",["list","-m","-f","{{.Path}}"],{cwd:root,encoding:"utf8"}).trim();
  const seen=new Set<string>();
  const goModules:Array<[string,string,string]>=[];
  const missing:MissingLicense[]=[];
  const fileFor=(modPath:string,version:string)=>{
    const slug=modPath.replace(/^github\.com\//,"").replaceAll("/","-").replaceAll("@","-");
    const verSlug=version.replace(/^v/,"").replaceAll(".","-");
    return `go-${slug}-v${verSlug}.txt`;
  };

  for(const {goos,goarch} of releaseGoTargets){
    for(const [modPath,version] of listGoModulesForTarget(goos,goarch)){
      if(!modPath||!version||modPath===mainModule)continue;
      if(`${modPath}@${version}`===pinnedSingTun)continue;
      const key=`${modPath}@${version}`;
      if(seen.has(key))continue;
      seen.add(key);
      goModules.push([modPath,version,fileFor(modPath,version)]);
    }
  }

  const supplements=loadSupplements();
  for(const [path,ver,file] of goModules){
    try{
      readGo(path,ver,file,supplements);
    }catch(err){
      if(err instanceof Error&&"searched" in err){
        const m=err as Error&MissingLicense;
        missing.push({module:`${path}@${ver}`,dir:m.dir,searched:m.searched});
      }else{
        throw err;
      }
    }
  }
  if(missing.length){
    const detail=missing.map(m=>`${m.module}: 未找到 ${m.searched.join(", ")}`).join("\n");
    throw new Error(`以下 Go 模块缺少可发布的许可证文件：\n${detail}`);
  }

  const manifest={
    generated:new Date().toISOString(),
    goTargets:releaseGoTargets.map(t=>`${t.goos}/${t.goarch}`),
    npm:npm.map(e=>`${e.id}@${e.version}`),
    go:goModules.map(([p,v])=>`${p}@${v}`),
  };
  writeFileSync(join(outDir,"MANIFEST.json"),JSON.stringify(manifest,null,2)+"\n","utf8");
  const keep=new Set([
    "MANIFEST.json","THIRD_PARTY_NOTICES.md","mihomo-GPL-3.0.txt","rocketscience-GPL-3.0.txt",
    "go-metacubex-sing-tun-v0.4.28.txt",
    ...npm.map(e=>`npm-${e.id.replace(/[@/]/g,"-")}-${e.version}.txt`),
    ...goModules.map(([, , file])=>file),
  ]);
  for(const name of readdirSync(outDir)){
    if(keep.has(name))continue;
    if(/^(go-|npm-)/.test(name))rmSync(join(outDir,name),{force:true});
  }
  console.log(`Collected ${readdirSync(outDir).length} license files into resources/licenses/ (go targets: ${manifest.goTargets.join(", ")})`);
}

try{main();}catch(e){console.error(e instanceof Error?e.message:String(e));process.exitCode=1;}
