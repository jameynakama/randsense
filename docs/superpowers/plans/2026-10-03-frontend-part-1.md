# Frontend, Part 1: Scaffold and Sentence Detail Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Create the SvelteKit app in `web/` and build the sentence component (word buttons, word card, structure outline, star button) on an accessible permalink page, `/s/{id}`, that works on phones.

**Architecture:** SvelteKit 3 with Svelte 5 runes. It is presentation only: server loads call the Go API at `API_ORIGIN`, and the browser calls `/api/...` on the same origin through Vite's dev and preview proxy. Tree logic lives in plain TypeScript (`src/lib/tree.ts`) with unit tests. Components get browser-mode component tests, and pages get Playwright e2e tests that run axe and a phone viewport against the real Go server. One small Go change gives each leaf its written form, so split separable verbs ("looked her up") render as the sentence reads.

**Tech Stack:** SvelteKit 3.0, Svelte 5.56+, Vite 8, TypeScript (strict), `@sveltejs/adapter-node`, Vitest 4 (browser mode with Playwright Chromium, plus Node), Playwright 1.60+, `@axe-core/playwright`, `@fontsource-variable/atkinson-hyperlegible-next`. The Go side uses the existing stack.

**Spec:** `docs/superpowers/specs/2026-10-03-frontend-design.md`. This plan covers the first half of build-order stage 6: the scaffold, the sentence component and the permalink page. Part 2 covers the home page and live feed, the stars page and the flag form.

## Global Constraints

- **Generator and versions:** scaffold with `npx sv@1.0.1 create`. Kit 3 has no `svelte.config.js`; its config lives in `sveltekit({...})` inside `vite.config.ts`.
- **Kit 3 imports:**
  - Import app code through `#lib/...` with a `.js` suffix (`#lib/tree.js`, `#lib/types.js`). Svelte components keep `.svelte` (`#lib/components/Sentence.svelte`).
  - `$env/*` is gone. Environment variables are declared in `src/env.ts` with `defineEnvVars` and imported from `$app/env/private`.
  - Page state comes from `$app/state`.
- **TypeScript and CSS:** "TypeScript in strict mode, plain CSS through Svelte's scoped styles (no Tailwind), `adapter-node`." One global stylesheet, `src/app.css`, holds the color tokens, the `.pill` button style, the focus ring and reduced motion.
- **Colors, from the spec's "Look":**
  - sentence words `dodgerblue`, never below 24 CSS px
  - hover and selection `#C2185B`
  - title `deeppink` (large only), smaller accents `#C2185B`
  - button outline `royalblue`, button text `slategray`, kept at least 18.67 CSS px bold
  - normal-size text in black or `royalblue`, never `dodgerblue`
- **Accessibility:** WCAG 2.2 AA.
  - A visible focus ring with 3:1 contrast, never removed.
  - Interactive targets at least 44 by 44 CSS px.
  - The sentence's group is labeled with its full text, and each word button with its word.
  - Escape closes the word card and focus returns to the word that opened it.
  - Star buttons use `aria-pressed` and say the count ("Star, 12 stars").
  - The structure outline is a nested list with `aria-expanded` buttons, not an ARIA tree.
  - Transitions turn off under `prefers-reduced-motion`.
  - One `h1` per page, plus `header` and `main` landmarks.
- **Small screens:**
  - one column below 640 CSS px
  - the word card is a bottom sheet below 640 CSS px
  - no horizontal page scroll at 320 CSS px wide
- **Tests:**
  - Axe runs on every page and on the open word card, with no violations allowed.
  - Every e2e test also runs in a phone project (390 by 844, `isMobile`, `hasTouch`).
  - One flow is tested by keyboard alone.
  - Before stage 6 ships, someone checks by hand with a screen reader, at 320 CSS px and at 200% text.
- **E2E database:** e2e tests run against the Go server and its `DATABASE_URL`, the dev database. They save sentences and stars there.
- **Chromium system libraries** are installed on this machine (`sudo npx playwright install-deps chromium` was run once).
- **Commits:** a conventional prefix and a capitalized summary, with **no `Co-Authored-By` trailer**. Comments describe what the code does now, with no history.

## Review Focus

These five cases follow from the spec but no feature description covers them. Each has a test in its owning task.

1. **A sentence with a split separable verb** ("She looked her up.") must render its word buttons as the sentence reads, never "looked up her". (Task 1: `TestRealizeShowsSplitSeparableVerbOnItsLeaves`; Task 3: `writes a split separable verb as the sentence does`)
2. **A long word on a 320px phone** (`pneumonoultramicroscopicsilicovolcanoconiosis`, which OEWN has) must wrap rather than push the page sideways. (Task 5: `wraps a long word on a narrow phone`; Task 9: `reflows at 320px`)
3. **A browser that blocks site data**, where `localStorage` throws, must still let people star and unstar until the page reloads. (Task 4: `still gives one token for the life of the page`)
4. **A permalink opened while the Go API is down or failing** must show a 502 error page, never a stack trace or a hang. (Task 9: `is a 502 when the API is down or failing`)
5. **A failed or double-clicked star:** a failed request keeps the count and pressed state and says what went wrong, and a double click sends only one request. (Task 8: `keeps its state and says so when the request fails`, `sends one request for a double click`)

---

### Task 1: Each leaf's written form, from Go

The sentence text splits separable verbs around their object ("looked her up"), but the tree's leaves keep the verb whole (`word: "looked up"`), so a frontend rendering leaves would write "looked up her". The Go side already works out the split in `separateParticles`. This task stores the result on the leaves as `display`.

**Files:**
- Modify: `service/internal/grammar/grammar.go` (`Node`)
- Modify: `service/internal/sentence/sentence.go` (`Realize`, where `text[l]` is read)
- Modify: `service/internal/api/handlers.go` (`clearWords`)
- Test: `service/internal/sentence/sentence_test.go`, `service/internal/api/handlers_test.go`
- Modify: `README.md`

**Interfaces:**
- Produces: `grammar.Node.Display string` (JSON `display`, omitted when empty). It is set only on a leaf whose written form differs from `word`. The frontend's `TreeNode.display` (Task 3) reads it.

- [ ] **Step 1: Write the failing sentence test**

Append to `service/internal/sentence/sentence_test.go`:

```go
func TestRealizeShowsSplitSeparableVerbOnItsLeaves(t *testing.T) {
	q := newFake()
	q.pronouns["nominative"] = store.Pronoun{Lemma: "she", Person: 3, Number: "singular", Gender: "fem"}
	q.framed = &store.Verb{Lemma: "look up", Separable: true}
	tree := node("S", node("NP", leaf("Pronoun")), node("VP", leaf("Verb:transitive"), node("NP", leaf("Pronoun"))))

	if _, err := sentence.Realize(context.Background(), q, tree, loadVerbs(t), newRNG(), 0); err != nil {
		t.Fatalf("Realize: %v", err)
	}

	subject := tree.Children[0].Children[0]
	verb, object := tree.Children[1].Children[0], tree.Children[1].Children[1].Children[0]
	head, _, _ := strings.Cut(verb.Word, " ")
	if verb.Display != head || object.Display != "her up" {
		t.Errorf("expected displays %q and \"her up\"; got %q and %q", head, verb.Display, object.Display)
	}
	if subject.Display != "" {
		t.Errorf("expected no display on an unsplit word; got %q", subject.Display)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd service && set -a && . ../.env && set +a && go test -run TestRealizeShowsSplitSeparableVerbOnItsLeaves ./internal/sentence/ ; cd ..`
