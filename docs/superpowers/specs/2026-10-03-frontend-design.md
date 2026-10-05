# Frontend and sentence persistence design

A web frontend for randsense, plus the backend it needs: saved sentences, a live feed, permalinks,
anonymous stars, flags with comments, and a single-admin flag viewer. The project direction in
`CLAUDE.md` still holds: the generator stays deterministic and the absurdity is the point.

## Goals

- Generate a sentence and show it big, in the old client's palette.
- Let anyone open a sentence to see what's underneath: each word's lemma, part of speech and frame,
  and the parse tree.
- Show the latest 30 sentences anyone generated, updating live. Watching sentences arrive was the
  best part of the original site.
- Give each sentence a permalink page with a good link preview.
- Let anyone star a sentence (public counts) or flag the sentence or one word in it, with a
  required comment.
- Let the one admin read flags and see which words collect the most complaints.
- Keep the public API open. Third-party sites used the original in forum signatures.

## Non-goals

User accounts, an interactive diagram builder, educational features (lexicon browsing, grammar
lessons, definitions, etymologies), disabling words through the admin (the roadmap's curation
loop), and the compiled lexicon. These come later and nothing here should block them.

## Repository

The GitHub repo `randsense` is a monorepo:

```
randsense/
  service/    the Go module as it is today: cmd/, internal/, migrations/, data/, sqlc.yml, .air.toml
  web/        SvelteKit app
  Justfile    top-level recipes for both halves
  CLAUDE.md, README.md, docs/
```

The Go module path stays `github.com/jameynakama/randsense`. The root Justfile pairs recipes:
`run-be` and `run-fe`, with `run` starting both, and likewise `test-be`, `test-fe` and `test`.

A monorepo was chosen because one developer changes the API and the frontend together.

## Architecture

- **Go owns all data:** generation, Postgres, stars, flags, the live stream and admin auth.
- **SvelteKit is presentation only.** Its server-side loaders call the Go API on localhost for
  first renders and link previews. In the browser it calls `/api/...` on the same origin.
- **Production** runs on the existing DigitalOcean droplet: nginx routes `/api/` to the Go service
  and everything else to SvelteKit's Node server (`adapter-node`). Both run as systemd units, and
  Go listens on localhost only.
- **Development:** Vite proxies `/api` to Go, so the browser sees one origin in both.

Rejected: Go templates with htmx. It would be one binary with no build step, but the diagram
builder planned for later needs a real component framework.

## Data

One migration adds:

**`sentences`**

| column | type | notes |
|---|---|---|
| `id` | text, primary key | 8 random base-62 characters, generated in Go, retried on collision. Unlike sequential IDs, it doesn't reveal the sentence count. |
| `text` | text | |
| `tree` | jsonb | the `grammar.Node` tree, as the API returns it today |
| `commonness` | numeric | the `commonness` floor the request used (see `README.md`) |
| `star_count` | integer, default 0 | kept in step with `stars` in the same transaction, so the feed never counts rows |
| `created_at` | timestamptz | indexed for the feed |

**`stars`**: `sentence_id`, `voter` (text), `created_at`. Unique on (`sentence_id`, `voter`).

**`flags`**: `id`, `sentence_id`, `word_index` (nullable; null means the whole sentence),
`lemma` and `pos` (nullable, copied from the tree when `word_index` is set), `comment` (text, not
null), `created_at`.

Copying `lemma` and `pos` onto the flag makes "most-flagged words" a plain `GROUP BY` without
reading stored trees. `word_index` is zero-based over the tree's leaves in order, commas
included. A comma leaf can't be flagged: the server rejects that index with 400.

## Tree data

Grammar nerds should have something to dig into, so the saved tree keeps what generation works
out rather than discarding it. Today a node has `symbol`, and a leaf also has `lemma` (the base
word) and `word` (as inflected); the slot's frame is the qualifier in `symbol`
(`Verb:transitive`). `grammar.Node` gains a typed `features` object, with empty fields omitted:

```json
{"symbol": "S", "features": {"tense": "past", "commonness": 0}, "children": [...]}
{"symbol": "NP", "features": {"person": 3, "number": "plural"}, "children": [...]}
{"symbol": "Verb:transitive", "lemma": "set on fire", "word": "set",
 "features": {"tense": "past", "form": "finite", "person": 3, "number": "singular",
              "frames": ["transitive", "intransitive"], "separable": true, "frequency": 3.12}}
{"symbol": "Pronoun:reflexive", "lemma": "herself", "word": "herself",
 "features": {"case": "reflexive", "person": 3, "number": "singular", "gender": "fem"}}
```

