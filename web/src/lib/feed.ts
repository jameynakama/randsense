import type { Sentence } from './types';

export const FEED_SIZE = 30;

// prepend puts newer sentences, newest first, on top of a feed. It skips any
// the feed already has and keeps the newest FEED_SIZE.
export function prepend(feed: Sentence[], newer: Sentence[]): Sentence[] {
	const have = new Set(feed.map((s) => s.id));
	const fresh = newer.filter((s) => !have.has(s.id));
	return [...fresh, ...feed].slice(0, FEED_SIZE);
}
