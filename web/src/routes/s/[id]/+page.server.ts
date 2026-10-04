import { getJSON } from '#lib/server/api.js';
import type { Sentence } from '#lib/types.js';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, fetch }) => ({
	sentence: await getJSON<Sentence>(fetch, `/api/v1/sentences/${encodeURIComponent(params.id)}`)
});
