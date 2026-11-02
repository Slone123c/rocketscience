import { expect, test } from "bun:test";
import { installErrorDisplay } from "../src/errorDisplay";

test("errors display only their message", () => {
  const original = Error.prototype.toString;
  installErrorDisplay();
  try {
    const call = new Error("内核未启动");
    call.name = "CallError";
    expect(String(call)).toBe("内核未启动");
    expect(String(new TypeError("bad"))).toBe("bad");
    expect(String(new Error(""))).toBe("Error");
  } finally {
    Error.prototype.toString = original;
  }
});
