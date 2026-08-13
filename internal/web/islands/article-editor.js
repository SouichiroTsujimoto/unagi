import { html, render } from "htm/preact";
import { useMemo, useState } from "preact/hooks";
import register from "preact-custom-element";

function ArticleEditor(props) {
  const isNew = props.isNew === "true";
  const articleId = props.articleId || "";
  const [slug, setSlug] = useState(props.slug || "");
  const [title, setTitle] = useState(props.title || "");
  const [emoji, setEmoji] = useState(props.emoji || "");
  const [type, setType] = useState(props.type || "tech");
  const [topics, setTopics] = useState(props.topics || "");
  const [bodyMd, setBodyMd] = useState(props.bodyMd || "");
  const [publishedAt, setPublishedAt] = useState(props.publishedAt || "");
  const [published, setPublished] = useState(props.published === "true");
  const [preview, setPreview] = useState("");
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [currentId, setCurrentId] = useState(articleId);

  const payload = useMemo(
    () => ({
      slug,
      title,
      emoji,
      type,
      topics: topics
        .split(",")
        .map((t) => t.trim())
        .filter(Boolean),
      bodyMd,
      publishedAt,
    }),
    [slug, title, emoji, type, topics, bodyMd, publishedAt],
  );

  async function api(path, method, body) {
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
  }

  async function save() {
    setBusy(true);
    setMessage("");
    try {
      if (isNew && !currentId) {
        const created = await api("/api/admin/articles", "POST", payload);
        setCurrentId(String(created.id));
        setMessage("作成しました");
        history.replaceState({}, "", `/admin/articles/${created.id}`);
      } else {
        await api(`/api/admin/articles/${currentId}`, "PUT", payload);
        setMessage("保存しました");
      }
    } catch (err) {
      setMessage(err.message || String(err));
    } finally {
      setBusy(false);
    }
  }

  async function publish() {
    setBusy(true);
    setMessage("");
    try {
      let id = currentId;
      if (!id) {
        const created = await api("/api/admin/articles", "POST", payload);
        id = String(created.id);
        setCurrentId(id);
        history.replaceState({}, "", `/admin/articles/${created.id}`);
      } else {
        await api(`/api/admin/articles/${id}`, "PUT", payload);
      }
      await api(`/api/admin/articles/${id}/publish`, "POST", { publishedAt });
      setPublished(true);
      setMessage("公開しました");
    } catch (err) {
      setMessage(err.message || String(err));
    } finally {
      setBusy(false);
    }
  }

  async function unpublish() {
    setBusy(true);
    try {
      await api(`/api/admin/articles/${currentId}/unpublish`, "POST");
      setPublished(false);
      setMessage("非公開にしました");
    } catch (err) {
      setMessage(err.message || String(err));
    } finally {
      setBusy(false);
    }
  }

  async function doPreview() {
    try {
      const data = await api("/api/admin/preview", "POST", { bodyMd });
      setPreview(data.html || "");
    } catch (err) {
      setMessage(err.message || String(err));
    }
  }

  async function upload(ev) {
    const file = ev.target.files?.[0];
    if (!file) return;
    setBusy(true);
    try {
      const signed = await api("/api/admin/media/sign", "POST", {
        filename: file.name,
        contentType: file.type,
        sizeBytes: file.size,
      });
      let putUrl = signed.signedUrl;
      if (signed.token && !String(putUrl).includes("token=")) {
        putUrl += (String(putUrl).includes("?") ? "&" : "?") + "token=" + encodeURIComponent(signed.token);
      }
      const up = await fetch(putUrl, {
        method: "PUT",
        headers: { "Content-Type": signed.contentType || file.type },
        body: file,
      });
      if (!up.ok) {
        throw new Error("storage upload failed");
      }
      const data = await api("/api/admin/media/complete", "POST", { objectKey: signed.objectKey });
      const markdownUrl = data.url || `/images/${signed.objectKey}`;
      setBodyMd((prev) => `${prev.trimEnd()}\n\n![](${markdownUrl})\n`);
      setMessage(`画像を追加: ${markdownUrl}`);
    } catch (err) {
      setMessage(err.message || String(err));
    } finally {
      setBusy(false);
      ev.target.value = "";
    }
  }

  return html`
    <div class="flex flex-col gap-5">
      <div class="grid gap-4 sm:grid-cols-2">
        <fieldset class="fieldset w-full gap-1.5 p-0">
          <label class="label px-0 text-sm font-medium" for="ae-title">Title</label>
          <input id="ae-title" class="input w-full" value=${title} onInput=${(e) => setTitle(e.target.value)} />
        </fieldset>
        <fieldset class="fieldset w-full gap-1.5 p-0">
          <label class="label px-0 text-sm font-medium" for="ae-slug">Slug</label>
          <input id="ae-slug" class="input w-full" value=${slug} onInput=${(e) => setSlug(e.target.value)} />
        </fieldset>
        <fieldset class="fieldset w-full gap-1.5 p-0">
          <label class="label px-0 text-sm font-medium" for="ae-emoji">Emoji</label>
          <input id="ae-emoji" class="input w-full" value=${emoji} onInput=${(e) => setEmoji(e.target.value)} />
        </fieldset>
        <fieldset class="fieldset w-full gap-1.5 p-0">
          <label class="label px-0 text-sm font-medium" for="ae-type">Type</label>
          <select id="ae-type" class="select w-full" value=${type} onChange=${(e) => setType(e.target.value)}>
            <option value="tech">tech</option>
            <option value="idea">idea</option>
          </select>
        </fieldset>
        <fieldset class="fieldset w-full gap-1.5 p-0 sm:col-span-2">
          <label class="label px-0 text-sm font-medium" for="ae-topics">Topics (comma separated)</label>
          <input id="ae-topics" class="input w-full" value=${topics} onInput=${(e) => setTopics(e.target.value)} />
        </fieldset>
        <fieldset class="fieldset w-full gap-1.5 p-0">
          <label class="label px-0 text-sm font-medium" for="ae-published-at">published_at (JST)</label>
          <input
            id="ae-published-at"
            class="input w-full"
            placeholder="2026-08-12 09:00"
            value=${publishedAt}
            onInput=${(e) => setPublishedAt(e.target.value)}
          />
        </fieldset>
        <fieldset class="fieldset w-full gap-1.5 p-0">
          <label class="label px-0 text-sm font-medium" for="ae-image">画像アップロード</label>
          <input
            id="ae-image"
            type="file"
            accept="image/png,image/jpeg,image/webp,image/gif"
            class="file-input w-full"
            onChange=${upload}
          />
        </fieldset>
      </div>

      <fieldset class="fieldset w-full gap-1.5 p-0">
        <label class="label px-0 text-sm font-medium" for="ae-body">Markdown</label>
        <textarea
          id="ae-body"
          class="textarea min-h-80 w-full font-mono text-sm"
          value=${bodyMd}
          onInput=${(e) => setBodyMd(e.target.value)}
        ></textarea>
      </fieldset>

      <div class="flex flex-wrap gap-2">
        <button class="btn btn-primary btn-sm" type="button" disabled=${busy} onClick=${save}>保存</button>
        <button class="btn btn-secondary btn-sm" type="button" disabled=${busy} onClick=${doPreview}>Preview</button>
        <button class="btn btn-accent btn-sm" type="button" disabled=${busy} onClick=${publish}>公開</button>
        ${published
          ? html`<button class="btn btn-ghost btn-sm" type="button" disabled=${busy} onClick=${unpublish}>非公開</button>`
          : null}
      </div>
      ${message ? html`<p class="text-sm text-base-content/60">${message}</p>` : null}
      ${preview
        ? html`<div class="article-prose rounded-xl border border-base-content/10 p-4" dangerouslySetInnerHTML=${{ __html: preview }}></div>`
        : null}
    </div>
  `;
}

register(ArticleEditor, "article-editor", [
  "article-id",
  "is-new",
  "slug",
  "title",
  "emoji",
  "type",
  "topics",
  "body-md",
  "published-at",
  "published",
]);

export { ArticleEditor, render, html };
