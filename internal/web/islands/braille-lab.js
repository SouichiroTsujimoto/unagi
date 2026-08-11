// Braille / dot-grid WAAPI lab widgets (vanilla CE — avoids preact-custom-element event/ref issues).

function prefersReducedMotion() {
  return window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ?? false;
}

function parseGlyphFrames(raw) {
  return String(raw || "")
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}

function parseGridFrames(raw) {
  return String(raw || "")
    .split("|")
    .map((s) => s.trim())
    .filter(Boolean);
}

function sizeClass(cols, rows) {
  if (cols === 2 && rows === 4) return "size-2x4";
  return `size-${cols}x${rows}`;
}

class BrailleWaapi extends HTMLElement {
  connectedCallback() {
    if (this._mounted) return;
    this._mounted = true;
    this._gen = 0;
    this._frames = parseGlyphFrames(this.getAttribute("frames"));
    const seed = this.getAttribute("seed") || "";
    const initial = this._frames[0] || "⠀";

    this.classList.add("braille-waapi");
    this.tabIndex = 0;
    this.title = seed;
    this.innerHTML =
      `<span class="glyph" aria-hidden="true">${initial}</span>` +
      `<span class="seed"></span>`;
    this.querySelector(".seed").textContent = seed;
    this._glyph = this.querySelector(".glyph");

    this.addEventListener("pointerenter", () => this.play());
    this.addEventListener("focus", () => this.play());
  }

  play() {
    const frames = this._frames;
    if (!frames.length || !this._glyph) return;
    if (this._timer) {
      clearInterval(this._timer);
      this._timer = 0;
    }
    if (prefersReducedMotion()) {
      this._glyph.textContent = frames[frames.length - 1] || frames[0];
      return;
    }
    let i = 0;
    this._glyph.textContent = frames[0];
    this._timer = setInterval(() => {
      i += 1;
      if (i >= frames.length) {
        clearInterval(this._timer);
        this._timer = 0;
        return;
      }
      this._glyph.textContent = frames[i];
    }, 70);
  }
}

class DotGridWaapi extends HTMLElement {
  connectedCallback() {
    if (this._mounted) return;
    this._mounted = true;
    this._gen = 0;

    const cols = Math.max(1, Number(this.getAttribute("cols")) || 5);
    const rows = Math.max(1, Number(this.getAttribute("rows")) || 5);
    const label = this.getAttribute("label") || "";
    this._frames = parseGridFrames(this.getAttribute("frames"));
    this._expected = cols * rows;

    const first = this._frames[0] || "";
    const dots = [];
    for (let i = 0; i < this._expected; i++) {
      const on = first[i] === "1" ? " is-on" : "";
      dots.push(`<span class="braille-dot${on}" style="--i:${i}"></span>`);
    }

    this.classList.add("dot-grid-waapi", sizeClass(cols, rows));
    this.style.setProperty("--cols", String(cols));
    this.style.setProperty("--rows", String(rows));
    this.tabIndex = 0;
    this.title = label;
    this.innerHTML =
      `<span class="dot-grid-face" aria-hidden="true">${dots.join("")}</span>` +
      `<span class="seed"></span>`;
    this.querySelector(".seed").textContent = label;
    this._face = this.querySelector(".dot-grid-face");

    this.addEventListener("pointerenter", () => this.play());
    this.addEventListener("focus", () => this.play());
  }

  applyBits(bits) {
    if (!this._face) return;
    for (let i = 0; i < this._expected; i++) {
      const dot = this._face.children[i];
      if (!dot) continue;
      dot.classList.toggle("is-on", bits[i] === "1");
    }
  }

  play() {
    const frames = this._frames;
    if (!frames.length) return;
    if (this._timer) {
      clearInterval(this._timer);
      this._timer = 0;
    }
    if (prefersReducedMotion()) {
      this.applyBits(frames[frames.length - 1] || frames[0]);
      return;
    }
    let i = 0;
    this.applyBits(frames[0]);
    this._timer = setInterval(() => {
      i += 1;
      if (i >= frames.length) {
        clearInterval(this._timer);
        this._timer = 0;
        return;
      }
      this.applyBits(frames[i]);
    }, 110);
  }
}

if (!customElements.get("braille-waapi")) {
  customElements.define("braille-waapi", BrailleWaapi);
}
if (!customElements.get("dot-grid-waapi")) {
  customElements.define("dot-grid-waapi", DotGridWaapi);
}
