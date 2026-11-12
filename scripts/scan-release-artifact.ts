import {existsSync,readFileSync} from "node:fs";
import {basename, join, resolve} from "node:path";
import {assertUnderBuildOutput, safeWalk} from "./fs-safe";

const root = resolve(import.meta.dir, "..");
const buildRoot = join(root, "build.noindex");
const args = process.argv.slice(2);
const target = resolve(args.find(arg => !arg.startsWith("--")) || buildRoot);

/** Paths that must not appear in public release trees (product-agnostic). */
const defaultPathFragments = [
  "design-system/",
  ".agents/",
  ".personal-explanation/",
  "plans/",
  "preview-telemetry",
];

/** Generic content heuristics; identity-specific strings belong in private config (see plans/002-scan-private-patterns.json). */
const defaultContentPatterns = [
  /design-system\/components\/bundle\.js/i,
  /\.personal-explanation\//i,
  /preview-telemetry\.html/i,
];

type ScanPrivateConfig = {
  pathFragments?: string[];
  contentPatterns?: string[];
};

function loadPrivateScanConfig(): ScanPrivateConfig {
  const candidates = [
    process.env.ROCKETSCIENCE_SCAN_PRIVATE_CONFIG,
    join(root, ".scan-private-patterns.json"),
  ].filter((p): p is string => typeof p === "string" && p.length > 0);
  for (const file of candidates) {
    if (!existsSync(file)) {
      continue;
    }
    try {
      return JSON.parse(readFileSync(file, "utf8")) as ScanPrivateConfig;
    } catch {
      console.warn(`忽略无效的扫描私密配置：${file}`);
    }
  }
  return {};
}

function compilePatterns(strings: string[]): RegExp[] {
  const out: RegExp[] = [];
  for (const raw of strings) {
    try {
      out.push(new RegExp(raw, "i"));
    } catch {
      console.warn(`忽略无效的正则：${raw}`);
    }
  }
  return out;
}

try {
  assertUnderBuildOutput(target, buildRoot);
  const extra = loadPrivateScanConfig();
  const privatePathFragments = [...defaultPathFragments, ...(extra.pathFragments ?? [])];
  const privateContentPatterns = [
    ...defaultContentPatterns,
    ...compilePatterns(extra.contentPatterns ?? []),
  ];

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