Expected: a compile failure: `verb.Display undefined`.

- [ ] **Step 3: Add `Display` and set it**

In `service/internal/grammar/grammar.go`, replace the `Node` doc comment and struct with:

```go
// Node is one constituent of a parse tree. A leaf's Symbol is a POS,
// optionally qualified ("Verb:transitive"). Once the leaf is filled from
// the lexicon, Lemma is the dictionary form and Word the inflected one.
// Display is how the leaf is written in the sentence when that differs
// from Word: a separable verb split around its object ("looked" and
// "her up").
type Node struct {
	Symbol   string   `json:"symbol"`
	Lemma    string   `json:"lemma,omitempty"`
	Word     string   `json:"word,omitempty"`
	Display  string   `json:"display,omitempty"`
	Features Features `json:"features,omitzero"`
	Children []*Node  `json:"children,omitempty"`
}
```

In `service/internal/sentence/sentence.go`, in `Realize`'s loop over `leaves`, record the written form on the leaf:

```go
		if t, ok := text[l]; ok {
			words[i] = t
			l.Display = t
		}
```

In `service/internal/api/handlers.go`, clear it with the rest of a posted tree:

```go
func clearWords(n *grammar.Node) {
	n.Lemma, n.Word, n.Display, n.Features = "", "", "", grammar.Features{}
	for _, c := range n.Children {
		clearWords(c)
	}
}
```

- [ ] **Step 4: Run the sentence tests**

Run: `cd service && set -a && . ../.env && set +a && go test ./internal/sentence/ ./internal/grammar/ ; cd ..`
Expected: PASS.

- [ ] **Step 5: Make the realize test cover a posted display**

In `service/internal/api/handlers_test.go`, in `TestRealizeSentenceRecomputesPostedFeatures`, change the posted noun from `{"symbol": "Noun"}` to `{"symbol": "Noun", "display": "stale"}` and add this check after the VP gender check:

```go
	if d := body.Tree.Children[0].Children[1].Display; d != "" {
		t.Errorf("noun display: got %q, want none", d)
	}
```

Run: `just test-be`
Expected: PASS. The posted display is cleared by `clearWords`. Remove `n.Display` from `clearWords`, check the test then fails on `noun display`, and put it back.

- [ ] **Step 6: Document `display` in `README.md`**

At the end of the paragraph that starts "Every node in a returned tree may carry a `features` object", add this sentence:

> A leaf whose written form differs from `word`, such as a separable verb split around its object ("looked her up"), carries `display`.

- [ ] **Step 7: Commit**

```bash
git add service/internal README.md
git commit -m "feat: Record split separable verbs on their tree leaves"
```

### Task 2: Scaffold the SvelteKit app

**Files:**
- Create: `web/` from `sv create`, then delete its demo files
- Create: `web/src/env.ts`, `web/src/app.css`, `web/src/routes/page.e2e.ts`
- Modify: `web/vite.config.ts`, `web/playwright.config.ts`, `web/package.json`, `web/src/routes/+layout.svelte`, `web/src/routes/+page.svelte`
- Modify: `Justfile`, `README.md`

**Interfaces:**
- Produces:
  - `API_ORIGIN` from `$app/env/private` (default `http://localhost:8080`)
  - the CSS tokens `--bg`, `--text`, `--word`, `--word-selected`, `--title`, `--accent`, `--outline`, `--button-text`, `--error`, `--focus`, and the global classes `.pill` and `.visually-hidden`
  - the `desktop` and `phone` Playwright projects
  - the recipes `just run-fe`, `just test-fe` and `just build-fe`, with `just run`, `just test` and `just build` covering both halves

- [ ] **Step 1: Generate the app**

From the repo root:

```bash
npx -y sv@1.0.1 create web --template minimal --types ts \
  --add prettier eslint vitest="usages:unit,component" playwright sveltekit-adapter="adapter:node" \
  --install npm
rm -rf web/src/routes/demo web/src/lib/vitest-examples
cd web && npm i -D @axe-core/playwright @fontsource-variable/atkinson-hyperlegible-next && npx playwright install chromium && cd ..
```

Expected: `web/package.json`, `web/vite.config.ts`, `web/playwright.config.ts`, `web/eslint.config.js`, `web/prettier.config.js` and `web/src/routes/+page.svelte` exist, and `web/src/routes/demo` does not.

- [ ] **Step 2: Write the failing home-page test**

`web/src/routes/page.e2e.ts`:

```ts
import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';

test('the home page has its heading and no axe violations', async ({ page }) => {
	await page.goto('/');

	await expect(page.getByRole('heading', { level: 1, name: 'RandSense' })).toBeVisible();
	await expect(page.getByRole('banner').getByRole('link', { name: 'RandSense' })).toBeVisible();
	await expect(page.getByRole('main')).toBeVisible();
	const { violations } = await new AxeBuilder({ page }).analyze();
	expect(violations).toEqual([]);
});
```

- [ ] **Step 3: Configure Playwright to start the Go server, run desktop and phone projects, and skip the browser install on each run**

Replace `web/playwright.config.ts` with:

```ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testMatch: '**/*.e2e.{ts,js}',
	webServer: [
		{
			// Uses DATABASE_URL and the admin settings from the root .env, which
			// `just test-fe` loads.
			command: 'go run ./cmd/server',
			cwd: '../service',
			url: 'http://localhost:8080/health',
			reuseExistingServer: true
		},
		{ command: 'npm run build && npm run preview', port: 4173, reuseExistingServer: true }
	],
	use: { baseURL: 'http://localhost:4173' },
	projects: [
		{ name: 'desktop', use: { ...devices['Desktop Chrome'] } },
		{
			name: 'phone',
			use: {
				...devices['Desktop Chrome'],
				viewport: { width: 390, height: 844 },
				isMobile: true,
				hasTouch: true
			}
		}
	]
});
```

In `web/package.json`, change the `test:e2e` script from `playwright install && playwright test` to `playwright test`.

- [ ] **Step 4: Run it to verify it fails**

Run: `cd web && set -a && . ../.env && set +a && npx playwright test src/routes/page.e2e.ts ; cd ..`
Expected: FAIL on both projects. The generated home page has no "RandSense" heading or banner link.

- [ ] **Step 5: Add the proxy, the env var, the global styles, the layout and the home page**

In `web/vite.config.ts`, add both proxies to the object passed to `defineConfig`, after `plugins`:

```ts
	server: { proxy: { '/api': 'http://localhost:8080' } },
	preview: { proxy: { '/api': 'http://localhost:8080' } },
```

`web/src/env.ts`:

```ts
import { defineEnvVars } from '@sveltejs/kit/env';

export const variables = defineEnvVars({
	API_ORIGIN: {
		description: 'Where the Go API listens, for server-side requests.',
		schema: (value) => value ?? 'http://localhost:8080'
	}
});
```

`web/src/app.css`:

