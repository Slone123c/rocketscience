import { chromium, type Locator, type Page } from "playwright-core";
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, rmSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const screensDir = join(root, "site/assets/screens");
const clipsDir = join(root, "site/assets/clips");
const docsAssetsDir = join(root, "docs/assets");
const baseUrl = "http://127.0.0.1:4173";
const chromePath = process.env.CHROME_PATH || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";

mkdirSync(screensDir, { recursive: true });
mkdirSync(clipsDir, { recursive: true });
mkdirSync(docsAssetsDir, { recursive: true });

const workDir = mkdtempSync(join(tmpdir(), "rs-shots-"));

async function isServerUp(url: string): Promise<boolean> {
  try {
    const res = await fetch(url);
    return res.ok;
  } catch {
    return false;
  }
}

async function ensurePreviewServer(): Promise<ChildProcess | null> {
  if (await isServerUp(baseUrl)) {
    console.log("Reusing running preview server at", baseUrl);
    return null;
  }
  console.log("Starting Vite preview server...");
  const child = spawn("bun", ["run", "preview"], {
    cwd: root,
    stdio: ["ignore", "pipe", "pipe"],
    detached: false,
  });
  for (let i = 0; i < 60; i++) {
    if (await isServerUp(baseUrl)) return child;
    await new Promise((r) => setTimeout(r, 250));
  }
  child.kill("SIGTERM");
  throw new Error("Preview server did not start in time");
}

function toWebp(pngPath: string, webpPath: string) {
  const res = spawnSync("cwebp", ["-quiet", "-q", "82", "-resize", "2560", "0", pngPath, "-o", webpPath], {
    stdio: "inherit",
  });
  if (res.status !== 0) {
    throw new Error(`cwebp failed for ${pngPath}`);
  }
}

function toMp4AndPoster(webmPath: string, mp4Path: string, posterWebpPath: string) {
  const ff = spawnSync(
    "ffmpeg",
    [
      "-y",
      "-i",
      webmPath,
      "-an",
      "-c:v",
      "libx264",
      "-pix_fmt",
      "yuv420p",
      "-crf",
      "26",
      "-preset",
      "medium",
      "-movflags",
      "+faststart",
      "-vf",
      "scale=trunc(iw/2)*2:trunc(ih/2)*2",
      mp4Path,
    ],
    { stdio: "pipe" }
  );
  if (ff.status !== 0) {
    throw new Error(`ffmpeg mp4 failed: ${ff.stderr?.toString()}`);
  }
  const posterPng = join(workDir, "poster.png");
  const pff = spawnSync(
    "ffmpeg",
    ["-y", "-sseof", "-1", "-i", mp4Path, "-frames:v", "1", posterPng],
    { stdio: "pipe" }
  );
  if (pff.status !== 0 || !existsSync(posterPng)) {
    spawnSync("ffmpeg", ["-y", "-i", mp4Path, "-frames:v", "1", posterPng], { stdio: "pipe" });
  }
  toWebp(posterPng, posterWebpPath);
}


interface CropBox {
  x: number;
  y: number;
  width: number;
  height: number;
}

function toFeatureMp4AndPoster(
  webmPath: string,
  mp4Path: string,
  posterWebpPath: string,
  trimStartSec: number,
  durationSec: number,
  box?: CropBox
) {
  const vfParts: string[] = [];
  if (box) {
    const x = Math.max(0, Math.floor(box.x));
    const y = Math.max(0, Math.floor(box.y));
    const w = Math.max(2, Math.floor(box.width / 2) * 2);
    const h = Math.max(2, Math.floor(box.height / 2) * 2);
    vfParts.push(`crop=${w}:${h}:${x}:${y}`);
  }
  vfParts.push("scale='min(1280,iw)':-2");
  vfParts.push("scale=trunc(iw/2)*2:trunc(ih/2)*2");

  const ff = spawnSync(
    "ffmpeg",
    [
      "-y",
      "-ss",
      trimStartSec.toFixed(2),
      "-t",
      durationSec.toFixed(2),
      "-i",
      webmPath,
      "-an",
      "-c:v",
      "libx264",
      "-pix_fmt",
      "yuv420p",
      "-crf",
      "28",
      "-preset",
      "slow",
      "-movflags",
      "+faststart",
      "-vf",
      vfParts.join(","),
      mp4Path,
    ],
    { stdio: "pipe" }
  );
  if (ff.status !== 0) {
    throw new Error(`ffmpeg feature mp4 failed: ${ff.stderr?.toString()}`);
  }
  const posterPng = join(workDir, "feature-poster.png");
  const pff = spawnSync(
    "ffmpeg",
    ["-y", "-sseof", "-1", "-i", mp4Path, "-frames:v", "1", posterPng],
    { stdio: "pipe" }
  );
  if (pff.status !== 0 || !existsSync(posterPng)) {
    spawnSync("ffmpeg", ["-y", "-i", mp4Path, "-frames:v", "1", posterPng], { stdio: "pipe" });
  }
  toWebp(posterPng, posterWebpPath);
}

