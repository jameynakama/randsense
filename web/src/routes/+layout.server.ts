import { getJSON } from '#lib/server/api.js';
import type { Grammar } from '#lib/types.js';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ fetch }) => ({
	grammar: await getJSON<Grammar>(fetch, '/api/v1/grammar')
});
