import { error } from '@sveltejs/kit';
import { API_ORIGIN } from '$app/env/private';

// A stuck API gets a 502 rather than a page that never loads.
const API_TIMEOUT_MS = 5000;

// getJSON reads path from the Go API for a server-side load. A 404 stays a
// 404. Any other failure, including no answer at all, is a 502.
export async function getJSON<T>(fetch: typeof globalThis.fetch, path: string): Promise<T> {
	let res: Response;
	try {
		res = await fetch(`${API_ORIGIN}${path}`, { signal: AbortSignal.timeout(API_TIMEOUT_MS) });
	} catch {
		error(502, 'The sentence service is unavailable.');
	}
	if (res.status === 404) error(404, 'Nothing is at this address.');
	if (!res.ok) error(502, 'The sentence service is unavailable.');
	return (await res.json()) as T;
}
