import { page } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { FakeEventSource } from '#lib/testing/eventsource.js';
import { another, sentence } from '#lib/testing/fixtures.js';
import type { Sentence } from '#lib/types.js';
import Feed from './Feed.svelte';

const second = another('bbbbbbbb', 'The moon sang.');
const third = another('cccccccc', 'A spoon wept.');
const texts = () =>
	page
		.getByRole('list')
		.getByRole('link')
		.all()
		.map((l) => l.element().textContent);

function answering(list: Sentence[]) {
	return vi.fn(
		async () =>
			new Response(JSON.stringify(list), { headers: { 'Content-Type': 'application/json' } })
	);
}

describe('Feed', () => {
	beforeEach(() => vi.stubGlobal('EventSource', FakeEventSource));
	afterEach(() => vi.unstubAllGlobals());

	it('links each sentence to its permalink', async () => {
		render(Feed, { initial: [sentence] });

		await expect
			.element(page.getByRole('link', { name: sentence.text }))
			.toHaveAttribute('href', '/s/aaaaaaaa');
	});

	it('puts a streamed sentence on top', async () => {
		render(Feed, { initial: [sentence] });

		FakeEventSource.latest!.emit('sentence', second);

		await expect.element(page.getByRole('link', { name: second.text })).toBeInTheDocument();
		expect(texts()).toEqual([second.text, sentence.text]);
	});

	it('catches up on every connection and shows each sentence once', async () => {
		const fetch = answering([second, sentence]);
		vi.stubGlobal('fetch', fetch);
		render(Feed, { initial: [sentence] });
		const source = FakeEventSource.latest!;

		source.open();
		source.emit('sentence', second);
		await expect.element(page.getByRole('link', { name: second.text })).toBeInTheDocument();
		source.open();

		await vi.waitFor(() => expect(fetch).toHaveBeenCalledTimes(2));
		expect(fetch).toHaveBeenCalledWith('/api/v1/sentences?limit=30');
		expect(texts()).toEqual([second.text, sentence.text]);
	});

	it('keeps a sentence that streams in while it catches up', async () => {
		let release!: () => void;
		vi.stubGlobal(
			'fetch',
			vi.fn(
				() =>
					new Promise<Response>(
						(r) =>
							(release = () =>
								r(
									new Response(JSON.stringify([sentence]), {
										headers: { 'Content-Type': 'application/json' }
									})
								))
					)
			)
		);
		render(Feed, { initial: [sentence] });
		const source = FakeEventSource.latest!;

		// The catch-up's page was read before the newer sentence was saved.
		source.open();
		source.emit('sentence', { ...second, created_at: '2026-10-04T12:00:00Z' });
		await expect.element(page.getByRole('link', { name: second.text })).toBeInTheDocument();
		release();

		await new Promise((r) => setTimeout(r, 50));

		expect(texts()).toEqual([second.text, sentence.text]);
	});

	it('holds new sentences while paused', async () => {
		render(Feed, { initial: [sentence] });
		const pause = page.getByRole('button', { name: 'Pause feed' });

		await pause.click();
		await expect.element(pause).toHaveAttribute('aria-pressed', 'true');
		FakeEventSource.latest!.emit('sentence', second);
		FakeEventSource.latest!.emit('sentence', third);

		await expect.element(page.getByText('2 new sentences waiting')).toBeInTheDocument();
		expect(texts()).toEqual([sentence.text]);

		await pause.click();
		await expect.element(page.getByRole('link', { name: third.text })).toBeInTheDocument();
		expect(texts()).toEqual([third.text, second.text, sentence.text]);
	});

	it('catches up into the waiting sentences while paused', async () => {
		vi.stubGlobal('fetch', answering([second, sentence]));
		render(Feed, { initial: [sentence] });

		await page.getByRole('button', { name: 'Pause feed' }).click();
		FakeEventSource.latest!.open();

		await expect.element(page.getByText('1 new sentence waiting')).toBeInTheDocument();
		expect(texts()).toEqual([sentence.text]);
	});

	it('updates star counts in place and passes them on', async () => {
		const onstars = vi.fn();
		render(Feed, { initial: [sentence], onstars });

		FakeEventSource.latest!.emit('stars', { id: 'aaaaaaaa', count: 9 });

		await expect.element(page.getByRole('button', { name: 'Star, 9 stars' })).toBeInTheDocument();
		expect(onstars).toHaveBeenCalledWith({ id: 'aaaaaaaa', count: 9 });
	});

	it('stops listening when it goes away', async () => {
		const { unmount } = render(Feed, { initial: [sentence] });

		unmount();

		expect(FakeEventSource.latest!.closed).toBe(true);
	});

	it('marks built sentences Homemade', async () => {
		render(Feed, { initial: [{ ...second, origin: 'built' }, third] });

		await expect.element(page.getByText('Homemade')).toBeInTheDocument();
		expect(page.getByText('Homemade').all()).toHaveLength(1);
	});
});
