import {expect,test} from "bun:test";
import {createTrayTasks} from "../src/trayTasks";

test("tray mutations have synchronous exclusive ownership",()=>{
  const tasks=createTrayTasks();
  const finish=tasks.beginAction();
  expect(finish).not.toBeNull();
  expect(tasks.beginAction()).toBeNull();
  expect(tasks.beginTest()).toBeNull();
  finish!();
  const next=tasks.beginAction();
  finish!(); // A late duplicate completion cannot release the next action.
  expect(tasks.beginAction()).toBeNull();
  next!();
  expect(tasks.beginAction()).not.toBeNull();
});

test("speed testing requires a live session and stays exclusive across groups",()=>{
  const tasks=createTrayTasks();
  expect(tasks.beginTest()).toBeNull();
  tasks.setSession("core-a/profile-a");
  const first=tasks.beginTest()!;
  expect(tasks.beginTest()).toBeNull();
  expect(first.current()).toBe(true);
  expect(first.finish()).toBe(true);
  expect(first.current()).toBe(false);
  expect(tasks.beginTest()).not.toBeNull();
});

test("unchanged refresh preserves a test; stop, restart, and profile changes reject it",()=>{
  const tasks=createTrayTasks();
  for(const next of ["", "core-b/profile-a", "core-b/profile-b"]){
    tasks.setSession("core-a/profile-a");
    const request=tasks.beginTest()!;
    expect(tasks.setSession("core-a/profile-a")).toBe(false);
    expect(request.current()).toBe(true);
    expect(tasks.setSession(next)).toBe(true);
    expect(request.current()).toBe(false);
    expect(request.finish()).toBe(false);
  }
});

test("an action cancels a test and its late completion cannot clear a newer test",()=>{
  const tasks=createTrayTasks();
  tasks.setSession("core-a/profile-a");
  const old=tasks.beginTest()!;
  const finish=tasks.beginAction()!;
  expect(old.current()).toBe(false);
  finish();
  const current=tasks.beginTest()!;
  expect(old.finish()).toBe(false);
  expect(current.current()).toBe(true);
  expect(current.finish()).toBe(true);
});

test("unmount cancellation rejects a result without blocking later tests",()=>{
  const tasks=createTrayTasks();
  tasks.setSession("core-a/profile-a");
  const request=tasks.beginTest()!;
  tasks.cancelTest();
  expect(request.finish()).toBe(false);
  expect(tasks.beginTest()).not.toBeNull();
});
