# Frontend, Part 2: Home Feed, Flags and Stars Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the public SvelteKit app: a home page that generates a sentence and shows a live feed of the latest 30 sentences, a "Something's wrong" flag form on every sentence, and a `/stars` page for this browser's stars.

**Architecture:**
- The work builds on Part 1's components in `web/src/lib/components/`.
- Both the home and permalink pages use a new `SentenceView`, which holds the sentence, its star and flag buttons, the word card, the flag form and the structure outline.
- The feed listens to the Go API's Server-Sent Events through a small `subscribe` helper in `#lib/stream.js`. Its list logic is a pure function in `#lib/feed.js`.
- Server loads go through one helper, `#lib/server/api.js`, which turns API failures and timeouts into 502s.
- The stars page renders only in the browser, because the voter token lives in the browser.

**Tech Stack:** SvelteKit 3.0, Svelte 5.57 (runes, `svelte/motion`'s `prefersReducedMotion`), TypeScript (strict), Vitest 4 (browser mode with Playwright Chromium, plus Node), Playwright 1.60+ with `@axe-core/playwright`. There are no Go changes, because the backend already serves everything this plan needs.

**Spec:** `docs/superpowers/specs/2026-10-03-frontend-design.md`. This plan covers the rest of build-order stage 6: the home page and live feed, the flag form and the stars page. Admin (stage 7) and deployment (stage 8) come later.

## Global Constraints

- **Kit 3 imports:**
  - Import app code through `#lib/...` with a `.js` suffix (`#lib/feed.js`). Svelte components keep `.svelte`.
  - Environment variables come from `$app/env/private`, and page state comes from `$app/state`.
  - Tests and helpers in `src/lib` import their siblings relatively without a suffix (`./feed`), as Part 1's specs do.
  - Playwright files import `src/lib/testing/e2e.ts` relatively.
- **Styling:** TypeScript in strict mode, plain CSS through Svelte's scoped styles (no Tailwind). Shared styles live in `src/app.css`.
- **Colors, from the spec's "Look":**
  - sentence words `dodgerblue`, never below 24 CSS px
  - hover and selection `#C2185B`
  - button outline `royalblue`, button text `slategray`, kept at least 18.67 CSS px bold
  - feed and other normal-size text in black or `royalblue`, never `dodgerblue`
- **Accessibility:** WCAG 2.2 AA.
  - Focus is always visible, and targets are at least 44 by 44 CSS px.
  - Escape closes the word card and the flag form, and focus returns to what opened them.
  - The live feed has a pause control (WCAG 2.2.2) and is never read aloud. A polite live region announces only the visitor's own generated sentence.
  - Feed transitions turn off under `prefers-reduced-motion`.
  - The flag form has visible labels, and its errors and character count are tied to the comment field with `aria-describedby`.
  - Each page has one `h1`, plus `header`, `nav` and `main` landmarks.
- **Small screens:** one column below 640 CSS px, no horizontal page scroll at 320 CSS px, and the word card never covers a focused control.
- **Flags:** a comment is 10 to 1,000 characters after trimming, counted in code points as the server counts. A comma can't be flagged.
- **Tests:**
  - Axe runs on every page and on the open flag form, with no violations allowed.
  - Every e2e test also runs in the `phone` project.
  - Flagging a word is tested by keyboard alone.
  - Before stage 6 ships, Jamey checks by hand with a screen reader, at 320 CSS px and at 200% text.
- **E2E database and concurrency:**
  - E2E tests run against the dev database (`DATABASE_URL`), in parallel workers that share one live feed.
  - Never assert where a sentence sits in the feed, only that it's there or not.
  - Generate only a few sentences per test, or other tests' sentences get pushed out of the 30-sentence feed.
- **Commits:** a conventional prefix and a capitalized summary, with **no `Co-Authored-By` trailer**. Comments describe what the code does now, with no history.

## Review Focus

The spec implies these five cases, but none of its feature descriptions covers them. Each has a test in its owning task.

1. **A visitor stars the home page's sentence, then presses Generate before the star request completes:** the star lands on the first sentence, and the new one shows its own count. (Task 2: `applies a late answer to the sentence it was for`)
2. **The stream drops and reconnects:** the feed catches up from the API and shows each sentence once, even when both the catch-up and the stream carry it. (Task 3: `catches up on every connection and shows each sentence once`)
3. **A burst of more than 30 sentences, or a long pause:** the feed never grows past 30 and shows the newest. (Task 3: `keeps the newest thirty, however many arrive at once`, `holds new sentences while paused`)
4. **A comment that's blank, under 10 characters once trimmed, over 1,000, or made of emoji:** the form refuses it or counts it the way the server does, without sending a request. (Task 4: `refuses a comment that is %s and says why`, `counts characters as the server does`)
5. **Storage that refuses writes** (a full quota, some in-app browsers): starring still works for the life of the page. (Task 2: `keeps the token and stars in memory instead`, `still stars when storage refuses writes`)

---

### Task 1: One helper for server loads

The permalink load's 404, 502 and timeout handling moves into a shared helper, so the home page gets it too.

**Files:**
- Create: `web/src/lib/server/api.ts`
- Test: `web/src/lib/server/api.spec.ts`
- Modify: `web/src/routes/s/[id]/+page.server.ts`
- Delete: `web/src/routes/s/[id]/page.server.spec.ts` (its cases move to `api.spec.ts`)

**Interfaces:**
- Produces: `getJSON<T>(fetch: typeof globalThis.fetch, path: string): Promise<T>` from `#lib/server/api.js`. `path` starts with `/api/v1/`. It throws SvelteKit's `error(404)` for a 404 and `error(502)` for any other failure, including no answer within 5 seconds.

- [ ] **Step 1: Write the failing tests**

`web/src/lib/server/api.spec.ts`:

```ts
import { isHttpError } from '@sveltejs/kit';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { getJSON } from './api';

async function statusFor(fetch: typeof globalThis.fetch): Promise<number> {
	try {
		await getJSON(fetch, '/api/v1/sentences/aaaaaaaa');
	} catch (e) {
		if (isHttpError(e)) return e.status;
		throw e;
	}
	return 200;
}

describe('getJSON', () => {
	afterEach(() => vi.restoreAllMocks());

	it('reads the API at API_ORIGIN', async () => {
		const fetch = vi.fn(async () => new Response('{"id": "aaaaaaaa"}'));

		expect(await getJSON(fetch, '/api/v1/sentences/aaaaaaaa')).toEqual({ id: 'aaaaaaaa' });
		expect(fetch).toHaveBeenCalledWith(
			'http://localhost:8080/api/v1/sentences/aaaaaaaa',
			expect.anything()
		);
	});

	it('is a 404 for something that is not there', async () => {
		expect(await statusFor(async () => new Response('{}', { status: 404 }))).toBe(404);
	});

	it('is a 502 when the API is down or failing', async () => {
		expect(
			await statusFor(async () => {
				throw new TypeError('fetch failed');
			})
		).toBe(502);
		expect(await statusFor(async () => new Response('{}', { status: 500 }))).toBe(502);
	});

	it('is a 502 when the API accepts the request but never answers', async () => {
		vi.spyOn(AbortSignal, 'timeout').mockReturnValue(AbortSignal.abort());
		const hang = (_: unknown, init?: RequestInit) =>
			new Promise<Response>((_, reject) => {
				const signal = init?.signal;
				if (signal?.aborted) reject(signal.reason);
				signal?.addEventListener('abort', () => reject(signal.reason));
			});

		expect(await statusFor(hang as typeof globalThis.fetch)).toBe(502);
	}, 2000);
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd web && npx vitest --run --project server src/lib/server/api.spec.ts ; cd ..`
Expected: FAIL: `Cannot find module './api'`.

- [ ] **Step 3: Implement, and move the permalink load onto it**

`web/src/lib/server/api.ts`:

```ts
import { error } from '@sveltejs/kit';
import { API_ORIGIN } from '$app/env/private';

// A stuck API gets a 502 rather than a page that never loads.
const API_TIMEOUT_MS = 5000;

// getJSON reads path from the Go API for a server-side load. A 404 stays a
// 404. Any other failure, including no answer at all, is a 502.
export async function getJSON<T>(fetch: typeof globalThis.fetch, path: string): Promise<T> {
	let res: Response;
	try {
		res = await fetch(`${API_ORIGIN}${path}`, { signal: AbortSignal.timeout(API_TIMEOUT_MS) });
	} catch {
		error(502, 'The sentence service is unavailable.');
	}
	if (res.status === 404) error(404, 'Nothing is at this address.');
	if (!res.ok) error(502, 'The sentence service is unavailable.');
	return (await res.json()) as T;
}
```

Replace `web/src/routes/s/[id]/+page.server.ts`:

```ts
import { getJSON } from '#lib/server/api.js';
import type { Sentence } from '#lib/types.js';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, fetch }) => ({
	sentence: await getJSON<Sentence>(fetch, `/api/v1/sentences/${encodeURIComponent(params.id)}`)
});
```

Delete the old load test, whose cases `api.spec.ts` now covers:

```bash
git rm 'web/src/routes/s/[id]/page.server.spec.ts'
```

- [ ] **Step 4: Run them to verify they pass**

Run: `cd web && npx vitest --run --project server && npm run check ; cd ..`
Expected: 28 passed (the 4 new tests, Part 1's tree and voter tests), and 0 type errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/server 'web/src/routes/s/[id]'
git commit -m "refactor: Share server-side API loading between pages"
```

### Task 2: A star button that survives a swapped sentence and full storage

On the home page, Generate replaces the sentence under the star button while a star request may be in flight. Storage that refuses writes currently breaks starring.

**Files:**
- Modify: `web/src/lib/voter.ts`, `web/src/lib/components/StarButton.svelte`
- Test: `web/src/lib/voter.spec.ts`, `web/src/lib/components/StarButton.svelte.spec.ts`

**Interfaces:**
- Consumes: Part 1's `voterToken`, `isStarred` and `setStarred` (signatures unchanged).
- Produces: `<StarButton id count />` (or `bind:count`), with its interface unchanged. It applies each answer only to the `id` it was requested for, ignores clicks while a request is in flight, and never throws on storage failures.

- [ ] **Step 1: Write the failing tests**

Append to `web/src/lib/voter.spec.ts`:

```ts
describe('storage that refuses writes', () => {
	it('keeps the token and stars in memory instead', () => {
		const full = {
			getItem: () => null,
			setItem: () => {
				throw new DOMException('full', 'QuotaExceededError');
			}
		};

		const first = voterToken(full);
		expect(voterToken(full)).toBe(first);
		setStarred('dddddddd', true, full);
		expect(isStarred('dddddddd', full)).toBe(true);
	});
});
```

In `web/src/lib/components/StarButton.svelte.spec.ts`, restore spies after each test as well as globals:

```ts
	afterEach(() => {
		vi.unstubAllGlobals();
		vi.restoreAllMocks();
	});
```

and add these tests at the end of the `describe` block:

```ts
	it('sends one request for two clicks in the same moment', async () => {
		const fetch = vi.fn(() => new Promise<Response>(() => {}));
		vi.stubGlobal('fetch', fetch);
		render(StarButton, { id: 'aaaaaaaa', count: 2 });
		const button = page.getByRole('button', { name: 'Star, 2 stars' }).element() as HTMLElement;

		// Both clicks land before the button can re-render as disabled.
		button.click();
		button.click();

		expect(fetch).toHaveBeenCalledOnce();
	});

	it('applies a late answer to the sentence it was for', async () => {
		let release!: () => void;
		vi.stubGlobal(
			'fetch',
			vi.fn(() => new Promise<Response>((r) => (release = () => r(answer(3)))))
		);
		const { rerender } = render(StarButton, { id: 'aaaaaaaa', count: 2 });

		await page.getByRole('button', { name: 'Star, 2 stars' }).click();
		await rerender({ id: 'bbbbbbbb', count: 5 });
		release();

		await vi.waitFor(() =>
			expect(JSON.parse(localStorage.getItem('randsense:starred')!)).toEqual(['aaaaaaaa'])
		);
		await expect
			.element(page.getByRole('button', { name: 'Star, 5 stars' }))
			.toHaveAttribute('aria-pressed', 'false');
	});

	it('still stars when storage refuses writes', async () => {
		vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
			throw new DOMException('full', 'QuotaExceededError');
		});
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => answer(3))
		);
		render(StarButton, { id: 'aaaaaaaa', count: 2 });

		await page.getByRole('button', { name: 'Star, 2 stars' }).click();

		await expect
			.element(page.getByRole('button', { name: 'Star, 3 stars' }))
			.toHaveAttribute('aria-pressed', 'true');
	});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd web && npx vitest --run src/lib/voter.spec.ts src/lib/components/StarButton.svelte.spec.ts ; cd ..`
Expected: 4 failed:
- `keeps the token and stars in memory instead` throws `QuotaExceededError: full`
- `sends one request for two clicks in the same moment` reports `called once, but got 2 times`
- `applies a late answer to the sentence it was for` expects `[ 'aaaaaaaa' ]` and gets `[ 'bbbbbbbb' ]`
- `still stars when storage refuses writes` times out looking for `Star, 3 stars`

- [ ] **Step 3: Implement**

Replace `web/src/lib/voter.ts`. It keeps the token and stars in memory when storage is blocked or refuses writes:

```ts
// The voter token is a random UUID this browser stars with. It isn't an
// account. The browser also remembers which sentences it starred, since
// the API only lists them a page at a time.

