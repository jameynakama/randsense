import { page } from 'vitest/browser';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import StarButton from './StarButton.svelte';

const answer = (count: number, status = 200) =>
	new Response(JSON.stringify({ count }), {
		status,
		headers: { 'Content-Type': 'application/json' }
	});

describe('StarButton', () => {
	beforeEach(() => localStorage.clear());
	afterEach(() => vi.unstubAllGlobals());

	it('stars with the voter token and shows the new count', async () => {
		const fetch = vi.fn(async () => answer(3));
		vi.stubGlobal('fetch', fetch);
		render(StarButton, { id: 'aaaaaaaa', count: 2 });

		await page.getByRole('button', { name: 'Star, 2 stars' }).click();

		const pressed = page.getByRole('button', { name: 'Star, 3 stars' });
		await expect.element(pressed).toHaveAttribute('aria-pressed', 'true');
		const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
		expect(url).toBe('/api/v1/sentences/aaaaaaaa/stars');
		expect(init.method).toBe('POST');
		expect((init.headers as Record<string, string>)['X-Voter']).toMatch(/^[0-9a-f-]{36}$/);
	});

	it('unstars a sentence this browser starred', async () => {
		localStorage.setItem('randsense:starred', JSON.stringify(['aaaaaaaa']));
		const fetch = vi.fn(async () => answer(1));
		vi.stubGlobal('fetch', fetch);
		render(StarButton, { id: 'aaaaaaaa', count: 2 });

		const button = page.getByRole('button', { name: 'Star, 2 stars' });
		await expect.element(button).toHaveAttribute('aria-pressed', 'true');
		await button.click();
		await expect
			.element(page.getByRole('button', { name: 'Star, 1 star' }))
			.toHaveAttribute('aria-pressed', 'false');
		expect((fetch.mock.calls[0] as unknown as [string, RequestInit])[1].method).toBe('DELETE');
	});

	it('keeps its state and says so when the request fails', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => answer(0, 500))
		);
		render(StarButton, { id: 'aaaaaaaa', count: 2 });

		await page.getByRole('button', { name: 'Star, 2 stars' }).click();

		await expect.element(page.getByRole('status')).toHaveTextContent('Couldn’t save your star');
		await expect
			.element(page.getByRole('button', { name: 'Star, 2 stars' }))
			.toHaveAttribute('aria-pressed', 'false');
	});

	it('sends one request for a double click', async () => {
		let release!: () => void;
		const fetch = vi.fn(() => new Promise<Response>((r) => (release = () => r(answer(3)))));
		vi.stubGlobal('fetch', fetch);
		render(StarButton, { id: 'aaaaaaaa', count: 2 });
		const button = page.getByRole('button', { name: 'Star, 2 stars' });

		await button.click();
		await button.click({ force: true });
		release();

		await expect.element(page.getByRole('button', { name: 'Star, 3 stars' })).toBeInTheDocument();
		expect(fetch).toHaveBeenCalledOnce();
	});
});
