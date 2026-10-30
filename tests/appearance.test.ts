import { describe, expect, test } from "bun:test";
import { applyAppearance, readAppearance, themeName } from "../src/appearance";

describe("appearance", () => {
  test("an empty store follows the system", () => {
    expect(readAppearance(null)).toBe("system");
    expect(readAppearance("")).toBe("system");
    expect(readAppearance("void")).toBe("system");
  });

  test("light and dark map to the prototype theme names", () => {
    expect(themeName("light", true)).toBe("paper");
    expect(themeName("dark", false)).toBe("void");
    expect(themeName("system", true)).toBe("void");
    expect(themeName("system", false)).toBe("paper");
  });

  test("system keeps listening and a fixed choice does not", () => {
    const root = { theme: "", setAttribute(_name: string, value: string) { this.theme = value; } };
    let matches = false;
    const listeners = new Set<() => void>();
    const media = {
      get matches() { return matches; },
      addEventListener(_type: "change", listener: () => void) { listeners.add(listener); },
      removeEventListener(_type: "change", listener: () => void) { listeners.delete(listener); },
    };
    const stopSystem = applyAppearance("system", root, media);
    expect(root.theme).toBe("paper");
    matches = true;
    listeners.forEach(listener => listener());
    expect(root.theme).toBe("void");
    stopSystem();
    matches = false;
    listeners.forEach(listener => listener());
    expect(root.theme).toBe("void");

    const stopDark = applyAppearance("dark", root, media);
    expect(root.theme).toBe("void");
    expect(listeners.size).toBe(0);
    stopDark();
  });
});
