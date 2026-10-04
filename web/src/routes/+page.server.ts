import { FEED_SIZE } from '#lib/feed.js';
import { getJSON } from '#lib/server/api.js';
import type { Sentence } from '#lib/types.js';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => ({
	sentences: await getJSON<Sentence[]>(fetch, `/api/v1/sentences?limit=${FEED_SIZE}`)
});
