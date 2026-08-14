import { html, render } from "htm/preact";
import { useCallback, useEffect, useMemo, useRef, useState } from "preact/hooks";
import register from "preact-custom-element";

const MAX_COMMENT_LENGTH = 1000;

function XLogo({ className = "engagement-x-logo" } = {}) {
  return html`
    <svg class=${className} viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path
        fill="currentColor"
        d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"
      />
    </svg>
  `;
}

function LockBadge() {
  return html`
    <span class="engagement-lock-badge" aria-hidden="true">
      <svg viewBox="0 0 24 24" focusable="false">
        <path
          fill="currentColor"
          d="M17 8h-1V6a4 4 0 0 0-8 0v2H7a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-8a2 2 0 0 0-2-2zm-7-2a2 2 0 1 1 4 0v2h-4zm7 12H7v-8h10z"
        />
      </svg>
    </span>
  `;
}

function SignInWithX({ href, className = "" }) {
  return html`
    <a class=${`x-signin-btn ${className}`.trim()} href=${href}>
      ${XLogo({ className: "x-signin-btn-logo" })}
      <span class="x-signin-btn-label">Xでログイン</span>
    </a>
  `;
}

async function api(path, method, body) {
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: {
      ...(body ? { "Content-Type": "application/json" } : {}),
      Accept: "application/json",
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (res.status === 204) {
    return null;
  }
  const text = await res.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = { message: text };
  }
  if (!res.ok) {
    const err = new Error(data?.message || data?.error || text || res.statusText);
    err.status = res.status;
    err.data = data;
    throw err;
  }
  return data;
}

