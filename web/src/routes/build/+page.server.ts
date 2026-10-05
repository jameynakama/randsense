import { getJSON } from '#lib/server/api.js';
import type { Sentence } from '#lib/types.js';
import type { PageServerLoad } from './$types';

// ?from= remixes a saved sentence; without it the builder starts empty.
export const load: PageServerLoad = async ({ url, fetch }) => {
	const from = url.searchParams.get('from');
	return {
		from: from
			? await getJSON<Sentence>(fetch, `/api/v1/sentences/${encodeURIComponent(from)}`)
			: null
	};
};
