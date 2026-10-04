import { page, userEvent } from 'vitest/browser';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { sentence } from '#lib/testing/fixtures.js';
import FlagForm from './FlagForm.svelte';

const created = () => new Response('{"id": 1}', { status: 201 });

describe('FlagForm', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('offers the whole sentence and each word, but no commas', async () => {
		render(FlagForm, { sentence, onclose: () => {} });

		const about = page.getByLabelText('About');
		await expect.element(about).toHaveDisplayValue('The whole sentence');
		const options = page.getByRole('option').all();
		expect(options.map((o) => o.element().textContent)).toEqual([
			'The whole sentence',
			'The (word 1)',
			'goose (word 2)',
			'devoured (word 3)',
			'her (word 4)',
			'but (word 5)',
			'she (word 6)',
			'sang (word 7)'
		]);
	});

	it('takes focus and closes on Escape', async () => {
		const onclose = vi.fn();
		render(FlagForm, { sentence, onclose });

		await expect.element(page.getByRole('heading', { name: 'Report a problem' })).toHaveFocus();
		await userEvent.keyboard('{Escape}');
		expect(onclose).toHaveBeenCalledOnce();
	});

	it('sends a word’s flag and thanks the reader', async () => {
		const fetch = vi.fn(async () => created());
		vi.stubGlobal('fetch', fetch);
		// `target` is also a mount option, so the props go in explicitly.
		render(FlagForm, { props: { sentence, target: 2, onclose: () => {} } });

		await expect.element(page.getByLabelText('About')).toHaveDisplayValue('devoured (word 3)');
		await page.getByLabelText('What’s wrong?').fill('  devour needs an object here  ');
		await page.getByRole('button', { name: 'Send' }).click();

		await expect
			.element(page.getByRole('status'))
			.toHaveTextContent('Thanks. Your report was sent.');
		const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
		expect(url).toBe('/api/v1/sentences/aaaaaaaa/flags');
		expect(init.method).toBe('POST');
		expect(JSON.parse(init.body as string)).toEqual({
			comment: 'devour needs an object here',
			word_index: 2
		});
	});

	it('flags the whole sentence without a word index', async () => {
		const fetch = vi.fn(async () => created());
		vi.stubGlobal('fetch', fetch);
		render(FlagForm, { sentence, onclose: () => {} });

		await page.getByLabelText('What’s wrong?').fill('This is not English.');
		await page.getByRole('button', { name: 'Send' }).click();

		await expect.element(page.getByRole('status')).toHaveTextContent('Thanks');
		const init = (fetch.mock.calls[0] as unknown as [string, RequestInit])[1];
		expect(JSON.parse(init.body as string)).toEqual({ comment: 'This is not English.' });
	});

	it.each([
		['blank', '           '],
		['too short once trimmed', '   too short   '],
		['too long', 'x'.repeat(1001)]
	])('refuses a comment that is %s and says why', async (_, comment) => {
		const fetch = vi.fn(async () => created());
		vi.stubGlobal('fetch', fetch);
		render(FlagForm, { sentence, onclose: () => {} });
		const field = page.getByLabelText('What’s wrong?');

		await field.fill(comment);
		await page.getByRole('button', { name: 'Send' }).click();

		await expect.element(field).toHaveAttribute('aria-invalid', 'true');
		await expect.element(field).toHaveAccessibleDescription(/Write 10 to 1,000 characters\./);
		expect(fetch).not.toHaveBeenCalled();
	});

	it('counts characters as the server does', async () => {
		render(FlagForm, { sentence, onclose: () => {} });

		// Each emoji is one character but two UTF-16 code units.
		await page.getByLabelText('What’s wrong?').fill('  🦆🦆🦆  ');

		await expect
			.element(page.getByLabelText('What’s wrong?'))
			.toHaveAccessibleDescription(/^3 of 1,000 characters/);
	});

	it('keeps the comment and says so when sending fails', async () => {
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => new Response('{}', { status: 500 }))
		);
		render(FlagForm, { sentence, onclose: () => {} });
		const field = page.getByLabelText('What’s wrong?');

		await field.fill('This is not English.');
		await page.getByRole('button', { name: 'Send' }).click();

		await expect
			.element(page.getByText('Couldn’t send your report. Try again.'))
			.toBeInTheDocument();
		await expect.element(field).toHaveValue('This is not English.');
	});
});