const TOKEN_KEY = 'randsense:voter';
const STARRED_KEY = 'randsense:starred';

type Store = Pick<Storage, 'getItem' | 'setItem'>;

// Values storage refused to keep. They last as long as the page.
const memory = new Map<string, string>();

// Blocked site data makes localStorage itself throw. That works like
// storage that refuses every write.
const blocked: Store = {
	getItem: () => null,
	setItem: () => {
		throw new Error('storage is blocked');
	}
};

function browserStore(): Store {
	try {
		return window.localStorage;
	} catch {
		return blocked;
	}
}

export function voterToken(store: Store = browserStore()): string {
	let token = read(store, TOKEN_KEY);
	if (!token) {
		token = crypto.randomUUID();
		write(store, TOKEN_KEY, token);
	}
	return token;
}

export function isStarred(id: string, store: Store = browserStore()): boolean {
	return starredIds(store).has(id);
}

export function setStarred(id: string, starred: boolean, store: Store = browserStore()): void {
	const ids = starredIds(store);
	if (starred) ids.add(id);
	else ids.delete(id);
	write(store, STARRED_KEY, JSON.stringify([...ids]));
}

function starredIds(store: Store): Set<string> {
	try {
		return new Set(JSON.parse(read(store, STARRED_KEY) ?? '[]'));
	} catch {
		return new Set();
	}
}

