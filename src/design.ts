import * as React from "react";
import * as ReactDOM from "react-dom";
import css from "../design-system/components/bundle.css?raw";
import tokens from "../design-system/tokens.json";
import type * as Design from "../design-system/components/index";
import { readAppearance, themeName } from "./appearance";

// The prototype ships as a browser bundle. Load it once against this apps React.
export async function loadDesign(): Promise<typeof Design> {
  const host = window as unknown as { React: typeof React; ReactDOM: typeof ReactDOM; RocketScience: typeof Design };
  host.React = React;
  host.ReactDOM = ReactDOM;
  const shared: string[] = [];
  for (const item of tokens.spacing.tokens) shared.push(`--${item.name}:${item.value}`);
  for (const item of tokens.radius.tokens) shared.push(`--${item.name}:${item.value}`);
  shared.push(`--font-sans:${tokens.type.families.sans}`, `--font-mono:${tokens.type.families.mono}`);
  const themeVars = (theme: "void" | "paper") => {
    const vars = tokens.color.tokens.map(item => `--${item.name}:${item.value[theme]}`);
    vars.push(`--shadow-float:${tokens.shadow.tokens[0].value[theme]}`, `color-scheme:${theme === "void" ? "dark" : "light"}`);
    return vars.join(";");
  };
  const stored = readAppearance(localStorage.getItem("rs-appearance"));
  document.documentElement.setAttribute("data-theme", themeName(stored, window.matchMedia("(prefers-color-scheme: dark)").matches));
  const style = document.createElement("style");
  // Use system font fallbacks offline; do not fetch web fonts during startup.
  style.textContent = `:root{${shared.join(";")}}\n:root,[data-theme="void"]{${themeVars("void")}}\n[data-theme="paper"]{${themeVars("paper")}}\n` + css.replace(/^@import[^\n]*\n/, "");
  document.head.appendChild(style);
  await import("../design-system/components/bundle.js");
  if (!host.RocketScience) throw new Error("无法加载设计组件");
  return host.RocketScience;
}