function toAnimatedWebp(webmPath: string, webpPath: string, trimStartSec: number, durationSec: number) {
  const ff = spawnSync(
    "ffmpeg",
    [
      "-y",
      "-ss",
      trimStartSec.toFixed(2),
      "-t",
      durationSec.toFixed(2),
      "-i",
      webmPath,
      "-vf",
      "fps=12,scale=960:-1:flags=lanczos",
      "-c:v",
      "libwebp_anim",
      "-lossless",
      "0",
      "-q:v",
      "75",
      "-loop",
      "0",
      "-an",
      webpPath,
    ],
    { stdio: "pipe" }
  );
  if (ff.status !== 0) {
    throw new Error(`ffmpeg animated webp failed: ${ff.stderr?.toString()}`);
  }
}

async function recordFeatureClip(
  browser: import("playwright-core").Browser,
  name: string,
  theme: "light" | "dark",
  viewport: { width: number; height: number },
  drive: (page: Page) => Promise<{ box?: CropBox; trimStartSec: number; durationSec: number }>
) {
  const videoDir = join(workDir, `feature-${name}-${theme}`);
  mkdirSync(videoDir, { recursive: true });
  const context = await browser.newContext({
    viewport,
    deviceScaleFactor: 1,
    colorScheme: theme,
    recordVideo: { dir: videoDir, size: viewport },
  });
  await context.addInitScript((t) => {
    localStorage.setItem("rs-appearance", t);
    localStorage.setItem("rs-first-run-dismissed", "1");
  }, theme);
  const page = await context.newPage();
  page.on("dialog", (d) => d.accept());
  const res = await drive(page);
  const videoPath = await page.video()?.path();
  await context.close();
  if (!videoPath || !existsSync(videoPath)) {
    throw new Error(`No recorded video for ${name}-${theme}`);
  }
  const mp4Path = join(clipsDir, `${name}-${theme}.mp4`);
  const posterPath = join(clipsDir, `${name}-${theme}-poster.webp`);
  toFeatureMp4AndPoster(videoPath, mp4Path, posterPath, res.trimStartSec, res.durationSec, res.box);
  const mb = (statSync(mp4Path).size / (1024 * 1024)).toFixed(2);
  console.log(`Recorded ${name}-${theme}.mp4 (${mb} MB) + poster`);
  return videoPath;
}

async function waitUiReady(page: Page) {
  await page.waitForSelector(".lf-sidebar, .tray-page", { timeout: 15000 });
  await page.waitForTimeout(350);
}

async function clickSidebar(page: Page, label: string) {
  const btn = page.locator(".lf-sidebar button, .lf-sidebar a").filter({ hasText: label }).first();
  await btn.click();
  await page.waitForTimeout(350);
}

async function snap(page: Page, name: string, theme: "light" | "dark") {
  const pngPath = join(workDir, `${name}-${theme}.png`);
  const webpPath = join(screensDir, `${name}-${theme}.webp`);
  await page.screenshot({ path: pngPath });
  toWebp(pngPath, webpPath);
  const sizeKb = (statSync(webpPath).size / 1024).toFixed(1);
  console.log(`Captured ${name}-${theme}.webp (${sizeKb} KB)`);
}

async function snapLocator(locator: Locator, name: string, theme: "light" | "dark") {
  await locator.scrollIntoViewIfNeeded();
  await locator.page().waitForTimeout(250);
  const pngPath = join(workDir, `${name}-${theme}.png`);
  const webpPath = join(screensDir, `${name}-${theme}.webp`);
  await locator.screenshot({ path: pngPath });
  toWebp(pngPath, webpPath);
  const sizeKb = (statSync(webpPath).size / 1024).toFixed(1);
  console.log(`Captured ${name}-${theme}.webp (${sizeKb} KB)`);
}