function formatTime(value) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  return new Intl.DateTimeFormat("ja-JP", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function loginHref(path) {
  const base = path || "/auth/x/login";
  const returnTo = `${window.location.pathname}${window.location.search || ""}`;
  const join = base.includes("?") ? "&" : "?";
  return `${base}${join}return_to=${encodeURIComponent(returnTo)}`;
}

function logoutHref(path) {
  const base = path || "/auth/x/logout";
  const returnTo = `${window.location.pathname}${window.location.search || ""}`;
  const join = base.includes("?") ? "&" : "?";
  return `${base}${join}return_to=${encodeURIComponent(returnTo)}`;
}

function xProfileHref(username) {
  const handle = String(username || "").trim().replace(/^@/, "");
  if (!handle) return "";
  return `https://x.com/${encodeURIComponent(handle)}`;
}

function clamp01(v) {
  return Math.min(1, Math.max(0, v));
}

function coordsFromClient(board, clientX, clientY) {
  const rect = board.getBoundingClientRect();
  if (rect.width <= 0 || rect.height <= 0) return null;
  return {
    x: clamp01((clientX - rect.left) / rect.width),
    y: clamp01((clientY - rect.top) / rect.height),
  };
}

function ArticleEngagement(props) {
  const slug = props.slug || "";
  const boardRef = useRef(null);
  const paletteRef = useRef(null);
  const [loading, setLoading] = useState(true);
  const [stickers, setStickers] = useState([]);
  const [comments, setComments] = useState([]);
  const [allowedEmoji, setAllowedEmoji] = useState([]);
  const [loginPath, setLoginPath] = useState("/auth/x/login");
  const [logoutPath, setLogoutPath] = useState("/auth/x/logout");
  const [authenticated, setAuthenticated] = useState(false);
  const [viewer, setViewer] = useState(null);
  const [palette, setPalette] = useState(null);
  const [boardMessage, setBoardMessage] = useState("");
  const [commentMessage, setCommentMessage] = useState("");
  const [commentBody, setCommentBody] = useState("");
  const [placing, setPlacing] = useState(false);
  const [commentBusy, setCommentBusy] = useState(false);
  const [loadError, setLoadError] = useState("");

  const load = useCallback(async () => {
    if (!slug) return;
    setLoading(true);
    setLoadError("");
    try {
      const data = await api(`/api/articles/${encodeURIComponent(slug)}/engagement`, "GET");
      setStickers(Array.isArray(data.stickers) ? data.stickers : []);
      setComments(Array.isArray(data.comments) ? data.comments : []);
      setAllowedEmoji(Array.isArray(data.allowedEmoji) ? data.allowedEmoji : []);
      setLoginPath(data.loginPath || "/auth/x/login");
      setLogoutPath(data.logoutPath || "/auth/x/logout");
      setAuthenticated(Boolean(data.authenticated));
      setViewer(data.viewer || null);
    } catch (err) {
      setLoadError(err.message || String(err));
    } finally {
      setLoading(false);
    }
  }, [slug]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!palette) return undefined;

    const onKeyDown = (event) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setPalette(null);
      }
    };

    const onPointerDown = (event) => {
      const target = event.target;
      if (!(target instanceof Element)) return;
      if (paletteRef.current?.contains(target)) return;
      if (boardRef.current?.contains(target) && target.closest(".engagement-board")) {
        return;
      }
      setPalette(null);
    };

    document.addEventListener("keydown", onKeyDown);
    document.addEventListener("pointerdown", onPointerDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      document.removeEventListener("pointerdown", onPointerDown);
    };
  }, [palette]);

  useEffect(() => {
    if (!palette) return;
    const first = paletteRef.current?.querySelector("button, a");
    first?.focus();
  }, [palette]);

  const remaining = useMemo(
    () => Math.max(0, MAX_COMMENT_LENGTH - [...commentBody].length),
    [commentBody],
  );

  function openPaletteAt(clientX, clientY) {
    if (!boardRef.current || placing || loading) return;
    const coords = coordsFromClient(boardRef.current, clientX, clientY);
    if (!coords) return;
    setBoardMessage("");
    setPalette(coords);
  }

  function onBoardClick(event) {
    if (event.target.closest(".engagement-sticker-palette")) return;
    if (event.target.closest(".engagement-sticker")) return;
    openPaletteAt(event.clientX, event.clientY);
  }

  function onBoardKeyDown(event) {
    if (event.key !== "Enter" && event.key !== " ") return;
    event.preventDefault();
    const rect = boardRef.current?.getBoundingClientRect();
    if (!rect) return;
    openPaletteAt(rect.left + rect.width / 2, rect.top + rect.height / 2);
  }

  async function placeEmoji(emoji) {
    if (!palette || placing) return;
    setPlacing(true);
    setBoardMessage("");
    try {
      const created = await api(`/api/articles/${encodeURIComponent(slug)}/stickers`, "POST", {
        emoji,
        x: palette.x,
        y: palette.y,
      });
      setStickers((prev) => [...prev, created]);
      setPalette(null);
    } catch (err) {
      setBoardMessage(err.message || String(err));
    } finally {
      setPlacing(false);
    }
  }

  async function placeAvatar() {
    if (!palette || placing || !authenticated) return;
    setPlacing(true);
    setBoardMessage("");
    try {
      const created = await api(`/api/articles/${encodeURIComponent(slug)}/avatar-stickers`, "POST", {
        x: palette.x,
        y: palette.y,
      });
      setStickers((prev) => [...prev, created]);
      setPalette(null);
      // setBoardMessage("アイコンを貼りました");
    } catch (err) {
      if (err.status === 401 && err.data?.loginPath) {
        setBoardMessage("Xアカウントでログインすると、自分のアイコンを貼れます");
        return;
      }
      if (err.status === 429) {
        setBoardMessage("ステッカーボードがいっぱいです");
        return;
      }
      setBoardMessage(err.message || String(err));
    } finally {
      setPlacing(false);
    }
  }

  async function submitComment(event) {
    event.preventDefault();
    if (!authenticated) return;
    setCommentBusy(true);
    setCommentMessage("");
    try {
      const created = await api(`/api/articles/${encodeURIComponent(slug)}/comments`, "POST", {
        body: commentBody,
      });
      setComments((prev) => [...prev, { ...created, mine: true }]);
      setCommentBody("");
      setCommentMessage("コメントを投稿しました");
    } catch (err) {
      if (err.status === 401 && err.data?.loginPath) {
        setCommentMessage("コメントするにはXアカウントでログインしてください");
        return;
      }
      if (err.status === 429) {
        setCommentMessage("この記事へのコメント上限に達しました");
        return;
      }
      setCommentMessage(err.message || String(err));
    } finally {
      setCommentBusy(false);
    }
  }

  async function deleteComment(comment) {
    if (!authenticated || !comment?.id || commentBusy) return;
    if (!window.confirm("このコメントを削除しますか？")) {
      return;
    }
    setCommentBusy(true);
    setCommentMessage("");
    try {
      await api(`/api/articles/${encodeURIComponent(slug)}/comments/${comment.id}`, "DELETE");
      setComments((prev) => prev.filter((item) => item.id !== comment.id));
      setCommentMessage("コメントを削除しました");
    } catch (err) {
      if (err.status === 401 && err.data?.loginPath) {
        setCommentMessage("削除するにはXアカウントでログインしてください");
        return;
      }
      setCommentMessage(err.message || String(err));
    } finally {
      setCommentBusy(false);
    }
  }

  const paletteStyle = palette
    ? `left:${palette.x * 100}%;top:${palette.y * 100}%`
    : "";

  const linkedLabel = viewer?.username ? `連携済み: @${viewer.username}` : "";

  return html`
    <section class="engagement space-y-10" aria-label="記事への反応">
      <section class="engagement-board-section" aria-labelledby="engagement-board-title">
        <header class="engagement-board-copy space-y-1">
          <h2 id="engagement-board-title" class="text-base font-semibold tracking-tight">ステッカーボード</h2>
          <p class="text-base-content/55 text-sm leading-relaxed">
            ボードをクリックしてステッカーを貼りましょう！Xアカウントでログインすると、自分のアイコンのステッカーを残せます。
          </p>
        </header>

        <div
          ref=${boardRef}
          class=${`engagement-board ${palette ? "is-picking" : ""} ${placing ? "is-busy" : ""}`}
          role="application"
          tabindex="0"
          aria-label="ステッカーボード。クリックしてパレットを開き、絵文字を貼ります"
          onClick=${onBoardClick}
          onKeyDown=${onBoardKeyDown}
        >
          ${loading
            ? html`<p class="engagement-board-empty">読み込み中…</p>`
            : stickers.length === 0 && !palette
              ? html`<p class="engagement-board-empty">まだステッカーがありません。好きな位置をクリックして貼ってみてください。</p>`
              : null}
          ${stickers.map((sticker) =>
            sticker.kind === "avatar"
              ? html`
                  <img
                    class="engagement-sticker engagement-sticker-avatar"
                    src=${sticker.value}
                    alt=${sticker.displayName || sticker.username || "avatar"}
                    width="52"
                    height="52"
                    style=${`left:${sticker.x * 100}%;top:${sticker.y * 100}%`}
                    loading="lazy"
                  />
                `
              : html`
                  <span
                    class="engagement-sticker engagement-sticker-emoji"
                    style=${`left:${sticker.x * 100}%;top:${sticker.y * 100}%`}
                    aria-hidden="true"
                  >
                    ${sticker.value}
                  </span>
                `,
          )}
          ${palette
            ? html`
                <span
                  class="engagement-sticker-target"
                  style=${paletteStyle}
                  aria-hidden="true"
                ></span>
                <div
                  ref=${paletteRef}
                  class="engagement-sticker-palette"
                  style=${paletteStyle}
                  role="dialog"
                  aria-label="ステッカーを選ぶ"
                >
                  <div class="engagement-sticker-palette-grid" role="listbox" aria-label="ステッカー">
                    ${allowedEmoji.map(
                      (emoji) => html`
                        <button
                          type="button"
                          class="engagement-emoji-btn btn btn-ghost btn-sm"
                          role="option"
                          aria-label=${`絵文字 ${emoji} を貼る`}
                          disabled=${placing}
                          onClick=${(event) => {
                            event.stopPropagation();
                            void placeEmoji(emoji);
                          }}
                        >
                          <span aria-hidden="true">${emoji}</span>
                        </button>
                      `,
                    )}
                    ${authenticated && viewer?.avatarUrl
                      ? html`
                          <button
                            type="button"
                            class="engagement-emoji-btn engagement-palette-avatar-btn btn btn-ghost btn-sm"
                            role="option"
                            aria-label="自分のアイコンを貼る"
                            disabled=${placing}
                            onClick=${(event) => {
                              event.stopPropagation();
                              void placeAvatar();
                            }}
                          >
                            <img
                              src=${viewer.avatarUrl}
                              alt=""
                              width="28"
                              height="28"
                              class="engagement-palette-avatar"
                              loading="lazy"
                            />
                          </button>
                        `
                      : html`
                          <a
                            class="engagement-emoji-btn engagement-x-lock-btn"
                            href=${loginHref(loginPath)}
                            role="option"
                            aria-label="Xアカウントでログイン"
                            onClick=${(event) => event.stopPropagation()}
                          >
                            <span class="engagement-x-lock-mark">
                              ${XLogo({ className: "engagement-x-logo" })}
                              ${LockBadge()}
                            </span>
                          </a>
                        `}
                  </div>
                  <button
                    type="button"
                    class="engagement-palette-cancel btn btn-ghost btn-xs"
                    disabled=${placing}
                    onClick=${(event) => {
                      event.stopPropagation();
                      setPalette(null);
                    }}
                  >
                    キャンセル
                  </button>
                </div>
              `
            : null}
        </div>

        <div class="engagement-board-account text-sm text-base-content/55">
          ${authenticated && linkedLabel
            ? html`
              <div class="flex flex-wrap items-center gap-3">
                <p>${linkedLabel}</p>
                <a class="link link-hover" href=${logoutHref(logoutPath)}>ログアウト</a>
              </div>
              `
            : null}
        </div>
        <div class="engagement-board-feedback text-sm" aria-live="polite">
          ${boardMessage ? html`<p class="text-base-content/55">${boardMessage}</p>` : null}
          ${loadError ? html`<p class="text-error" role="alert">${loadError}</p>` : null}
        </div>
      </section>

      <section class="engagement-comments space-y-4" aria-labelledby="engagement-comments-title">
        <header class="space-y-1">
          <h2 id="engagement-comments-title" class="text-base font-semibold tracking-tight">コメント</h2>
        </header>

        ${loading
          ? html`<p class="text-sm text-base-content/55">読み込み中…</p>`
          : comments.length === 0
            ? html`<p class="text-sm text-base-content/55">まだコメントはありません。</p>`
            : html`
                <ul class="engagement-comment-list space-y-4">
                  ${comments.map((comment) => {
                    const profileHref = xProfileHref(comment.username);
                    const avatar = comment.avatarUrl
                      ? html`<img
                          class="engagement-comment-avatar"
                          src=${comment.avatarUrl}
                          alt=""
                          width="40"
                          height="40"
                          loading="lazy"
                        />`
                      : html`<span class="engagement-comment-avatar is-fallback" aria-hidden="true">X</span>`;
                    const identity = html`
                      <div class="min-w-0">
                        <p class="engagement-comment-name">
                          ${comment.displayName || comment.username || "anonymous"}
                          ${comment.username
                            ? html`<span class="engagement-comment-handle">@${comment.username}</span>`
                            : null}
                        </p>
                        <time class="engagement-comment-time" datetime=${comment.createdAt}>
                          ${formatTime(comment.createdAt)}
                        </time>
                      </div>
                    `;
                    return html`
                      <li class="engagement-comment">
                        <div class="engagement-comment-meta">
                          ${profileHref
                            ? html`
                                <a
                                  class="engagement-comment-author"
                                  href=${profileHref}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  aria-label=${`@${comment.username} のXプロフィールを開く`}
                                >
                                  ${avatar}
                                  ${identity}
                                </a>
                              `
                            : html`
                                <div class="engagement-comment-author is-static">
                                  ${avatar}
                                  ${identity}
                                </div>
                              `}
                          ${comment.mine
                            ? html`
                                <button
                                  type="button"
                                  class="engagement-comment-delete btn btn-ghost btn-xs"
                                  disabled=${commentBusy}
                                  aria-label="このコメントを削除"
                                  onClick=${() => void deleteComment(comment)}
                                >
                                  削除
                                </button>
                              `
                            : null}
                        </div>
                        <p class="engagement-comment-body">${comment.body}</p>
                      </li>
                    `;
                  })}
                </ul>
              `}

        <form class="engagement-comment-form space-y-3" onSubmit=${submitComment}>
          <fieldset class="fieldset gap-1.5 p-0">
            <label class="label text-sm font-medium text-base-content" for=${`comment-body-${slug}`}>
              コメントを書く
            </label>
            <div class=${`engagement-comment-compose ${authenticated ? "" : "is-locked"}`}>
              <textarea
                id=${`comment-body-${slug}`}
                name="body"
                class="textarea w-full min-h-28 text-base"
                maxlength=${MAX_COMMENT_LENGTH}
                required=${authenticated}
                disabled=${!authenticated}
                readonly=${!authenticated}
                value=${authenticated ? commentBody : ""}
                placeholder=${authenticated ? "" : " "}
                onInput=${(event) => setCommentBody(event.currentTarget.value)}
                aria-describedby=${`comment-help-${slug}`}
              ></textarea>
              ${!authenticated
                ? html`
                    <div class="engagement-comment-lock">
                      ${SignInWithX({ href: loginHref(loginPath) })}
                    </div>
                  `
                : null}
            </div>
            <p id=${`comment-help-${slug}`} class="text-xs text-base-content/45">
              ${authenticated
                ? `最大 ${MAX_COMMENT_LENGTH} 文字 · 残り ${remaining}`
                : "ログインするとコメントを書けます"}
            </p>
          </fieldset>
          ${authenticated
            ? html`
                <div class="flex flex-wrap items-center gap-3">
                  <button type="submit" class="btn btn-primary btn-sm min-h-8" disabled=${commentBusy}>
                    ${commentBusy ? "送信中…" : "コメントする"}
                  </button>
                  <a class="link link-hover text-sm" href=${logoutHref(logoutPath)}>ログアウト</a>
                </div>
              `
            : null}
          <p class="text-sm text-base-content/55" aria-live="polite">${commentMessage}</p>
        </form>
      </section>
    </section>
  `;
}

register(ArticleEngagement, "article-engagement", ["slug"]);

export { ArticleEngagement, render, html };