| node | features |
|---|---|
| root | `tense`, `commonness` |
| NP | `person`, `number` (its agreement) |
| noun | `number`, `frequency` |
| verb | `tense`, `form` (`finite`, `base` or `gerund`), `person`, `number` (what it agreed with), `frames` (every frame the lemma has), `separable`, `frequency` |
| adjective, adverb | `frequency` |
| pronoun | `case`, `person`, `number`, `gender` |
| determiner | `type`, `number` |

`frequency` is the Zipf value from SUBTLEX-US, absent when the word has none. Lexicon row IDs are
left out on purpose: ingest truncates and reloads, so they change on every re-ingest. Lemma plus
part of speech is the stable key. The realize endpoint returns the same features.

## API

All under `/api/v1`. Responses are JSON unless noted. List endpoints use limit-offset pagination,
ordered by `created_at` descending with `id` as the tie-breaker.

### Public

| Endpoint | Behavior |
|---|---|
| `GET /sentences/random[?commonness=N]` | Generates, **saves**, broadcasts to the stream, and returns `{id, text, tree, star_count, created_at}`. CORS open to any origin. |
| `GET /sentences?limit=30&offset=0` | Latest saved sentences, newest first. |
| `GET /sentences/{id}` | One sentence, or 404. |
| `GET /sentences/stream` | Server-Sent Events. See below. |
| `POST /sentences/{id}/stars` | Star. The voter token comes in an `X-Voter` header. Idempotent. Returns `{count}` and broadcasts. |
| `DELETE /sentences/{id}/stars` | Unstar, same header. Idempotent. Returns `{count}` and broadcasts. |
| `GET /stars` | Sentences the `X-Voter` token starred, by star time, newest first. |
| `POST /sentences/{id}/flags` | `{comment, word_index?}`. The comment must be 10 to 1,000 characters after trimming. The server checks `word_index` against the stored tree and fills `lemma` and `pos` itself. 201 on success. |
| `POST /sentences/realize` | Unchanged. Not saved and not broadcast: it's a debugging tool and must not flood the feed. |

The voter token is a random UUID the browser makes on first visit and keeps in local storage. It
isn't a user account and holds nothing personal. Clearing storage lets someone star again, which
is acceptable for a toy.

### Admin

| Endpoint | Behavior |
|---|---|
| `POST /admin/login` | `{password}`. Compares against `ADMIN_PASSWORD_HASH` (bcrypt) and sets the session cookie. Failures wait about a second before answering. |
| `POST /admin/logout` | Clears the cookie. |
| `GET /admin/flags` | Flags newest first, each with its sentence's `id`, `text` and `tree`. |
| `GET /admin/flagged-words` | `{lemma, pos, count}`, most-flagged first. |

Every `/admin/*` route except `login` requires a valid session.

## Admin auth

There's one admin and no user table.

- `ADMIN_PASSWORD_HASH` holds a bcrypt hash and is set in the systemd unit's environment. The
  password itself is never stored. `just hash-password` prints a hash for a typed password.
- A successful login sets a cookie holding an expiry time signed with HMAC-SHA256, keyed by
  `SESSION_SECRET` (also from the environment). The cookie is `HttpOnly`, `Secure` (except in
  development), `SameSite=Strict`, and expires after 14 days.
- Middleware on `/admin/*` checks the signature and expiry.
- Changing the password or the secret means updating the environment and restarting, and changing
  the secret logs every session out.

Rejected: nginx basic auth. It needs no code, but it protects nothing when Go is reached directly,
it's absent in development, and the browser popup handles background API calls badly.

## Live stream (SSE)

Server-Sent Events, not WebSockets: data only flows from server to browser, and stars and flags
are ordinary POSTs. SSE is plain HTTP and browsers reconnect by themselves. nginx buffers
responses by default, so the stream sets `X-Accel-Buffering: no` to turn that off.

- **Events:** `sentence` (the full sentence JSON) and `stars` (`{id, count}`).
- **Hub:** one goroutine keeps the set of clients, each with a small buffered channel. Saving a
  sentence or changing a star count publishes to the hub, which sends to every client without
  blocking. A client whose buffer is full is dropped, and its browser reconnects.
