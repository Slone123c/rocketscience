import {execFileSync,spawnSync} from "node:child_process";
import {cpSync,existsSync,mkdirSync,mkdtempSync,rmSync,symlinkSync} from "node:fs";
import {dirname,join,resolve} from "node:path";
import {tmpdir} from "node:os";

const root=resolve(import.meta.dir,"..");
const args=process.argv.slice(2);
const requested=args.find(arg=>arg.startsWith("--arch="))?.slice(7);
const native=process.arch==="arm64"?"arm64":"amd64";
const target=requested||native;
let stage:string|undefined;
// TUN's default mixed stack and the gvisor stack need gVisor, which mihomo
// only compiles in with this tag (its own releases set it too).
const goFlags=[process.env.GOFLAGS,"-tags=with_gvisor"].filter(Boolean).join(" ");
function run(command:string,arguments_:string[],cwd:string){
  const result=spawnSync(command,arguments_,{cwd,stdio:"inherit",env:{...process.env,ROCKETSCIENCE_ARCH:target,GOFLAGS:goFlags}});
  if(result.error)throw result.error;
  if(result.status!==0)throw new Error(`${command} 未完成构建（退出码 ${result.status}）`);
}
try {
  if(process.platform!=="darwin")throw new Error("macOS 应用打包需要在 macOS 运行");
  if(!["arm64","amd64"].includes(target)||args.some(arg=>arg!==`--arch=${target}`)||args.length>1)
    throw new Error("用法：bun run build [--arch=arm64|--arch=amd64]");
  let project=root;
  if(target!==native){
    stage=mkdtempSync(join(tmpdir(),"rocketscience-cross-build-"));
    // Include current tracked edits and untracked source, respecting Git ignores.
    // Cross builds must not overwrite the native resources used by mygo dev.
    const files=execFileSync("git",["ls-files","-z","--cached","--others","--exclude-standard"],{cwd:root,encoding:"utf8"}).split("\0").filter(Boolean);
    for(const file of new Set(files)){
      const source=join(root,file);if(!existsSync(source))continue;
      const destination=join(stage,file);mkdirSync(dirname(destination),{recursive:true});cpSync(source,destination,{recursive:true});
    }
    const gitDir=execFileSync("git",["rev-parse","--git-dir"],{cwd:root,encoding:"utf8"}).trim();
    if(gitDir)symlinkSync(resolve(root,gitDir),join(stage,".git"),"dir");
    symlinkSync(join(root,"node_modules"),join(stage,"node_modules"),"dir");
    project=stage;
  }
  run("bun",["scripts/collect-licenses.ts"],project);
  run("sh",["scripts/build-helper.sh"],project);
  run("bun",["x","mygo","build","-platform",`darwin/${target}`,"-o",join(root,"build.noindex")],project);
  run("bun",["scripts/verify-bundle.ts",`--arch=${target}`,join(root,"build.noindex",`darwin-${target}`,"RocketScience.app")],root);
  // Keep build artifacts out of launchers as well as Spotlight. The installed
  // copy is registered separately; never unregister /Applications here.
  const registrar="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister";
  if(existsSync(registrar))spawnSync(registrar,["-u",join(root,"build.noindex",`darwin-${target}`,"RocketScience.app")],{stdio:"ignore"});
}catch(error){
  console.error(error instanceof Error?error.message:String(error));process.exitCode=1;
}finally{
  if(stage)rmSync(stage,{recursive:true,force:true});
}
