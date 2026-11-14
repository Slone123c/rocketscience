(() => {
  const root = document.documentElement;
  const mediaDark = window.matchMedia("(prefers-color-scheme: dark)");
  const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");
  const STORAGE_KEY = "rs-site-theme";

  function resolvedTheme() {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === "paper" || saved === "void") return saved;
    return mediaDark.matches ? "void" : "paper";
  }

  function isFallbackDemo() {
    return window.innerWidth < 760 || reducedMotion.matches;
  }

  function syncHeroDemo(theme) {
    const box = document.getElementById("hero-demo");
    const frame = document.getElementById("hero-demo-frame");
    if (!box || !(frame instanceof HTMLIFrameElement)) return;

    if (isFallbackDemo()) {
      box.classList.add("is-fallback");
      box.classList.remove("is-loaded");
      if (frame.getAttribute("src")) frame.removeAttribute("src");
      return;
    }

    box.classList.remove("is-fallback");
    const base = frame.getAttribute("data-demo-src") || "demo/?preview=running";
    const appearance = theme === "void" ? "dark" : "light";
    const nextUrl = `${base}${base.includes("?") ? "&" : "?"}appearance=${appearance}`;
    if (frame.getAttribute("src") !== nextUrl) {
      box.classList.remove("is-loaded");
      frame.setAttribute("src", nextUrl);
    }
  }

  function applyTheme(theme) {
    root.setAttribute("data-theme", theme);
    const isDark = theme === "void";
    document.querySelectorAll("img[data-light][data-dark]").forEach((img) => {
      const target = isDark ? img.getAttribute("data-dark") : img.getAttribute("data-light");
      if (target && img.getAttribute("src") !== target) {
        img.setAttribute("src", target);
      }
    });
    document.querySelectorAll("video[data-light-src][data-dark-src]").forEach((video) => {
      if (!(video instanceof HTMLVideoElement)) return;
      const targetSrc = isDark ? video.getAttribute("data-dark-src") : video.getAttribute("data-light-src");
      const targetPoster = isDark ? video.getAttribute("data-dark-poster") : video.getAttribute("data-light-poster");
      if (targetPoster && video.getAttribute("poster") !== targetPoster) {
        video.setAttribute("poster", targetPoster);
      }
      if (targetSrc && video.getAttribute("src") !== targetSrc) {
        const wasPlaying = !video.paused;
        video.setAttribute("src", targetSrc);
        video.load();
        if (wasPlaying && !reducedMotion.matches) {
          void video.play().catch(() => {});
        }
      }
    });
    syncHeroDemo(theme);
    const toggle = document.getElementById("theme-toggle");
    if (toggle) {
      const lightLabel = toggle.getAttribute("data-label-light") || "Light";
      const darkLabel = toggle.getAttribute("data-label-dark") || "Dark";
      toggle.textContent = isDark ? lightLabel : darkLabel;
      toggle.setAttribute("aria-pressed", String(isDark));
    }
  }

  const heroBox = document.getElementById("hero-demo");
  const heroFrame = document.getElementById("hero-demo-frame");
  if (heroBox && heroFrame instanceof HTMLIFrameElement) {
    heroFrame.addEventListener("load", () => {
      if (!isFallbackDemo() && heroFrame.getAttribute("src")) {
        heroBox.classList.add("is-loaded");
      }
    });
    const updateScale = () => {
      const w = heroBox.clientWidth;
      if (w > 0) {
        heroBox.style.setProperty("--demo-scale", String(w / 1280));
      }
    };
    updateScale();
    if ("ResizeObserver" in window) {
      new ResizeObserver(updateScale).observe(heroBox);
    }
    window.addEventListener("resize", () => {
      updateScale();
      syncHeroDemo(resolvedTheme());
    });
    reducedMotion.addEventListener("change", () => syncHeroDemo(resolvedTheme()));
  }

  applyTheme(resolvedTheme());

  mediaDark.addEventListener("change", () => {
    if (!localStorage.getItem(STORAGE_KEY)) {
      applyTheme(resolvedTheme());
    }
  });

  const toggleBtn = document.getElementById("theme-toggle");
  if (toggleBtn) {
    toggleBtn.addEventListener("click", () => {
      const next = resolvedTheme() === "void" ? "paper" : "void";
      localStorage.setItem(STORAGE_KEY, next);
      applyTheme(next);
    });
  }

  const videos = Array.from(document.querySelectorAll("video[data-clip]"));
  if (videos.length > 0) {
    if (reducedMotion.matches) {
      videos.forEach((video) => {
        video.removeAttribute("autoplay");
        video.pause();
        const btn = video.parentElement?.querySelector(".clip-card__play");
        if (btn) {
          btn.setAttribute("data-visible", "true");
          btn.addEventListener("click", () => {
            if (video.paused) {
              void video.play();
              btn.textContent = btn.getAttribute("data-pause-label") || "Pause";
            } else {
              video.pause();
              btn.textContent = btn.getAttribute("data-play-label") || "Play";
            }
          });
        }
      });
    } else if ("IntersectionObserver" in window) {
      const observer = new IntersectionObserver(
        (entries) => {
          entries.forEach((entry) => {
            const video = entry.target;
            if (!(video instanceof HTMLVideoElement)) return;
            if (entry.isIntersecting) {
              void video.play().catch(() => {});
            } else {
              video.pause();
            }
          });
        },
        { threshold: 0.35 }
      );
      videos.forEach((v) => observer.observe(v));
    }
  }
})();

document.querySelectorAll(".copy-btn").forEach((btn) => {
  btn.addEventListener("click", async () => {
    const code = btn.parentElement?.querySelector("code");
    if (!code) return;
    const orig = btn.textContent;
    const done = btn.getAttribute("data-copied") || "Copied";
    try {
      await navigator.clipboard.writeText(code.textContent || "");
      btn.textContent = done;
      setTimeout(() => { btn.textContent = orig; }, 1500);
    } catch {}
  });
});