- **Handler:** sets `Content-Type: text/event-stream`, `Cache-Control: no-cache` and
  `X-Accel-Buffering: no`, then loops over the client channel, a 25-second keepalive ticker (a
  `:` comment line) and `r.Context().Done()`, flushing after each write.
- **Reconnects:** no replay. When the browser's `EventSource` reopens, the page refetches the
  latest 30 over plain HTTP.
- **Scaling:** one process broadcasts from memory. A second process would publish through Postgres
  `LISTEN`/`NOTIFY` instead. That isn't needed now, and nothing here blocks it.

## Frontend

SvelteKit, TypeScript in strict mode, plain CSS through Svelte's scoped styles (no Tailwind),
`adapter-node`.

### Look

The old client is `jameynakama/randsense-client` (React, 2021); this section describes it so it
needn't be read again. Keep its palette and readability, drop its tilting buttons, and change the
layout freely.

What it was:

- A single centered column, 60% wide (90% below 1024px), all text centered. The font was the
  system stack (`-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, ...`) on a white page.
- A "RandSense" `h1` at 5rem in `deeppink`.
- A "Generate sentence" button: transparent background, 2px `cornflowerblue` border,
  1rem radius, 1rem × 2rem padding, 1.5rem bold uppercase `slategray` text. On hover and focus
  the text turned `cornflowerblue` (it also rotated 3° and scaled up; that goes). Disabled, it
  was `ghostwhite` with a `slategrey` border and text.
- The sentence: each word a borderless button at 4rem in `dodgerblue`, turning `hotpink` on
  hover, with 0.5rem margins. Punctuation sat tight against the word before it.
- Under it, "See sentence data" and "Is this grammatically incorrect?" buttons in the same style.
  The second became a disabled "Thank you!" after one click, with no comment.
- Clicking a word, or "See sentence data", showed the raw JSON in a collapsible JSON viewer,
  1.5rem, 70% wide and left-aligned. A word's panel had a "Vote to remove this word?" button that
  also became "Thank you!".
- Errors showed as bold red text.

The new look keeps the old palette for identity and makes the chrome iOS-like and neutral, with
darker shades where a color fails WCAG AA contrast on white or the `#F2F2F7` gray fill:

- white background, black body text, `#6C6C70` secondary text (4.7:1 on the fill)
- sentence words in `dodgerblue` (3.2:1, which passes only as large text, so the sentence never
  goes below 24 CSS px), turning `#C2185B` (5.9:1) on hover or selection in place of `hotpink`
  (2.7:1). `dodgerblue` fails on the gray fill, so it stays on white
- the title in `deeppink` (3.6:1, large text only, so at least 18.67 CSS px bold); smaller
  accents in `#C2185B`
- no outlined pills. Capsule buttons in three weights, all in action blue `#0A5FC2` (4.75:1
  even as text on its 10% tint over the fill): filled for the one primary action per view
  (Generate, Send), tinted for stars and Close, plain text for secondary actions
- a compact header bar: the title left, the stars link right. The stars page shows its `h1` in
  the bar as "RandSense / Your stars"
- sentence lists as one rounded gray group with hairline separators, text left and the star
  button right; the word card and flag form as gray cards, the word card a bottom sheet on phones
- a fixed type scale: 15, 17, 22 and 28 px, plus the sentence
- feed and other normal-size text in black or action blue, never `dodgerblue`
- sentences set large (about 4rem on desktop), smaller on phones in a single column
- one readable font for the UI and sentences, falling back to the system stack
- the raw-JSON viewer is replaced by the word card and structure outline below

### Pages

- **`/`**: title, a "Generate" button, the current sentence, then the live feed of the latest 30.
- **`/s/{id}`**: the permalink, with the sentence and its full detail view. Server-rendered Open
  Graph tags use the sentence as the title.
- **`/stars`**: this browser's starred sentences.
- **`/admin/login`** and **`/admin`**: flags newest first, and a "most-flagged words" tab.

### Sentence component

Used on the home and permalink pages.

