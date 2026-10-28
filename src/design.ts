import * as React from "react";
import * as ReactDOM from "react-dom";
import css from "../design-system/components/bundle.css?raw";
import tokens from "../design-system/tokens.json";
import type * as Design from "../design-system/components/index";

// The prototype ships as a browser bundle. Load it once against this apps React.
export async function loadDesign(): Promise<typeof Design> {
  const host = window as unknown as { React: typeof React; ReactDOM: typeof ReactDOM; RocketScience: typeof Design };
  host.React = React;
  host.ReactDOM = ReactDOM;
  const vars: string[] = [];
  for (const item of tokens.color.tokens) vars.push(`--${item.name}:${item.value.void}`);
  for (const item of tokens.spacing.tokens) vars.push(`--${item.name}:${item.value}`);
  for (const item of tokens.radius.tokens) vars.push(`--${item.name}:${item.value}`);
  vars.push(`--font-sans:${tokens.type.families.sans}`, `--font-mono:${tokens.type.families.mono}`, `--shadow-float:${tokens.shadow.tokens[0].value.void}`);
  const style = document.createElement("style");
  // Use system font fallbacks offline; do not fetch web fonts during startup.
  style.textContent = `:root{${vars.join(";")}}\n` + css.replace(/^@import[^\n]*\n/, "");
  document.head.appendChild(style);
  await import("../design-system/components/bundle.js");
  if (!host.RocketScience) throw new Error("无法加载设计组件");
  return host.RocketScience;
}
