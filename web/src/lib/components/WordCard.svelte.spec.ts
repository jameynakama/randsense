import { page, userEvent } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { sentence } from '#lib/testing/fixtures.js';
import '../../app.css';
import WordCard from './WordCard.svelte';

describe('WordCard', () => {
	const answer = (defs: string[] | null) => async () =>
		defs ? Response.json({ definitions: defs }) : new Response(null, { status: 404 });

	beforeEach(() => vi.stubGlobal('fetch', vi.fn(answer(null))));
	afterEach(() => vi.unstubAllGlobals());

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

	it('shows three definitions, then all of them on request', async () => {
		const fetch = vi.fn(answer(['bird', 'fool', 'poke', 'cook', 'tailor']));
		vi.stubGlobal('fetch', fetch);
		render(WordCard, { tree: sentence.tree, index: 1, onclose: () => {} });

		const list = page.getByRole('list', { name: 'Definitions' });
		await expect.element(list.getByRole('listitem').nth(2)).toHaveTextContent('poke');
		expect(list.getByRole('listitem').elements()).toHaveLength(3);
		expect(fetch).toHaveBeenCalledWith('/api/v1/words/noun/goose');
		await expect.element(page.getByText('Open English WordNet')).toBeInTheDocument();

		const more = page.getByRole('button', { name: 'Show all 5' });
		await expect.element(more).toHaveAttribute('aria-expanded', 'false');
		await more.click();
		await expect.element(list.getByRole('listitem').nth(4)).toHaveTextContent('tailor');
	});

	it('looks up the lemma, not the inflected word', async () => {
		const fetch = vi.fn(answer(['eat greedily']));
		vi.stubGlobal('fetch', fetch);
		render(WordCard, { tree: sentence.tree, index: 2, onclose: () => {} });

		await expect.element(page.getByText('eat greedily')).toBeInTheDocument();
		expect(fetch).toHaveBeenCalledWith('/api/v1/words/verb/devour');
		await expect
			.element(page.getByRole('link', { name: 'Open in Wiktionary' }))
			.toHaveAttribute('href', 'https://en.wiktionary.org/wiki/devour#English');
	});

	it('links a closed-class word to Wiktionary without looking it up', async () => {
		const fetch = vi.fn(answer(['should not appear']));
		vi.stubGlobal('fetch', fetch);
		render(WordCard, { tree: sentence.tree, index: 0, onclose: () => {} });

		const link = page.getByRole('link', { name: 'Open in Wiktionary' });
		await expect
			.element(link)
			.toHaveAttribute('href', 'https://en.wiktionary.org/wiki/the#English');
		await expect.element(link).toHaveAttribute('target', '_blank');
		expect(fetch).not.toHaveBeenCalled();
	});

	it('has no definitions section when there are none or the lookup fails', async () => {
		const { rerender } = render(WordCard, { tree: sentence.tree, index: 1, onclose: () => {} });
		await expect
			.element(page.getByRole('link', { name: 'Open in Wiktionary' }))
			.toBeInTheDocument();
		await expect.element(page.getByText('Definitions')).not.toBeInTheDocument();

		vi.stubGlobal(
			'fetch',
			vi.fn(async () => new Response(null, { status: 500 }))
		);
		await rerender({ index: 2 });
		await expect.element(page.getByRole('heading', { name: 'devoured' })).toBeInTheDocument();
		await expect.element(page.getByText('Definitions')).not.toBeInTheDocument();
	});

	it("drops a word's late definitions once another word is open", async () => {
		let releaseGoose = () => {};
		vi.stubGlobal(
			'fetch',
			vi.fn((url: string) =>
				url.endsWith('/goose')
					? new Promise<Response>(
							// A real Response's body takes a task to read; this answers in
							// microtasks, so one timeout is enough for it to land.
							(r) =>
								(releaseGoose = () =>
									r({
										ok: true,
										status: 200,
										json: async () => ({ definitions: ['bird'] })
									} as Response))
						)
					: Promise.resolve(Response.json({ definitions: ['eat greedily'] }))
			)
		);
		const { rerender } = render(WordCard, { tree: sentence.tree, index: 1, onclose: () => {} });

		await rerender({ index: 2 });
		await expect.element(page.getByText('eat greedily')).toBeInTheDocument();
		releaseGoose();
		await new Promise((r) => setTimeout(r));
		await expect.element(page.getByText('bird')).not.toBeInTheDocument();
	});
});
