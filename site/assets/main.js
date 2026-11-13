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

  function applyTheme(theme) {
    root.setAttribute("data-theme", theme);
    const isDark = theme === "void";
    document.querySelectorAll("img[data-light][data-dark]").forEach((img) => {
      const target = isDark ? img.getAttribute("data-dark") : img.getAttribute("data-light");
      if (target && img.getAttribute("src") !== target) {
        img.setAttribute("src", target);
      }
    });
    const toggle = document.getElementById("theme-toggle");
    if (toggle) {
      const lightLabel = toggle.getAttribute("data-label-light") || "Light";
      const darkLabel = toggle.getAttribute("data-label-dark") || "Dark";
      toggle.textContent = isDark ? lightLabel : darkLabel;
      toggle.setAttribute("aria-pressed", String(isDark));
    }
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
