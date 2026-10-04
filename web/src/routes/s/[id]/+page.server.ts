import { error } from '@sveltejs/kit';
import { API_ORIGIN } from '$app/env/private';
import type { Sentence } from '#lib/types.js';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, fetch }) => {
	let res: Response;
	try {
		res = await fetch(`${API_ORIGIN}/api/v1/sentences/${encodeURIComponent(params.id)}`);
	} catch {
		error(502, 'The sentence service is unavailable.');
	}
	if (res.status === 404) error(404, 'That sentence doesn’t exist.');
	if (!res.ok) error(502, 'The sentence service is unavailable.');
	return { sentence: (await res.json()) as Sentence };
};
