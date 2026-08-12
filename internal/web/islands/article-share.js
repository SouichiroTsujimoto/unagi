// Article header share control: Web Share API with clipboard fallback.

class ArticleShare extends HTMLElement {
  connectedCallback() {
    if (this._mounted) return;
    this._mounted = true;
    this._btn = this.querySelector("button");
    if (!this._btn) return;
    this._defaultLabel = this._btn.getAttribute("aria-label") || "記事を共有";
    this._btn.addEventListener("click", () => {
      void this.share();
    });
  }

  shareURL() {
    return this.getAttribute("url") || window.location.href;
  }

  async share() {
    const title = this.getAttribute("title") || document.title;
    const text = this.getAttribute("text") || "";
    const url = this.shareURL();

    if (typeof navigator.share === "function") {
      try {
        await navigator.share({ title, text, url });
        return;
      } catch (err) {
        if (err && err.name === "AbortError") return;
      }
    }

    try {
      await navigator.clipboard.writeText(url);
      this.flashLabel("リンクをコピーしました");
    } catch {
      this.flashLabel("共有に失敗しました");
    }
  }

  flashLabel(label) {
    if (!this._btn) return;
    this._btn.setAttribute("aria-label", label);
    if (this._flashTimer) clearTimeout(this._flashTimer);
    this._flashTimer = setTimeout(() => {
      this._btn.setAttribute("aria-label", this._defaultLabel);
      this._flashTimer = 0;
    }, 2000);
  }
}

if (!customElements.get("article-share")) {
  customElements.define("article-share", ArticleShare);
}
