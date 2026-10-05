import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { flag } from '#lib/testing/fixtures.js';
import FlagList from './FlagList.svelte';

describe('FlagList', () => {
	it('shows the sentence, the flagged word and the comment', async () => {
		render(FlagList, { flags: [flag] });

		const item = page.getByRole('listitem');
		await expect
			.element(item.getByRole('link', { name: flag.sentence.text }))
			.toHaveAttribute('href', '/s/aaaaaaaa');
		await expect.element(item).toHaveTextContent('About “she” (she, pronoun)');
		await expect.element(item).toHaveTextContent('Who is she?');
		await expect
			.element(item.getByRole('time'))
			.toHaveAttribute('datetime', '2026-10-04T09:30:00Z');
	});

	it('says when the whole sentence was flagged', async () => {
		render(FlagList, { flags: [{ ...flag, word_index: null, lemma: null, pos: null }] });

		await expect.element(page.getByRole('listitem')).toHaveTextContent('About the whole sentence');
	});

	it('says when there are no flags', async () => {
		render(FlagList, { flags: [] });

		await expect.element(page.getByText('No one has flagged anything yet.')).toBeVisible();
	});
});
