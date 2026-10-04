import type { Sentence } from './types';

export const FEED_SIZE = 30;

// prepend puts newer sentences, newest first, on top of a feed. It skips any
// the feed already has and keeps the newest FEED_SIZE.
export function prepend(feed: Sentence[], newer: Sentence[]): Sentence[] {
	const have = new Set(feed.map((s) => s.id));
	const fresh = newer.filter((s) => !have.has(s.id));
	return [...fresh, ...feed].slice(0, FEED_SIZE);
}

// catchUp merges a fresh page from the API into the feed. The page has
// current star counts, but the stream may have delivered newer sentences
// while it loaded, so those stay on top.
export function catchUp(feed: Sentence[], page: Sentence[]): Sentence[] {
	const newest = page.length ? Date.parse(page[0].created_at) : -Infinity;
	return prepend(
		page,
		feed.filter((s) => Date.parse(s.created_at) > newest)
	);
}
