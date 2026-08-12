// Hydrate deferred OGP/link cards and fade in media once loaded.

function markThumbLoaded(img) {
  if (!(img instanceof HTMLImageElement)) return;
  img.classList.add("is-loaded");
  const skel = img.parentElement?.querySelector(".article-linkcard-thumb-skel");
  if (skel) skel.remove();
}

function markArticleImageFailed(img) {
  if (!(img instanceof HTMLImageElement) || img.dataset.failed === "1") return;
  img.dataset.failed = "1";
  img.hidden = true;

  const message = document.createElement("span");
  message.className = "article-image-error";
  message.setAttribute("role", "status");
  message.textContent = img.alt
    ? `画像「${img.alt}」を表示できませんでした。`
    : "画像を表示できませんでした。";
  img.insertAdjacentElement("afterend", message);
}

function enhanceMedia(root = document) {
  root.querySelectorAll(".article-prose img").forEach((img) => {
    if (img.closest(".article-linkcard, .article-embed")) return;
    if (img.dataset.enhanced === "1") return;
    img.dataset.enhanced = "1";
    if (img.complete) {
      if (img.naturalWidth === 0) markArticleImageFailed(img);
      return;
    }
    img.addEventListener("error", () => markArticleImageFailed(img), { once: true });
  });

  root.querySelectorAll(".article-linkcard-thumb-img").forEach((img) => {
    if (img.dataset.enhanced === "1") return;
    img.dataset.enhanced = "1";
    if (img.complete && img.naturalWidth > 0) {
      markThumbLoaded(img);
      return;
    }
    img.addEventListener("load", () => markThumbLoaded(img), { once: true });
    img.addEventListener(
      "error",
      () => {
        img.classList.add("is-loaded");
        const skel = img.parentElement?.querySelector(".article-linkcard-thumb-skel");
        if (skel) skel.remove();
      },
      { once: true },
    );
  });

  root.querySelectorAll(".article-embed-frame-media").forEach((frame) => {
    if (frame.dataset.enhanced === "1") return;
    frame.dataset.enhanced = "1";
    const skel = frame.parentElement?.querySelector(".article-embed-frame-skel");
    const done = () => {
      frame.classList.add("is-loaded");
      if (skel) skel.remove();
    };
    frame.addEventListener("load", done, { once: true });
    // Some embeds never fire load reliably; clear skeleton after a short wait.
    window.setTimeout(done, 2500);
  });
}

async function hydratePendingCards(root = document) {
  const pending = [...root.querySelectorAll("figure[data-linkcard-url]")];
  if (pending.length === 0) return;

  const urls = [...new Set(pending.map((el) => el.getAttribute("data-linkcard-url")).filter(Boolean))];
  const byURL = new Map();

  for (let i = 0; i < urls.length; i += 20) {
    const chunk = urls.slice(i, i + 20);
    try {
      const res = await fetch("/api/linkcards", {
        method: "POST",
        headers: { "content-type": "application/json", accept: "application/json" },
        body: JSON.stringify({ urls: chunk }),
      });
      if (!res.ok) continue;
      const data = await res.json();
      for (const item of data.cards || []) {
        if (item && item.url && item.html) byURL.set(item.url, item.html);
      }
    } catch {
      // leave skeletons / noscript fallback
    }
  }

  pending.forEach((el) => {
    const url = el.getAttribute("data-linkcard-url");
    const html = byURL.get(url);
    if (!html) {
      el.removeAttribute("aria-busy");
      return;
    }
    const wrap = document.createElement("div");
    wrap.innerHTML = html.trim();
    const next = wrap.firstElementChild;
    if (!next) return;
    el.replaceWith(next);
    enhanceMedia(next);
  });
}

class ArticleLinkcards extends HTMLElement {
  connectedCallback() {
    if (this._mounted) return;
    this._mounted = true;
    const root = this.closest("article") || document;
    enhanceMedia(root);
    void hydratePendingCards(root);
  }
}

if (!customElements.get("article-linkcards")) {
  customElements.define("article-linkcards", ArticleLinkcards);
}

enhanceMedia(document);
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", () => enhanceMedia(document), { once: true });
}