```css
/* Colors are checked against white for WCAG AA. See "Look" in the
   frontend design spec for each ratio. */
:root {
	--bg: #fff;
	--text: #000;
	--word: dodgerblue; /* 3.2:1, large text only */
	--word-selected: #c2185b;
	--title: deeppink; /* 3.6:1, large text only */
	--accent: #c2185b;
	--outline: royalblue;
	--button-text: slategray; /* 4.1:1, bold 18.67px and up only */
	--error: #b00020;
	--focus: #000;
	font-family:
		'Atkinson Hyperlegible Next Variable',
		-apple-system,
		BlinkMacSystemFont,
		'Segoe UI',
		Roboto,
		sans-serif;
	color: var(--text);
	background: var(--bg);
}

body {
	margin: 0;
}

:focus-visible {
	outline: 3px solid var(--focus);
	outline-offset: 2px;
}

.pill {
	font: inherit;
	font-size: 1.5rem;
	font-weight: bold;
	text-transform: uppercase;
	color: var(--button-text);
	background: transparent;
	border: 2px solid var(--outline);
	border-radius: 1rem;
	padding: 0.5rem 1.5rem;
	min-block-size: 44px;
	cursor: pointer;
}

.pill:hover {
	color: var(--outline);
}

.pill:disabled {
	cursor: progress;
}

.visually-hidden {
	position: absolute;
	inline-size: 1px;
	block-size: 1px;
	overflow: hidden;
	clip-path: inset(50%);
	white-space: nowrap;
}

@media (prefers-reduced-motion: reduce) {
	*,
	*::before,
	*::after {
		animation-duration: 0.01ms !important;
		transition-duration: 0.01ms !important;
	}
}
```

Replace `web/src/routes/+layout.svelte`:

```svelte
<script lang="ts">
	import '@fontsource-variable/atkinson-hyperlegible-next';
	import '../app.css';
	import favicon from '#lib/assets/favicon.svg';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<header>
	<a class="title" href="/">RandSense</a>
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

	main {
		inline-size: min(60rem, 100% - 2rem);
		margin-inline: auto;
		text-align: center;
	}
</style>
```

Replace `web/src/routes/+page.svelte`. Part 2 replaces this with the generator and live feed:

```svelte
<svelte:head>
	<title>RandSense</title>
</svelte:head>

<h1>RandSense</h1>
<p>Grammatically sound nonsense.</p>
```

- [ ] **Step 6: Run it to verify it passes**

Run: `cd web && set -a && . ../.env && set +a && npx playwright test src/routes/page.e2e.ts && npm run check && npm run lint ; cd ..`
Expected: 2 passed (desktop and phone), `svelte-check` reports 0 errors and 0 warnings, and lint is clean. Run `npm run format` first if Prettier flags generated files.

- [ ] **Step 7: Add the recipes**

In the root `Justfile`, replace the `test`, `run` and `build` recipes with:

```just
# Run all tests
test args="": (test-be args) test-fe

# Start everything with hot reload
run:
    #!/usr/bin/env bash
    trap 'kill 0' EXIT
    just run-be & just run-fe & wait

# Build everything
build: build-be build-fe
```

and add, after `build-be`:

```just
# Start the SvelteKit dev server
[working-directory: 'web']
run-fe:
    npm run dev

# Type-check, lint and run the web unit, component and e2e tests
[working-directory: 'web']
test-fe:
    npm run check
    npm run lint
    npx vitest --run
    npx playwright test

# Build the SvelteKit app
[working-directory: 'web']
build-fe:
    npm run build
```

Run: `just test-fe && just build`
Expected: both succeed. `just run` should start air and Vite together, with `http://localhost:5173/` serving the home page and `http://localhost:5173/api/v1/sentences/random` proxying to Go. Check it, then stop both with Ctrl-C.

- [ ] **Step 8: Document the web half in `README.md`**

After the Setup block, add:

~~~markdown
The web app needs Node 24+ and Playwright's Chromium:

```bash
cd web && npm install && npx playwright install chromium && cd ..
sudo npx playwright install-deps chromium   # once per machine, on Linux
```

`just run` starts both the Go server and the SvelteKit dev server at `http://localhost:5173`,
which proxies `/api` to Go. Server-side page loads call Go at `API_ORIGIN` (default
`http://localhost:8080`).
~~~

In the Commands table, add:

```
| `just run-fe`                   | Start the SvelteKit dev server           |
| `just test-fe`                  | Type-check, lint and test the web app    |
```

In the Tests section, add:

> `just test-fe` runs the web e2e tests against the Go server and the dev database, starting the server if it isn't running, so they save sentences and stars there.

- [ ] **Step 9: Commit**

```bash
git add web Justfile README.md
git commit -m "feat: Scaffold the SvelteKit app with accessible global styles"
```

### Task 3: Tree types and helpers

**Files:**
- Create: `web/src/lib/types.ts`, `web/src/lib/tree.ts`
- Test: `web/src/lib/tree.spec.ts`

**Interfaces:**
- Consumes: the API's tree, including `display` (Task 1) and `features`.
- Produces:
  - from `#lib/types.js`: the types `Features`, `TreeNode` and `Sentence`
  - from `#lib/tree.js`: `leaves(tree): Leaf[]` with `Leaf = { node: TreeNode; path: number[] }`, `pos(node): string`, `frame(node): string | undefined`, `tokens(tree): Token[]` with `Token = { index: number; text: string; punctuation: boolean }`, `symbolName(node): string`, `role(tree, path): string | null` and `featureLabels(features | undefined): string[]`

- [ ] **Step 1: Write the types**

`web/src/lib/types.ts`:

```ts
// The shapes the Go API returns. See the README's API section.

export interface Features {
	tense?: string;
	commonness?: number;
	form?: 'finite' | 'base' | 'gerund';
	person?: number;
	number?: string;
	case?: string;
	gender?: string;
	type?: string;
	frames?: string[];
	separable?: boolean;
	frequency?: number;
}

export interface TreeNode {
	symbol: string;
	lemma?: string;
	word?: string;
	// How the leaf is written when it differs from word: a separable verb
	// split around its object ("looked" ... "her up").
	display?: string;
	features?: Features;
	children?: TreeNode[];
}

export interface Sentence {
	id: string;
	text: string;
	tree: TreeNode;
	star_count: number;
	created_at: string;
}
```

- [ ] **Step 2: Write the failing tests**

