import {expect,test} from "bun:test";
import {createGroupMeasurements} from "../src/groupMeasurements";

test("measurements require a running session and deduplicate one group",()=>{
  const jobs=createGroupMeasurements();expect(jobs.begin("A")).toBeNull();
  jobs.setSession("core-a");const a=jobs.begin("A")!;
  expect(jobs.begin("A")).toBeNull();expect(a.current()).toBe(true);
  expect(a.finish()).toBe(true);expect(jobs.begin("A")).not.toBeNull();
});
test("different groups retain independent indicators and may complete in either order",()=>{
  const jobs=createGroupMeasurements();jobs.setSession("core-a");
  const a=jobs.begin("A")!,b=jobs.begin("B")!;
  expect(a.finish()).toBe(true);expect(b.current()).toBe(true);
  expect(jobs.begin("A")).not.toBeNull();expect(b.finish()).toBe(true);
});
test("stopping or restarting discards pending results",()=>{
  const jobs=createGroupMeasurements();jobs.setSession("core-a");const a=jobs.begin("A")!;
  jobs.setSession("");expect(a.current()).toBe(false);expect(a.finish()).toBe(false);
  jobs.setSession("core-b");expect(jobs.begin("A")).not.toBeNull();
});
test("unchanged refresh preserves ownership, while leaving the page invalidates it",()=>{
  const jobs=createGroupMeasurements();jobs.setSession("core-a");const a=jobs.begin("A")!;
  expect(jobs.setSession("core-a")).toBe(false);expect(a.current()).toBe(true);
  jobs.cancel();expect(a.finish()).toBe(false);expect(jobs.begin("A")).not.toBeNull();
});
test("a late completion cannot erase a replacement request after an identical session returns",()=>{
  const jobs=createGroupMeasurements();jobs.setSession("core-a");const old=jobs.begin("A")!;
  jobs.setSession("");jobs.setSession("core-a");const current=jobs.begin("A")!;
  expect(old.finish()).toBe(false);expect(current.current()).toBe(true);expect(current.finish()).toBe(true);
});