// Storage that is full or refuses writes keeps values in memory instead,
// for the life of the page. Memory is read first because it holds whatever
// storage refused.
function read(store: Store, key: string): string | null {
	return memory.get(key) ?? store.getItem(key);
}

function write(store: Store, key: string, value: string): void {
	try {
		store.setItem(key, value);
		memory.delete(key);
	} catch {
		memory.set(key, value);
	}
}
```

Replace `web/src/lib/components/StarButton.svelte`:

```svelte
<script lang="ts">
	import { isStarred, setStarred, voterToken } from '#lib/voter.js';

	let { id, count = $bindable() }: { id: string; count: number } = $props();

	// Read from storage after hydration, so the server's HTML (never starred)
	// matches what the browser hydrates.
	let starred = $state(false);
	let busy = $state(false);
	let problem = $state('');

	$effect(() => {
		starred = isStarred(id);
		problem = '';
	});

	async function toggle() {
		if (busy) return;
		// The page can show another sentence before the answer arrives, so the
		// answer is applied to the sentence it was for.
		const target = id;
		const starring = !starred;
		busy = true;
		problem = '';
		try {
			const res = await fetch(`/api/v1/sentences/${target}/stars`, {
				method: starring ? 'POST' : 'DELETE',
				headers: { 'X-Voter': voterToken() }
			});
			if (!res.ok) throw new Error(`status ${res.status}`);
			const answer = ((await res.json()) as { count: number }).count;
			setStarred(target, starring);
			if (id === target) {
				count = answer;
				starred = starring;
			}
		} catch {
			if (id === target) {
				problem = starring
					? 'Couldn’t save your star. Try again.'
					: 'Couldn’t remove your star. Try again.';
			}
		} finally {
			busy = false;
		}
	}
</script>

<span class="star">
	<button
		type="button"
		class="pill"
		aria-pressed={starred}
		aria-label="Star, {count} {count === 1 ? 'star' : 'stars'}"
		disabled={busy}
		onclick={toggle}><span aria-hidden="true">{starred ? '★' : '☆'}</span> {count}</button
	>
	<span role="status" class="problem">{problem}</span>
</span>

<style>
	.problem {
		display: block;
		color: var(--error);
	}
</style>
```

- [ ] **Step 4: Run them to verify they pass**

Run: `cd web && npx vitest --run src/lib/voter.spec.ts src/lib/components/StarButton.svelte.spec.ts && npm run check && npm run lint ; cd ..`
Expected: 12 passed, 0 type errors, and lint clean.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/voter.ts web/src/lib/voter.spec.ts web/src/lib/components/StarButton.svelte web/src/lib/components/StarButton.svelte.spec.ts
git commit -m "fix: Keep stars working through swapped sentences and full storage"
```

### Task 3: Live feed

**Files:**
- Create: `web/src/lib/feed.ts`, `web/src/lib/stream.ts`, `web/src/lib/testing/eventsource.ts`, `web/src/lib/components/Feed.svelte`
- Test: `web/src/lib/feed.spec.ts`, `web/src/lib/components/Feed.svelte.spec.ts`
- Modify: `web/src/lib/testing/fixtures.ts`, `web/src/app.css`

**Interfaces:**
- Consumes: `StarButton` (Task 2), the `Sentence` type and fixture (Part 1), `GET /api/v1/sentences?limit=30`, and `GET /api/v1/sentences/stream` with its `sentence` and `stars` events.
- Produces:
  - `FEED_SIZE = 30` and `prepend(feed, newer): Sentence[]` from `#lib/feed.js`
  - `subscribe({ onOpen?, onSentence?, onStars? }): () => void` and the type `StarsEvent = { id: string; count: number }` from `#lib/stream.js`
  - `FakeEventSource` from `#lib/testing/eventsource.js`, for component tests
  - `another(id, text): Sentence` from `#lib/testing/fixtures.js`
  - `<Feed initial={Sentence[]} onstars?={(e: StarsEvent) => void} />`: a region named "Latest sentences" that holds a `ul.sentence-list` of permalink links with star buttons, and a "Pause feed" toggle with `aria-pressed`
  - the global class `.sentence-list` in `app.css`, which the stars page reuses (Task 7)

- [ ] **Step 1: Write the failing list tests**

`web/src/lib/feed.spec.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { FEED_SIZE, prepend } from './feed';
import type { Sentence } from './types';

const s = (id: string): Sentence => ({
	id,
	text: id,
	tree: { symbol: 'S' },
	star_count: 0,
	created_at: '2026-10-04T12:00:00Z'
});
const ids = (feed: Sentence[]) => feed.map((x) => x.id);

describe('prepend', () => {
	it('puts newer sentences on top', () => {
		expect(ids(prepend([s('b'), s('a')], [s('d'), s('c')]))).toEqual(['d', 'c', 'b', 'a']);
	});

	it('skips sentences the feed already has', () => {
		expect(ids(prepend([s('b'), s('a')], [s('c'), s('b')]))).toEqual(['c', 'b', 'a']);
	});

	it('keeps the newest thirty, however many arrive at once', () => {
		const feed = Array.from({ length: FEED_SIZE }, (_, i) => s(`old${i}`));
		const burst = Array.from({ length: 40 }, (_, i) => s(`new${i}`));

		const got = prepend(feed, burst);

		expect(got).toHaveLength(FEED_SIZE);
		expect(got[0].id).toBe('new0');
		expect(got.at(-1)!.id).toBe('new29');
	});
});
```

Run: `cd web && npx vitest --run --project server src/lib/feed.spec.ts ; cd ..`
Expected: FAIL: `Cannot find module './feed'`.

- [ ] **Step 2: Implement the list logic**

`web/src/lib/feed.ts`:

```ts
import type { Sentence } from './types';

export const FEED_SIZE = 30;

// prepend puts newer sentences, newest first, on top of a feed. It skips any
// the feed already has and keeps the newest FEED_SIZE.
export function prepend(feed: Sentence[], newer: Sentence[]): Sentence[] {
	const have = new Set(feed.map((s) => s.id));
	const fresh = newer.filter((s) => !have.has(s.id));
	return [...fresh, ...feed].slice(0, FEED_SIZE);
}
```

Run: `cd web && npx vitest --run --project server src/lib/feed.spec.ts ; cd ..`
Expected: 3 passed.

- [ ] **Step 3: Add the test helpers**

`web/src/lib/testing/eventsource.ts`:

```ts
// FakeEventSource stands in for the browser's EventSource in component
// tests: vi.stubGlobal('EventSource', FakeEventSource).
export class FakeEventSource extends EventTarget {
	static latest: FakeEventSource | undefined;
	url: string;
	closed = false;

	constructor(url: string) {
		super();
		this.url = url;
		FakeEventSource.latest = this;
	}

	close() {
		this.closed = true;
	}

	// open is the stream connecting or reconnecting.
	open() {
		this.dispatchEvent(new Event('open'));
	}

	emit(name: string, data: unknown) {
		this.dispatchEvent(new MessageEvent(name, { data: JSON.stringify(data) }));
	}
}
```

