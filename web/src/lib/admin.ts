import { leaves } from './tree';
import type { Flag, FlaggedWord } from './types';

export const ADMIN_PAGE = 30;

// LoggedOut means the admin session has ended and the admin must log in
// again.
export class LoggedOut extends Error {}

// login reports whether the API accepted password, which sets the session
// cookie. It throws when the API can't answer.
export async function login(fetch: typeof globalThis.fetch, password: string): Promise<boolean> {
	const res = await fetch('/api/v1/admin/login', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ password })
	});
	if (res.status === 401) return false;
	if (!res.ok) throw new Error(`status ${res.status}`);
	return true;
}

export async function logout(fetch: typeof globalThis.fetch): Promise<void> {
	const res = await fetch('/api/v1/admin/logout', { method: 'POST' });
	// An ended session is already logged out.
	if (!res.ok && res.status !== 401) throw new Error(`status ${res.status}`);
}

async function list<T>(fetch: typeof globalThis.fetch, path: string, offset: number): Promise<T[]> {
	const res = await fetch(`/api/v1/admin/${path}?limit=${ADMIN_PAGE}&offset=${offset}`);
	if (res.status === 401) throw new LoggedOut();
	if (!res.ok) throw new Error(`status ${res.status}`);
	return res.json();
}

// flags fetches a page of flags, newest first.
export const flags = (fetch: typeof globalThis.fetch, offset: number) =>
	list<Flag>(fetch, 'flags', offset);

// flaggedWords fetches a page of flagged words, most flags first.
export const flaggedWords = (fetch: typeof globalThis.fetch, offset: number) =>
	list<FlaggedWord>(fetch, 'flagged-words', offset);

// flaggedWord is the flagged word as the sentence writes it, or null when
// the whole sentence was flagged.
export function flaggedWord(flag: Flag): string | null {
	if (flag.word_index === null) return null;
	const { node } = leaves(flag.sentence.tree)[flag.word_index];
	return node.display ?? node.word ?? null;
}

// wordKey tells flagged words apart: a lemma can be flagged as a noun and
// as a verb.
export const wordKey = (w: FlaggedWord) => `${w.lemma}/${w.pos}`;

// appendNew adds the items of next that shown lacks. New flags shift later
// pages, so the next page can repeat what's already shown.
export function appendNew<T>(shown: T[], next: T[], key: (t: T) => string | number): T[] {
	const have = new Set(shown.map(key));
	return [...shown, ...next.filter((t) => !have.has(key(t)))];
}
