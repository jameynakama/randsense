import { describe, expect, it, vi } from 'vitest';
import {
	ADMIN_PAGE,
	LoggedOut,
	appendNew,
	flaggedWord,
	flaggedWords,
	flags,
	login,
	logout
} from './admin';
import { flag } from './testing/fixtures';

const answer = (status: number, body: unknown = null) =>
	vi.fn(
		async () =>
			new Response(body === null ? null : JSON.stringify(body), {
				status,
				headers: { 'Content-Type': 'application/json' }
			})
	);

describe('login', () => {
	it('posts the password and reports success', async () => {
		const fetch = answer(204);

		expect(await login(fetch, 'randsense')).toBe(true);
		const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
		expect(url).toBe('/api/v1/admin/login');
		expect(init.method).toBe('POST');
		expect(JSON.parse(init.body as string)).toEqual({ password: 'randsense' });
	});

	it('reports a wrong password', async () => {
		expect(await login(answer(401, { error: 'wrong password' }), 'nope')).toBe(false);
	});

	it('throws when the API can’t answer', async () => {
		await expect(login(answer(502), 'randsense')).rejects.toThrow();
	});
});

describe('logout', () => {
	it('treats an ended session as logged out', async () => {
		await expect(logout(answer(401))).resolves.toBeUndefined();
	});

	it('throws when the API can’t answer', async () => {
		await expect(logout(answer(500))).rejects.toThrow();
	});
});

describe('admin lists', () => {
	it('asks for a page of flags at an offset', async () => {
		const fetch = answer(200, [flag]);

		expect(await flags(fetch, 30)).toEqual([flag]);
		expect(fetch.mock.calls[0]).toEqual([`/api/v1/admin/flags?limit=${ADMIN_PAGE}&offset=30`]);
	});

	it('asks for a page of flagged words', async () => {
		const fetch = answer(200, []);

		await flaggedWords(fetch, 0);
		expect(fetch.mock.calls[0]).toEqual([
			`/api/v1/admin/flagged-words?limit=${ADMIN_PAGE}&offset=0`
		]);
	});

	it('throws LoggedOut when the session has ended', async () => {
		await expect(flags(answer(401), 0)).rejects.toBeInstanceOf(LoggedOut);
	});

	it('throws a plain error for other failures', async () => {
		const failure = flaggedWords(answer(500), 0);
		await expect(failure).rejects.toThrow();
		await expect(failure).rejects.not.toBeInstanceOf(LoggedOut);
	});
});

describe('flaggedWord', () => {
	it('counts the comma when finding the flagged leaf', () => {
		expect(flaggedWord(flag)).toBe('she');
	});

	it('is null for a whole-sentence flag', () => {
		expect(flaggedWord({ ...flag, word_index: null, lemma: null, pos: null })).toBeNull();
	});
});

describe('appendNew', () => {
	it('adds only what isn’t shown, in order', () => {
		const shown = [{ id: 3 }, { id: 2 }];
		const next = [{ id: 2 }, { id: 1 }, { id: 0 }];

		expect(appendNew(shown, next, (t) => t.id)).toEqual([
			{ id: 3 },
			{ id: 2 },
			{ id: 1 },
			{ id: 0 }
		]);
	});
});
