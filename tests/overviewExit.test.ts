import {describe, expect, test} from "bun:test";
import {resolveOverviewExit} from "../src/overviewExit";
import type {Proxy} from "../src/mygo";

describe("overview exit resolution", () => {
  const mockGroups: Proxy[] = [
    {
      name: "PROXY",
      type: "Selector",
      now: "Auto",
      all: ["Auto", "HK01", "DIRECT"],
      rules: 10,
      fallback: false,
      nodes: [
        { name: "Auto", type: "URLTest", udp: true, delay: null, delayAt: "" },
        { name: "HK01", type: "ss", udp: true, delay: 42, delayAt: "" }
      ]
    },
    {
      name: "Auto",
      type: "URLTest",
      now: "HK01",
      all: ["HK01"],
      rules: 0,
      fallback: false,
      nodes: [
        { name: "HK01", type: "ss", udp: true, delay: 42, delayAt: "" }
      ]
    }
  ];

  test("resolves leaf node and delay for rule mode", () => {
    const result = resolveOverviewExit({
      groups: mockGroups,
      mode: "rule",
      flagOf: (name) => name === "HK01" ? { code: "HK", emoji: "🇭🇰" } : null
    });
    expect(result).not.toBeNull();
    expect(result?.nodeName).toBe("HK01");
    expect(result?.delay).toBe(42);
    expect(result?.text).toBe("🇭🇰 HK01 · 42 ms");
  });

  test("returns null in direct mode", () => {
    const result = resolveOverviewExit({
      groups: mockGroups,
      mode: "direct"
    });
    expect(result).toBeNull();
  });

  test("prefers manual delays over node recorded delays", () => {
    const result = resolveOverviewExit({
      groups: mockGroups,
      mode: "rule",
      delays: {
        PROXY: { values: { HK01: 28 }, completedAt: Date.now() }
      }
    });
    expect(result?.delay).toBe(28);
    expect(result?.text).toBe("HK01 · 28 ms");
  });

  test("gracefully omits delay when delay is unavailable or non-positive", () => {
    const groupsNoDelay: Proxy[] = [
      {
        name: "PROXY",
        type: "Selector",
        now: "JP01",
        all: ["JP01"],
        rules: 5,
        fallback: false,
        nodes: [{ name: "JP01", type: "ss", udp: true, delay: null, delayAt: "" }]
      }
    ];
    const result = resolveOverviewExit({
      groups: groupsNoDelay,
      mode: "rule",
      flagOf: () => ({ code: "JP", emoji: "🇯🇵" })
    });
    expect(result?.delay).toBeUndefined();
    expect(result?.text).toBe("🇯🇵 JP01");
  });
});

