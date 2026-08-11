import { html } from "htm/preact";
import { useState } from "preact/hooks";
import register from "preact-custom-element";

async function parseJSON(res) {
  const text = await res.text();
  try {
    return JSON.parse(text);
  } catch {
    throw new Error(text || res.statusText);
  }
}

function b64urlToBuffer(value) {
  const pad = "=".repeat((4 - (value.length % 4)) % 4);
  const str = (value + pad).replace(/-/g, "+").replace(/_/g, "/");
  const raw = atob(str);
  const bytes = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  return bytes.buffer;
}

function bufferToB64url(buffer) {
  const bytes = new Uint8Array(buffer);
  let str = "";
  for (const b of bytes) str += String.fromCharCode(b);
  return btoa(str).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
}

function reviveCreationOptions(options) {
  options.challenge = b64urlToBuffer(options.challenge);
  options.user.id = b64urlToBuffer(options.user.id);
  if (options.excludeCredentials) {
    options.excludeCredentials = options.excludeCredentials.map((c) => ({
      ...c,
      id: b64urlToBuffer(c.id),
    }));
  }
  return options;
}

function reviveRequestOptions(options) {
  options.challenge = b64urlToBuffer(options.challenge);
  if (options.allowCredentials) {
    options.allowCredentials = options.allowCredentials.map((c) => ({
      ...c,
      id: b64urlToBuffer(c.id),
    }));
  }
  return options;
}

function credentialToJSON(cred) {
  const clientExt = cred.getClientExtensionResults ? cred.getClientExtensionResults() : {};
  return {
    id: cred.id,
    rawId: bufferToB64url(cred.rawId),
    type: cred.type,
    response: {
      clientDataJSON: bufferToB64url(cred.response.clientDataJSON),
      attestationObject: cred.response.attestationObject
        ? bufferToB64url(cred.response.attestationObject)
        : undefined,
      authenticatorData: cred.response.authenticatorData
        ? bufferToB64url(cred.response.authenticatorData)
        : undefined,
      signature: cred.response.signature ? bufferToB64url(cred.response.signature) : undefined,
      userHandle: cred.response.userHandle ? bufferToB64url(cred.response.userHandle) : undefined,
    },
    clientExtensionResults: clientExt,
  };
}

function Field({ id, label, children, hint }) {
  return html`
    <fieldset class="fieldset w-full gap-1.5 p-0">
      <label class="label px-0 text-sm font-medium text-base-content" for=${id}>${label}</label>
      ${children}
      ${hint
        ? html`<p class="text-base-content/45 text-xs leading-relaxed">${hint}</p>`
        : null}
    </fieldset>
  `;
}

function AdminSetup() {
  const [token, setToken] = useState("");
  const [name, setName] = useState("Primary");
  const [error, setError] = useState("");
  const [recovery, setRecovery] = useState(null);
  const [busy, setBusy] = useState(false);

  async function onSubmit(ev) {
    ev.preventDefault();
    setError("");
    const trimmed = token.trim();
    if (!trimmed) {
      setError("bootstrap tokenを入力してください");
      return;
    }
    setBusy(true);
    try {
      const beginRes = await fetch("/api/admin/setup/begin", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ token: trimmed }),
      });
      if (!beginRes.ok) throw new Error(await beginRes.text());
      const body = await parseJSON(beginRes);
      const publicKey = reviveCreationOptions(body.publicKey);
      const cred = await navigator.credentials.create({ publicKey });
      const finishRes = await fetch(
        `/api/admin/setup/finish?name=${encodeURIComponent(name.trim() || "Primary")}`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
            "X-Bootstrap-Token": trimmed,
          },
          body: JSON.stringify(credentialToJSON(cred)),
        },
      );
      if (!finishRes.ok) throw new Error(await finishRes.text());
      const result = await parseJSON(finishRes);
      if (result.recoveryCodes?.length) {
        setRecovery(result.recoveryCodes);
      }
      setTimeout(() => {
        location.href = "/admin";
      }, 2800);
    } catch (err) {
      setError(err.message || String(err));
    } finally {
      setBusy(false);
    }
  }

  return html`
    <form class="flex max-w-md flex-col gap-5" onSubmit=${onSubmit}>
      <${Field}
        id="bootstrap-token"
        label="Bootstrap token"
        hint="起動時に生成したワンタイムtokenです。"
      >
        <input
          id="bootstrap-token"
          name="token"
          type="password"
          class="input w-full"
          autocomplete="off"
          spellcheck="false"
          value=${token}
          onInput=${(e) => setToken(e.currentTarget.value)}
          required
        />
      <//>
      <${Field} id="passkey-name" label="Passkey名">
        <input
          id="passkey-name"
          name="name"
          type="text"
          class="input w-full"
          autocomplete="off"
          value=${name}
          onInput=${(e) => setName(e.currentTarget.value)}
        />
      <//>
      <button class="btn btn-primary w-fit" type="submit" disabled=${busy}>
        ${busy ? "登録中…" : "Passkeyを登録"}
      </button>
      ${error
        ? html`<p class="text-error text-sm" role="alert" aria-live="assertive">${error}</p>`
        : null}
      ${recovery
        ? html`<div
            class="rounded-xl border border-base-content/10 bg-base-200/60 p-4"
            role="status"
            aria-live="polite"
          >
            <p class="mb-2 text-sm font-medium">Recovery codes(一度だけ表示されます)</p>
            <pre class="font-mono text-sm whitespace-pre-wrap">${recovery.join("\n")}</pre>
          </div>`
        : null}
    </form>
  `;
}