Append to `web/src/lib/testing/fixtures.ts`:

```ts
// another is the fixture sentence under a different id and text.
export function another(id: string, text: string): Sentence {
	return { ...structuredClone(sentence), id, text };
}
```

- [ ] **Step 4: Write the failing component tests**

`web/src/lib/components/Feed.svelte.spec.ts`:

```ts
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
});
```

Run: `cd web && npx vitest --run --project client src/lib/components/Feed.svelte.spec.ts ; cd ..`
Expected: FAIL: `Failed to resolve import "./Feed.svelte"`.

- [ ] **Step 5: Implement the stream helper, the component and the shared list style**

`web/src/lib/stream.ts`:

```ts
import type { Sentence } from './types';

export interface StarsEvent {
	id: string;
	count: number;
}

export interface StreamHandlers {
	// Runs on every connection, reconnects included. The stream doesn't
	// replay what was missed, so this is where a page catches up.
	onOpen?: () => void;
	onSentence?: (s: Sentence) => void;
	onStars?: (e: StarsEvent) => void;
}

// subscribe listens to the live stream until the returned function is
// called. The browser reconnects by itself when the connection drops.
export function subscribe(handlers: StreamHandlers): () => void {
	const source = new EventSource('/api/v1/sentences/stream');
	source.addEventListener('open', () => handlers.onOpen?.());
	source.addEventListener('sentence', (e) => handlers.onSentence?.(JSON.parse(e.data)));
	source.addEventListener('stars', (e) => handlers.onStars?.(JSON.parse(e.data)));
	return () => source.close();
}
```

`web/src/lib/components/Feed.svelte`. The `svelte-ignore` is needed: `initial` seeds the feed once, and the stream keeps it current after that.

```svelte
<script lang="ts">
	import { onMount } from 'svelte';
	import { flip } from 'svelte/animate';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fly } from 'svelte/transition';
	import { FEED_SIZE, prepend } from '#lib/feed.js';
	import { subscribe, type StarsEvent } from '#lib/stream.js';
	import type { Sentence } from '#lib/types.js';
	import StarButton from './StarButton.svelte';

	let { initial, onstars }: { initial: Sentence[]; onstars?: (e: StarsEvent) => void } = $props();

	// The feed starts from the server's page, then the stream takes over.
	// svelte-ignore state_referenced_locally
	let feed = $state(initial);
	// What arrived while paused, newest first.
	let pending = $state<Sentence[]>([]);
	let paused = $state(false);
	const duration = $derived(prefersReducedMotion.current ? 0 : 300);
	const headingId = $props.id();

	onMount(() =>
		subscribe({
			onOpen: catchUp,
			onSentence: (s) => {
				if (paused) pending = prepend(pending, [s]);
				else feed = prepend(feed, [s]);
			},
			onStars: (e) => {
				for (const s of [...feed, ...pending]) if (s.id === e.id) s.star_count = e.count;
				onstars?.(e);
			}
		})
	);

	async function catchUp() {
		let latest: Sentence[];
		try {
			const res = await fetch(`/api/v1/sentences?limit=${FEED_SIZE}`);
			if (!res.ok) return;
			latest = await res.json();
		} catch {
			// The stream reconnects when the API is back, and that catches up.
			return;
		}
		if (!paused) {
			feed = latest;
			return;
		}
		const unseen = latest.filter((s) => !feed.some((f) => f.id === s.id));
		pending = prepend(pending, unseen);
	}

	function togglePause() {
		paused = !paused;
		if (!paused) {
			feed = prepend(feed, pending);
			pending = [];
		}
	}
</script>

<section class="feed" aria-labelledby={headingId}>
	<div class="head">
		<h2 id={headingId}>Latest sentences</h2>
		<button type="button" class="pill" aria-pressed={paused} onclick={togglePause}
			>Pause feed</button
		>
	</div>
	{#if pending.length}
		<p class="waiting">
			{pending.length}
			{pending.length === 1 ? 'new sentence' : 'new sentences'} waiting
		</p>
	{/if}
	<ul class="sentence-list">
		{#each feed as s (s.id)}
			<li animate:flip={{ duration }} in:fly={{ y: -16, duration }}>
				<a href="/s/{s.id}">{s.text}</a>
				<StarButton id={s.id} count={s.star_count} />
			</li>
		{/each}
	</ul>
</section>

<style>
	.feed {
		margin-block: 3rem;
	}

	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: center;
		gap: 0.5rem 1.5rem;
	}

	.waiting {
		color: var(--outline);
	}
</style>
```

In `web/src/app.css`, add before `.visually-hidden`:

```css
/* A list of sentences that link to their permalinks, each with its star
   button: the home page's feed and the stars page. */
.sentence-list {
	padding: 0;
	list-style: none;
	font-size: 1.25rem;
}

.sentence-list li {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	justify-content: center;
	gap: 0.5rem 1rem;
	padding-block: 0.75rem;
	border-block-end: 1px solid #ddd;
}

.sentence-list a {
	color: var(--outline);
	overflow-wrap: anywhere;
}
```

- [ ] **Step 6: Run them to verify they pass**

Run: `cd web && npx vitest --run src/lib/feed.spec.ts src/lib/components/Feed.svelte.spec.ts && npm run check && npm run lint ; cd ..`
Expected: 10 passed, 0 type errors, and lint clean.

- [ ] **Step 7: Commit**

```bash
git add web/src/lib/feed.ts web/src/lib/feed.spec.ts web/src/lib/stream.ts web/src/lib/testing web/src/lib/components/Feed.svelte web/src/lib/components/Feed.svelte.spec.ts web/src/app.css
git commit -m "feat: Add a live feed of the latest sentences with a pause control"
```

### Task 4: Flag form

**Files:**
- Create: `web/src/lib/components/FlagForm.svelte`
- Test: `web/src/lib/components/FlagForm.svelte.spec.ts`

**Interfaces:**
- Consumes: `tokens` (Part 1), the fixture, and `POST /api/v1/sentences/{id}/flags` with `{comment, word_index?}`, which returns 201.
- Produces: `<FlagForm sentence={s} bind:target onclose={() => void} />`.
  - It's a region named "Report a problem" that focuses its heading when it opens and calls `onclose` on Escape, Cancel or Close.
  - `target` is a leaf index, or `null` for the whole sentence.
  - The select is labeled "About" and the comment "What's wrong?" (with a typographic apostrophe: `What’s wrong?`).

- [ ] **Step 1: Write the failing tests**

`web/src/lib/components/FlagForm.svelte.spec.ts`. `target` is also one of Svelte's mount options, so the test that sets it passes props under `props`:

```ts
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
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd web && npx vitest --run --project client src/lib/components/FlagForm.svelte.spec.ts ; cd ..`
Expected: FAIL: `Failed to resolve import "./FlagForm.svelte"`.

- [ ] **Step 3: Implement**

`web/src/lib/components/FlagForm.svelte`:

