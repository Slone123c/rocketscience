import {lstatSync, readdirSync, type Stats} from "node:fs";
import {join, resolve} from "node:path";

/** Walk a package tree without following symlinks or leaving the resolved root. */
export function safeWalk(root: string, visit: (absolutePath: string, stat: Stats) => void): void {
  const base = resolve(root);
  function step(directory: string) {
    let entries: ReturnType<typeof readdirSync>;
    try {
      entries = readdirSync(directory, {withFileTypes: true});
    } catch {
      return;
    }
    for (const entry of entries) {
      const absolute = join(directory, entry.name);
      let stat: Stats;
      try {
        stat = lstatSync(absolute);
      } catch {
        continue;
      }
      if (stat.isSymbolicLink()) {
        continue;
      }
      if (!absolute.startsWith(base)) {
        continue;
      }
      if (stat.isDirectory()) {
        step(absolute);
      } else if (stat.isFile()) {
        visit(absolute, stat);
      }
    }
  }
  step(base);
}

export function assertUnderBuildOutput(root: string, buildRoot: string) {
  const resolved = resolve(root);
  const allowed = resolve(buildRoot);
  if (resolved === allowed || resolved.startsWith(allowed + "/") || resolved.startsWith(allowed + "\\")) {
    return;
  }
  if (/[/\\]Applications[/\\]/i.test(resolved) && !resolved.includes(`${allowed}`)) {
    throw new Error(`扫描根目录不能指向已安装的 /Applications 应用：${resolved}`);
  }
}
