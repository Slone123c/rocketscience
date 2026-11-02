/** Stop the core and recover any interrupted takeover before presenting the tour. */
export async function prepareOnboarding(
  recover: () => Promise<void>,
  status: () => Promise<{ running: boolean; recoveryPending: boolean }>,
) {
  await recover();
  const current = await status();
  if (current.running || current.recoveryPending) throw new Error("网络恢复尚未完成");
  return current;
}

/** Advance only on an actual stopped-to-running transition at the launch step. */
export function shouldAdvanceAfterLaunch(open: boolean, step: number, wasRunning: boolean, running: boolean) {
  return open && step === 2 && !wasRunning && running;
}

/** Continue the pending start once a takeover is on; never toggle a core that is already running. */
export function takeoverGuideAction(open: boolean, busy: boolean, running: boolean, hasTakeover: boolean): "none" | "close" | "start" {
  if (!open || busy) return "none";
  if (running) return "close";
  return hasTakeover ? "start" : "none";
}