```svelte
<script lang="ts">
	import { tokens } from '#lib/tree.js';
	import type { Sentence } from '#lib/types.js';

	let {
		sentence,
		target = $bindable(null),
		onclose
	}: { sentence: Sentence; target?: number | null; onclose: () => void } = $props();

	// The API's limits on a comment, after trimming.
	const MIN = 10;
	const MAX = 1000;

	const words = $derived(tokens(sentence.tree).filter((t) => !t.punctuation));
	let comment = $state('');
	// Counted in code points, as the server counts.
	const length = $derived([...comment.trim()].length);
	let problem = $state('');
	let sending = $state(false);
	let sent = $state(false);
	const id = $props.id();
	let heading: HTMLHeadingElement | undefined = $state();

	$effect(() => {
		heading?.focus();
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (length < MIN || length > MAX) {
			problem = 'Write 10 to 1,000 characters.';
			return;
		}
		problem = '';
		sending = true;
		try {
			const res = await fetch(`/api/v1/sentences/${sentence.id}/flags`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ comment: comment.trim(), word_index: target ?? undefined })
			});
			if (!res.ok) throw new Error(`status ${res.status}`);
			sent = true;
		} catch {
			problem = 'Couldn’t send your report. Try again.';
		} finally {
			sending = false;
		}
	}
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<section class="flag" aria-labelledby="{id}-heading">
	<h2 id="{id}-heading" tabindex="-1" bind:this={heading}>Report a problem</h2>
	<p role="status">{sent ? 'Thanks. Your report was sent.' : ''}</p>
	{#if sent}
		<button type="button" class="pill" onclick={onclose}>Close</button>
	{:else}
		<form onsubmit={submit} novalidate>
			<label for="{id}-target">About</label>
			<select id="{id}-target" bind:value={target}>
				<option value={null}>The whole sentence</option>
				{#each words as w, n (w.index)}
					<option value={w.index}>{w.text} (word {n + 1})</option>
				{/each}
			</select>
			<label for="{id}-comment">What’s wrong?</label>
			<textarea
				id="{id}-comment"
				rows="4"
				bind:value={comment}
				aria-invalid={problem ? 'true' : undefined}
				aria-describedby="{id}-count {id}-problem"></textarea>
			<p id="{id}-count" class="hint">{length} of 1,000 characters, at least 10</p>
			<p id="{id}-problem" class="problem" aria-live="polite">{problem}</p>
			<div class="buttons">
				<button type="submit" class="pill" disabled={sending}>Send</button>
				<button type="button" class="pill" onclick={onclose}>Cancel</button>
			</div>
		</form>
	{/if}
</section>

<style>
	.flag {
		text-align: start;
		border: 2px solid var(--outline);
		border-radius: 1rem;
		padding: 1rem 1.5rem;
		max-inline-size: 40rem;
		margin: 1.5rem auto;
		overflow-wrap: anywhere;
	}

	h2 {
		margin-block-start: 0;
	}

	form {
		display: grid;
		gap: 0.5rem;
	}

	label {
		font-weight: bold;
	}

	select,
	textarea {
		font: inherit;
		min-block-size: 44px;
		border: 2px solid var(--outline);
		border-radius: 0.5rem;
		padding: 0.5rem;
		min-inline-size: 0;
	}

	.hint,
	.problem {
		margin: 0;
	}

	.problem {
		color: var(--error);
	}

	.buttons {
		display: flex;
		flex-wrap: wrap;
		gap: 1rem;
	}
</style>
```

- [ ] **Step 4: Run them to verify they pass**

Run: `cd web && npx vitest --run --project client src/lib/components/FlagForm.svelte.spec.ts && npm run check && npm run lint ; cd ..`
Expected: 9 passed, 0 type errors, and lint clean.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/components/FlagForm.svelte web/src/lib/components/FlagForm.svelte.spec.ts
git commit -m "feat: Add a flag form for a sentence or one of its words"
```

### Task 5: SentenceView, on the permalink page

The permalink page's sentence, card, star and outline move into `SentenceView`, which adds the "Something's wrong" button. Behavior changes on the permalink:
- The structure outline starts closed. The spec's "Show structure" expands it.
- The star count updates live from the stream.
- While the flag form is open, pressing a word sets the report's target without opening the word card.

**Files:**
- Create: `web/src/lib/components/SentenceView.svelte`, `web/src/lib/testing/e2e.ts`
- Test: `web/src/lib/components/SentenceView.svelte.spec.ts`
- Modify: `web/src/routes/s/[id]/+page.svelte`, `web/src/routes/s/[id]/page.e2e.ts`

**Interfaces:**
- Consumes: `Sentence`, `wordId`, `StarButton`, `Structure` and `WordCard` (Part 1), `FlagForm` (Task 4), and `subscribe` (Task 3).
- Produces:
  - `<SentenceView sentence={s} bind:count />`. Its selected word and open flag form reset when `sentence.id` changes.
  - From `src/lib/testing/e2e.ts`: `newSentence(request): Promise<Sentence>`, `expectNoAxeViolations(page)` and `expectNoSidewaysScroll(page)`.

- [ ] **Step 1: Write the failing component tests**

`web/src/lib/components/SentenceView.svelte.spec.ts`:

```ts
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
```

Run: `cd web && npx vitest --run --project client src/lib/components/SentenceView.svelte.spec.ts ; cd ..`
Expected: FAIL: `Failed to resolve import "./SentenceView.svelte"`.

- [ ] **Step 2: Implement the component**

`web/src/lib/components/SentenceView.svelte`:

```svelte
<script lang="ts">
	import { tick } from 'svelte';
	import { leaves } from '#lib/tree.js';
	import type { Sentence as SentenceData } from '#lib/types.js';
	import FlagForm from './FlagForm.svelte';
	import Sentence, { wordId } from './Sentence.svelte';
	import StarButton from './StarButton.svelte';
	import Structure from './Structure.svelte';
	import WordCard from './WordCard.svelte';

	let { sentence, count = $bindable() }: { sentence: SentenceData; count: number } = $props();

	// Both reset when another sentence takes this one's place.
	let selected = $derived.by<number | null>(() => {
		void sentence.id;
		return null;
	});
	let flagging = $derived.by(() => {
		void sentence.id;
		return false;
	});
	const selectedPath = $derived(selected === null ? null : leaves(sentence.tree)[selected].path);
	let flagButton: HTMLButtonElement | undefined = $state();

	async function closeCard() {
		const opener = selected;
		selected = null;
		await tick();
		if (opener !== null) document.getElementById(wordId(sentence.id, opener))?.focus();
	}

	async function closeFlagForm() {
		flagging = false;
		await tick();
		flagButton?.focus();
	}
</script>

<Sentence {sentence} bind:selected />

<div class="actions">
	<StarButton id={sentence.id} bind:count />
	<button
		type="button"
		class="pill"
		aria-expanded={flagging}
		bind:this={flagButton}
		onclick={() => (flagging = !flagging)}>Something’s wrong</button
	>
</div>

<!-- While the flag form is open, pressing a word picks its target instead of opening its card. -->
{#if flagging}
	<FlagForm {sentence} bind:target={selected} onclose={closeFlagForm} />
{:else if selected !== null}
	<WordCard tree={sentence.tree} index={selected} onclose={closeCard} />
{/if}

<Structure tree={sentence.tree} {selectedPath} />

<style>
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-start;
		justify-content: center;
		gap: 1rem;
		margin-block: 1.5rem;
	}