`web/src/lib/tree.spec.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { featureLabels, frame, leaves, role, symbolName, tokens } from './tree';
import type { TreeNode } from './types';

const leaf = (symbol: string, word: string, extra: Partial<TreeNode> = {}): TreeNode => ({
	symbol,
	lemma: word,
	word,
	...extra
});
const node = (symbol: string, ...children: TreeNode[]): TreeNode => ({ symbol, children });

// "the goose under the moon devoured her, but she urged us to sing"
const tree = node(
	'S',
	node(
		'Clause',
		node(
			'NP',
			leaf('Determiner', 'the'),
			leaf('Noun', 'goose'),
			node(
				'PP',
				leaf('Preposition', 'under'),
				node('NP', leaf('Determiner', 'the'), leaf('Noun', 'moon'))
			)
		),
		node('VP', leaf('Verb:transitive', 'devoured'), node('NP', leaf('Pronoun', 'her')))
	),
	leaf('Comma', ','),
	leaf('Conjunction:coordinating', 'but'),
	node(
		'Clause',
		node('NP', leaf('Pronoun', 'she')),
		node(
			'VP',
			leaf('Verb:transitive-to-infinitive', 'urged'),
			node('NP', leaf('Pronoun', 'us')),
			node('InfVP', leaf('To', 'to'), node('VP', leaf('Verb:intransitive', 'sing')))
		)
	)
);

describe('leaves', () => {
	it('lists leaves in sentence order with their paths', () => {
		const got = leaves(tree);
		expect(got.map((l) => l.node.word).join(' ')).toBe(
			'the goose under the moon devoured her , but she urged us to sing'
		);
		expect(got[4].path).toEqual([0, 0, 2, 1, 1]);
	});
});

describe('tokens', () => {
	it('capitalizes the first word and marks commas', () => {
		const got = tokens(tree);
		expect(got[0].text).toBe('The');
		expect(got[7]).toEqual({ index: 7, text: ',', punctuation: true });
		expect(got.filter((t) => t.punctuation)).toHaveLength(1);
	});

	it('writes a split separable verb as the sentence does', () => {
		const split = node(
			'S',
			node('NP', leaf('Pronoun', 'she')),
			node(
				'VP',
				leaf('Verb:transitive', 'looked up', { lemma: 'look up', display: 'looked' }),
				node('NP', leaf('Pronoun', 'her', { display: 'her up' }))
			)
		);

		expect(tokens(split).map((t) => t.text)).toEqual(['She', 'looked', 'her up']);
	});
});

describe('role', () => {
	const roleOf = (word: string) => {
		const l = leaves(tree).find((l) => l.node.word === word)!;
		return role(tree, l.path);
	};

	it.each([
		['the', 'in the subject'],
		['goose', 'in the subject'],
		['moon', 'in a prepositional phrase'],
		['under', 'in a prepositional phrase'],
		['her', 'in the object'],
		['she', 'in the subject'],
		['us', 'in the object'],
		['sing', 'in an infinitive']
	])('%s is %s', (word, want) => {
		expect(roleOf(word)).toBe(want);
	});

	it.each(['devoured', ',', 'but', 'urged'])('%s has no role', (word) => {
		expect(roleOf(word)).toBeNull();
	});
});

describe('frame and symbolName', () => {
	it('reads frames only from verbs', () => {
		expect(frame(leaf('Verb:transitive', 'x'))).toBe('transitive');
		expect(frame(leaf('Pronoun:reflexive', 'x'))).toBeUndefined();
	});

	it('names phrases and parts of speech', () => {
		expect(symbolName(node('NP'))).toBe('noun phrase');
		expect(symbolName(leaf('Verb:transitive', 'x'))).toBe('verb');
	});
});

describe('featureLabels', () => {
	it('labels a finite verb', () => {
		expect(
			featureLabels({
				tense: 'past',
				form: 'finite',
				person: 3,
				number: 'plural',
				frames: ['transitive', 'intransitive'],
				separable: true,
				frequency: 3.25
			})
		).toEqual([
			'past tense',
			'finite',
			'3rd person plural',
			'frames: transitive, intransitive',
			'separable',
			'frequency 3.25 (Zipf)'
		]);
	});

	it('labels pronouns, determiners and the root', () => {
		expect(
			featureLabels({ case: 'reflexive', person: 1, number: 'singular', gender: 'epicene' })
		).toEqual(['1st person singular', 'reflexive', 'any gender']);
		expect(featureLabels({ type: 'definite', number: 'either' })).toEqual([
			'either number',
			'definite'
		]);
		expect(featureLabels({ tense: 'present', commonness: 0 })).toEqual([
			'present tense',
			'commonness floor 0'
		]);
	});

	it('has nothing to say without features', () => {
		expect(featureLabels(undefined)).toEqual([]);
	});
});
```

- [ ] **Step 3: Run them to verify they fail**

Run: `cd web && npx vitest --run --project server src/lib/tree.spec.ts ; cd ..`
Expected: FAIL: `Failed to resolve import "./tree"`.

- [ ] **Step 4: Implement**

`web/src/lib/tree.ts`:

```ts
import type { Features, TreeNode } from './types';

export interface Leaf {
	node: TreeNode;
	// Child indexes from the root down to the leaf.
	path: number[];
}

// leaves lists the tree's leaves in sentence order. Their positions are the
// API's word_index.
export function leaves(tree: TreeNode, path: number[] = []): Leaf[] {
	if (!tree.children?.length) return [{ node: tree, path }];
	return tree.children.flatMap((c, i) => leaves(c, [...path, i]));
}

export function pos(node: TreeNode): string {
	return node.symbol.split(':')[0];
}

// frame is a verb slot's frame ("transitive" in "Verb:transitive").
export function frame(node: TreeNode): string | undefined {
	const [p, qualifier] = node.symbol.split(':');
	return p === 'Verb' ? qualifier : undefined;
}

export interface Token {
	index: number;
	text: string;
	punctuation: boolean;
}

// tokens is the sentence as written: one token per leaf, the first word
// capitalized. The final period isn't a leaf, so callers add it.
export function tokens(tree: TreeNode): Token[] {
	return leaves(tree).map(({ node }, index) => {
		const text = node.display ?? node.word ?? '';
		return {
			index,
			text: index === 0 ? text.charAt(0).toUpperCase() + text.slice(1) : text,
			punctuation: pos(node) === 'Comma'
		};
	});
}

const phraseNames: Record<string, string> = {
	S: 'sentence',
	Clause: 'clause',
	NP: 'noun phrase',
	VP: 'verb phrase',
	PP: 'prepositional phrase',
	InfVP: 'infinitive',
	GerVP: 'gerund phrase',
	ADJ: 'adjective phrase'
};

// symbolName reads a symbol out: "noun phrase" for NP, "verb" for
// Verb:transitive.
export function symbolName(node: TreeNode): string {
	return phraseNames[node.symbol] ?? pos(node).toLowerCase();
}

// role is where the leaf at path sits in its clause, from the nearest
// phrase that says: a prepositional phrase, an infinitive, a gerund phrase,
// or the subject or object. null when none applies.
export function role(tree: TreeNode, path: number[]): string | null {
	const chain = [tree];
	for (const i of path) chain.push(chain[chain.length - 1].children![i]);

	for (let depth = chain.length - 1; depth > 0; depth--) {
		const node = chain[depth];
		const parent = chain[depth - 1];
		switch (node.symbol) {
			case 'PP':
				return 'in a prepositional phrase';
			case 'InfVP':
				return 'in an infinitive';
			case 'GerVP':
				return 'in a gerund phrase';
		}
		const nominal = node.symbol === 'NP' || ['Noun', 'Pronoun'].includes(pos(node));
		if (!nominal || parent.symbol === 'NP') continue;
		const at = path[depth - 1];
		const siblings = parent.children!;
		if (siblings.slice(at + 1).some((s) => s.symbol === 'VP')) return 'in the subject';
		if (parent.symbol === 'VP' && siblings.slice(0, at).some((s) => pos(s) === 'Verb')) {
			return 'in the object';
		}
	}
	return null;
}

const ordinals: Record<number, string> = { 1: '1st', 2: '2nd', 3: '3rd' };
const forms: Record<string, string> = { finite: 'finite', base: 'base form', gerund: '-ing form' };
const genders: Record<string, string> = {
	fem: 'feminine',
	masc: 'masculine',
	neuter: 'neuter',
	epicene: 'any gender'
};

// featureLabels writes a node's features out for people: "past tense",
// "3rd person plural", "reflexive".
export function featureLabels(f: Features | undefined): string[] {
	if (!f) return [];
	const number = f.number === 'either' ? 'either number' : f.number;
	const labels: string[] = [];
	if (f.tense) labels.push(`${f.tense} tense`);
	if (f.form) labels.push(forms[f.form]);
	if (f.person) labels.push(`${ordinals[f.person]} person${number ? ` ${number}` : ''}`);
	else if (number) labels.push(number);
	if (f.case) labels.push(f.case);
	if (f.gender) labels.push(genders[f.gender] ?? f.gender);
	if (f.type) labels.push(f.type);
	if (f.frames?.length) labels.push(`frames: ${f.frames.join(', ')}`);
	if (f.separable) labels.push('separable');
	if (f.frequency !== undefined) labels.push(`frequency ${f.frequency} (Zipf)`);
	if (f.commonness !== undefined) labels.push(`commonness floor ${f.commonness}`);
	return labels;
}
```

