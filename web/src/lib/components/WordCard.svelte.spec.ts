import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { sentence } from '#lib/testing/fixtures.js';
import WordCard from './WordCard.svelte';

describe('WordCard', () => {
	it('shows the word, lemma, part of speech, frame, role and features', async () => {
		render(WordCard, { tree: sentence.tree, index: 2, onclose: () => {} });

		const card = page.getByRole('region', { name: 'devoured' });
		await expect.element(card).toBeInTheDocument();
		await expect.element(card.getByText('devour', { exact: true })).toBeInTheDocument();
		await expect.element(card.getByText('verb', { exact: true })).toBeInTheDocument();
		await expect.element(card.getByText('transitive', { exact: true })).toBeInTheDocument();
		await expect.element(card.getByText('past tense')).toBeInTheDocument();
		await expect.element(card.getByText('3rd person singular')).toBeInTheDocument();
	});

	it('shows a role when the word has one, and none otherwise', async () => {
		const { rerender } = render(WordCard, { tree: sentence.tree, index: 1, onclose: () => {} });
		await expect.element(page.getByText('in the subject')).toBeInTheDocument();

		await rerender({ index: 4 });
		await expect.element(page.getByText('Role')).not.toBeInTheDocument();
	});

	it('takes focus and closes on Escape', async () => {
		const onclose = vi.fn();
		render(WordCard, { tree: sentence.tree, index: 1, onclose });

		await expect.element(page.getByRole('heading', { name: 'goose' })).toHaveFocus();
		await userEvent.keyboard('{Escape}');
		expect(onclose).toHaveBeenCalledOnce();
	});

	it('wraps a long word on a narrow phone instead of scrolling sideways', async () => {
		await page.viewport(320, 640);
		const long = structuredClone(sentence);
		const goose = long.tree.children![0].children![0].children![1];
		goose.word = goose.lemma = 'pneumonoultramicroscopicsilicovolcanoconiosis';
		const { container } = render(WordCard, { tree: long.tree, index: 1, onclose: () => {} });

		const card = container.querySelector('section')!;
		expect(card.scrollWidth).toBeLessThanOrEqual(card.clientWidth);
	});
});