async function captureThemeScreens(browser: import("playwright-core").Browser, theme: "light" | "dark") {
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 2,
    colorScheme: theme,
  });
  await context.addInitScript((t) => {
    localStorage.setItem("rs-appearance", t);
    localStorage.setItem("rs-first-run-dismissed", "1");
  }, theme);

  const page = await context.newPage();
  page.on("dialog", (d) => d.accept());

  // 1. Overview (running) + overview-detail (gauges/chart panel)
  await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
  await waitUiReady(page);
  await snap(page, "overview", theme);
  const overviewPanel = page.locator(".hero > .panel").first();
  if (await overviewPanel.count()) {
    await snapLocator(overviewPanel, "overview-detail", theme);
  }

  // 2. Trajectory view (full + detail)
  const traj = page.locator("section.panel:has-text('连接轨迹')").first();
  if (await traj.count()) {
    await traj.scrollIntoViewIfNeeded();
    await page.waitForTimeout(300);
    await snap(page, "trajectory", theme);
    await snapLocator(traj, "trajectory-detail", theme);
    await page.evaluate(() => window.scrollTo(0, 0));
  }

  // 3. Proxies (after triggering a latency test) + proxies-detail (expanded main group card)
  await clickSidebar(page, "代理");
  const testBtn = page.locator("button:has-text('测速')").first();
  if (await testBtn.count()) {
    await testBtn.click();
    await page.waitForTimeout(600);
  }
  await snap(page, "proxies", theme);
  const proxyDetailCtx = await browser.newContext({
    viewport: { width: 760, height: 800 },
    deviceScaleFactor: 2,
    colorScheme: theme,
  });
  await proxyDetailCtx.addInitScript((t) => {
    localStorage.setItem("rs-appearance", t);
    localStorage.setItem("rs-first-run-dismissed", "1");
  }, theme);
  const proxyDetailPage = await proxyDetailCtx.newPage();
  await proxyDetailPage.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
  await waitUiReady(proxyDetailPage);
  await clickSidebar(proxyDetailPage, "代理");
  const pdTestBtn = proxyDetailPage.locator("button:has-text('测速')").first();
  if (await pdTestBtn.count()) {
    await pdTestBtn.click();
    await proxyDetailPage.waitForTimeout(600);
  }
  const mainGroup = proxyDetailPage.locator("section.proxy-section[aria-label='主出口']").first();
  if (await mainGroup.count()) {
    await snapLocator(mainGroup, "proxies-detail", theme);
  }
  await proxyDetailCtx.close();

  // 4. Rules walkthrough + rules-detail
  await clickSidebar(page, "规则");
  await snap(page, "rules", theme);
  const ovrPanel = page.locator(".ovr").first();
  if (await ovrPanel.count()) {
    await snapLocator(ovrPanel, "rules-detail", theme);
  }

  // 5. Connections
  await clickSidebar(page, "连接");
  await snap(page, "connections", theme);

  // 6. Profiles workspace + editor sheet + profiles-detail
  await clickSidebar(page, "配置");
  await snap(page, "profiles", theme);
  const editBtn = page.locator("button:has-text('编辑')").first();
  if (await editBtn.count()) {
    await editBtn.click();
    await page.waitForTimeout(350);
    await snap(page, "profile-editor", theme);
    const mainArea = page.locator("main").first();
    if (await mainArea.count()) {
      await snapLocator(mainArea, "profiles-detail", theme);
    }
    const closeBtn = page.locator("button[aria-label*='关闭'], button:has-text('取消')").first();
    if (await closeBtn.count()) await closeBtn.click();
    await page.waitForTimeout(250);
  }

  // 7. Settings -> system proxy / TUN page + settings-proxy-detail
  await clickSidebar(page, "设置");
  await snap(page, "settings-proxy", theme);
  const settingsTopic = page.locator(".settings-topic:not([hidden]) .lf-set").first();
  if (await settingsTopic.count()) {
    await snapLocator(settingsTopic, "settings-proxy-detail", theme);
  }

  // 8. Onboarding tour first step
  const tourBtn = page.locator("button:has-text('第一次使用')").first();
  if (await tourBtn.count()) {
    await tourBtn.click();
    await page.waitForTimeout(450);
    await snap(page, "onboarding", theme);
  }

  // 9. Network check (mid-flow, result, and network-check-detail)
  await page.goto(`${baseUrl}/?preview=checkRecommend`, { waitUntil: "networkidle" });
  await waitUiReady(page);
  const checkBtn = page.locator("#tour-check button").first();
  await checkBtn.click();
  await page.waitForTimeout(1100);
  await snap(page, "network-check-mid", theme);
  await page.waitForTimeout(2400);
  await snap(page, "network-check-result", theme);
  const checkPop = page.locator(".lf-chkpop__panel").first();
  if (await checkPop.count()) {
    await snapLocator(checkPop, "network-check-detail", theme);
  }

  // 10. Tray / menu-bar panel
  const trayCtx = await browser.newContext({
    viewport: { width: 320, height: 640 },
    deviceScaleFactor: 2,
    colorScheme: theme,
  });
  await trayCtx.addInitScript((t) => {
    localStorage.setItem("rs-appearance", t);
  }, theme);
  const trayPage = await trayCtx.newPage();
  await trayPage.goto(`${baseUrl}/?panel=tray&preview=running`, { waitUntil: "networkidle" });
  await waitUiReady(trayPage);
  await snap(trayPage, "tray", theme);
  await trayCtx.close();

  await context.close();
}

