import { chromium, type Locator, type Page } from "playwright-core";
import { spawn, spawnSync, type ChildProcess } from "node:child_process";
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, rmSync, statSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const root = resolve(import.meta.dir, "..");
const screensDir = join(root, "site/assets/screens");
const clipsDir = join(root, "site/assets/clips");
const docsAssetsDir = join(root, "docs/assets");
const evidenceDir = process.env.RS_EVIDENCE_DIR || "";
const baseUrl = "http://127.0.0.1:4173";
const chromePath = process.env.CHROME_PATH || "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome";

mkdirSync(screensDir, { recursive: true });
mkdirSync(clipsDir, { recursive: true });
mkdirSync(docsAssetsDir, { recursive: true });
if (evidenceDir) mkdirSync(evidenceDir, { recursive: true });

const workDir = mkdtempSync(join(tmpdir(), "rs-shots-"));

interface CropBox {
  x: number;
  y: number;
  width: number;
  height: number;
}

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
  const res = spawnSync("cwebp", ["-quiet", "-q", "85", "-resize", "2560", "0", pngPath, "-o", webpPath], {
    stdio: "inherit",
  });
  if (res.status !== 0) {
    throw new Error(`cwebp failed for ${pngPath}`);
  }
}

function encodeFramesToMp4AndPoster(
  framesDir: string,
  fps: number,
  targetWidth: number,
  crf: number,
  mp4Path: string,
  posterWebpPath: string,
  sharpFramePath?: string
) {
  const ff = spawnSync(
    "ffmpeg",
    [
      "-y",
      "-framerate",
      String(fps),
      "-i",
      join(framesDir, "%04d.png"),
      "-an",
      "-c:v",
      "libx264",
      "-pix_fmt",
      "yuv420p",
      "-crf",
      String(crf),
      "-preset",
      "slow",
      "-movflags",
      "+faststart",
      "-vf",
      `scale=${targetWidth}:-2:flags=lanczos`,
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
    ["-y", "-sseof", "-0.5", "-i", mp4Path, "-frames:v", "1", posterPng],
    { stdio: "pipe" }
  );
  if (pff.status !== 0 || !existsSync(posterPng)) {
    spawnSync("ffmpeg", ["-y", "-i", mp4Path, "-frames:v", "1", posterPng], { stdio: "pipe" });
  }
  toWebp(posterPng, posterWebpPath);
  if (sharpFramePath) {
    const midFrame = join(framesDir, "0020.png");
    const firstFrame = join(framesDir, "0001.png");
    copyFileSync(existsSync(midFrame) ? midFrame : firstFrame, sharpFramePath);
  }
}

function encodeFramesToAnimatedWebp(framesDir: string, fps: number, webpPath: string, sharpFramePath?: string) {
  const ff = spawnSync(
    "ffmpeg",
    [
      "-y",
      "-framerate",
      String(fps),
      "-i",
      join(framesDir, "%04d.png"),
      "-vf",
      "scale=1760:1100:flags=lanczos",
      "-c:v",
      "libwebp_anim",
      "-lossless",
      "0",
      "-q:v",
      "90",
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
  if (sharpFramePath) {
    const midFrame = join(framesDir, "0020.png");
    const firstFrame = join(framesDir, "0001.png");
    copyFileSync(existsSync(midFrame) ? midFrame : firstFrame, sharpFramePath);
  }
}

async function captureLoop(
  page: Page,
  framesDir: string,
  fps: number,
  clip: CropBox | undefined,
  drive: () => Promise<void>
): Promise<number> {
  mkdirSync(framesDir, { recursive: true });
  const intervalMs = Math.round(1000 / fps);
  let done = false;
  let count = 0;

  const loopPromise = (async () => {
    while (!done) {
      const start = Date.now();
      count++;
      const framePath = join(framesDir, `${String(count).padStart(4, "0")}.png`);
      try {
        await page.screenshot({ path: framePath, clip });
      } catch {
        break;
      }
      const elapsed = Date.now() - start;
      const wait = Math.max(0, intervalMs - elapsed);
      if (wait > 0 && !done) {
        await new Promise((r) => setTimeout(r, wait));
      }
    }
  })();

  try {
    await drive();
  } finally {
    done = true;
  }
  await loopPromise;
  return count;
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

  await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
  await waitUiReady(page);
  await snap(page, "overview", theme);
  const overviewPanel = page.locator(".hero > .panel").first();
  if (await overviewPanel.count()) {
    await snapLocator(overviewPanel, "overview-detail", theme);
  }

  const traj = page.locator("section.panel:has-text('连接轨迹')").first();
  if (await traj.count()) {
    await traj.scrollIntoViewIfNeeded();
    await page.waitForTimeout(300);
    await snap(page, "trajectory", theme);
    await snapLocator(traj, "trajectory-detail", theme);
    await page.evaluate(() => window.scrollTo(0, 0));
  }

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

  await clickSidebar(page, "规则");
  await snap(page, "rules", theme);
  const ovrPanel = page.locator(".ovr").first();
  if (await ovrPanel.count()) {
    await snapLocator(ovrPanel, "rules-detail", theme);
  }

  await clickSidebar(page, "连接");
  await snap(page, "connections", theme);

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

  await clickSidebar(page, "设置");
  await snap(page, "settings-proxy", theme);
  const settingsTopic = page.locator(".settings-topic:not([hidden]) .lf-set").first();
  if (await settingsTopic.count()) {
    await snapLocator(settingsTopic, "settings-proxy-detail", theme);
  }

  const tourBtn = page.locator("button:has-text('第一次使用')").first();
  if (await tourBtn.count()) {
    await tourBtn.click();
    await page.waitForTimeout(450);
    await snap(page, "onboarding", theme);
  }

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

async function runClipJob(
  browser: import("playwright-core").Browser,
  jobName: string,
  theme: "light" | "dark",
  viewport: { width: number; height: number },
  fps: number,
  targetWidth: number,
  crf: number,
  setup: (page: Page) => Promise<CropBox | undefined>,
  drive: (page: Page) => Promise<void>,
  outMp4Path: string,
  outPosterPath: string,
  sharpFramePath?: string
) {
  const framesDir = join(workDir, `frames-${jobName}`);
  const context = await browser.newContext({
    viewport,
    deviceScaleFactor: 2,
    colorScheme: theme,
  });
  await context.addInitScript((t) => {
    localStorage.setItem("rs-appearance", t);
    localStorage.setItem("rs-first-run-dismissed", "1");
  }, theme);
  const page = await context.newPage();
  page.on("dialog", (d) => d.accept());
  const clip = await setup(page);
  const count = await captureLoop(page, framesDir, fps, clip, () => drive(page));
  await context.close();
  encodeFramesToMp4AndPoster(framesDir, fps, targetWidth, crf, outMp4Path, outPosterPath, sharpFramePath);
  const mb = (statSync(outMp4Path).size / 1048576).toFixed(2);
  console.log(`Recorded ${jobName} (${count} frames, ${mb} MB)`);
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

    // 1. README animated WebP (DSF 2, 1280x800 -> 1760x1100, 10 fps, 5.6s)
    {
      const framesDir = join(workDir, "frames-readme-live");
      const context = await browser.newContext({
        viewport: { width: 1280, height: 800 },
        deviceScaleFactor: 2,
        colorScheme: "light",
      });
      await context.addInitScript(() => {
        localStorage.setItem("rs-appearance", "light");
        localStorage.setItem("rs-first-run-dismissed", "1");
      });
      const page = await context.newPage();
      await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
      await waitUiReady(page);
      await page.waitForTimeout(400);
      const count = await captureLoop(page, framesDir, 10, undefined, async () => {
        await page.waitForTimeout(5600);
      });
      await context.close();
      const animPath = join(docsAssetsDir, "overview-live.webp");
      encodeFramesToAnimatedWebp(framesDir, 10, animPath, evidenceDir ? join(evidenceDir, "rev1-sharp-readme-overview.png") : undefined);
      console.log(`Generated docs/assets/overview-live.webp (${count} frames, ${(statSync(animPath).size / 1048576).toFixed(2)} MB)`);
    }

    // 2. #interactions clips (DSF 2 screenshot loop, 1920x1200, crf 24)
    await runClipJob(
      browser,
      "onboarding",
      "dark",
      { width: 1280, height: 800 },
      12,
      1920,
      24,
      async (page) => {
        await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
        await waitUiReady(page);
        return undefined;
      },
      async (page) => {
        await page.waitForTimeout(400);
        await page.locator("button:has-text('第一次使用')").first().click();
        await page.waitForTimeout(1200);
        for (let i = 0; i < 3; i++) {
          const nextBtn = page.locator("button:has-text('下一步')").first();
          if (await nextBtn.count()) {
            await nextBtn.click();
            await page.waitForTimeout(1300);
          }
        }
        await page.waitForTimeout(1000);
      },
      join(clipsDir, "onboarding.mp4"),
      join(clipsDir, "onboarding-poster.webp"),
      evidenceDir ? join(evidenceDir, "rev1-sharp-onboarding.png") : undefined
    );

    await runClipJob(
      browser,
      "network-check",
      "dark",
      { width: 1280, height: 800 },
      12,
      1920,
      24,
      async (page) => {
        await page.goto(`${baseUrl}/?preview=checkRecommend`, { waitUntil: "networkidle" });
        await waitUiReady(page);
        return undefined;
      },
      async (page) => {
        await page.waitForTimeout(400);
        await page.locator("#tour-check button").first().click();
        await page.waitForTimeout(4200);
      },
      join(clipsDir, "network-check.mp4"),
      join(clipsDir, "network-check-poster.webp"),
      evidenceDir ? join(evidenceDir, "rev1-sharp-network-check.png") : undefined
    );

    await runClipJob(
      browser,
      "proxy-test",
      "dark",
      { width: 1280, height: 800 },
      12,
      1920,
      24,
      async (page) => {
        await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
        await waitUiReady(page);
        await clickSidebar(page, "代理");
        await page.waitForTimeout(300);
        return undefined;
      },
      async (page) => {
        await page.waitForTimeout(400);
        const testBtn = page.locator("button:has-text('测速')").first();
        if (await testBtn.count()) {
          await testBtn.click();
          await page.waitForTimeout(1600);
        }
        const nodeBtn = page.locator("button:has-text('日本 01')").first();
        if (await nodeBtn.count()) {
          await nodeBtn.click();
          await page.waitForTimeout(1400);
        }
        await page.waitForTimeout(800);
      },
      join(clipsDir, "proxy-test.mp4"),
      join(clipsDir, "proxy-test-poster.webp"),
      evidenceDir ? join(evidenceDir, "rev1-sharp-proxy-test.png") : undefined
    );

    // 3. 8 Feature clips in both light and dark, ALL 16:10, encoded at 1600px wide, crf 25
    for (const theme of ["light", "dark"] as const) {
      const sharp = (n: string) => (evidenceDir && theme === "light" ? join(evidenceDir, `rev1-sharp-${n}.png`) : undefined);

      // F1: Overview (hero deck 16:10)
      await runClipJob(
        browser,
        `feature-overview-${theme}`,
        theme,
        { width: 1024, height: 640 },
        12,
        1600,
        25,
        async (page) => {
          await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
          await waitUiReady(page);
          await page.waitForTimeout(300);
          return { x: 244, y: 66, width: 768, height: 480 };
        },
        async (page) => {
          await page.waitForTimeout(5500);
        },
        join(clipsDir, `feature-overview-${theme}.mp4`),
        join(clipsDir, `feature-overview-${theme}-poster.webp`),
        sharp("feature-overview")
      );

      // F2: Proxies (whole 代理 content column, latency test + sort/select)
      await runClipJob(
        browser,
        `feature-proxies-${theme}`,
        theme,
        { width: 1024, height: 640 },
        12,
        1600,
        25,
        async (page) => {
          await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
          await waitUiReady(page);
          await clickSidebar(page, "代理");
          await page.waitForTimeout(300);
          return { x: 244, y: 66, width: 768, height: 480 };
        },
        async (page) => {
          await page.waitForTimeout(500);
          const testBtn = page.locator("section.proxy-section[aria-label='主出口'] button:has-text('测速')").first();
          if (await testBtn.count()) {
            await testBtn.click();
            await page.waitForTimeout(2000);
          }
          const sortBtn = page.locator("section.proxy-section[aria-label='主出口'] button[aria-label*='排序'], section.proxy-section[aria-label='主出口'] button[title*='排序']").first();
          if (await sortBtn.count()) {
            await sortBtn.click();
            await page.waitForTimeout(1200);
          } else {
            const node2 = page.locator("section.proxy-section[aria-label='主出口'] button:has-text('日本 01')").first();
            if (await node2.count()) await node2.click();
            await page.waitForTimeout(1200);
          }
          await page.waitForTimeout(1000);
        },
        join(clipsDir, `feature-proxies-${theme}.mp4`),
        join(clipsDir, `feature-proxies-${theme}-poster.webp`),
        sharp("feature-proxies")
      );

      // F3: Trajectory (pulsing origin + zoom interaction + readable hop list)
      await runClipJob(
        browser,
        `feature-trajectory-${theme}`,
        theme,
        { width: 1024, height: 640 },
        12,
        1600,
        25,
        async (page) => {
          await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
          await waitUiReady(page);
          const traj = page.locator("section.panel:has-text('连接轨迹')").first();
          await traj.scrollIntoViewIfNeeded();
          await page.waitForTimeout(400);
          const box = await traj.boundingBox();
          const topY = box ? Math.max(0, Math.min(640 - 480, Math.floor(box.y - 8))) : 150;
          return { x: 244, y: topY, width: 768, height: 480 };
        },
        async (page) => {
          await page.waitForTimeout(1600);
          const zoomIn = page.locator("section.panel:has-text('连接轨迹') button[aria-label*='放大'], section.panel:has-text('连接轨迹') button:has-text('+')").first();
          if (await zoomIn.count()) {
            await zoomIn.click();
            await page.waitForTimeout(1500);
            const zoomOut = page.locator("section.panel:has-text('连接轨迹') button[aria-label*='缩小'], section.panel:has-text('连接轨迹') button:has-text('-')").first();
            if (await zoomOut.count()) {
              await zoomOut.click();
              await page.waitForTimeout(1500);
            }
          } else {
            await page.waitForTimeout(3200);
          }
        },
        join(clipsDir, `feature-trajectory-${theme}.mp4`),
        join(clipsDir, `feature-trajectory-${theme}-poster.webp`),
        sharp("feature-trajectory")
      );

      // F4: Network check (popover fixed & centered, no unrelated UI)
      await runClipJob(
        browser,
        `feature-network-check-${theme}`,
        theme,
        { width: 640, height: 400 },
        12,
        1600,
        25,
        async (page) => {
          await page.goto(`${baseUrl}/?preview=checkRecommend`, { waitUntil: "networkidle" });
          await waitUiReady(page);
          await page.locator("#tour-check button").first().click();
          await page.waitForSelector(".lf-chkpop__panel", { timeout: 5000 });
          await page.evaluate(() => {
            const main = document.querySelector("main") as HTMLElement | null;
            if (main) main.style.visibility = "hidden";
            const sb = document.querySelector(".lf-sidebar") as HTMLElement | null;
            if (sb) sb.style.background = "transparent";
            const panel = document.querySelector(".lf-chkpop__panel") as HTMLElement | null;
            if (panel) {
              panel.style.position = "fixed";
              panel.style.left = "50%";
              panel.style.top = "50%";
              panel.style.transform = "translate(-50%, -50%)";
              panel.style.margin = "0";
              panel.style.zIndex = "9999";
            }
          });
          return { x: 0, y: 0, width: 640, height: 400 };
        },
        async (page) => {
          await page.waitForTimeout(4800);
        },
        join(clipsDir, `feature-network-check-${theme}.mp4`),
        join(clipsDir, `feature-network-check-${theme}-poster.webp`),
        sharp("feature-network-check")
      );

      // F5: Profiles (profile list -> editor sheet opens fully inside frame)
      await runClipJob(
        browser,
        `feature-profiles-${theme}`,
        theme,
        { width: 1120, height: 700 },
        12,
        1600,
        25,
        async (page) => {
          await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
          await waitUiReady(page);
          await clickSidebar(page, "配置");
          await page.waitForTimeout(300);
          return { x: 224, y: 0, width: 896, height: 560 };
        },
        async (page) => {
          await page.waitForTimeout(800);
          const editBtn = page.locator("button:has-text('编辑')").first();
          if (await editBtn.count()) {
            await editBtn.click();
            await page.waitForTimeout(2200);
            const closeBtn = page.locator("button[aria-label*='关闭'], button:has-text('取消')").first();
            if (await closeBtn.count()) {
              await closeBtn.click();
              await page.waitForTimeout(1200);
            }
          }
        },
        join(clipsDir, `feature-profiles-${theme}.mp4`),
        join(clipsDir, `feature-profiles-${theme}-poster.webp`),
        sharp("feature-profiles")
      );

      // F6: Rules (group expands/toggles + template added, filled 16:10 frame)
      await runClipJob(
        browser,
        `feature-rules-${theme}`,
        theme,
        { width: 960, height: 600 },
        12,
        1600,
        25,
        async (page) => {
          await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
          await waitUiReady(page);
          await clickSidebar(page, "规则");
          await page.waitForTimeout(300);
          return { x: 244, y: 66, width: 704, height: 440 };
        },
        async (page) => {
          await page.waitForTimeout(700);
          const tplBtn = page.locator("button:has-text('局域网直连')").first();
          if (await tplBtn.count()) {
            await tplBtn.click();
            await page.waitForTimeout(1400);
          }
          const sw = page.locator("#rule-first-group .lf-switch, #rule-first-group [role='switch']").first();
          if (await sw.count()) {
            await sw.click();
            await page.waitForTimeout(1200);
            await sw.click();
            await page.waitForTimeout(1000);
          }
        },
        join(clipsDir, `feature-rules-${theme}.mp4`),
        join(clipsDir, `feature-rules-${theme}-poster.webp`),
        sharp("feature-rules")
      );

      // F7: Tray (16:10 centered clean frame, live speeds + mode switch)
      await runClipJob(
        browser,
        `feature-tray-${theme}`,
        theme,
        { width: 640, height: 400 },
        12,
        1600,
        25,
        async (page) => {
          await page.goto(`${baseUrl}/?panel=tray&preview=running`, { waitUntil: "networkidle" });
          await waitUiReady(page);
          await page.evaluate(() => {
            document.body.style.display = "grid";
            document.body.style.placeItems = "center";
            document.body.style.minHeight = "100vh";
            const tray = document.querySelector(".tray-page") as HTMLElement | null;
            if (tray) {
              tray.style.width = "320px";
              tray.style.maxHeight = "380px";
              tray.style.overflow = "hidden";
              tray.style.borderRadius = "12px";
              tray.style.boxShadow = "0 12px 32px rgba(0,0,0,0.18)";
            }
          });
          return { x: 0, y: 0, width: 640, height: 400 };
        },
        async (page) => {
          await page.waitForTimeout(1400);
          const globalBtn = page.locator("button:has-text('全局')").first();
          if (await globalBtn.count()) {
            await globalBtn.click();
            await page.waitForTimeout(1500);
            const ruleBtn = page.locator("button:has-text('规则')").first();
            if (await ruleBtn.count()) {
              await ruleBtn.click();
              await page.waitForTimeout(1300);
            }
          }
        },
        join(clipsDir, `feature-tray-${theme}.mp4`),
        join(clipsDir, `feature-tray-${theme}-poster.webp`),
        sharp("feature-tray")
      );

      // F8: Settings proxy (16:10 content area, switch toggle)
      await runClipJob(
        browser,
        `feature-settings-proxy-${theme}`,
        theme,
        { width: 960, height: 600 },
        12,
        1600,
        25,
        async (page) => {
          await page.goto(`${baseUrl}/?preview=running`, { waitUntil: "networkidle" });
          await waitUiReady(page);
          await clickSidebar(page, "设置");
          await page.waitForTimeout(300);
          return { x: 244, y: 66, width: 704, height: 440 };
        },
        async (page) => {
          await page.waitForTimeout(800);
          const sw = page.locator(".settings-topic:not([hidden]) .lf-set [role='switch'], .settings-topic:not([hidden]) .lf-set input[type='checkbox']").first();
          if (await sw.count()) {
            await sw.click();
            await page.waitForTimeout(1500);
            await sw.click();
            await page.waitForTimeout(1200);
          } else {
            await page.waitForTimeout(2800);
          }
        },
        join(clipsDir, `feature-settings-proxy-${theme}.mp4`),
        join(clipsDir, `feature-settings-proxy-${theme}-poster.webp`),
        sharp("feature-settings-proxy")
      );
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
