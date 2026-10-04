import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { sentence } from '#lib/testing/fixtures.js';
import Sentence from './Sentence.svelte';

describe('Sentence', () => {
	it('labels its group with the whole sentence and makes words buttons', async () => {
		render(Sentence, { sentence });

		await expect.element(page.getByRole('group', { name: sentence.text })).toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: 'The' })).toBeInTheDocument();
		await expect.element(page.getByRole('button', { name: ',' })).not.toBeInTheDocument();
		expect(page.getByRole('button').all()).toHaveLength(7);
	});

	it('selects a word and unselects it on a second press', async () => {
		render(Sentence, { sentence });
		const goose = page.getByRole('button', { name: 'goose' });

		await goose.click();
		await expect.element(goose).toHaveAttribute('aria-pressed', 'true');
		await goose.click();
		await expect.element(goose).toHaveAttribute('aria-pressed', 'false');
	});

	it('wraps a long word on a narrow phone instead of scrolling sideways', async () => {
		await page.viewport(320, 640);
		const long = structuredClone(sentence);
		long.tree.children![0].children![0].children![1].word =
			'pneumonoultramicroscopicsilicovolcanoconiosis';
		const { container } = render(Sentence, { sentence: long });

		expect(container.scrollWidth).toBeLessThanOrEqual(320);
	});
});