- [ ] **Step 5: Run them to verify they pass**

Run: `cd web && npx vitest --run --project server src/lib/tree.spec.ts && npm run check ; cd ..`
Expected: 20 passed, and 0 type errors.

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/types.ts web/src/lib/tree.ts web/src/lib/tree.spec.ts
git commit -m "feat: Add tree helpers for words, roles and feature labels"
```

### Task 4: Voter token

**Files:**
- Create: `web/src/lib/voter.ts`
- Test: `web/src/lib/voter.spec.ts`

**Interfaces:**
- Produces from `#lib/voter.js`: `voterToken(store?): string`, `isStarred(id, store?): boolean` and `setStarred(id, starred, store?): void`. `store` defaults to `localStorage`, or to an in-memory map when storage is unusable.

- [ ] **Step 1: Write the failing tests**

`web/src/lib/voter.spec.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { isStarred, setStarred, voterToken } from './voter';

function store(initial: Record<string, string> = {}) {
	const m = new Map(Object.entries(initial));
	return {
		getItem: (k: string) => m.get(k) ?? null,
		setItem: (k: string, v: string) => void m.set(k, v)
	};
}

describe('voterToken', () => {
	it('makes a UUID once and keeps it', () => {
		const s = store();
		const first = voterToken(s);
		expect(first).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
		expect(voterToken(s)).toBe(first);
	});
});

describe('starred sentences', () => {
	it('remembers stars and unstars', () => {
		const s = store();
		setStarred('aaaaaaaa', true, s);
		setStarred('bbbbbbbb', true, s);
		setStarred('aaaaaaaa', false, s);
		expect(isStarred('aaaaaaaa', s)).toBe(false);
		expect(isStarred('bbbbbbbb', s)).toBe(true);
	});

	it('treats a corrupt list as empty', () => {
		const s = store({ 'randsense:starred': 'not json' });
		expect(isStarred('aaaaaaaa', s)).toBe(false);
		setStarred('aaaaaaaa', true, s);
		expect(isStarred('aaaaaaaa', s)).toBe(true);
	});
});

describe('without usable browser storage', () => {
	it('still gives one token for the life of the page', () => {
		// Node has no window, so this is the path a browser with blocked site
		// data takes when localStorage throws.
		const first = voterToken();
		expect(voterToken()).toBe(first);
		setStarred('cccccccc', true);
		expect(isStarred('cccccccc')).toBe(true);
	});
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd web && npx vitest --run --project server src/lib/voter.spec.ts ; cd ..`
Expected: FAIL: `Failed to resolve import "./voter"`.

- [ ] **Step 3: Implement**

`web/src/lib/voter.ts`:

```ts
// The voter token is a random UUID this browser stars with. It isn't an
// account. The browser also remembers which sentences it starred, since
// the API only lists them a page at a time.

const TOKEN_KEY = 'randsense:voter';
const STARRED_KEY = 'randsense:starred';

// Blocked site data makes localStorage throw, so stars then last only as
// long as the page.
const memory = new Map<string, string>();
const fallback: Pick<Storage, 'getItem' | 'setItem'> = {
	getItem: (k) => memory.get(k) ?? null,
	setItem: (k, v) => void memory.set(k, v)
};

type Store = Pick<Storage, 'getItem' | 'setItem'>;

function browserStore(): Store {
	try {
		return window.localStorage;
	} catch {
		return fallback;
	}
}

export function voterToken(store: Store = browserStore()): string {
	let token = store.getItem(TOKEN_KEY);
	if (!token) {
		token = crypto.randomUUID();
		store.setItem(TOKEN_KEY, token);
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
	store.setItem(STARRED_KEY, JSON.stringify([...ids]));
}

function starredIds(store: Store): Set<string> {
	try {
		return new Set(JSON.parse(store.getItem(STARRED_KEY) ?? '[]'));
	} catch {
		return new Set();
	}
}
```

- [ ] **Step 4: Run them to verify they pass**

Run: `cd web && npx vitest --run --project server src/lib/voter.spec.ts ; cd ..`
Expected: 4 passed.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/voter.ts web/src/lib/voter.spec.ts
git commit -m "feat: Keep an anonymous voter token and this browser's stars"
```

### Task 5: Sentence component

**Files:**
- Create: `web/src/lib/testing/fixtures.ts`, `web/src/lib/components/Sentence.svelte`
- Test: `web/src/lib/components/Sentence.svelte.spec.ts`

**Interfaces:**
- Consumes: `tokens` (Task 3) and `Sentence` (Task 3).
- Produces:
  - `<Sentence sentence={s} bind:selected />`, where `selected` is a `number | null` leaf index
  - `wordId(sentenceId, index): string`, from the component's module script: `import Sentence, { wordId } from '#lib/components/Sentence.svelte'`
  - `sentence` from `#lib/testing/fixtures.js`: "The goose devoured her, but she sang.", with leaves `the goose devoured her , but she sang` at indexes 0 to 7

- [ ] **Step 1: Write the fixture**

`web/src/lib/testing/fixtures.ts`:

```ts
import type { Sentence, TreeNode } from '#lib/types.js';

const leaf = (symbol: string, word: string, extra: Partial<TreeNode> = {}): TreeNode => ({
	symbol,
	lemma: word,
	word,
	...extra
});
const node = (symbol: string, ...children: TreeNode[]): TreeNode => ({ symbol, children });

// sentence is "The goose devoured her, but she sang." as the API returns it.
export const sentence: Sentence = {
	id: 'aaaaaaaa',
	text: 'The goose devoured her, but she sang.',
	star_count: 2,
	created_at: '2026-10-03T12:00:00Z',
	tree: {
		...node(
			'S',
			node(
				'Clause',
				node(
					'NP',
					leaf('Determiner', 'the', { features: { type: 'definite', number: 'either' } }),
					leaf('Noun', 'goose', { features: { number: 'singular', frequency: 3.5 } })
				),
				node(
					'VP',
					leaf('Verb:transitive', 'devoured', {
						lemma: 'devour',
						features: {
							tense: 'past',
							form: 'finite',
							person: 3,
							number: 'singular',
							frames: ['transitive']
						}
					}),
					node(
						'NP',
						leaf('Pronoun', 'her', {
							features: { case: 'accusative', person: 3, number: 'singular', gender: 'fem' }
						})
					)
				)
			),
			leaf('Comma', ','),
			leaf('Conjunction:coordinating', 'but'),
			node(
				'Clause',
				node('NP', leaf('Pronoun', 'she')),
				node('VP', leaf('Verb:intransitive', 'sang', { lemma: 'sing' }))
			)
		),
		features: { tense: 'past', commonness: 0 }
	}
};
```

