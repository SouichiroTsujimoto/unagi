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
      authenticatorData: cred.response.authenticatorData
        ? bufferToB64url(cred.response.authenticatorData)
        : undefined,
      signature: cred.response.signature ? bufferToB64url(cred.response.signature) : undefined,
      userHandle: cred.response.userHandle ? bufferToB64url(cred.response.userHandle) : undefined,
    },
    clientExtensionResults: clientExt,
  };
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
      const publicKey = reviveRequestOptions(body.options?.publicKey || body.publicKey || body.options);
      const challengeId = body.challenge_id || body.challengeId;
      if (!challengeId) throw new Error("missing challenge_id");
      const assertion = await navigator.credentials.get({ publicKey });
      const finishRes = await fetch("/api/admin/login/finish", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          challenge_id: challengeId,
          credential_response: credentialToJSON(assertion),
        }),
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

register(AdminLogin, "admin-login", []);

export { AdminLogin };
