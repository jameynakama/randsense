import { isHttpError } from '@sveltejs/kit';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { load } from './+page.server';

type Event = Parameters<typeof load>[0];

async function statusFor(fetch: typeof globalThis.fetch): Promise<number> {
	try {
		await load({ params: { id: 'aaaaaaaa' }, fetch } as unknown as Event);
	} catch (e) {
		if (isHttpError(e)) return e.status;
		throw e;
	}
	return 200;
}

describe('permalink load', () => {
	afterEach(() => vi.restoreAllMocks());

	it('is a 404 for an unknown sentence', async () => {
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
