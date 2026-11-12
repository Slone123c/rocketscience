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
  for(const name of ["LICENSE","LICENSE.md","LICENSE.txt","COPYING","COPYING.md","NOTICE","NOTICE.txt"]){
    const p=join(moduleDir,name);
    if(existsSync(p)&&statSync(p).isFile())return readFileSync(p,"utf8");
  }
  throw new Error(`Go 模块目录中未找到许可证：${moduleDir}`);
}

function moduleDir(modPath:string,version:string):string{
  const cache=execFileSync("go",["env","GOMODCACHE"],{cwd:root,encoding:"utf8"}).trim();
  const escaped=modPath.replace("@","%40")+"@"+version;
  const direct=join(cache,modPath+"@"+version);
  if(existsSync(direct))return direct;
  const nested=join(cache,escaped);
  if(existsSync(nested))return nested;
  throw new Error(`未下载 Go 模块：${modPath} ${version}`);
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

  const goModules:Array<[string,string,string]>=[
    ["github.com/egoist/mygo","v0.7.1","go-egoist-mygo-v0.7.1.txt"],
    ["github.com/metacubex/mihomo","v1.19.33","go-metacubex-mihomo-v1.19.33.txt"],
    ["github.com/metacubex/bbolt","v0.0.0-20260706163408-d4ec34ad7c48","go-metacubex-bbolt.txt"],
    ["github.com/ebitengine/purego","v0.11.1","go-ebitengine-purego-v0.11.1.txt"],
    ["github.com/oschwald/maxminddb-golang","v1.12.0","go-oschwald-maxminddb-golang-v1.12.0.txt"],
    ["golang.org/x/net","v0.59.0","go-golang-org-x-net-v0.59.0.txt"],
    ["golang.org/x/sys","v0.48.0","go-golang-org-x-sys-v0.48.0.txt"],
    ["gopkg.in/yaml.v3","v3.0.1","go-gopkg-in-yaml-v3-v3.0.1.txt"],
  ];
  for(const [path,ver,file] of goModules)readGo(path,ver,file);

  const manifest={generated:new Date().toISOString(),npm:npm.map(e=>`${e.id}@${e.version}`),go:goModules.map(([p,v])=>`${p}@${v}`)};
  writeFileSync(join(outDir,"MANIFEST.json"),JSON.stringify(manifest,null,2)+"\n","utf8");
  console.log(`Collected ${readdirSync(outDir).length} license files into resources/licenses/`);
}

try{main();}catch(e){console.error(e instanceof Error?e.message:String(e));process.exitCode=1;}
