import {describe, expect, test} from "bun:test";
import {readFileSync} from "node:fs";
import {screenComponents} from "../src/screenComponents";

const sources = [
  "src/OverviewDeck.tsx",
  "src/TrajectoryPanel.tsx",
  "src/ProxyList.tsx",
  "src/main.tsx",
  "src/ConnectionList.tsx",
  "src/ProfilesWorkspace.tsx",
  "src/SettingsPages.tsx",
  "src/NetworkCheck.tsx",
  "src/Onboarding.tsx",
  "src/TrayPanel.tsx",
  "src/DNSSettings.tsx",
  "src/TunSettings.tsx",
  "src/ProxyBypass.tsx",
  "src/WiFiSettings.tsx",
  "src/CoreSettings.tsx",
  "src/GeoSettings.tsx",
  "src/InboundSettings.tsx",
  "src/ControllerSettings.tsx",
  "src/LANSharing.tsx",
  "src/KernelSettings.tsx",
  "src/StartupSettings.tsx",
  "src/SpeedSettings.tsx",
].map(path => readFileSync(new URL("../" + path, import.meta.url), "utf8")).join("\n");

describe("screen component names", () => {
  test("each sidebar view uses the official component name", () => {
    const missing = Object.entries(screenComponents).flatMap(([view, names]) =>
      names.filter(name => !sources.includes("DS." + name)).map(name => view + " " + name));
    expect(missing).toEqual([]);
  });
});
