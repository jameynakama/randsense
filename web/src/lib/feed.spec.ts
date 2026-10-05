import { describe, expect, it } from 'vitest';
import { catchUp, FEED_SIZE, prepend } from './feed';
import type { Sentence } from './types';

const s = (id: string, created_at = '2026-10-04T12:00:00Z'): Sentence => ({
	origin: 'generated',
	id,
	text: id,
	tree: { symbol: 'S' },
	star_count: 0,
	created_at
});
const ids = (feed: Sentence[]) => feed.map((x) => x.id);

describe('prepend', () => {
	it('puts newer sentences on top', () => {
		expect(ids(prepend([s('b'), s('a')], [s('d'), s('c')]))).toEqual(['d', 'c', 'b', 'a']);
	});

	it('skips sentences the feed already has', () => {
		expect(ids(prepend([s('b'), s('a')], [s('c'), s('b')]))).toEqual(['c', 'b', 'a']);
	});

	it('keeps the newest thirty, however many arrive at once', () => {
		const feed = Array.from({ length: FEED_SIZE }, (_, i) => s(`old${i}`));
		const burst = Array.from({ length: 40 }, (_, i) => s(`new${i}`));

		const got = prepend(feed, burst);

		expect(got).toHaveLength(FEED_SIZE);
		expect(got[0].id).toBe('new0');
		expect(got.at(-1)!.id).toBe('new29');
	});
});

describe('catchUp', () => {
	it('takes the fresh page, with its star counts', () => {
		const fresh = { ...s('a'), star_count: 4 };

		expect(catchUp([s('a')], [fresh])).toEqual([fresh]);
	});

	it('keeps what streamed in after the page was read', () => {
		const page = [s('b', '2026-10-04T12:00:01Z'), s('a')];
		const feed = [s('c', '2026-10-04T12:00:01.5Z'), s('a')];

		expect(ids(catchUp(feed, page))).toEqual(['c', 'b', 'a']);
	});

	it('drops what fell off the page while disconnected', () => {
		const page = [s('z', '2026-10-04T13:00:00Z')];

		expect(ids(catchUp([s('a')], page))).toEqual(['z']);
	});
});
