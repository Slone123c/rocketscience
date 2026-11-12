import {execFileSync} from "node:child_process";
import {copyFileSync,existsSync,mkdirSync,readFileSync,readdirSync,statSync,writeFileSync} from "node:fs";
import {basename,join,resolve} from "node:path";

const root=resolve(import.meta.dir,"..");
const outDir=join(root,"resources","licenses");

type Entry={id:string;version:string;licenseHint:string;source:"npm"|"go"|"repo";path:string};

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

function findGoLicense(moduleDir:string):string{
  for(const name of ["LICENSE","LICENSE.md","LICENSE.txt","LICENSE.gpl","COPYING","COPYING.md","NOTICE","NOTICE.txt"]){
    const p=join(moduleDir,name);
    if(existsSync(p)&&statSync(p).isFile())return readFileSync(p,"utf8");
  }
  const goMod=join(moduleDir,"go.mod");
  if(existsSync(goMod)){
    return [
      "No LICENSE/COPYING file was found in this module revision.",
      "See the upstream module repository for license terms.",
      "---",
      readFileSync(goMod,"utf8"),
    ].join("\n");
  }
  throw new Error(`Go 模块目录中未找到许可证：${moduleDir}`);
}

function moduleDir(modPath:string,version:string):string{
  const out=execFileSync("go",["list","-m","-json",`${modPath}@${version}`],{cwd:root,encoding:"utf8"});
  const info=JSON.parse(out) as {Dir?:string};
  if(!info.Dir||!existsSync(info.Dir))throw new Error(`未下载 Go 模块：${modPath} ${version}`);
  return info.Dir;
}

function readGo(modPath:string,version:string,filename:string){
  execFileSync("go",["mod","download",`${modPath}@${version}`],{cwd:root,stdio:"ignore"});
  const dir=moduleDir(modPath,version);
  const body=findGoLicense(dir);
  const hint=body.includes("GNU GENERAL PUBLIC LICENSE")?"GPL-3.0":body.includes("MIT License")?"MIT":"See file";
  writeEntry({id:modPath,version,licenseHint:hint,source:"go",path:modPath},body,filename);
}

function main(){
  mkdirSync(outDir,{recursive:true});
  // First-party
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
  const depLines=execFileSync(
    "go",
    ["list","-deps","-f","{{if .Module}}{{.Module.Path}} {{.Module.Version}}{{end}}","./..."],
    {cwd:root,encoding:"utf8"},
  )
    .split("\n")
    .map(line=>line.trim())
    .filter(Boolean);
  const seen=new Set<string>();
  const goModules:Array<[string,string,string]>=[];
  const fileFor=(modPath:string,version:string)=>{
    const slug=modPath.replace(/^github\.com\//,"").replaceAll("/","-").replaceAll("@","-");
    const verSlug=version.replace(/^v/,"").replaceAll(".","-");
    return `go-${slug}-v${verSlug}.txt`;
  };
  for(const line of depLines){
    const space=line.indexOf(" ");
    if(space<0)continue;
    const modPath=line.slice(0,space);
    const version=line.slice(space+1);
    if(!modPath||!version||modPath===mainModule)continue;
  if(`${modPath}@${version}`===pinnedSingTun)continue;
    const key=`${modPath}@${version}`;
    if(seen.has(key))continue;
    seen.add(key);
    goModules.push([modPath,version,fileFor(modPath,version)]);
  }
  for(const [path,ver,file] of goModules)readGo(path,ver,file);

  const manifest={generated:new Date().toISOString(),npm:npm.map(e=>`${e.id}@${e.version}`),go:goModules.map(([p,v])=>`${p}@${v}`)};
  writeFileSync(join(outDir,"MANIFEST.json"),JSON.stringify(manifest,null,2)+"\n","utf8");
  console.log(`Collected ${readdirSync(outDir).length} license files into resources/licenses/`);
}

try{main();}catch(e){console.error(e instanceof Error?e.message:String(e));process.exitCode=1;}
