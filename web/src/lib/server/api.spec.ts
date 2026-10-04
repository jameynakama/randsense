import { isHttpError } from '@sveltejs/kit';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { getJSON } from './api';

async function statusFor(fetch: typeof globalThis.fetch): Promise<number> {
	try {
		await getJSON(fetch, '/api/v1/sentences/aaaaaaaa');
	} catch (e) {
		if (isHttpError(e)) return e.status;
		throw e;
	}
	return 200;
}

describe('getJSON', () => {
	afterEach(() => vi.restoreAllMocks());

	it('reads the API at API_ORIGIN', async () => {
		const fetch = vi.fn(async () => new Response('{"id": "aaaaaaaa"}'));

		expect(await getJSON(fetch, '/api/v1/sentences/aaaaaaaa')).toEqual({ id: 'aaaaaaaa' });
		expect(fetch).toHaveBeenCalledWith(
			'http://localhost:8080/api/v1/sentences/aaaaaaaa',
			expect.anything()
		);
	});

	it('is a 404 for something that is not there', async () => {
		expect(await statusFor(async () => new Response('{}', { status: 404 }))).toBe(404);
	});

	it('is a 502 when the API is down or failing', async () => {
		expect(
			await statusFor(async () => {
				throw new TypeError('fetch failed');
			})
		).toBe(502);
		expect(await statusFor(async () => new Response('{}', { status: 500 }))).toBe(502);
	});

	it('is a 502 when the API accepts the request but never answers', async () => {
		vi.spyOn(AbortSignal, 'timeout').mockReturnValue(AbortSignal.abort());
		const hang = (_: unknown, init?: RequestInit) =>
			new Promise<Response>((_, reject) => {
				const signal = init?.signal;
				if (signal?.aborted) reject(signal.reason);
				signal?.addEventListener('abort', () => reject(signal.reason));
			});

		expect(await statusFor(hang as typeof globalThis.fetch)).toBe(502);
	}, 2000);
});