- Words render as buttons. Punctuation sits against the word before it.
- Clicking a word opens its card: the word, lemma, part of speech, the slot's frame (from the
  qualified symbol, such as `Verb:transitive`), every field in its `features` (labeled readably:
  "past tense", "3rd person plural", "reflexive"), and its grammatical role, derived from its
  position in the tree (subject, object, inside an infinitive or a prepositional phrase). A word
  with no recognizable role shows none. It's all computed from the tree, with no extra requests.
  Commas aren't clickable.
- "Show structure" expands the tree as an indented, collapsible outline (S › NP › VP...). The
  selected word highlights its branch. A drawn diagram waits for the diagram builder.
- A star button with its live count.
- A "Something's wrong" button opens the flag form. Its target selector defaults to "the whole
  sentence"; clicking a word switches the target to that word and highlights it. If a word was
  already selected, the form opens targeting it. A separate comment field is required: 10 to
  1,000 characters after trimming.

### Feed

Each item shows the sentence text (linking to its permalink) and a star button with its count. New
sentences slide in at the top, star counts update in place, and the list stays at 30.

### Accessibility

Meet WCAG 2.2 AA from the first component.

- **Contrast:** the colors under "Look" are chosen to pass. Focus shows as a visible ring with at
  least 3:1 contrast against the colors next to it, and is never removed.
- **Keyboard:** everything works without a mouse. Words are native buttons in reading order.
  Escape closes the word card and the flag form, and focus returns to what opened them.
- **Screen readers:** the sentence's group is labeled with its full text, and each word button
  with its word. Opening the word card moves focus to its heading. Star buttons use
  `aria-pressed` and include the count in their name ("Star, 12 stars").
- **Structure outline:** a nested list with expand and collapse buttons (`aria-expanded`), not an
  ARIA tree widget. That's the simplest pattern that works.
- **Admin tabs:** the full ARIA tabs pattern: tab and panel roles, labels, and arrow-key
  movement between tabs.
- **The live feed** updates by itself, so it has a pause control (WCAG 2.2.2), and new sentences
  aren't read aloud one by one. A polite live region announces only the visitor's own generated
  sentence.
- **Motion:** the feed's slide-in and other transitions turn off under
  `prefers-reduced-motion`.
- **Forms:** the flag form and admin login have visible labels. Errors and the comment's
  character count are tied to their fields with `aria-describedby`.
- **Semantics:** one `h1` per page, landmarks (`header`, `main`, `nav`) and `lang="en"`.

### Small screens

Phones get the same features, laid out for one column and touch:

- one column below about 640 CSS px; the sentence shrinks to no less than 24 CSS px
- interactive targets at least 44 by 44 CSS px, words included (stricter than AA's 24), and
  nothing that only works on hover
- the word card opens as a bottom sheet on narrow screens, and never covers a focused control
  (WCAG 2.4.11)
- at 320 CSS px wide the page reflows with no horizontal scrolling, and text enlarges to 200%
  without losing content. The structure outline narrows its indents to fit, and scrolls
  sideways inside its own box only for a branch too deep to fit any other way

## Rate limits

nginx `limit_req` per IP:

- generation (`/api/v1/sentences/random`): generous, since signature embeds load from each
  viewer's own IP
- stars and flags: tight
- admin login: tight

The exact rates are set in the nginx config at deploy time.

## Testing

- **Go:** the existing style. Real-Postgres integration tests, one database per package (a new
  database-backed package needs its own suffix), and 100% coverage as the goal.
- **Web:** Vitest for logic (tree helpers, role detection, the voter token) and Playwright for the
  main flows: generate, star, flag a word, a feed update arriving over SSE, and admin login.
  Playwright runs axe (`@axe-core/playwright`) on every page and its key open states (word card,
  flag form, structure outline) with no violations allowed, runs the main flows again in a
  phone viewport (about 390 by 844), and walks one flow by keyboard alone: generate, open a word,
  flag it. Axe can't prove conformance, so screen-reader behavior, reflow at 320 CSS px and 200%
  text get a manual check before each stage ships.

## Build order

The backend (stages 1 to 5) is built. What remains, in order, each stage ending working and
committed:

6. SvelteKit app in `web/`: before it ships, someone checks it by hand with a screen reader, at
   320 CSS px and at 200% text.
7. Admin UI, under the same accessibility and small-screen rules.
8. Deployment: the nginx site config (routing, SSE settings, rate limits), two systemd units, and
   a build-and-deploy recipe.
