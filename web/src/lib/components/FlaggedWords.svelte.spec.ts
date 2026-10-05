import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import FlaggedWords from './FlaggedWords.svelte';

describe('FlaggedWords', () => {
	it('lists each word with its part of speech and count', async () => {
		render(FlaggedWords, {
			words: [
				{ lemma: 'record', pos: 'Noun', count: 4 },
				{ lemma: 'record', pos: 'Verb', count: 2 }
			]
		});

		const rows = page.getByRole('row');
		await expect.element(rows.nth(0)).toHaveTextContent('WordPart of speechFlags');
		await expect.element(rows.nth(1)).toHaveTextContent('recordnoun4');
		await expect.element(rows.nth(2)).toHaveTextContent('recordverb2');
	});

	it('says when no words are flagged', async () => {
		render(FlaggedWords, { words: [] });

		await expect.element(page.getByText('No words have been flagged yet.')).toBeVisible();
	});
});
