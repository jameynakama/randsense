import type { Sentence } from './types';
import { voterToken } from './voter';

export const STARS_PAGE = 30;

// starred fetches a page of the sentences this browser starred, newest star
// first. It throws when the API can't answer.
export async function starred(fetch: typeof globalThis.fetch, offset: number): Promise<Sentence[]> {
	const res = await fetch(`/api/v1/stars?limit=${STARS_PAGE}&offset=${offset}`, {
		headers: { 'X-Voter': voterToken() }
	});
	if (!res.ok) throw new Error(`status ${res.status}`);
	return res.json();
}
