// The popover has one mutation and one speed-test indicator. Keep ownership
// outside React state so two callbacks in the same event turn cannot overlap.
export function createTrayTasks() {
  let session = "", revision = 0, action = false;
  let test: object | null = null;
  function cancelTest() { test = null; revision++; }
  return {
    setSession(next: string) {
      if (next === session) return false;
      session = next; cancelTest(); return true;
    },
    cancelTest,
    beginAction() {
      if (action) return null;
      action = true; cancelTest();
      let finished = false;
      return () => { if (!finished) { finished = true; action = false; } };
    },
    beginTest() {
      if (action || test || !session) return null;
      const owner = {}, captured = revision;
      test = owner;
      const current = () => test === owner && revision === captured;
      return {
        current,
        finish() { if (!current()) return false; test = null; return true; }
      };
    }
  };
}
