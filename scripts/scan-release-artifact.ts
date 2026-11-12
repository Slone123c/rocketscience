import {readFileSync} from "node:fs";
import {basename, join, resolve} from "node:path";
import {assertUnderBuildOutput, safeWalk} from "./fs-safe";

const root = resolve(import.meta.dir, "..");
const buildRoot = join(root, "build.noindex");
const args = process.argv.slice(2);
const target = resolve(args.find(arg => !arg.startsWith("--")) || buildRoot);

const privatePathFragments = [
  "design-system/",
  ".agents/",
  ".personal-explanation/",
  "plans/",
  "preview-telemetry",
];

const privateContentPatterns = [
  /design-system\/components\/bundle\.js/i,
  /rocket-science/i,
  /rocketscience/i,
  /00000000\+rocket-science@users\.noreply\.github\.com/,
];

try {
  assertUnderBuildOutput(target, buildRoot);
  let files = 0;
  let bytes = 0;
  const hits: string[] = [];
  safeWalk(target, (file, stat) => {
    files++;
    bytes += stat.size;
    const relative = file.slice(target.length + 1).replaceAll("\\", "/");
    for (const fragment of privatePathFragments) {
      if (relative.includes(fragment)) {
        hits.push(`${relative}: excluded path fragment ${fragment}`);
      }
    }
    if (stat.size > 8 * 1024 * 1024) {
      return;
    }
    const textLike = /\.(json|html|css|js|ts|tsx|md|txt|plist|desktop|xml)$/i.test(file);
    if (!textLike) {
      return;
    }
    try {
      const body = readFileSync(file, "utf8");
      for (const pattern of privateContentPatterns) {
        if (pattern.test(body)) {
          hits.push(`${relative}: content matched ${pattern}`);
        }
      }
    } catch {
      // binary or unreadable — skip content scan
    }
  });
  if (hits.length) {
    console.error("发布包扫描发现私有材料线索：\n" + hits.join("\n"));
    process.exitCode = 1;
  } else {
    console.log(`发布包扫描通过：${basename(target)}（${files} 个文件，约 ${(bytes / 1048576).toFixed(1)} MB）；未跟随符号链接。`);
  }
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
}
