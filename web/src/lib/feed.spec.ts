import { describe, expect, it } from 'vitest';
import { FEED_SIZE, prepend } from './feed';
import type { Sentence } from './types';

const s = (id: string): Sentence => ({
	id,
	text: id,
	tree: { symbol: 'S' },
	star_count: 0,
	created_at: '2026-10-04T12:00:00Z'
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
