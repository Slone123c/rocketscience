import {describe,expect,test} from "bun:test";
import {navigateSettings,settingsTopic} from "../src/settingsNavigation";

describe("settings navigation",()=>{
  test("returning through the sidebar retains the selected topic",()=>{
    const advanced=navigateSettings("proxy","设置","core");
    const away=navigateSettings(advanced.topic,"代理");
    const back=navigateSettings(away.topic,"设置");
    expect(back).toEqual({page:"设置",topic:"core"});
  });
  test("guided takeover and virtual interface links override a remembered topic",()=>{
    const guided=navigateSettings("core","设置","proxy");
    expect(guided.topic).toBe("proxy");
    expect(navigateSettings(guided.topic,"设置","net")).toEqual({page:"设置",topic:"net"});
  });
  test("invalid persisted or linked topics fall back to usable proxy settings",()=>{
    for(const value of [null,"","removed-topic","__proto__"])expect(settingsTopic(value)).toBe("proxy");
    expect(navigateSettings("core","设置","removed-topic").topic).toBe("proxy");
    expect(navigateSettings("dns","日志","core")).toEqual({page:"日志",topic:"dns"});
  });
});
