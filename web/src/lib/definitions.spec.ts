import { describe, expect, it, vi } from 'vitest';
import { definitions, wiktionaryUrl } from './definitions';

describe('definitions', () => {
	it('asks for the lemma escaped and returns its glosses', async () => {
		const fetch = vi.fn(async () => Response.json({ definitions: ['a polyp'] }));

		expect(await definitions(fetch, 'noun', 'sea anemone')).toEqual(['a polyp']);
		expect(fetch).toHaveBeenCalledWith('/api/v1/words/noun/sea%20anemone');
	});

	it('has none for a word the lexicon lacks', async () => {
		const fetch = vi.fn(async () => new Response(null, { status: 404 }));

		expect(await definitions(fetch, 'noun', 'unicorn')).toEqual([]);
	});

	it('throws when the API fails', async () => {
		const fetch = vi.fn(async () => new Response(null, { status: 500 }));

		await expect(definitions(fetch, 'noun', 'goose')).rejects.toThrow();
	});
});

describe('wiktionaryUrl', () => {
	it.each([
		['goose', 'https://en.wiktionary.org/wiki/goose#English'],
		['sea anemone', 'https://en.wiktionary.org/wiki/sea_anemone#English'],
		['Hopi', 'https://en.wiktionary.org/wiki/Hopi#English'],
		["o'clock", "https://en.wiktionary.org/wiki/o'clock#English"],
		['café', 'https://en.wiktionary.org/wiki/caf%C3%A9#English']
	])('links %s to its English entry', (lemma, url) => {
		expect(wiktionaryUrl(lemma)).toBe(url);
	});
});
