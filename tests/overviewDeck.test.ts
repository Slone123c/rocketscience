import {describe,expect,test} from "bun:test";
import {ignitionButtonState,modeLabel,takeoverNote,takeoverSummary} from "../src/OverviewDeck";

describe("overview flight copy",()=>{
  test("matches the prototype takeover phrases",()=>{
    expect(takeoverSummary(true,true)).toBe("系统代理 + TUN");
    expect(takeoverSummary(false,false)).toBe("未接管");
    expect(takeoverNote("idle",true)).toBe("");
    expect(takeoverNote("igniting",true)).toBe("正在接管…");
    expect(takeoverNote("orbit",true,"127.0.0.1:7890")).toBe("已接管 · 127.0.0.1:7890");
    expect(takeoverNote("orbit",false)).toBe("");
    expect(modeLabel("global")).toBe("全局");
    expect(modeLabel("direct")).toBe("直连");
    expect(modeLabel("rule")).toBe("规则");
    expect(modeLabel(undefined)).toBe("—");
  });

  test("a finished launch keeps the rocket in igniting until the hold ends", () => {
    expect(ignitionButtonState({ initialized: true, running: true, starting: false, stopping: false, hold: true })).toBe("igniting");
    expect(ignitionButtonState({ initialized: true, running: true, starting: false, stopping: false, hold: false })).toBe("orbit");
    expect(ignitionButtonState({ initialized: true, running: false, starting: true, stopping: false, hold: false })).toBe("igniting");
    expect(ignitionButtonState({ initialized: true, running: false, starting: false, stopping: false, hold: true })).toBe("idle");
    expect(ignitionButtonState({ initialized: true, running: true, starting: false, stopping: true, hold: false })).toBe("igniting");
  });
});
