import { chromium, type Page } from "playwright-core";
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { existsSync, mkdirSync, mkdtempSync, rmSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const screensDir = join(root, "site/assets/screens");
const clipsDir = join(root, "site/assets/clips");
const baseUrl = "http://127.0.0.1:4173";
const chromePath = process.env.CHROME_PATH || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";

mkdirSync(screensDir, { recursive: true });
mkdirSync(clipsDir, { recursive: true });

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
    ["-y", "-ss", "00:00:02.0", "-i", mp4Path, "-vframes", "1", posterPng],
    { stdio: "pipe" }
  );
  if (pff.status !== 0 || !existsSync(posterPng)) {
    // Fallback to first frame
    spawnSync("ffmpeg", ["-y", "-i", mp4Path, "-vframes", "1", posterPng], { stdio: "pipe" });
  }
  toWebp(posterPng, posterWebpPath);
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

  // 1. Overview (running)
  await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
  await waitUiReady(page);
  await snap(page, "overview", theme);

  // 2. Trajectory view (scroll down on overview to show trajectory panel)
  const traj = page.locator("section.panel:has-text('连接轨迹')").first();
  if (await traj.count()) {
    await traj.scrollIntoViewIfNeeded();
    await page.waitForTimeout(300);
    await snap(page, "trajectory", theme);
    await page.evaluate(() => window.scrollTo(0, 0));
  }

  // 3. Proxies (after triggering a latency test)
  await clickSidebar(page, "代理");
  const testBtn = page.locator("button:has-text('测速')").first();
  if (await testBtn.count()) {
    await testBtn.click();
    await page.waitForTimeout(600);
  }
  await snap(page, "proxies", theme);

  // 4. Rules walkthrough
  await clickSidebar(page, "规则");
  await snap(page, "rules", theme);

  // 5. Connections
  await clickSidebar(page, "连接");
  await snap(page, "connections", theme);

  // 6. Profiles workspace + editor sheet
  await clickSidebar(page, "配置");
  await snap(page, "profiles", theme);
  const editBtn = page.locator("button:has-text('编辑')").first();
  if (await editBtn.count()) {
    await editBtn.click();
    await page.waitForTimeout(350);
    await snap(page, "profile-editor", theme);
    const closeBtn = page.locator("button[aria-label*='关闭'], button:has-text('取消')").first();
    if (await closeBtn.count()) await closeBtn.click();
    await page.waitForTimeout(250);
  }

  // 7. Settings -> system proxy / TUN page
  await clickSidebar(page, "设置");
  await snap(page, "settings-proxy", theme);

  // 8. Onboarding tour first step
  const tourBtn = page.locator("button:has-text('第一次使用')").first();
  if (await tourBtn.count()) {
    await tourBtn.click();
    await page.waitForTimeout(450);
    await snap(page, "onboarding", theme);
  }

  // 9. Network check (mid-flow and result)
  await page.goto(`${baseUrl}/?preview=checkRecommend`, { waitUntil: "networkidle" });
  await waitUiReady(page);
  const checkBtn = page.locator("#tour-check button").first();
  await checkBtn.click();
  await page.waitForTimeout(1100);
  await snap(page, "network-check-mid", theme);
  await page.waitForTimeout(2400);
  await snap(page, "network-check-result", theme);

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
      await page.waitForTimeout(800);
      await page.locator("button:has-text('第一次使用')").first().click();
      await page.waitForTimeout(1400);
      for (let i = 0; i < 3; i++) {
        const nextBtn = page.locator("button:has-text('下一步')").first();
        if (await nextBtn.count()) {
          await nextBtn.click();
          await page.waitForTimeout(1500);
        }
      }
      await page.waitForTimeout(800);
    });

    // Clip 2: Network check (~6.5s)
    await recordClip(browser, "network-check", async (page) => {
      await page.goto(`${baseUrl}/?preview=checkRecommend`, { waitUntil: "networkidle" });
      await waitUiReady(page);
      await page.waitForTimeout(800);
      await page.locator("#tour-check button").first().click();
      await page.waitForTimeout(4200);
    });

    // Clip 3: Proxy group latency test + node selection (~6.5s)
    await recordClip(browser, "proxy-test", async (page) => {
      await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
      await waitUiReady(page);
      await page.waitForTimeout(600);
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
        await page.waitForTimeout(1400);
      }
      await page.waitForTimeout(700);
    });
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