function AdminLogin() {
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onLogin() {
    setError("");
    setBusy(true);
    try {
      const beginRes = await fetch("/api/admin/login/begin", { method: "POST" });
      if (!beginRes.ok) throw new Error(await beginRes.text());
      const body = await parseJSON(beginRes);
      const publicKey = reviveRequestOptions(body.publicKey);
      const assertion = await navigator.credentials.get({ publicKey });
      const finishRes = await fetch("/api/admin/login/finish", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(credentialToJSON(assertion)),
      });
      if (!finishRes.ok) throw new Error(await finishRes.text());
      location.href = "/admin";
    } catch (err) {
      setError(err.message || String(err));
    } finally {
      setBusy(false);
    }
  }

  return html`
    <div class="flex max-w-md flex-col gap-5">
      <button class="btn btn-primary w-fit" type="button" disabled=${busy} onClick=${onLogin}>
        ${busy ? "認証中…" : "Passkeyでログイン"}
      </button>
      ${error
        ? html`<p class="text-error text-sm" role="alert" aria-live="assertive">${error}</p>`
        : null}
    </div>
  `;
}

function AdminPasskeys(props) {
  const csrf = props.csrf || "";
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function onAdd() {
    setError("");
    setBusy(true);
    try {
      const beginRes = await fetch("/api/admin/passkeys/begin", {
        method: "POST",
        headers: { "X-CSRF-Token": csrf },
      });
      if (!beginRes.ok) throw new Error(await beginRes.text());
      const body = await parseJSON(beginRes);
      const publicKey = reviveCreationOptions(body.publicKey);
      const cred = await navigator.credentials.create({ publicKey });
      const name = window.prompt("Passkey名", "Backup") || "Passkey";
      const finishRes = await fetch(`/api/admin/passkeys/finish?name=${encodeURIComponent(name)}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": csrf,
        },
        body: JSON.stringify(credentialToJSON(cred)),
      });
      if (!finishRes.ok) throw new Error(await finishRes.text());
      location.reload();
    } catch (err) {
      setError(err.message || String(err));
      setBusy(false);
    }
  }

  return html`
    <div class="flex flex-col gap-3">
      <button class="btn btn-primary w-fit" type="button" disabled=${busy} onClick=${onAdd}>
        ${busy ? "登録中…" : "Passkeyを追加"}
      </button>
      ${error
        ? html`<p class="text-error text-sm" role="alert" aria-live="assertive">${error}</p>`
        : null}
    </div>
  `;
}

register(AdminSetup, "admin-setup", []);
register(AdminLogin, "admin-login", []);
register(AdminPasskeys, "admin-passkeys", ["csrf"]);

export { AdminSetup, AdminLogin, AdminPasskeys };