- [ ] **Step 2: Write the failing tests**

`web/src/lib/components/Sentence.svelte.spec.ts`:

```ts
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
```

- [ ] **Step 3: Run them to verify they fail**

Run: `cd web && npx vitest --run --project client src/lib/components/Sentence.svelte.spec.ts ; cd ..`
Expected: FAIL: `Failed to resolve import "./Sentence.svelte"`.

- [ ] **Step 4: Implement**

`web/src/lib/components/Sentence.svelte`:

```svelte
<script lang="ts" module>
	export function wordId(sentenceId: string, index: number): string {
		return `word-${sentenceId}-${index}`;
	}
</script>

<script lang="ts">
	import { tokens } from '#lib/tree.js';
	import type { Sentence } from '#lib/types.js';

	let { sentence, selected = $bindable(null) }: { sentence: Sentence; selected?: number | null } =
		$props();

	const words = $derived(tokens(sentence.tree));
</script>

<!-- eslint-disable svelte/no-useless-mustaches -- {' '} keeps the space between words, which a bare space beside a tag doesn't reliably do -->
<p class="sentence" role="group" aria-label={sentence.text}>
	{#each words as t (t.index)}
		{#if t.punctuation}<span class="punct">{t.text}</span>{:else}{#if t.index > 0}{' '}{/if}<button
				type="button"
				id={wordId(sentence.id, t.index)}
				class="word"
				aria-pressed={selected === t.index}
				onclick={() => (selected = selected === t.index ? null : t.index)}>{t.text}</button
			>{/if}
	{/each}<span class="punct">.</span>
</p>

<style>
	.sentence {
		margin: 0;
		font-size: clamp(1.5rem, 8vw, 4rem);
		line-height: 1.3;
		overflow-wrap: anywhere;
	}

	.word {
		font: inherit;
		color: var(--word);
		background: none;
		border: none;
		padding: 0.1em 0.15em;
		margin: 0;
		min-block-size: 44px;
		min-inline-size: 44px;
		cursor: pointer;
		border-radius: 0.2em;
		overflow-wrap: anywhere;
	}

	.word:hover,
	.word[aria-pressed='true'] {
		color: var(--word-selected);
	}

	.word[aria-pressed='true'] {
		text-decoration: underline;
		text-underline-offset: 0.15em;
	}

	.punct {
		color: var(--word);
	}
</style>
```

- [ ] **Step 5: Run them to verify they pass**

Run: `cd web && npx vitest --run --project client src/lib/components/Sentence.svelte.spec.ts && npm run check && npm run lint ; cd ..`
Expected: 3 passed, 0 type errors, and lint clean.

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/testing web/src/lib/components/Sentence.svelte web/src/lib/components/Sentence.svelte.spec.ts
git commit -m "feat: Render a sentence as selectable word buttons"
```

### Task 6: Word card

**Files:**
- Create: `web/src/lib/components/WordCard.svelte`
- Test: `web/src/lib/components/WordCard.svelte.spec.ts`

**Interfaces:**
- Consumes: `leaves`, `pos`, `frame`, `role` and `featureLabels` (Task 3), and the fixture (Task 5).
- Produces: `<WordCard tree={t} index={i} onclose={() => void} />`, a region labeled by the word. It focuses its heading when it opens and calls `onclose` on Escape or Close.

- [ ] **Step 1: Write the failing tests**

`web/src/lib/components/WordCard.svelte.spec.ts`:

```ts
import { page, userEvent } from 'vitest/browser';
import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { sentence } from '#lib/testing/fixtures.js';
import WordCard from './WordCard.svelte';