</style>
```

Run: `cd web && npx vitest --run --project client src/lib/components/SentenceView.svelte.spec.ts ; cd ..`
Expected: 5 passed.

- [ ] **Step 3: Write the shared e2e helpers and the failing flag test**

`web/src/lib/testing/e2e.ts`:

```ts
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
```

Replace `web/src/routes/s/[id]/page.e2e.ts`. It uses the shared helpers and adds the keyboard flag test. The focus test now measures the card on every Tab and stops when focus leaves the page. Its old version measured the card once and compared it with `body`, so it failed on desktop once nothing followed the closed outline.

```ts
import { expect, test } from '@playwright/test';
import {
	expectNoAxeViolations,
	expectNoSidewaysScroll,
	newSentence
} from '../../../lib/testing/e2e';

test('shows a saved sentence with link preview tags', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);

	await expect(page.getByRole('group', { name: s.text })).toBeVisible();
	await expect(page.locator('meta[property="og:title"]')).toHaveAttribute('content', s.text);
	await expect(page).toHaveTitle(`${s.text} · RandSense`);
	await expectNoAxeViolations(page);
});

test('an unknown sentence is a 404 page', async ({ page }) => {
	const res = await page.goto('/s/zzzzzzzz');

	expect(res?.status()).toBe(404);
	await expect(page.getByRole('heading', { name: 'No sentence here' })).toBeVisible();
	await expectNoAxeViolations(page);
});

test('opens a word card by keyboard and returns focus on Escape', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);
	const first = page.getByRole('group', { name: s.text }).getByRole('button').first();

	await first.focus();
	await page.keyboard.press('Enter');
	await expect(page.getByRole('region')).toBeVisible();
	await expect(page.getByRole('region').getByRole('heading')).toBeFocused();
	await expectNoAxeViolations(page);

	await page.keyboard.press('Escape');
	await expect(page.getByRole('region')).toHaveCount(0);
	await expect(first).toBeFocused();
});

test('stars and unstars, remembering the star across reloads', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);

	await page.getByRole('button', { name: `Star, ${s.star_count} stars` }).click();
	const starred = page.getByRole('button', { name: /^Star, 1 star$/ });
	await expect(starred).toHaveAttribute('aria-pressed', 'true');

	await page.reload();
	await expect(starred).toHaveAttribute('aria-pressed', 'true');
	await starred.click();
	await expect(page.getByRole('button', { name: 'Star, 0 stars' })).toHaveAttribute(
		'aria-pressed',
		'false'
	);
});

test('reflows at 320px without sideways scrolling', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.setViewportSize({ width: 320, height: 640 });
	await page.goto(`/s/${s.id}`);
	await page.getByRole('group', { name: s.text }).getByRole('button').first().click();

	await expectNoSidewaysScroll(page);
});

test('keeps keyboard focus out from under the open word card', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);
	await page.getByRole('group', { name: s.text }).getByRole('button').first().click();
	const card = page.getByRole('region');
	const focused = page.locator(':focus');

	// Tab through what follows the card, until focus leaves the page.
	for (let i = 0; i < 4; i++) {
		await page.keyboard.press('Tab');
		if ((await focused.count()) === 0) break;
		if ((await card.locator(':focus').count()) > 0) continue;
		const f = (await focused.boundingBox())!;
		const c = (await card.boundingBox())!;
		const overlaps =
			f.y + f.height > c.y && f.y < c.y + c.height && f.x + f.width > c.x && f.x < c.x + c.width;
		expect(overlaps).toBe(false);
	}
});

test('flags a word by keyboard alone', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);
	const opener = page.getByRole('button', { name: 'Something’s wrong' });

	await opener.focus();
	await page.keyboard.press('Enter');
	await expect(page.getByRole('heading', { name: 'Report a problem' })).toBeFocused();
	await expectNoAxeViolations(page);
	await page.keyboard.press('Tab');
	await expect(page.getByLabel('About')).toBeFocused();
	await page.keyboard.press('ArrowDown');
	await expect(page.getByLabel('About')).toHaveValue('0');
	await page.keyboard.press('Tab');
	await page.keyboard.type('This word should not be here.');
	await page.keyboard.press('Tab');
	await page.keyboard.press('Enter');

	await expect(page.getByRole('status').filter({ hasText: 'Thanks' })).toBeVisible();
	await page.keyboard.press('Escape');
	await expect(opener).toBeFocused();
});
```

Run: `cd web && set -a && . ../.env && set +a && npx playwright test 'src/routes/s' ; cd ..`
Expected: FAIL, for `flags a word by keyboard alone` only, in both projects: there's no "Something’s wrong" button yet.

- [ ] **Step 4: Move the permalink page onto `SentenceView`**

Replace `web/src/routes/s/[id]/+page.svelte`:

```svelte
<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import SentenceView from '#lib/components/SentenceView.svelte';
	import { subscribe } from '#lib/stream.js';

	let { data } = $props();

	// Resets when the page moves to another sentence.
	let stars = $derived(data.sentence.star_count);

	onMount(() =>
		subscribe({
			onStars: (e) => {
				if (e.id === data.sentence.id) stars = e.count;
			}
		})
	);
</script>

<svelte:head>
	<title>{data.sentence.text} · RandSense</title>
	<meta name="description" content="A grammatically sound, randomly generated sentence." />
	<meta property="og:site_name" content="RandSense" />
	<meta property="og:type" content="article" />
	<meta property="og:title" content={data.sentence.text} />
	<meta property="og:url" content={page.url.href} />
	<meta name="twitter:card" content="summary" />
</svelte:head>

<h1 class="visually-hidden">A random sentence</h1>

<SentenceView sentence={data.sentence} bind:count={stars} />
```

- [ ] **Step 5: Run everything for this task**

Run: `cd web && set -a && . ../.env && set +a && npx playwright test 'src/routes/s' && npx vitest --run && npm run check && npm run lint ; cd ..`
Expected: 14 Playwright tests pass (7 in each project). Vitest passes 70, with 0 type errors and lint clean.

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/components/SentenceView.svelte web/src/lib/components/SentenceView.svelte.spec.ts web/src/lib/testing/e2e.ts 'web/src/routes/s/[id]'
git commit -m "feat: Flag sentences from the permalink page, with live star counts"
```

### Task 6: Home page

The home page generates a sentence on arrival and when Generate is pressed, shows it with `SentenceView`, and puts the feed under it. Generation happens in the browser, not in the server load, so link previews and crawlers don't fill the feed. The header gains a `nav` with a "Your stars" link. The visible title stays the header's link, so the page's `h1` is visually hidden, as on the permalink page.

**Files:**
- Create: `web/src/routes/+page.server.ts`
- Modify: `web/src/routes/+page.svelte`, `web/src/routes/+layout.svelte`, `web/src/routes/page.e2e.ts`

**Interfaces:**
- Consumes: `getJSON` (Task 1), `FEED_SIZE` and `Feed` (Task 3), `SentenceView` (Task 5), the e2e helpers (Task 5), and `GET /api/v1/sentences/random`.
- Produces: the route `/`, and a header `nav` named "Main" whose "Your stars" link points at `/stars` (Task 7) and carries `aria-current="page"` there.

- [ ] **Step 1: Write the failing e2e tests**

Replace `web/src/routes/page.e2e.ts`:

