import { html } from "htm/preact";
import { useCallback, useEffect, useMemo, useState } from "preact/hooks";
import register from "preact-custom-element";
import { api, formatWhen } from "./admin-common.js";

function AdminComments(props) {
  const articleId = props.articleId || "";
  const [comments, setComments] = useState([]);
  const [selected, setSelected] = useState(() => new Set());
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    if (!articleId) return;
    setError("");
    try {
      const data = await api(`/api/admin/articles/${articleId}/comments`, "GET");
      setComments(Array.isArray(data.comments) ? data.comments : []);
      setSelected(new Set());
    } catch (err) {
      setError(err.message || String(err));
    }
  }, [articleId]);

  useEffect(() => {
    void load();
  }, [load]);

  const allSelected = useMemo(
    () => comments.length > 0 && comments.every((c) => selected.has(c.id)),
    [comments, selected],
  );

  function toggleAll(checked) {
    if (!checked) {
      setSelected(new Set());
      return;
    }
    setSelected(new Set(comments.map((c) => c.id)));
  }

  function toggleOne(id, checked) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  }

  async function setStatus(commentID, status) {
    setBusy(true);
    setMessage("");
    setError("");
    try {
      await api(`/api/admin/articles/${articleId}/comments/${commentID}`, "PATCH", { status });
      setMessage(status === "hidden" ? "コメントを非表示にしました" : "コメントを再表示しました");
      await load();
    } catch (err) {
      setError(err.message || String(err));
    } finally {
      setBusy(false);
    }
  }

  async function deleteSelected() {
    const ids = [...selected];
    if (ids.length === 0) return;
    if (!window.confirm(`選択したコメント ${ids.length} 件を削除しますか？`)) {
      return;
    }
    setBusy(true);
    setMessage("");
    setError("");
    try {
      const data = await api(`/api/admin/articles/${articleId}/comments`, "DELETE", { ids });
      setMessage(`${data.deleted ?? ids.length} 件を削除しました`);
      await load();
    } catch (err) {
      setError(err.message || String(err));
    } finally {
      setBusy(false);
    }
  }

  async function deleteAll() {
    if (comments.length === 0) return;
    if (!window.confirm(`この記事のコメントをすべて削除しますか？(${comments.length} 件)`)) {
      return;
    }
    setBusy(true);
    setMessage("");
    setError("");
    try {
      const data = await api(`/api/admin/articles/${articleId}/comments`, "DELETE", { all: true });
      setMessage(`${data.deleted ?? comments.length} 件をすべて削除しました`);
      await load();
    } catch (err) {
      setError(err.message || String(err));
    } finally {
      setBusy(false);
    }
  }

  if (!articleId) {
    return html`<p class="text-base-content/55 text-[15px]">記事保存後にコメントを管理できます。</p>`;
  }

  return html`
    <div class="space-y-4">
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="btn btn-sm btn-outline"
          disabled=${busy || selected.size === 0}
          onClick=${deleteSelected}
        >
          選択を削除${selected.size > 0 ? ` (${selected.size})` : ""}
        </button>
        <button
          type="button"
          class="btn btn-sm btn-outline btn-error"
          disabled=${busy || comments.length === 0}
          onClick=${deleteAll}
        >
          すべて削除
        </button>
        <button type="button" class="btn btn-sm btn-ghost" disabled=${busy} onClick=${() => void load()}>
          再読込
        </button>
      </div>

      ${message ? html`<p class="text-sm text-success">${message}</p>` : null}
      ${error ? html`<p class="text-sm text-error">${error}</p>` : null}

      ${comments.length === 0
        ? html`<p class="text-base-content/55 text-[15px]">コメントはまだありません。</p>`
        : html`
            <div class="overflow-x-auto rounded-xl border border-base-content/10">
              <table class="table table-sm">
                <thead>
                  <tr>
                    <th class="w-10">
                      <input
                        type="checkbox"
                        class="checkbox checkbox-sm"
                        checked=${allSelected}
                        aria-label="すべて選択"
                        onChange=${(e) => toggleAll(e.currentTarget.checked)}
                      />
                    </th>
                    <th>#</th>
                    <th>投稿者</th>
                    <th>本文</th>
                    <th>状態</th>
                    <th>投稿日時</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  ${comments.map(
                    (item, index) => html`
                      <tr key=${item.id} class=${item.status === "hidden" ? "opacity-60" : ""}>
                        <td>
                          <input
                            type="checkbox"
                            class="checkbox checkbox-sm"
                            checked=${selected.has(item.id)}
                            aria-label=${`コメント ${index + 1} を選択`}
                            onChange=${(e) => toggleOne(item.id, e.currentTarget.checked)}
                          />
                        </td>
                        <td class="text-base-content/55">${index + 1}</td>
                        <td class="text-sm">
                          <span class="font-medium">${item.displayName || item.username || "anonymous"}</span>
                          ${item.username
                            ? html`<span class="text-base-content/45"> @${item.username}</span>`
                            : null}
                        </td>
                        <td class="max-w-xs whitespace-pre-wrap break-words text-sm">${item.body}</td>
                        <td class="text-sm">
                          ${item.status === "hidden"
                            ? html`<span class="badge badge-ghost badge-sm">非表示</span>`
                            : html`<span class="badge badge-success badge-sm">公開</span>`}
                        </td>
                        <td class="text-sm text-base-content/55">${formatWhen(item.createdAt)}</td>
                        <td class="text-right">
                          ${item.status === "hidden"
                            ? html`
                                <button
                                  type="button"
                                  class="btn btn-ghost btn-xs"
                                  disabled=${busy}
                                  onClick=${() => void setStatus(item.id, "visible")}
                                >
                                  再表示
                                </button>
                              `
                            : html`
                                <button
                                  type="button"
                                  class="btn btn-ghost btn-xs"
                                  disabled=${busy}
                                  onClick=${() => void setStatus(item.id, "hidden")}
                                >
                                  非表示
                                </button>
                              `}
                        </td>
                      </tr>
                    `,
                  )}
                </tbody>
              </table>
            </div>
          `}
    </div>
  `;
}

register(AdminComments, "admin-comments", ["article-id"], { shadow: false });
