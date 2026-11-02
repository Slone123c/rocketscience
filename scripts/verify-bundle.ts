import {execFileSync} from "node:child_process";
import {cpSync,existsSync,mkdtempSync, readFileSync,rmSync,statSync,writeFileSync} from "node:fs";
import {tmpdir} from "node:os";
import {createHash} from "node:crypto";
import {gunzipSync} from "node:zlib";
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

function unsignedCodeHash(path:string):string {
  const data=readFileSync(path);
  requireCondition(data.readUInt32LE(0)===0xfeedfacf,"mihomo 必须为单架构 64 位 Mach-O");
  const count=data.readUInt32LE(16);
  let offset=32;
  for(let i=0;i<count;i++){
    requireCondition(offset+8<=data.length,"Mach-O 加载命令损坏");
    const command=data.readUInt32LE(offset),size=data.readUInt32LE(offset+4);
    requireCondition(size>=8&&offset+size<=data.length,"Mach-O 加载命令长度无效");
    if(command===0x19&&size>=72&&data.toString("ascii",offset+8,offset+24).replace(/\0/g,"")==="__LINKEDIT"){
      // codesign can grow virtual reserve without changing the unsigned bytes.
      // Normalize only that reserve; preserve every actual data/code byte.
      data.writeBigUInt64LE(data.readBigUInt64LE(offset+48),offset+32);
    }
    offset+=size;
  }
  return createHash("sha256").update(data).digest("hex");
}

try {
  requireCondition(process.platform==="darwin","应用包检查需要在 macOS 运行");
  const args=process.argv.slice(2);
  const target=args.find(arg=>arg.startsWith("--arch="))?.slice(7)||(process.arch==="arm64"?"arm64":"amd64");
  requireCondition(target==="arm64"||target==="amd64","架构必须为 arm64 或 amd64");
  const app=resolve(args.find(arg=>!arg.startsWith("--"))||`build.noindex/darwin-${target}/RocketScience.app`);
  const contents=join(app,"Contents");
  const plist=JSON.parse(run("plutil",["-convert","json","-o","-",join(contents,"Info.plist")]));
  requireCondition(plist.CFBundleIdentifier==="app.rocketscience.desktop","应用标识不匹配");
  const minimum=String(plist.LSMinimumSystemVersion||"");
  versionParts(minimum);
  requireCondition(plist.NSLocationUsageDescription&&plist.NSLocationWhenInUseUsageDescription,"缺少 Wi-Fi 名称读取的权限说明");
  const resources=join(contents,"Resources");
  requireCondition(plist.CFBundleIconFile==="AppIcon.icns","应用缺少图标声明");
  const icon=readFileSync(join(resources,"AppIcon.icns"));
  requireCondition(icon.length>8&&icon.toString("ascii",0,4)==="icns"&&icon.readUInt32BE(4)===icon.length,"应用图标资源无效");
  const main=join(contents,"MacOS",plist.CFBundleExecutable);
  const nativeArch=target==="arm64"?"arm64":"x86_64";
  // TUN's default mixed stack fails at runtime without gVisor (see build-macos.ts).
  requireCondition(/-tags=\S*with_gvisor/.test(run("go",["version","-m",main])),"主程序未包含 gVisor，TUN 默认网络栈将无法启动");
  const binaries=[main,...["rs-system-proxy","rs-tun-helper","libRocketNetwork.dylib"].map(name=>join(resources,"bin",name))];
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
  for(const symbol of ["rs_wifi_snapshot","rs_wifi_free","rs_wifi_request_access","rs_app_icon","rs_tray_speeds"])
    requireCondition(new RegExp(`\\b_${symbol}\\b`).test(symbols),`网络模块缺少接口：${symbol}`);
  requireCondition(run("otool",["-D",library]).includes("@rpath/libRocketNetwork.dylib"),"网络模块保留了开发路径");
  const license=join(resources,"licenses","mihomo-GPL-3.0.txt");
  const geo=join(resources,"geoip.metadb");
  const geosite=join(resources,"geosite.dat");
  requireCondition(existsSync(geo)&&statSync(geo).size>1024*1024,"应用缺少 geoip.metadb");
  requireCondition(existsSync(geosite)&&statSync(geosite).size>1024*1024,"应用缺少 geosite.dat");
  requireCondition(existsSync(license)&&readFileSync(license,"utf8").includes("GNU GENERAL PUBLIC LICENSE"),"缺少 mihomo 许可证");
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