```ts
import { expect, test, type Page } from '@playwright/test';
import { expectNoAxeViolations, expectNoSidewaysScroll, newSentence } from '../lib/testing/e2e';

// Other tests generate sentences at the same time, so these look for a
// sentence anywhere in the feed rather than on top.
const inFeed = (page: Page, text: string) =>
	page
		.getByRole('region', { name: 'Latest sentences' })
		.getByRole('link', { name: text, exact: true });

// ownSentence waits for the sentence the page generated on arrival.
async function ownSentence(page: Page): Promise<string> {
	const group = page.getByRole('group');
	await expect(group).toBeVisible();
	return (await group.getAttribute('aria-label'))!;
}

test('greets a visitor with a fresh sentence that the feed shows too', async ({ page }) => {
	await page.goto('/');

	await expect(inFeed(page, await ownSentence(page))).toBeVisible();
	await expect(page.getByRole('banner').getByRole('link', { name: 'RandSense' })).toBeVisible();
	await expectNoAxeViolations(page);
});

test('generates another sentence and announces it', async ({ page }) => {
	await page.goto('/');
	const first = await ownSentence(page);

	await page.getByRole('button', { name: 'Generate' }).click();

	const group = page.getByRole('group');
	await expect(group).not.toHaveAttribute('aria-label', first);
	const second = (await group.getAttribute('aria-label'))!;
	await expect(page.locator('[aria-live="polite"]', { hasText: second })).toHaveCount(1);
});

test('shows sentences generated elsewhere as they arrive', async ({ page }) => {
	await page.goto('/');
	// The page's own sentence in the feed shows the stream is connected.
	await expect(inFeed(page, await ownSentence(page))).toBeVisible();

	const elsewhere = await newSentence(page.request);

	await expect(inFeed(page, elsewhere.text)).toBeVisible();
});

test('holds the feed still while paused', async ({ page }) => {
	await page.goto('/');
	await expect(inFeed(page, await ownSentence(page))).toBeVisible();

	await page.getByRole('button', { name: 'Pause feed' }).click();
	const elsewhere = await newSentence(page.request);
	await expect(page.getByText(/new sentences? waiting/)).toBeVisible();
	await expect(inFeed(page, elsewhere.text)).toHaveCount(0);

	await page.getByRole('button', { name: 'Pause feed' }).click();
	await expect(inFeed(page, elsewhere.text)).toBeVisible();
});

test('reflows the home page at 320px without sideways scrolling', async ({ page }) => {
	await page.setViewportSize({ width: 320, height: 640 });
	await page.goto('/');
	await ownSentence(page);

	await expectNoSidewaysScroll(page);
});
```

Run: `cd web && set -a && . ../.env && set +a && npx playwright test src/routes/page.e2e.ts ; cd ..`
Expected: FAIL in both projects. The placeholder home page has no sentence group, feed or Generate button.

- [ ] **Step 2: Implement the load, the page and the nav**

`web/src/routes/+page.server.ts`:

```ts
import { FEED_SIZE } from '#lib/feed.js';
import { getJSON } from '#lib/server/api.js';
import type { Sentence } from '#lib/types.js';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ fetch }) => ({
	sentences: await getJSON<Sentence[]>(fetch, `/api/v1/sentences?limit=${FEED_SIZE}`)
});
```

Replace `web/src/routes/+page.svelte`:

```svelte
<script lang="ts">
	import { onMount } from 'svelte';
	import Feed from '#lib/components/Feed.svelte';
	import SentenceView from '#lib/components/SentenceView.svelte';
	import type { Sentence } from '#lib/types.js';

	let { data } = $props();

	let current = $state<Sentence | null>(null);
	let generating = $state(false);
	let problem = $state('');

	async function generate() {
		generating = true;
		problem = '';
		try {
			const res = await fetch('/api/v1/sentences/random');
			if (!res.ok) throw new Error(`status ${res.status}`);
			current = await res.json();
		} catch {
			problem = 'Couldn’t make a sentence. Try again.';
		} finally {
			generating = false;
		}
	}

	// Each visit starts with a fresh sentence. Generating here, in the
	// browser, keeps link previews and crawlers from filling the feed.
	onMount(generate);
</script>

<svelte:head>
	<title>RandSense</title>
	<meta name="description" content="Grammatically sound, randomly generated nonsense." />
</svelte:head>

<h1 class="visually-hidden">Random sentences</h1>

<button type="button" class="pill" disabled={generating} onclick={generate}>Generate</button>
<p class="problem" role="status">{problem}</p>
<!-- Announces the visitor's own sentence. The feed is never read aloud. -->
<p class="visually-hidden" aria-live="polite">{current?.text ?? ''}</p>

{#if current}
	<SentenceView sentence={current} bind:count={current.star_count} />
{/if}

<Feed
	initial={data.sentences}
	onstars={(e) => {
		if (current?.id === e.id) current.star_count = e.count;
	}}
/>

<style>
	.problem {
		color: var(--error);
	}
</style>
```

Replace `web/src/routes/+layout.svelte`:

```svelte
<script lang="ts">
	import '@fontsource-variable/atkinson-hyperlegible-next';
	import '../app.css';
	import { page } from '$app/state';
	import favicon from '#lib/assets/favicon.svg';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<header>
	<a class="title" href="/">RandSense</a>
	<nav aria-label="Main">
		<a href="/stars" aria-current={page.url.pathname === '/stars' ? 'page' : undefined}
			>Your stars</a
		>
	</nav>
</header>

<main>
	{@render children()}
</main>

<style>
	header {
		text-align: center;
		padding-block: 1rem;
	}

	.title {
		color: var(--title);
		font-size: 2.5rem;
		font-weight: bold;
		text-decoration: none;
	}

	nav a {
		display: inline-block;
		min-block-size: 44px;
		line-height: 44px;
		font-weight: bold;
		color: var(--outline);
	}

	nav a[aria-current='page'] {
		color: var(--accent);
	}

	main {
		inline-size: min(60rem, 100% - 2rem);
		margin-inline: auto;
		text-align: center;
	}
</style>
```

- [ ] **Step 3: Run them to verify they pass**

Run: `cd web && set -a && . ../.env && set +a && npx playwright test && npm run check && npm run lint ; cd ..`
Expected: 24 passed (home and permalink, in both projects), 0 type errors, and lint clean.

- [ ] **Step 4: Commit**

```bash
git add web/src/routes/+page.server.ts web/src/routes/+page.svelte web/src/routes/+layout.svelte web/src/routes/page.e2e.ts
git commit -m "feat: Generate on the home page and show the live feed under it"
```

### Task 7: Stars page

**Files:**
- Create: `web/src/lib/stars.ts`, `web/src/routes/stars/+page.ts`, `web/src/routes/stars/+page.svelte`
- Test: `web/src/routes/stars/page.e2e.ts`

**Interfaces:**
- Consumes: `voterToken` and `StarButton` (Task 2), `.sentence-list` (Task 3), the e2e helpers (Task 5), the nav link (Task 6), and `GET /api/v1/stars?limit&offset` with `X-Voter`.
- Produces:
  - `STARS_PAGE = 30` and `starred(fetch, offset): Promise<Sentence[]>` from `#lib/stars.js`
  - the route `/stars`, rendered in the browser only, with a "Show more" button while the last page came back with 30 sentences

- [ ] **Step 1: Write the failing e2e tests**

`web/src/routes/stars/page.e2e.ts`. The paging test stars existing sentences rather than generating 31, which would push the home tests' sentences out of the feed while they run:

