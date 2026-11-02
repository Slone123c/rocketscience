import type {Proxy} from "./mygo";
import type {DelayMeasurement} from "./proxyNodes";
import {proxyLayout} from "./proxyLayout";

export interface OverviewExitOptions {
  groups: Proxy[];
  mode?: string;
  delays?: Record<string, DelayMeasurement>;
  flagOf?: (name: string) => { code: string; emoji: string } | null;
}

export interface OverviewExitResult {
  nodeName: string;
  delay?: number;
  text: string;
}

export function resolveOverviewExit({groups, mode = "rule", delays, flagOf}: OverviewExitOptions): OverviewExitResult | null {
  const normMode = mode.toLowerCase();
  if (normMode === "direct") {
    return null;
  }
  if (!groups || groups.length === 0) {
    return null;
  }
  const layout = proxyLayout(groups, normMode);
  const main = layout.main;
  if (!main) return null;

  const leaf = main.leaf || main.group.now || "";
  if (!leaf || leaf === "DIRECT" || leaf === "REJECT" || leaf === "REJECT-DROP") {
    return null;
  }

  // Look up manual delay or group node delay
  let manual = delays?.[main.group.name]?.values?.[leaf];
  if (manual === undefined && delays) {
    for (const groupName of Object.keys(delays)) {
      if (delays[groupName]?.values?.[leaf] !== undefined) {
        manual = delays[groupName].values[leaf];
        break;
      }
    }
  }

  let targetNode = main.group.nodes?.find(n => n.name === leaf);
  if (!targetNode) {
    for (const g of groups) {
      const found = g.nodes?.find(n => n.name === leaf);
      if (found) {
        targetNode = found;
        break;
      }
    }
  }

  const rawDelay = manual !== undefined ? manual : targetNode?.delay;
  const delay = typeof rawDelay === "number" && rawDelay > 0 ? rawDelay : undefined;

  // Format node with flag if available
  const flag = flagOf ? flagOf(leaf)?.emoji : null;
  const nodeWithFlag = flag ? `${flag} ${leaf}` : leaf;

  const text = delay !== undefined ? `${nodeWithFlag} · ${delay} ms` : nodeWithFlag;

  return { nodeName: leaf, delay, text };
}

