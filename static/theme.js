(() => {
  const KEY = "unigo-theme";
  const root = document.documentElement;
  const light = root.getAttribute("data-theme-light") || "light";
  const dark = root.getAttribute("data-theme-dark") || "dark";

  function preferred() {
    const stored = localStorage.getItem(KEY);
    if (stored === light || stored === dark) {
      return stored;
    }
    return matchMedia("(prefers-color-scheme: dark)").matches ? dark : light;
  }

  function apply(theme) {
    root.setAttribute("data-theme", theme);
    for (const el of document.querySelectorAll("input.theme-controller")) {
      el.checked = theme === el.value;
    }
  }

  apply(preferred());
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", () => apply(preferred()), { once: true });
  }

  document.addEventListener("change", (event) => {
    const el = event.target;
    if (!(el instanceof HTMLInputElement) || !el.classList.contains("theme-controller")) {
      return;
    }
    const theme = el.checked ? el.value : light;
    localStorage.setItem(KEY, theme);
    root.setAttribute("data-theme", theme);
  });
})();
