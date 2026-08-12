import { html } from "htm/preact";
import { useCallback, useEffect, useMemo, useState } from "preact/hooks";
import register from "preact-custom-element";

function formatWhen(iso) {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return String(iso);
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function stickerLabel(item) {
  if (item.kind === "avatar") {
    const name = item.displayName || item.username || "avatar";
    return `avatar · ${name}`;
  }
  return "";
}

function AdminStickers(props) {
  const articleId = props.articleId || "";
  const [stickers, setStickers] = useState([]);
  const [selected, setSelected] = useState(() => new Set());
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const api = useCallback(
    async (path, method, body) => {
      const res = await fetch(path, {
        method,
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: body ? JSON.stringify(body) : undefined,
      });
      const text = await res.text();
      let data = null;
      try {
        data = text ? JSON.parse(text) : null;
      } catch {
        data = { message: text };
      }
      if (!res.ok) {
        throw new Error(data?.message || data?.error || text || res.statusText);
      }
      return data;
    },
    [],
  );

  const load = useCallback(async () => {
    if (!articleId) return;
    setError("");
    try {
      const data = await api(`/api/admin/articles/${articleId}/stickers`, "GET");
      setStickers(Array.isArray(data.stickers) ? data.stickers : []);
      setSelected(new Set());
    } catch (err) {
      setError(err.message || String(err));
    }
  }, [api, articleId]);

  useEffect(() => {
    void load();
  }, [load]);

  const allSelected = useMemo(
    () => stickers.length > 0 && stickers.every((s) => selected.has(s.id)),
    [stickers, selected],
  );

  function toggleAll(checked) {
    if (!checked) {
      setSelected(new Set());
      return;
    }
    setSelected(new Set(stickers.map((s) => s.id)));
  }

  function toggleOne(id, checked) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  }

  async function deleteSelected() {
    const ids = [...selected];
    if (ids.length === 0) return;
    if (!window.confirm(`選択したステッカー ${ids.length} 件を剥がしますか？`)) {
      return;
    }
    setBusy(true);
    setMessage("");
    setError("");
    try {
      const data = await api(`/api/admin/articles/${articleId}/stickers`, "DELETE", { ids });
      setMessage(`${data.deleted ?? ids.length} 件を剥がしました`);
      await load();
    } catch (err) {
      setError(err.message || String(err));
    } finally {
      setBusy(false);
    }
  }

  async function deleteAll() {
    if (stickers.length === 0) return;
    if (!window.confirm(`この記事のステッカーをすべて削除しますか？(${stickers.length} 件)`)) {
      return;
    }
    setBusy(true);
    setMessage("");
    setError("");
    try {
      const data = await api(`/api/admin/articles/${articleId}/stickers`, "DELETE", { all: true });
      setMessage(`${data.deleted ?? stickers.length} 件をすべて削除しました`);
      await load();
    } catch (err) {
      setError(err.message || String(err));
    } finally {
      setBusy(false);
    }
  }

  if (!articleId) {
    return html`<p class="text-base-content/55 text-[15px]">記事保存後にステッカーを管理できます。</p>`;
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
          選択を剥がす${selected.size > 0 ? ` (${selected.size})` : ""}
        </button>
        <button
          type="button"
          class="btn btn-sm btn-outline btn-error"
          disabled=${busy || stickers.length === 0}
          onClick=${deleteAll}
        >
          すべて削除
        </button>
        <button type="button" class="btn btn-sm btn-ghost" disabled=${busy} onClick=${() => void load()}>
          再読込
        </button>
      </div>

      ${message
        ? html`<p class="text-sm text-success">${message}</p>`
        : null}
      ${error ? html`<p class="text-sm text-error">${error}</p>` : null}

      ${stickers.length === 0
        ? html`<p class="text-base-content/55 text-[15px]">ステッカーはまだありません。</p>`
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
                    <th>ステッカー</th>
                    <th>種別</th>
                    <th>位置</th>
                    <th>貼付日時</th>
                  </tr>
                </thead>
                <tbody>
                  ${stickers.map(
                    (item, index) => html`
                      <tr key=${item.id}>
                        <td>
                          <input
                            type="checkbox"
                            class="checkbox checkbox-sm"
                            checked=${selected.has(item.id)}
                            aria-label=${`ステッカー ${index + 1} を選択`}
                            onChange=${(e) => toggleOne(item.id, e.currentTarget.checked)}
                          />
                        </td>
                        <td class="text-base-content/55">${index + 1}</td>
                        <td>
                          ${item.kind === "emoji"
                            ? html`<span class="text-lg leading-none">${item.value}</span>`
                            : html`<span class="text-sm">${stickerLabel(item) || item.value || item.kind}</span>`}
                        </td>
                        <td class="text-sm text-base-content/55">${item.kind}</td>
                        <td class="font-mono text-xs text-base-content/55">
                          ${Number(item.x).toFixed(2)}, ${Number(item.y).toFixed(2)}
                        </td>
                        <td class="text-sm text-base-content/55">${formatWhen(item.createdAt)}</td>
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

register(AdminStickers, "admin-stickers", ["article-id"], { shadow: false });
