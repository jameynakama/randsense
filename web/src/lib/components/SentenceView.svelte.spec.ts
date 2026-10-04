import { page, userEvent } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { another, sentence } from '#lib/testing/fixtures.js';
import SentenceView from './SentenceView.svelte';

describe('SentenceView', () => {
	it('opens a word’s card and keeps the structure closed until asked', async () => {
		render(SentenceView, { sentence, count: 2 });

		await page.getByRole('button', { name: 'goose' }).click();

		await expect.element(page.getByRole('region', { name: 'goose' })).toBeInTheDocument();
		await expect
			.element(page.getByRole('button', { name: 'Show structure' }))
			.toHaveAttribute('aria-expanded', 'false');
	});

	it('opens the flag form on the selected word', async () => {
		render(SentenceView, { sentence, count: 2 });

		await page.getByRole('button', { name: 'goose' }).click();
		await page.getByRole('button', { name: 'Something’s wrong' }).click();

		await expect.element(page.getByLabelText('About')).toHaveDisplayValue('goose (word 2)');
		await expect.element(page.getByRole('region', { name: 'goose' })).not.toBeInTheDocument();
	});

	it('retargets the open flag form when a word is pressed', async () => {
		render(SentenceView, { sentence, count: 2 });

		await page.getByRole('button', { name: 'Something’s wrong' }).click();
		await expect.element(page.getByLabelText('About')).toHaveDisplayValue('The whole sentence');
		await page.getByRole('button', { name: 'sang' }).click();

		await expect.element(page.getByLabelText('About')).toHaveDisplayValue('sang (word 7)');
		await expect
			.element(page.getByRole('button', { name: 'sang' }))
			.toHaveAttribute('aria-pressed', 'true');
	});

	it('returns focus to the button that opened the flag form', async () => {
		render(SentenceView, { sentence, count: 2 });
		const opener = page.getByRole('button', { name: 'Something’s wrong' });

		await opener.click();
		await expect.element(page.getByRole('heading', { name: 'Report a problem' })).toHaveFocus();
		await userEvent.keyboard('{Escape}');

		await expect.element(opener).toHaveFocus();
		await expect.element(opener).toHaveAttribute('aria-expanded', 'false');
	});

	it('starts over when another sentence takes its place', async () => {
		const { rerender } = render(SentenceView, { sentence, count: 2 });

		await page.getByRole('button', { name: 'goose' }).click();
		await rerender({ sentence: another('bbbbbbbb', 'The goose devoured her, but she sang.') });

		await expect.element(page.getByRole('region')).not.toBeInTheDocument();
	});
});
