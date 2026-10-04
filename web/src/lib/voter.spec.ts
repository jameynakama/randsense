import { describe, expect, it } from 'vitest';
import { isStarred, setStarred, voterToken } from './voter';

function store(initial: Record<string, string> = {}) {
	const m = new Map(Object.entries(initial));
	return {
		getItem: (k: string) => m.get(k) ?? null,
		setItem: (k: string, v: string) => void m.set(k, v)
	};
}

describe('voterToken', () => {
	it('makes a UUID once and keeps it', () => {
		const s = store();
		const first = voterToken(s);
		expect(first).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
		expect(voterToken(s)).toBe(first);
	});
});

describe('starred sentences', () => {
	it('remembers stars and unstars', () => {
		const s = store();
		setStarred('aaaaaaaa', true, s);
		setStarred('bbbbbbbb', true, s);
		setStarred('aaaaaaaa', false, s);
		expect(isStarred('aaaaaaaa', s)).toBe(false);
		expect(isStarred('bbbbbbbb', s)).toBe(true);
	});

	it('treats a corrupt list as empty', () => {
		const s = store({ 'randsense:starred': 'not json' });
		expect(isStarred('aaaaaaaa', s)).toBe(false);
		setStarred('aaaaaaaa', true, s);
		expect(isStarred('aaaaaaaa', s)).toBe(true);
	});
});

describe('without usable browser storage', () => {
	it('still gives one token for the life of the page', () => {
		// Node has no window, so this is the path a browser with blocked site
		// data takes when localStorage throws.
		const first = voterToken();
		expect(voterToken()).toBe(first);
		setStarred('cccccccc', true);
		expect(isStarred('cccccccc')).toBe(true);
	});
});

describe('storage that refuses writes', () => {
	it('keeps the token and stars in memory instead', () => {
		const full = {
			getItem: () => null,
			setItem: () => {
				throw new DOMException('full', 'QuotaExceededError');
			}
		};

		const first = voterToken(full);
		expect(voterToken(full)).toBe(first);
		setStarred('dddddddd', true, full);
		expect(isStarred('dddddddd', full)).toBe(true);
	});
});
