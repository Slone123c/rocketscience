import {spawnSync} from "node:child_process";
import {existsSync,mkdtempSync,rmSync} from "node:fs";
import {tmpdir} from "node:os";
import {join,resolve} from "node:path";

const root=resolve(import.meta.dir,"..");
const destination="/Applications/RocketScience.app";
const registrar="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister";
const args=process.argv.slice(2);
const arch=args.find(arg=>arg.startsWith("--arch="))?.slice(7);
const native=process.arch==="arm64"?"arm64":"amd64";
const target=arch||native;

function run(command:string,arguments_:string[]){
  const result=spawnSync(command,arguments_,{cwd:root,stdio:"inherit"});
  if(result.error)throw result.error;
  if(result.status!==0)throw new Error(`${command} 未完成（退出码 ${result.status}）`);
}

try {
  if(process.platform!=="darwin")throw new Error("安装需要在 macOS 运行");
  if(!["arm64","amd64"].includes(target)||args.some(arg=>arg!==`--arch=${target}`))
    throw new Error("用法：bun run install:app [--arch=arm64|--arch=amd64]");
  if(target!==native)throw new Error(`当前机器是 ${native}，不能把 ${target} 安装到 /Applications`);

  run("bun",["scripts/build-macos.ts",`--arch=${target}`]);
  const built=join(root,"build.noindex",`darwin-${target}`,"RocketScience.app");
  if(!existsSync(built))throw new Error(`构建产物不存在：${built}`);

  const quit=spawnSync("osascript",["-e",'tell application id "app.rocketscience.desktop" to quit'],{stdio:"ignore"});
  if(quit.status===0)spawnSync("sleep",["1"]);

  const backupDir=mkdtempSync(join(tmpdir(),"rocketscience-install-"));
  const backup=join(backupDir,"RocketScience.app");
  if(existsSync(destination))run("ditto",[destination,backup]);
  const staging=destination+".installing";
  rmSync(staging,{recursive:true,force:true});
  try {
    run("ditto",[built,staging]);
    rmSync(destination,{recursive:true,force:true});
    run("mv",[staging,destination]);
  }catch(error){
    rmSync(staging,{recursive:true,force:true});
    if(!existsSync(destination)&&existsSync(backup))run("mv",[backup,destination]);
    throw error;
  }
  try {
    run("codesign",["--verify","--deep","--strict",destination]);
  }catch(error){
    rmSync(destination,{recursive:true,force:true});
    if(existsSync(backup))run("mv",[backup,destination]);
    throw error;
  }
  rmSync(backupDir,{recursive:true,force:true});
  console.log(`已安装 ${destination}`);
}catch(error){
  console.error(error instanceof Error?error.message:String(error));
  process.exitCode=1;
}