```ts
import { randomUUID } from 'node:crypto';
import { expect, test } from '@playwright/test';
import { expectNoAxeViolations, newSentence } from '../../lib/testing/e2e';
import type { Sentence } from '../../lib/types';

test('lists what this browser starred', async ({ page }) => {
	const s = await newSentence(page.request);
	await page.goto(`/s/${s.id}`);
	await page.getByRole('button', { name: /^Star, / }).click();
	await expect(page.getByRole('button', { name: /^Star, / })).toHaveAttribute(
		'aria-pressed',
		'true'
	);

	await page.getByRole('navigation').getByRole('link', { name: 'Your stars' }).click();

	await expect(page.getByRole('heading', { level: 1, name: 'Your stars' })).toBeVisible();
	await expect(page.getByRole('link', { name: s.text })).toHaveAttribute('href', `/s/${s.id}`);
	await expectNoAxeViolations(page);
});

test('says so when nothing is starred', async ({ page }) => {
	await page.goto('/stars');

	await expect(
		page.getByText('You haven’t starred any sentences in this browser yet.')
	).toBeVisible();
	await expectNoAxeViolations(page);
});

test('shows more than one page of stars', async ({ page }) => {
	const voter = randomUUID();
	await page.addInitScript((v) => localStorage.setItem('randsense:voter', v), voter);
	// Star sentences that already exist: generating 31 would push the home
	// page tests' sentences out of the feed while they run.
	const res = await page.request.get('/api/v1/sentences?limit=31');
	const ids: string[] = (await res.json()).map((s: Sentence) => s.id);
	while (ids.length < 31) ids.push((await newSentence(page.request)).id);
	for (const id of ids) {
		const res = await page.request.post(`/api/v1/sentences/${id}/stars`, {
			headers: { 'X-Voter': voter }
		});
		expect(res.ok()).toBe(true);
	}

	await page.goto('/stars');
	const items = page.getByRole('main').getByRole('listitem');
	await expect(items).toHaveCount(30);
	await page.getByRole('button', { name: 'Show more' }).click();

	await expect(items).toHaveCount(31);
	await expect(page.getByRole('button', { name: 'Show more' })).toHaveCount(0);
});
```

Run: `cd web && set -a && . ../.env && set +a && npx playwright test src/routes/stars ; cd ..`
Expected: FAIL in both projects. `/stars` is a 404, so there's no "Your stars" heading.

- [ ] **Step 2: Implement**

`web/src/lib/stars.ts`:

```ts
import type { Sentence } from './types';
import { voterToken } from './voter';

export const STARS_PAGE = 30;

// starred fetches a page of the sentences this browser starred, newest star
// first. It throws when the API can't answer.
export async function starred(fetch: typeof globalThis.fetch, offset: number): Promise<Sentence[]> {
	const res = await fetch(`/api/v1/stars?limit=${STARS_PAGE}&offset=${offset}`, {
		headers: { 'X-Voter': voterToken() }
	});
	if (!res.ok) throw new Error(`status ${res.status}`);
	return res.json();
}
```

`web/src/routes/stars/+page.ts`:

```ts
import { error } from '@sveltejs/kit';
import { starred } from '#lib/stars.js';
import type { PageLoad } from './$types';

// Stars belong to the voter token, which only the browser has.
export const ssr = false;

export const load: PageLoad = async ({ fetch }) => {
	try {
		return { sentences: await starred(fetch, 0) };
	} catch {
		error(502, 'The sentence service is unavailable.');
	}
};
```

`web/src/routes/stars/+page.svelte`:

```svelte
<script lang="ts">
	import StarButton from '#lib/components/StarButton.svelte';
	import { STARS_PAGE, starred } from '#lib/stars.js';

	let { data } = $props();

	// svelte-ignore state_referenced_locally
	let sentences = $state(data.sentences);
	// svelte-ignore state_referenced_locally
	let more = $state(data.sentences.length === STARS_PAGE);
	let loading = $state(false);
	let problem = $state('');

	async function showMore() {
		loading = true;
		problem = '';
		try {
			const next = await starred(fetch, sentences.length);
			sentences = [...sentences, ...next];
			more = next.length === STARS_PAGE;
		} catch {
			problem = 'Couldn’t load more. Try again.';
		} finally {
			loading = false;
		}
	}
</script>

<svelte:head>
	<title>Your stars · RandSense</title>
</svelte:head>

<h1>Your stars</h1>

{#if sentences.length}
	<ul class="sentence-list">
		{#each sentences as s (s.id)}
			<li>
				<a href="/s/{s.id}">{s.text}</a>
				<StarButton id={s.id} count={s.star_count} />
			</li>
		{/each}
	</ul>
	{#if more}
		<button type="button" class="pill" disabled={loading} onclick={showMore}>Show more</button>
	{/if}
{:else}
	<p>You haven’t starred any sentences in this browser yet.</p>
{/if}
<p class="problem" role="status">{problem}</p>

<style>
	.problem {
		color: var(--error);
	}
</style>
```

- [ ] **Step 3: Run everything**

Run: `just test`
Expected:
- the Go suite passes
- `svelte-check` reports 0 errors and 0 warnings, and lint is clean
- Vitest passes 70 tests
- Playwright passes 30: 5 home, 7 permalink and 3 stars tests, each in the desktop and phone projects

Run it twice. The e2e tests share one live feed across parallel workers, so a second passing run checks that they don't interfere with each other.

- [ ] **Step 4: Commit**

```bash
git add web/src/lib/stars.ts web/src/routes/stars
git commit -m "feat: List this browser's starred sentences"
```

### Task 8: Scaffold leftovers and docs

**Files:**
- Delete: `web/README.md` (the stock `sv` template) and `web/src/lib/index.ts` (a placeholder)
- Modify: `web/package.json`, `CLAUDE.md`, `docs/superpowers/specs/2026-10-03-frontend-design.md`

- [ ] **Step 1: Remove the scaffold leftovers**

```bash
git rm web/README.md web/src/lib/index.ts
```

In `web/package.json`, remove the `"#lib": "./src/lib/index.js",` line from `imports`, which pointed at the deleted file. Keep `"#lib/*": "./src/lib/*"`.

Run: `cd web && npm run check && npx vitest --run ; cd ..`
Expected: 0 errors, and 70 passed.

- [ ] **Step 2: Update the roadmap and the build order**

In `CLAUDE.md`, replace the end of Roadmap item 1, from "The backend" through "must account for that.", with:

> The backend and the public app are built; the spec's "Build order" lists what remains.

In the spec's "Build order", replace item 6 with:

> 6. SvelteKit app in `web/`: before it ships, someone checks it by hand with a screen reader, at 320 CSS px and at 200% text.

- [ ] **Step 3: Commit**

```bash
git add web/package.json CLAUDE.md docs/superpowers/specs/2026-10-03-frontend-design.md
git commit -m "docs: Point the roadmap at the hand check and the admin UI"
```

- [ ] **Step 4: Hand check (Jamey)**

Start `just run` and open `http://localhost:5173/`.
- With a screen reader, Tab through the home page:
  - the generated sentence is announced once
  - the feed is not read aloud as sentences arrive
  - pausing works
  - a flag can be filed and the form closed with Escape
- In devtools, at 320 CSS px wide and again at 200% text zoom, check the home, permalink and stars pages for sideways scrolling or cut-off content.

Note anything wrong as a finding. Once it passes, delete item 6 from the spec's "Build order".