async function recordClip(
  browser: import("playwright-core").Browser,
  name: string,
  drive: (page: Page) => Promise<void>
) {
  const videoDir = join(workDir, `video-${name}`);
  mkdirSync(videoDir, { recursive: true });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
    colorScheme: "dark",
    recordVideo: { dir: videoDir, size: { width: 1280, height: 800 } },
  });
  await context.addInitScript(() => {
    localStorage.setItem("rs-appearance", "dark");
    localStorage.setItem("rs-first-run-dismissed", "1");
  });
  const page = await context.newPage();
  page.on("dialog", (d) => d.accept());
  await drive(page);
  const videoPath = await page.video()?.path();
  await context.close();
  if (!videoPath || !existsSync(videoPath)) {
    throw new Error(`No recorded video for ${name}`);
  }
  const mp4Path = join(clipsDir, `${name}.mp4`);
  const posterPath = join(clipsDir, `${name}-poster.webp`);
  toMp4AndPoster(videoPath, mp4Path, posterPath);
  const mb = (statSync(mp4Path).size / (1024 * 1024)).toFixed(2);
  console.log(`Recorded ${name}.mp4 (${mb} MB) + poster`);
}

async function main() {
  const server = await ensurePreviewServer();
  const browser = await chromium.launch({
    executablePath: chromePath,
    headless: true,
    args: ["--no-sandbox", "--disable-gpu-sandbox"],
  });

  try {
    await captureThemeScreens(browser, "light");
    await captureThemeScreens(browser, "dark");

    // Clip 1: Onboarding tour (~7s)
    await recordClip(browser, "onboarding", async (page) => {
      await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
      await waitUiReady(page);
      await page.waitForTimeout(600);
      await page.locator("button:has-text('第一次使用')").first().click();
      await page.waitForTimeout(1400);
      for (let i = 0; i < 3; i++) {
        const nextBtn = page.locator("button:has-text('下一步')").first();
        if (await nextBtn.count()) {
          await nextBtn.click();
          await page.waitForTimeout(1500);
        }
      }
      await page.waitForTimeout(1200);
    });

    // Clip 2: Network check (~6.5s)
    await recordClip(browser, "network-check", async (page) => {
      await page.goto(`${baseUrl}/?preview=checkRecommend`, { waitUntil: "networkidle" });
      await waitUiReady(page);
      await page.waitForTimeout(600);
      await page.locator("#tour-check button").first().click();
      await page.waitForTimeout(4500);
    });

    // Clip 3: Proxy group latency test + node selection (~6.5s)
    await recordClip(browser, "proxy-test", async (page) => {
      await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
      await waitUiReady(page);
      await page.waitForTimeout(500);
      await clickSidebar(page, "代理");
      await page.waitForTimeout(700);
      const testBtn = page.locator("button:has-text('测速')").first();
      if (await testBtn.count()) {
        await testBtn.click();
        await page.waitForTimeout(1600);
      }
      const nodeBtn = page.locator("button:has-text('新加坡'), button:has-text('日本'), button:has-text('香港')").first();
      if (await nodeBtn.count()) {
        await nodeBtn.click();
        await page.waitForTimeout(1600);
      }
      await page.waitForTimeout(800);
    });

    for (const theme of ["light", "dark"] as const) {
      // Feature 1: Overview gauges & chart (also used for README animated WebP on light)
      {
        const videoDir = join(workDir, `readme-overview-${theme}`);
        mkdirSync(videoDir, { recursive: true });
        const context = await browser.newContext({
          viewport: { width: 1280, height: 800 },
          deviceScaleFactor: 1,
          colorScheme: theme,
          recordVideo: { dir: videoDir, size: { width: 1280, height: 800 } },
        });
        await context.addInitScript((t) => {
          localStorage.setItem("rs-appearance", t);
          localStorage.setItem("rs-first-run-dismissed", "1");
        }, theme);
        const page = await context.newPage();
        const t0 = Date.now();
        await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
        await waitUiReady(page);
        await page.waitForTimeout(800);
        const trimStartSec = (Date.now() - t0) / 1000;
        const box = (await page.locator(".hero > .panel").first().boundingBox()) ?? undefined;
        await page.waitForTimeout(5500);
        const durationSec = (Date.now() - t0) / 1000 - trimStartSec;
        const videoPath = await page.video()?.path();
        await context.close();
        if (!videoPath || !existsSync(videoPath)) throw new Error("No overview video");
        const mp4Path = join(clipsDir, `feature-overview-${theme}.mp4`);
        const posterPath = join(clipsDir, `feature-overview-${theme}-poster.webp`);
        toFeatureMp4AndPoster(videoPath, mp4Path, posterPath, trimStartSec, durationSec, box);
        console.log(`Recorded feature-overview-${theme}.mp4 (${(statSync(mp4Path).size / 1048576).toFixed(2)} MB) + poster`);
        if (theme === "light") {
          const animPath = join(docsAssetsDir, "overview-live.webp");
          toAnimatedWebp(videoPath, animPath, trimStartSec, 6.0);
          console.log(`Generated docs/assets/overview-live.webp (${(statSync(animPath).size / 1048576).toFixed(2)} MB)`);
        }
      }

      // Feature 2: Proxies latency test & node switch
      await recordFeatureClip(browser, "feature-proxies", theme, { width: 1280, height: 800 }, async (page) => {
        const t0 = Date.now();
        await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
        await waitUiReady(page);
        await clickSidebar(page, "代理");
        await page.waitForTimeout(400);
        const trimStartSec = (Date.now() - t0) / 1000;
        const section = page.locator("section.proxy-section[aria-label='主出口']").first();
        const box = (await section.boundingBox()) ?? undefined;
        await page.waitForTimeout(600);
        const testBtn = section.locator("button:has-text('测速')").first();
        if (await testBtn.count()) {
          await testBtn.click();
          await page.waitForTimeout(1600);
        }
        const node2 = section.locator("button:has-text('日本 01')").first();
        if (await node2.count()) {
          await node2.click();
          await page.waitForTimeout(1400);
        }
        const node1 = section.locator("button:has-text('美国 02')").first();
        if (await node1.count()) {
          await node1.click();
          await page.waitForTimeout(1000);
        }
        return { box, trimStartSec, durationSec: (Date.now() - t0) / 1000 - trimStartSec };
      });

      // Feature 3: Trajectory map pulsing origin & path
      await recordFeatureClip(browser, "feature-trajectory", theme, { width: 1280, height: 800 }, async (page) => {
        const t0 = Date.now();
        await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
        await waitUiReady(page);
        const traj = page.locator("section.panel:has-text('连接轨迹')").first();
        await traj.scrollIntoViewIfNeeded();
        await page.waitForTimeout(500);
        const trimStartSec = (Date.now() - t0) / 1000;
        const box = (await traj.boundingBox()) ?? undefined;
        await page.waitForTimeout(5000);
        return { box, trimStartSec, durationSec: (Date.now() - t0) / 1000 - trimStartSec };
      });

      // Feature 4: Network check steps resolving
      await recordFeatureClip(browser, "feature-network-check", theme, { width: 1280, height: 800 }, async (page) => {
        const t0 = Date.now();
        await page.goto(`${baseUrl}/?preview=checkRecommend`, { waitUntil: "networkidle" });
        await waitUiReady(page);
        await page.locator("#tour-check button").first().click();
        await page.waitForSelector(".lf-chkpop__panel", { timeout: 5000 });
        await page.waitForTimeout(150);
        const trimStartSec = (Date.now() - t0) / 1000;
        const box = (await page.locator(".lf-chkpop__panel").first().boundingBox()) ?? undefined;
        await page.waitForTimeout(4500);
        return { box, trimStartSec, durationSec: (Date.now() - t0) / 1000 - trimStartSec };
      });

      // Feature 5: Profiles editor sheet opening & closing
      await recordFeatureClip(browser, "feature-profiles", theme, { width: 1280, height: 800 }, async (page) => {
        const t0 = Date.now();
        await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
        await waitUiReady(page);
        await clickSidebar(page, "配置");
        await page.waitForTimeout(350);
        const trimStartSec = (Date.now() - t0) / 1000;
        const box = (await page.locator("main").first().boundingBox()) ?? undefined;
        await page.waitForTimeout(900);
        const editBtn = page.locator("button:has-text('编辑')").first();
        if (await editBtn.count()) {
          await editBtn.click();
          await page.waitForTimeout(2000);
          const closeBtn = page.locator("button[aria-label*='关闭'], button:has-text('取消')").first();
          if (await closeBtn.count()) {
            await closeBtn.click();
            await page.waitForTimeout(1100);
          }
        }
        return { box, trimStartSec, durationSec: (Date.now() - t0) / 1000 - trimStartSec };
      });

      // Feature 6: Rules overrides toggling a group
      await recordFeatureClip(browser, "feature-rules", theme, { width: 1280, height: 800 }, async (page) => {
        const t0 = Date.now();
        await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
        await waitUiReady(page);
        await clickSidebar(page, "规则");
        await page.waitForTimeout(350);
        const trimStartSec = (Date.now() - t0) / 1000;
        const ovr = page.locator(".ovr").first();
        const box = (await ovr.boundingBox()) ?? undefined;
        await page.waitForTimeout(900);
        const sw = ovr.locator("[role='switch'], input[type='checkbox'], button").first();
        if (await sw.count()) {
          await sw.click();
          await page.waitForTimeout(1600);
          await sw.click();
          await page.waitForTimeout(1200);
        } else {
          await page.waitForTimeout(2800);
        }
        return { box, trimStartSec, durationSec: (Date.now() - t0) / 1000 - trimStartSec };
      });

      // Feature 7: Tray panel live speeds & mode switch
      await recordFeatureClip(browser, "feature-tray", theme, { width: 320, height: 640 }, async (page) => {
        const t0 = Date.now();
        await page.goto(`${baseUrl}/?panel=tray&preview=running`, { waitUntil: "networkidle" });
        await waitUiReady(page);
        await page.waitForTimeout(400);
        const trimStartSec = (Date.now() - t0) / 1000;
        await page.waitForTimeout(1400);
        const globalBtn = page.locator("button:has-text('全局')").first();
        if (await globalBtn.count()) {
          await globalBtn.click();
          await page.waitForTimeout(1400);
          const ruleBtn = page.locator("button:has-text('规则')").first();
          if (await ruleBtn.count()) {
            await ruleBtn.click();
            await page.waitForTimeout(1400);
          }
        }
        return { trimStartSec, durationSec: (Date.now() - t0) / 1000 - trimStartSec };
      });

      // Feature 8: Settings proxy toggle
      await recordFeatureClip(browser, "feature-settings-proxy", theme, { width: 1280, height: 800 }, async (page) => {
        const t0 = Date.now();
        await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
        await waitUiReady(page);
        await clickSidebar(page, "设置");
        await page.waitForTimeout(350);
        const trimStartSec = (Date.now() - t0) / 1000;
        const topic = page.locator(".settings-topic:not([hidden]) .lf-set").first();
        const box = (await topic.boundingBox()) ?? undefined;
        await page.waitForTimeout(900);
        const sw = topic.locator("[role='switch'], input[type='checkbox']").first();
        if (await sw.count()) {
          await sw.click();
          await page.waitForTimeout(1500);
          await sw.click();
          await page.waitForTimeout(1200);
        } else {
          await page.waitForTimeout(2700);
        }
        return { box, trimStartSec, durationSec: (Date.now() - t0) / 1000 - trimStartSec };
      });
    }
  } finally {
    await browser.close();
    if (server) {
      server.kill("SIGTERM");
    }
    rmSync(workDir, { recursive: true, force: true });
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
