import {execFileSync} from "node:child_process";
import {existsSync, readFileSync, statSync} from "node:fs";
import {join, resolve} from "node:path";

function run(command:string, args:string[]):string {
  return execFileSync(command,args,{encoding:"utf8",stdio:["ignore","pipe","pipe"]}).trim();
}
function requireCondition(condition:unknown,message:string):asserts condition {
  if(!condition)throw new Error(message);
}
function versionParts(version:string):number[] {
  requireCondition(/^\d+(\.\d+){0,2}$/.test(version),`无效的系统版本：${version}`);
  return version.split(".").map(Number);
}
function exceeds(actual:string,declared:string):boolean {
  const a=versionParts(actual),d=versionParts(declared);
  for(let i=0;i<3;i++) {
    if((a[i]||0)!==(d[i]||0))return (a[i]||0)>(d[i]||0);
  }
  return false;
}

try {
  requireCondition(process.platform==="darwin","应用包检查需要在 macOS 运行");
  const target=process.arch==="arm64"?"arm64":"amd64";
  const app=resolve(process.argv[2]||`build/darwin-${target}/RocketScience.app`);
  const contents=join(app,"Contents");
  const plist=JSON.parse(run("plutil",["-convert","json","-o","-",join(contents,"Info.plist")]));
  requireCondition(plist.CFBundleIdentifier==="app.rocketscience.desktop","应用标识不匹配");
  const minimum=String(plist.LSMinimumSystemVersion||"");
  versionParts(minimum);
  requireCondition(plist.NSLocationUsageDescription&&plist.NSLocationWhenInUseUsageDescription,"缺少 Wi-Fi 名称读取的权限说明");
  const resources=join(contents,"Resources");
  const main=join(contents,"MacOS",plist.CFBundleExecutable);
  const nativeArch=target==="arm64"?"arm64":"x86_64";
  const binaries=[main,...["mihomo","rs-system-proxy","rs-tun-supervisor","libRocketNetwork.dylib"].map(name=>join(resources,"bin",name))];
  for(const file of binaries) {
    requireCondition(existsSync(file)&&statSync(file).isFile(),`应用缺少文件：${file}`);
    requireCondition((statSync(file).mode&0o111)!==0,`文件不可执行：${file}`);
    const archs=run("lipo",["-archs",file]).split(/\s+/);
    requireCondition(archs.includes(nativeArch),`文件不支持 ${nativeArch}：${file}`);
    const commands=run("otool",["-l",file]);
    const minimums=[...commands.matchAll(/\bminos\s+([\d.]+)/g)].map(match=>match[1]);
    for(const match of commands.matchAll(/LC_VERSION_MIN_MACOSX[\s\S]*?\bversion\s+([\d.]+)/g))minimums.push(match[1]);
    requireCondition(minimums.length>0,`无法判断最低系统版本：${file}`);
    requireCondition(minimums.every(actual=>!exceeds(actual,minimum)),`文件要求 macOS ${minimums.join("/")}，高于应用声明的 ${minimum}：${file}`);
    run("codesign",["--verify","--strict",file]);
  }
  const library=join(resources,"bin","libRocketNetwork.dylib");
  const symbols=run("nm",["-gU",library]);
  for(const symbol of ["rs_wifi_snapshot","rs_wifi_free","rs_wifi_request_access"])
    requireCondition(new RegExp(`\\b_${symbol}\\b`).test(symbols),`网络模块缺少接口：${symbol}`);
  requireCondition(run("otool",["-D",library]).includes("@rpath/libRocketNetwork.dylib"),"网络模块保留了开发路径");
  const license=join(resources,"licenses","mihomo-GPL-3.0.txt");
  requireCondition(existsSync(license)&&readFileSync(license,"utf8").includes("GNU GENERAL PUBLIC LICENSE"),"缺少 mihomo 许可证");
  const coreVersion=run(join(resources,"bin","mihomo"),["-v"]);
  requireCondition(coreVersion.startsWith("Mihomo ")&&coreVersion.includes("v1.19.33"),"应用内 mihomo 版本与固定版本不一致");
  run("codesign",["--verify","--deep","--strict",app]);
  const dmg=join(app,"..",`rocketscience-${plist.CFBundleShortVersionString}-${target==="arm64"?"arm64":"amd64"}.dmg`);
  requireCondition(existsSync(dmg),"缺少 DMG 安装镜像");
  run("hdiutil",["verify",dmg]);
  console.log(`应用包检查通过：${nativeArch} / macOS ${minimum}+；内核、辅助程序、网络模块、许可证、签名完整性与 DMG 校验通过。`);
  console.log("当前为本地开发签名；开发者签名、公证和旧系统实机运行尚未验证。");
}catch(error) {
  console.error(error instanceof Error?error.message:String(error));
  process.exitCode=1;
}