describe('WordCard', () => {
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
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd web && npx vitest --run --project client src/lib/components/WordCard.svelte.spec.ts ; cd ..`
Expected: FAIL: `Failed to resolve import "./WordCard.svelte"`.

- [ ] **Step 3: Implement**

`web/src/lib/components/WordCard.svelte`:

```svelte
<script lang="ts">
	import { featureLabels, frame, leaves, pos, role } from '#lib/tree.js';
	import type { TreeNode } from '#lib/types.js';

	let { tree, index, onclose }: { tree: TreeNode; index: number; onclose: () => void } = $props();

	const leaf = $derived(leaves(tree)[index]);
	const labels = $derived(featureLabels(leaf.node.features));
	const wordRole = $derived(role(tree, leaf.path));
	const headingId = $props.id();
	let heading: HTMLHeadingElement | undefined = $state();

	// Each newly opened word moves focus to its heading.
	$effect(() => {
		void index;
		heading?.focus();
	});
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onclose()} />

<section class="card" aria-labelledby={headingId}>
	<h2 id={headingId} tabindex="-1" bind:this={heading}>{leaf.node.display ?? leaf.node.word}</h2>
	<dl>
		<dt>Lemma</dt>
		<dd>{leaf.node.lemma}</dd>
		<dt>Part of speech</dt>
		<dd>{pos(leaf.node).toLowerCase()}</dd>
		{#if frame(leaf.node)}
			<dt>Frame</dt>
			<dd>{frame(leaf.node)}</dd>
		{/if}
		{#if wordRole}
			<dt>Role</dt>
			<dd>{wordRole}</dd>
		{/if}
	</dl>
	{#if labels.length}
		<ul class="features" aria-label="Features">
			{#each labels as label (label)}
				<li>{label}</li>
			{/each}
		</ul>
	{/if}
	<button type="button" class="pill" onclick={onclose}>Close</button>
</section>

<style>
	.card {
		text-align: start;
		border: 2px solid var(--outline);
		border-radius: 1rem;
		padding: 1rem 1.5rem;
		background: var(--bg);
		max-inline-size: 40rem;
		margin-inline: auto;
	}

	h2 {
		margin-block-start: 0;
		color: var(--word);
		font-size: 2rem;
	}

	dl {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: 0.25rem 1rem;
	}

	dt {
		font-weight: bold;
	}

	dd {
		margin: 0;
	}

	.features {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		padding: 0;
		list-style: none;
	}

	.features li {
		border: 1px solid var(--outline);
		border-radius: 1rem;
		padding: 0.1rem 0.75rem;
	}

	@media (max-width: 640px) {
		.card {
			position: fixed;
			inset-inline: 0;
			inset-block-end: 0;
			max-block-size: 60vh;
			overflow-y: auto;
			border-radius: 1rem 1rem 0 0;
			margin: 0;
			box-shadow: 0 -0.25rem 1rem rgb(0 0 0 / 0.2);
		}
	}
</style>
```

- [ ] **Step 4: Run them to verify they pass**

Run: `cd web && npx vitest --run --project client src/lib/components/WordCard.svelte.spec.ts && npm run check && npm run lint ; cd ..`
Expected: 3 passed, 0 type errors, and lint clean.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/components/WordCard.svelte web/src/lib/components/WordCard.svelte.spec.ts
git commit -m "feat: Show a word's lemma, frame, role and features in a card"
```

### Task 7: Structure outline

**Files:**
- Create: `web/src/lib/components/Branch.svelte`, `web/src/lib/components/Structure.svelte`
- Test: `web/src/lib/components/Structure.svelte.spec.ts`

**Interfaces:**
- Consumes: `symbolName` (Task 3) and the fixture (Task 5).
- Produces: `<Structure tree={t} selectedPath={number[] | null} bind:open />`. It is a nested list in which each phrase is an `aria-expanded` button named "NP, noun phrase". Every list item from the root down to the selected leaf gets the `on-branch` class, and the leaf's label gets `aria-current="true"`.

- [ ] **Step 1: Write the failing tests**

`web/src/lib/components/Structure.svelte.spec.ts`:

```ts
import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { sentence } from '#lib/testing/fixtures.js';
import Structure from './Structure.svelte';

describe('Structure', () => {
	it('opens and closes the outline', async () => {
		render(Structure, { tree: sentence.tree });
		const toggle = page.getByRole('button', { name: 'Show structure' });

		// Hidden, so it's out of the accessibility tree until opened.
		await expect
			.element(page.getByRole('list', { name: 'Sentence structure' }))
			.not.toBeInTheDocument();
		await toggle.click();
		await expect.element(page.getByRole('list', { name: 'Sentence structure' })).toBeVisible();
		await expect
			.element(page.getByRole('button', { name: 'Hide structure' }))
			.toHaveAttribute('aria-expanded', 'true');
		await expect
			.element(page.getByRole('button', { name: 'NP, noun phrase' }).first())
			.toBeVisible();
	});

	it('collapses a branch', async () => {
		render(Structure, { tree: sentence.tree, open: true });
		const vp = page.getByRole('button', { name: 'VP, verb phrase' }).first();

		await vp.click();
		await expect.element(vp).toHaveAttribute('aria-expanded', 'false');
		await expect.element(page.getByText('devoured')).not.toBeVisible();
	});

	it('marks the selected word and its branch', async () => {
		render(Structure, { tree: sentence.tree, open: true, selectedPath: [0, 0, 1] });

		const current = page.getByText('goose').element().closest('[aria-current]');
		expect(current?.getAttribute('aria-current')).toBe('true');
		expect(document.querySelectorAll('li.on-branch')).toHaveLength(4);
	});
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd web && npx vitest --run --project client src/lib/components/Structure.svelte.spec.ts ; cd ..`
Expected: FAIL: `Failed to resolve import "./Structure.svelte"`.

- [ ] **Step 3: Implement**

`web/src/lib/components/Branch.svelte`:

```svelte
<script lang="ts">
	import { symbolName } from '#lib/tree.js';
	import type { TreeNode } from '#lib/types.js';
	import Branch from './Branch.svelte';

	let {
		node,
		path,
		selectedPath
	}: { node: TreeNode; path: number[]; selectedPath: number[] | null } = $props();

	let open = $state(true);
	const kids = $derived(node.children ?? []);
	const onBranch = $derived(selectedPath !== null && path.every((v, i) => selectedPath[i] === v));
	const isSelected = $derived(onBranch && selectedPath!.length === path.length);
</script>

<li class:on-branch={onBranch}>
	{#if kids.length}
		<button
			type="button"
			class="node"
			aria-expanded={open}
			aria-label="{node.symbol}, {symbolName(node)}"
			onclick={() => (open = !open)}>{node.symbol}</button
		>
		<ul hidden={!open}>
			{#each kids as child, i (i)}
				<Branch node={child} path={[...path, i]} {selectedPath} />
			{/each}
		</ul>
	{:else}
		<span class="leaf" aria-current={isSelected ? 'true' : undefined}
			>{node.symbol} <span class="leaf-word">{node.display ?? node.word}</span></span
		>
	{/if}
</li>

<style>
	li {
		list-style: none;
		padding-inline-start: 1rem;
		border-inline-start: 2px solid transparent;
	}

	li.on-branch {
		border-inline-start-color: var(--word-selected);
	}

	.node {
		font: inherit;
		background: none;
		border: none;
		color: var(--text);
		min-block-size: 44px;
		cursor: pointer;
	}

	.node::before {
		content: '▸ ';
	}

	.node[aria-expanded='true']::before {
		content: '▾ ';
	}

	.leaf {
		display: inline-block;
		min-block-size: 44px;
		line-height: 44px;
	}

	.leaf-word {
		color: var(--accent);
	}

	.leaf[aria-current='true'] .leaf-word {
		font-weight: bold;
		text-decoration: underline;
	}

	ul {
		padding: 0;
		margin: 0;
	}

	@media (max-width: 640px) {
		li {
			padding-inline-start: 0.5rem;
		}
	}
</style>
```

`web/src/lib/components/Structure.svelte`:

```svelte
<script lang="ts">
	import type { TreeNode } from '#lib/types.js';
	import Branch from './Branch.svelte';

	let {
		tree,
		selectedPath = null,
		open = $bindable(false)
	}: { tree: TreeNode; selectedPath?: number[] | null; open?: boolean } = $props();

	const outlineId = $props.id();
</script>

<div class="structure">
	<button
		type="button"
		class="pill"
		aria-expanded={open}
		aria-controls={outlineId}
		onclick={() => (open = !open)}>{open ? 'Hide structure' : 'Show structure'}</button
	>
	<ul id={outlineId} class="outline" hidden={!open} aria-label="Sentence structure">
		<Branch node={tree} path={[]} {selectedPath} />
	</ul>
</div>

<style>
	.outline {
		text-align: start;
		padding: 0;
		margin-block: 1rem;
		overflow-x: auto;
	}
</style>
```

- [ ] **Step 4: Run them to verify they pass**

Run: `cd web && npx vitest --run --project client src/lib/components/Structure.svelte.spec.ts && npm run check && npm run lint ; cd ..`
Expected: 3 passed, 0 type errors, and lint clean.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/components/Branch.svelte web/src/lib/components/Structure.svelte web/src/lib/components/Structure.svelte.spec.ts
git commit -m "feat: Show the parse tree as a collapsible outline"
```

### Task 8: Star button

**Files:**
- Create: `web/src/lib/components/StarButton.svelte`
- Test: `web/src/lib/components/StarButton.svelte.spec.ts`

**Interfaces:**
- Consumes: `voterToken`, `isStarred` and `setStarred` (Task 4), and the API's `POST` and `DELETE /api/v1/sentences/{id}/stars`, which return `{count}`.
- Produces: `<StarButton id={id} bind:count />`, a toggle button with `aria-pressed` named "Star, N stars", and a `role="status"` message when a request fails. Part 2's feed binds `count` so the button shows counts that arrive over the stream.

- [ ] **Step 1: Write the failing tests**

`web/src/lib/components/StarButton.svelte.spec.ts`:

```ts
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
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd web && npx vitest --run --project client src/lib/components/StarButton.svelte.spec.ts ; cd ..`
Expected: FAIL: `Failed to resolve import "./StarButton.svelte"`.

- [ ] **Step 3: Implement**

`web/src/lib/components/StarButton.svelte`. The `$state` and `$effect` pair is deliberate: reading storage during server rendering would make the server's HTML disagree with the browser's.

```svelte
<script lang="ts">
	import { isStarred, setStarred, voterToken } from '#lib/voter.js';

	let { id, count = $bindable() }: { id: string; count: number } = $props();

	// Read from storage after hydration, so the server's HTML (never starred)
	// matches what the browser hydrates.
	// eslint-disable-next-line svelte/prefer-writable-derived
	let starred = $state(false);
	let busy = $state(false);
	let problem = $state('');

	$effect(() => {
		starred = isStarred(id);
	});

	async function toggle() {
		busy = true;
		problem = '';
		try {
			const res = await fetch(`/api/v1/sentences/${id}/stars`, {
				method: starred ? 'DELETE' : 'POST',
				headers: { 'X-Voter': voterToken() }
			});
			if (!res.ok) throw new Error(`status ${res.status}`);
			count = ((await res.json()) as { count: number }).count;
			starred = !starred;
			setStarred(id, starred);
		} catch {
			problem = starred
				? 'Couldn’t remove your star. Try again.'
				: 'Couldn’t save your star. Try again.';
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

	.problem:empty {
		display: none;
	}
</style>
```

- [ ] **Step 4: Run them to verify they pass**

Run: `cd web && npx vitest --run --project client src/lib/components/StarButton.svelte.spec.ts && npm run check && npm run lint ; cd ..`
Expected: 4 passed, 0 type errors, and lint clean.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/components/StarButton.svelte web/src/lib/components/StarButton.svelte.spec.ts
git commit -m "feat: Add an accessible star toggle"
```

### Task 9: Permalink page

**Files:**
- Create: `web/src/routes/s/[id]/+page.server.ts`, `web/src/routes/s/[id]/+page.svelte`, `web/src/routes/+error.svelte`
- Test: `web/src/routes/s/[id]/page.server.spec.ts`, `web/src/routes/s/[id]/page.e2e.ts`

**Interfaces:**
- Consumes: everything from Tasks 2 to 8, `API_ORIGIN`, and `GET /api/v1/sentences/{id}` (200 or 404).
- Produces: the route `/s/{id}`, with Open Graph tags whose title is the sentence. It renders a 404 page for an unknown ID and a 502 page when the API is down or failing.

- [ ] **Step 1: Write the failing load tests**

`web/src/routes/s/[id]/page.server.spec.ts`:

```ts
import { isHttpError } from '@sveltejs/kit';
import { describe, expect, it } from 'vitest';
import { load } from './+page.server';

type Event = Parameters<typeof load>[0];

async function statusFor(fetch: typeof globalThis.fetch): Promise<number> {
	try {
		await load({ params: { id: 'aaaaaaaa' }, fetch } as unknown as Event);
	} catch (e) {
		if (isHttpError(e)) return e.status;
		throw e;
	}
	return 200;
}

describe('permalink load', () => {
	it('is a 404 for an unknown sentence', async () => {
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
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd web && npx vitest --run --project server page.server ; cd ..`
Expected: FAIL: `Failed to resolve import "./+page.server"`.

- [ ] **Step 3: Implement the load**

`web/src/routes/s/[id]/+page.server.ts`:

```ts
import { error } from '@sveltejs/kit';
import { API_ORIGIN } from '$app/env/private';
import type { Sentence } from '#lib/types.js';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, fetch }) => {
	let res: Response;
	try {
		res = await fetch(`${API_ORIGIN}/api/v1/sentences/${encodeURIComponent(params.id)}`);
	} catch {
		error(502, 'The sentence service is unavailable.');
	}
	if (res.status === 404) error(404, 'That sentence doesn’t exist.');
	if (!res.ok) error(502, 'The sentence service is unavailable.');
	return { sentence: (await res.json()) as Sentence };
};
```

Run: `cd web && npx svelte-kit sync && npx vitest --run --project server page.server ; cd ..`
Expected: 2 passed.

- [ ] **Step 4: Write the failing e2e tests**

`web/src/routes/s/[id]/page.e2e.ts`:

```ts
import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';

async function newSentence(page: Page): Promise<{ id: string; text: string; star_count: number }> {
	const res = await page.request.get('/api/v1/sentences/random');
	expect(res.ok()).toBe(true);
	return res.json();
}

async function expectNoAxeViolations(page: Page) {
	const { violations } = await new AxeBuilder({ page }).analyze();
	expect(violations).toEqual([]);
}

test('shows a saved sentence with link preview tags', async ({ page }) => {
	const s = await newSentence(page);
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
	const s = await newSentence(page);
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
	const s = await newSentence(page);
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
	const s = await newSentence(page);
	await page.setViewportSize({ width: 320, height: 640 });
	await page.goto(`/s/${s.id}`);
	await page.getByRole('group', { name: s.text }).getByRole('button').first().click();

	const overflow = await page.evaluate(
		() => document.documentElement.scrollWidth - window.innerWidth
	);
	expect(overflow).toBeLessThanOrEqual(0);
});
```

Run: `cd web && set -a && . ../.env && set +a && npx playwright test 'src/routes/s' ; cd ..`
Expected: FAIL. The page has no sentence group, and the error page has no "No sentence here" heading.

- [ ] **Step 5: Implement the page and the error page**

`web/src/routes/s/[id]/+page.svelte`. `selected` and `stars` are writable `$derived` values, so both reset when client-side navigation moves to another sentence:

```svelte
<script lang="ts">
	import { tick } from 'svelte';
	import { page } from '$app/state';
	import Sentence, { wordId } from '#lib/components/Sentence.svelte';
	import StarButton from '#lib/components/StarButton.svelte';
	import Structure from '#lib/components/Structure.svelte';
	import WordCard from '#lib/components/WordCard.svelte';
	import { leaves } from '#lib/tree.js';

	let { data } = $props();

	// Both reset when the page moves to another sentence.
	let selected = $derived.by<number | null>(() => {
		void data.sentence.id;
		return null;
	});
	let stars = $derived(data.sentence.star_count);
	const selectedPath = $derived(
		selected === null ? null : leaves(data.sentence.tree)[selected].path
	);

	async function closeCard() {
		const opener = selected;
		selected = null;
		await tick();
		if (opener !== null) document.getElementById(wordId(data.sentence.id, opener))?.focus();
	}
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

<Sentence sentence={data.sentence} bind:selected />

<div class="actions">
	<StarButton id={data.sentence.id} bind:count={stars} />
</div>

{#if selected !== null}
	<WordCard tree={data.sentence.tree} index={selected} onclose={closeCard} />
{/if}

<Structure tree={data.sentence.tree} {selectedPath} open />

<style>
	.actions {
		margin-block: 1.5rem;
	}
</style>
```

`web/src/routes/+error.svelte`:

```svelte
<script lang="ts">
	import { page } from '$app/state';
</script>

<svelte:head>
	<title>{page.status === 404 ? 'Not found' : 'Something went wrong'} · RandSense</title>
</svelte:head>

<h1>{page.status === 404 ? 'No sentence here' : 'Something went wrong'}</h1>
<p>{page.error?.message}</p>
<p><a href="/">Back to RandSense</a></p>
```

- [ ] **Step 6: Run everything**

Run: `just test`
Expected: the Go suite passes. In the web half, `svelte-check` reports 0 errors and 0 warnings, lint is clean, Vitest passes 39 tests, and Playwright passes 12 tests: one home test and five permalink tests, each in the desktop and phone projects.

- [ ] **Step 7: Check by hand**

Start `just run` and open a permalink from `curl -s localhost:8080/api/v1/sentences/random | jq -r .id`:
- With a screen reader (Orca, NVDA or VoiceOver), Tab through the words, open one, and close it with Escape.
- In devtools, at 320 CSS px wide and again at 200% text zoom, check that nothing scrolls sideways or gets cut off.

Note anything wrong as a finding.

- [ ] **Step 8: Commit**

```bash
git add web/src/routes
git commit -m "feat: Add the sentence permalink page with link previews"
```
