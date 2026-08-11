import { useState } from 'preact/hooks';
import { html } from 'htm/preact';
import register from 'preact-custom-element';

function parseAccounts(value) {
	if (Array.isArray(value)) {
		return value;
	}
	if (value == null || value === '') {
		return [];
	}
	try {
		const parsed = typeof value === 'string' ? JSON.parse(value) : value;
		return Array.isArray(parsed) ? parsed : [];
	} catch {
		return [];
	}
}

function AccountPanel({ accounts: accountsProp }) {
	const [accounts, setAccounts] = useState(() => parseAccounts(accountsProp));
	const [email, setEmail] = useState('');
	const [status, setStatus] = useState({ message: '', isError: false });
	const [submitting, setSubmitting] = useState(false);

	async function onSubmit(event) {
		event.preventDefault();
		setSubmitting(true);
		setStatus({ message: '', isError: false });
		try {
			const response = await fetch('/api/accounts', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
				body: JSON.stringify({ email }),
			});
			const data = await response.json().catch(() => ({}));
			if (!response.ok) {
				setStatus({ message: data.error || '追加に失敗しました。', isError: true });
				return;
			}
			setAccounts((list) => [data, ...list]);
			setEmail('');
			setStatus({ message: '追加しました。', isError: false });
		} catch {
			setStatus({ message: '追加に失敗しました。', isError: true });
		} finally {
			setSubmitting(false);
		}
	}

	async function onDelete(id) {
		setStatus({ message: '', isError: false });
		try {
			const response = await fetch(`/api/accounts/${id}`, {
				method: 'DELETE',
				headers: { Accept: 'application/json' },
			});
			if (!response.ok) {
				const data = await response.json().catch(() => ({}));
				setStatus({ message: data.error || '削除に失敗しました。', isError: true });
				return;
			}
			setAccounts((list) => list.filter((item) => item.id !== id));
			setStatus({ message: '削除しました。', isError: false });
		} catch {
			setStatus({ message: '削除に失敗しました。', isError: true });
		}
	}

	return html`
		<div class="grid gap-10 sm:grid-cols-2 sm:gap-8">
			<form class="flex flex-col gap-3" onSubmit=${onSubmit}>
				<fieldset class="fieldset gap-1.5">
					<label class="label text-sm font-medium text-base-content" for="email">アカウントを追加</label>
					<input
						id="email"
						class="input input-sm w-full"
						name="email"
						type="email"
						required
						autocomplete="email"
						placeholder="hello@example.com"
						value=${email}
						onInput=${(event) => setEmail(event.currentTarget.value)}
						aria-describedby="form-status"
					/>
				</fieldset>
				<button class="btn btn-primary btn-sm w-fit" type="submit" disabled=${submitting}>
					追加する
				</button>
				<p
					id="form-status"
					class=${`min-h-5 text-sm ${status.isError ? 'text-error' : 'text-success'}`}
					role=${status.isError ? 'alert' : undefined}
					aria-live=${status.isError ? undefined : 'polite'}
				>
					${status.message}
				</p>
			</form>

			<div class="flex flex-col gap-3">
				<div class="flex items-baseline justify-between gap-3">
					<h3 class="text-sm font-medium">登録済み</h3>
					<span class="text-base-content/40 text-xs tabular-nums">${accounts.length}</span>
				</div>
				${accounts.length === 0
					? html`<p class="text-base-content/45 text-sm">まだ登録されていません。</p>`
					: html`
							<ul class="divide-base-300 divide-y">
								${accounts.map(
									(item) => html`
										<li key=${item.id} class="flex items-center justify-between gap-3 py-2.5">
											<span class="truncate text-sm">${item.email}</span>
											<button
												class="btn btn-ghost btn-xs text-base-content/40"
												type="button"
												aria-label=${`${item.email}を削除`}
												onClick=${() => onDelete(item.id)}
											>
												削除
											</button>
										</li>
									`,
								)}
							</ul>
						`}
			</div>
		</div>
	`;
}

AccountPanel.observedAttributes = ['accounts'];
register(AccountPanel, 'account-panel', ['accounts']);
