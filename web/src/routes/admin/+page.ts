import { error, redirect } from '@sveltejs/kit';
import { LoggedOut, flaggedWords, flags } from '#lib/admin.js';
import type { PageLoad } from './$types';

// The session cookie lives in the browser, so the page loads its data there.
export const ssr = false;

export const load: PageLoad = async ({ fetch }) => {
	try {
		const [f, w] = await Promise.all([flags(fetch, 0), flaggedWords(fetch, 0)]);
		return { flags: f, words: w };
	} catch (e) {
		if (e instanceof LoggedOut) redirect(307, '/admin/login');
		error(502, 'The sentence service is unavailable.');
	}
};
