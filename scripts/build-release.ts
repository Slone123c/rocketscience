import {spawnSync} from "node:child_process";
import {existsSync} from "node:fs";
import {join, resolve} from "node:path";

const root=resolve(import.meta.dir,"..");
const buildRoot=join(root,"build.noindex");
const args=process.argv.slice(2);
const platformArg=args.find(arg=>arg.startsWith("--platform="))?.slice(11);
const platforms=platformArg?platformArg.split(","):[
  process.platform==="darwin"?`darwin/${process.arch==="arm64"?"arm64":"amd64"}`:null,
  "windows/amd64",
  "linux/amd64",
].filter(Boolean) as string[];
const goFlags=[process.env.GOFLAGS,"-tags=with_gvisor"].filter(Boolean).join(" ");
function run(command:string,arguments_:string[],cwd:string=root,extraEnv:Record<string,string>={}){
  const result=spawnSync(command,arguments_,{cwd,stdio:"inherit",env:{...process.env,...extraEnv,GOFLAGS:goFlags}});
  if(result.error)throw result.error;
  if(result.status!==0)throw new Error(`${command} 未完成（退出码 ${result.status}）`);
}
try {
  run("bun",["scripts/collect-licenses.ts"]);
  for(const platform of platforms){
    const [,arch]=platform.split("/");
    if(platform.startsWith("darwin/")){
      if(process.platform!=="darwin"){console.warn(`跳过 ${platform}：当前主机不是 macOS`);continue;}
      run("sh",["scripts/build-helper.sh"],root,{ROCKETSCIENCE_ARCH:arch});
    }
    run("bun",["x","mygo","build","-platform",platform,"-o",buildRoot]);
    const artifactDir=join(buildRoot,platform.replace("/","-"));
    if(existsSync(artifactDir)){
      run("bun",["scripts/scan-release-artifact.ts",artifactDir]);
      run("bun",["scripts/verify-release-artifact.ts",artifactDir]);
    }
  }
}catch(error){
  console.error(error instanceof Error?error.message:String(error));
  process.exitCode=1;
}
