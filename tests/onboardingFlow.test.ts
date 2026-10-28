import { describe, expect, test } from "bun:test";
import { prepareOnboarding, shouldAdvanceAfterLaunch } from "../src/onboardingFlow";

describe("new user tour lifecycle", () => {
  test("recovers before checking state and preserves the selected takeover preference", async () => {
    const state = { running: true, recoveryPending: true, systemProxy: true };
    const events: string[] = [];
    await prepareOnboarding(async () => { events.push("recover"); state.running = false; state.recoveryPending = false; }, async () => { events.push("status"); return state; });
    expect(events).toEqual(["recover", "status"]);
    expect(state.systemProxy).toBe(true);
  });
  test("does not start the tour if recovery fails or remains incomplete", async () => {
    let checked = false;
    await expect(prepareOnboarding(async () => { throw new Error("restore failed"); }, async () => { checked = true; return { running: false, recoveryPending: false }; })).rejects.toThrow("restore failed");
    expect(checked).toBe(false);
    for (const state of [{ running: true, recoveryPending: false }, { running: false, recoveryPending: true }]) {
      await expect(prepareOnboarding(async () => {}, async () => state)).rejects.toThrow("网络恢复尚未完成");
    }
  });
  test("advances only when launch succeeds while the launch step is visible", () => {
    expect(shouldAdvanceAfterLaunch(true, 3, false, true)).toBe(true);
    expect(shouldAdvanceAfterLaunch(true, 3, false, false)).toBe(false);
    expect(shouldAdvanceAfterLaunch(true, 3, true, true)).toBe(false);
    expect(shouldAdvanceAfterLaunch(false, 3, false, true)).toBe(false);
    expect(shouldAdvanceAfterLaunch(true, 2, false, true)).toBe(false);
  });
});
