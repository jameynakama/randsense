import { error } from '@sveltejs/kit';
import { starred } from '#lib/stars.js';
import type { PageLoad } from './$types';

// Stars belong to the voter token, which only the browser has.
export const ssr = false;

export const load: PageLoad = async ({ fetch }) => {
	try {
		return { sentences: await starred(fetch, 0) };
	} catch {
		error(502, 'The sentence service is unavailable.');
	}
};
