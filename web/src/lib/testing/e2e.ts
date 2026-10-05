import AxeBuilder from '@axe-core/playwright';
import { expect, type APIRequestContext, type Page } from '@playwright/test';
import type { Sentence } from '../types';

// newSentence generates and saves a sentence, as a visitor elsewhere would.
export async function newSentence(request: APIRequestContext): Promise<Sentence> {
	const res = await request.get('/api/v1/sentences/random');
	expect(res.ok()).toBe(true);
	return res.json();
}

export async function expectNoAxeViolations(page: Page) {
	const { violations } = await new AxeBuilder({ page }).analyze();
	expect(violations).toEqual([]);
}

export async function expectNoSidewaysScroll(page: Page) {
	const overflow = await page.evaluate(
		() => document.documentElement.scrollWidth - window.innerWidth
	);
	expect(overflow).toBeLessThanOrEqual(0);
}

// The development admin password, as .env.example sets it.
const ADMIN_PASSWORD = 'randsense';

// logIn starts an admin session in page's browser context.
export async function logIn(page: Page) {
	const res = await page.request.post('/api/v1/admin/login', {
		data: { password: ADMIN_PASSWORD }
	});
	expect(res.ok()).toBe(true);
}

// flagSentence reports a problem with a sentence, or the word at wordIndex,
// as a visitor would.
export async function flagSentence(
	request: APIRequestContext,
	id: string,
	comment: string,
	wordIndex?: number
) {
	const res = await request.post(`/api/v1/sentences/${id}/flags`, {
		data: { comment, word_index: wordIndex }
	});
	expect(res.ok()).toBe(true);
}
