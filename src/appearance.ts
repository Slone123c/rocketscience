import { useLayoutEffect, useState } from "react";

export type Appearance = "light" | "dark" | "system";

const STORAGE_KEY = "rs-appearance";

export function readAppearance(stored: string | null): Appearance {
  return stored === "light" || stored === "dark" || stored === "system" ? stored : "system";
}

export function themeName(appearance: Appearance, prefersDark: boolean): "paper" | "void" {
  if (appearance === "light") return "paper";
  if (appearance === "dark") return "void";
  return prefersDark ? "void" : "paper";
}

interface ThemeRoot {
  setAttribute(name: string, value: string): void;
}

interface ColorSchemeMedia {
  matches: boolean;
  addEventListener(type: "change", listener: () => void): void;
  removeEventListener(type: "change", listener: () => void): void;
}

export function applyAppearance(appearance: Appearance, root: ThemeRoot, media: ColorSchemeMedia): () => void {
  const paint = () => root.setAttribute("data-theme", themeName(appearance, media.matches));
  paint();
  if (appearance !== "system") return () => {};
  media.addEventListener("change", paint);
  return () => media.removeEventListener("change", paint);
}

export function useAppearance() {
  const [appearance, setAppearance] = useState<Appearance>(() => {
    const params = new URLSearchParams(location.search);
    const queryAppearance = params.has("preview") ? params.get("appearance") : null;
    return readAppearance(queryAppearance ?? localStorage.getItem(STORAGE_KEY));
  });
  useLayoutEffect(() => applyAppearance(appearance, document.documentElement, window.matchMedia("(prefers-color-scheme: dark)")), [appearance]);
  function choose(value: string) {
    if (value !== "light" && value !== "dark" && value !== "system") return;
    localStorage.setItem(STORAGE_KEY, value);
    setAppearance(value);
  }
  return { appearance, choose };
}
