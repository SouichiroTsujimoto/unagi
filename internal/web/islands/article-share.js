import { html } from "htm/preact";
import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import register from "preact-custom-element";

const IDLE_LABEL = "共有";
const BUTTON_LABEL = "記事を共有";
const FEEDBACK_MS = 1200;

function prefersNativeShare() {
  return window.matchMedia("(hover: none) and (pointer: coarse)").matches;
}

async function copyURL(url) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(url);
    return;
  }
  const ta = document.createElement("textarea");
  ta.value = url;
  ta.setAttribute("readonly", "");
  ta.style.position = "fixed";
  ta.style.opacity = "0";
  document.body.appendChild(ta);
  ta.select();
  const ok = document.execCommand("copy");
  ta.remove();
  if (!ok) throw new Error("copy failed");
}

/** SSR wrap/button/icon → braille vnode only. */
function brailleFromChildren(children) {
  const items = children == null ? [] : Array.isArray(children) ? children : [children];
  const first = items.find(Boolean);
  if (!first || typeof first !== "object" || !first.props) return children;

  const className = String(first.props.class || first.props.className || "");
  if (className.includes("article-share-wrap") || className.includes("article-share-tip")) {
    const kids = first.props.children;
    const list = kids == null ? [] : Array.isArray(kids) ? kids : [kids];
    const btn = list.find((node) => node && node.type === "button");
    return btn?.props?.children ?? null;
  }
  if (first.type === "button") {
    return first.props.children ?? null;
  }
  return children;
}

function ArticleShare({ title, url, text, children }) {
  const btnRef = useRef(null);
  const timerRef = useRef(0);

  // feedback wins over hover. hoverAllowed stays false after click until pointerleave.
  const [feedback, setFeedback] = useState(null); // { text, error } | null
  const [hovering, setHovering] = useState(false);
  const [hoverAllowed, setHoverAllowed] = useState(true);
  const icon = brailleFromChildren(children);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const showFeedback = useCallback((message, isError = false) => {
    if (timerRef.current) clearTimeout(timerRef.current);
    setFeedback({ text: message, error: isError });
    btnRef.current?.blur();
    timerRef.current = window.setTimeout(() => {
      setFeedback(null);
      timerRef.current = 0;
    }, FEEDBACK_MS);
  }, []);

  const share = useCallback(async () => {
    setHoverAllowed(false);
    setHovering(false);
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = 0;
    setFeedback(null);

    const shareTitle = title || document.title;
    const shareText = text || "";
    const shareUrl = url || window.location.href;
    const payload = { title: shareTitle, text: shareText, url: shareUrl };

    if (
      prefersNativeShare() &&
      typeof navigator.share === "function" &&
      (!navigator.canShare || navigator.canShare(payload))
    ) {
      try {
        await navigator.share(payload);
        return;
      } catch (err) {
        if (err && err.name === "AbortError") return;
      }
    }

    try {
      await copyURL(shareUrl);
      showFeedback("リンクをコピーしました");
    } catch {
      showFeedback("コピーに失敗しました", true);
    }
  }, [title, url, text, showFeedback]);

  const bubble = feedback ?? (hoverAllowed && hovering ? { text: IDLE_LABEL, error: false } : null);

  return html`
    <div class="article-share-wrap">
      <button
        ref=${btnRef}
        type="button"
        class="article-braille-host article-share-btn"
        aria-label=${feedback ? feedback.text : BUTTON_LABEL}
        onPointerEnter=${() => {
          if (hoverAllowed && !feedback) setHovering(true);
        }}
        onPointerLeave=${() => {
          setHoverAllowed(true);
          setHovering(false);
        }}
        onClick=${() => {
          void share();
        }}
      >
        ${icon}
      </button>
      ${bubble
        ? html`
            <div
              class=${`article-share-bubble${bubble.error ? " is-error" : ""}`}
              role="status"
              aria-live="polite"
            >
              ${bubble.text}
            </div>
          `
        : null}
    </div>
  `;
}

register(ArticleShare, "article-share", ["title", "url", "text"]);

export { ArticleShare };
