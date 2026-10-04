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
