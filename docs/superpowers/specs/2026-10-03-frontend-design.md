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

Rename the GitHub repo `randsense-service` to `randsense` and make it a monorepo:

```
randsense/
  service/    the Go module as it is today: cmd/, internal/, migrations/, data/, sqlc.yml, .air.toml
  web/        SvelteKit app
  Justfile    top-level recipes for both halves
  CLAUDE.md, README.md, docs/
```

Move the Go code with `git mv` in one commit before any other change, so history follows the
files. The Go module path stays `github.com/jameynakama/randsense`. The Justfile gets `run-be`
(today's `run`), `run-fe`, and `run`, which starts both. Other recipes follow the same pattern
(`test-be`, `test-fe`, `test`).

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

The new look, from that:

- white background, black body text
- sentence words in `dodgerblue`, `hotpink` on hover or selection
- the title and accents in `deeppink`
- pill buttons with a `cornflowerblue` outline and `slategray` text, turning `cornflowerblue` on
  hover, with no transform
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

## Rate limits

nginx `limit_req` per IP:

- generation (`/api/v1/sentences/random`): generous, since signature embeds load from each
  viewer's own IP
- stars and flags: tight
- admin login: tight

The exact rates are set in the nginx config at deploy time.

## Testing

- **Go:** the existing style. Real-Postgres integration tests, one database per package (a new
  database-backed package needs its own suffix), and 100% coverage as the goal. Add N+1 tests for
  the list endpoints, hub unit tests (broadcast, dropping slow clients, cleanup on disconnect), a
  stream test over a real HTTP connection, and auth tests (wrong password, tampered cookie, expired
  cookie).
- **Web:** Vitest for logic (tree helpers, role detection, the voter token) and Playwright for the
  main flows: generate, star, flag a word, a feed update arriving over SSE, and admin login.

## Build order

Each stage ends working and committed.

1. Monorepo move, Justfile recipes, doc paths, GitHub rename.
2. Tree features (see "Tree data"), then saved sentences: migration, short IDs, saving on
   `random`, the list and single-sentence endpoints. Features come first so every saved tree
   has them.
3. Stars and flags endpoints.
4. SSE hub and stream endpoint, and broadcasting from `random` and the star endpoints.
5. Admin auth and admin endpoints, and `just hash-password`.
6. SvelteKit app: scaffold, sentence component, home page with live feed, permalink, stars page,
   flag form.
7. Admin UI.
8. Deployment: the nginx site config (routing, SSE settings, rate limits), two systemd units, and
   a build-and-deploy recipe.

Stages 2 to 5 are backend only and can be tried with `curl` and `http` as they land.
