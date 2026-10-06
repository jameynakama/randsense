import { page, userEvent } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { another, grammar, sentence } from '#lib/testing/fixtures.js';
import SentenceView from './SentenceView.svelte';

describe('SentenceView', () => {
	// Word cards look up definitions; these tests don't need any.
	beforeEach(() =>
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => new Response(null, { status: 404 }))
		)
	);
	afterEach(() => vi.unstubAllGlobals());

	it('opens a word’s card and keeps the diagram closed until asked', async () => {
		render(SentenceView, { sentence, grammar, count: 2 });

		await page.getByRole('button', { name: 'goose' }).click();

		await expect.element(page.getByRole('region', { name: 'goose' })).toBeInTheDocument();
		await expect
			.element(page.getByRole('button', { name: 'Show diagram' }))
			.toHaveAttribute('aria-expanded', 'false');
	});

	it('opens the flag form on the selected word', async () => {
		render(SentenceView, { sentence, grammar, count: 2 });

		await page.getByRole('button', { name: 'goose' }).click();
		await page.getByRole('button', { name: 'Something’s wrong' }).click();

		await expect.element(page.getByLabelText('About')).toHaveDisplayValue('goose (word 2)');
		await expect.element(page.getByRole('region', { name: 'goose' })).not.toBeInTheDocument();
	});

	it('retargets the open flag form when a word is pressed', async () => {
		render(SentenceView, { sentence, grammar, count: 2 });

		await page.getByRole('button', { name: 'Something’s wrong' }).click();
		await expect.element(page.getByLabelText('About')).toHaveDisplayValue('The whole sentence');
		await page.getByRole('button', { name: 'sang' }).click();

		await expect.element(page.getByLabelText('About')).toHaveDisplayValue('sang (word 7)');
		await expect
			.element(page.getByRole('button', { name: 'sang' }))
			.toHaveAttribute('aria-pressed', 'true');
	});

	it('returns focus to the button that opened the flag form', async () => {
		render(SentenceView, { sentence, grammar, count: 2 });
		const opener = page.getByRole('button', { name: 'Something’s wrong' });

		await opener.click();
		await expect.element(page.getByRole('heading', { name: 'Report a problem' })).toHaveFocus();
		await userEvent.keyboard('{Escape}');

		await expect.element(opener).toHaveFocus();
		await expect.element(opener).toHaveAttribute('aria-expanded', 'false');
	});

	it('opens no word card when the flag form closes', async () => {
		render(SentenceView, { sentence, grammar, count: 2 });

		await page.getByRole('button', { name: 'Something’s wrong' }).click();
		await page.getByRole('button', { name: 'goose' }).click();
		await userEvent.keyboard('{Escape}');

		await expect.element(page.getByRole('region')).not.toBeInTheDocument();
	});

	it('starts over when another sentence takes its place', async () => {
		const { rerender } = render(SentenceView, { sentence, grammar, count: 2 });

		await page.getByRole('button', { name: 'goose' }).click();
		await rerender({ sentence: another('bbbbbbbb', 'The goose devoured her, but she sang.') });

		await expect.element(page.getByRole('region')).not.toBeInTheDocument();
	});

	it('draws the diagram with the selected word marked', async () => {
		render(SentenceView, { sentence, grammar, count: 2 });

		await page.getByRole('button', { name: 'goose' }).click();
		await page.getByRole('button', { name: 'Show diagram' }).click();

		const diagram = page.getByRole('list', { name: 'Sentence diagram' });
		await expect.element(diagram).toBeVisible();
		expect(diagram.getByText('goose').element().getAttribute('aria-current')).toBe('true');
		await expect
			.element(page.getByRole('button', { name: 'Hide diagram' }))
			.toHaveAttribute('aria-expanded', 'true');
	});

	it('redraws an open diagram for the next sentence', async () => {
		const screen = render(SentenceView, { sentence, grammar, count: 2 });
		await page.getByRole('button', { name: 'Show diagram' }).click();

		const next = another('bbbbbbbb', 'The goose devoured her, but she wept.');
		next.tree.children![3].children![1].children![0].word = 'wept';
		await screen.rerender({ sentence: next, grammar, count: 0 });

		const diagram = page.getByRole('list', { name: 'Sentence diagram' });
		await expect.element(diagram.getByText('wept')).toBeVisible();
		await expect.element(diagram.getByText('sang')).not.toBeInTheDocument();
	});

	it('marks a built sentence Homemade, and only a built one', async () => {
		const { rerender } = render(SentenceView, { sentence, grammar, count: 2 });
		await expect.element(page.getByText('Homemade')).not.toBeInTheDocument();

		await rerender({ sentence: { ...sentence, origin: 'built' } });
		await expect.element(page.getByText('Homemade')).toBeInTheDocument();
	});

	it('links to a remix of the sentence', async () => {
		render(SentenceView, { sentence, grammar, count: 2 });

		await expect
			.element(page.getByRole('link', { name: 'Remix', exact: true }))
			.toHaveAttribute('href', '/build?from=aaaaaaaa');
	});
});
