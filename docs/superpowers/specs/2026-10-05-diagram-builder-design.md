# Sentence diagram and builder design

Every sentence gets a drawn diagram. In the builder, anyone grows a diagram slot by slot, fills
it with words, locks the words they like and rerolls the rest. The app should be fun,
educational and interactive. The direction in `CLAUDE.md` still holds: the grammar and
lexicon are deterministic, and the builder only ever offers grammatical structure.

## Goals

- Draw every generated sentence as a constituency tree.
- Let anyone build a tree from `S` down, choosing only among the grammar's own expansions, so nobody can
  build an ungrammatical diagram.
- Fill a full or partial tree with words, lock words, and reroll the rest with agreement intact.
- Keep a built sentence: it gets a permalink and stars, and joins the feed with a Homemade badge.
- Remix any sentence: open it in the builder with its tree and words.
- Serve the grammar to the frontend, so editing `grammar.toml` never needs a frontend change.

## Non-goals

These come later, and nothing here should block them:

- A diagram game: serve a sentence and have the player diagram it. It reuses the tree, the
  renderer and the builder's slots.
- Dragging rule cards onto slots. Tapping comes first because it works the same with a keyboard,
  a screen reader and a phone.
- Picking words yourself instead of rerolling. That is a lexicon-browsing UI, and it overlaps
  with the curation loop.
- A commonness slider. The builder uses the API's default.
- A Reed-Kellogg ("school") view of the same tree.

## Decisions

- **Constituency tree, not Reed-Kellogg.** It matches the generator's tree node for node.
  Reed-Kellogg has to be derived from roles, and coordination, subordinate clauses and the dummy
  "it" clauses would each need their own layout rules.
- **Build top-down.** Every slot offers only its rules from `grammar.toml`, so correctness comes
  from construction and the browser needs no parser. Two alternatives were rejected:
  - Bottom-up snapping (tiles bracket into phrases) needs a parser in the browser, and a legal
    row can still be impossible to finish.
  - Left-to-right prediction needs an Earley parser, and one row can match several trees.
- **Classic layout.** The tree grows down, the words sit on a baseline in sentence order, and the
  bottom row is the tappable sentence. On phones it opens fitted to the width, and panning and
  zooming are fine. A sideways tree fits phones better, but it doesn't read as a sentence or as a
  diagram. Brackets under the sentence break when the sentence wraps, and empty slots can't grow
  down.
- **Rerolls are throwaway; Keep is deliberate.** Saving every reroll would bury the good ones.
  Because keeping is a choice, kept sentences can join the feed without flooding it.
- **Locks hold lemmas, not words.** A locked word is re-inflected for agreement, so a locked
  plural noun still drags its verb along. That is the educational point of locking.

## Grammar endpoint

`GET /api/v1/grammar` returns:

- `start`: `"S"`
- `phrases`: each nonterminal with its `label`, its `description` and its `rules` in toml order.
  Each rule is its expansion, a list of symbols.
- `slots`: each part of speech and each qualified slot that appears in a rule, with a `label`, a
  `description` and, for verb frames, an `example`. For instance, `Verb:transitive` is "transitive
  verb", "takes an object", "devoured the goose".

Weights are not served. The server expands holes itself (see Realize), and toml order already
lists the common rules first.

The labels, descriptions and examples live in `grammar.toml`, in `[phrase.X]` and `[slot."X"]`
tables. `Load` rejects a label for a symbol the rules never use. `Grammar.Labeled` reports any
phrase, or any slot a rule uses, that has no label. The server refuses to start on that error,
and the project grammar's test checks it too, so a new construction can't ship unlabeled. Load
itself doesn't require labels, so tests can keep their small unlabeled grammars.

The grammar is fixed for the life of the process, so the response is cacheable. SvelteKit loads
it server-side for `/build`.

## Grammar check

`realize` checks trees with `Grammar.Check(tree)`. It requires:

- the root is the start symbol
- every inner node's child symbols equal one of that symbol's rules exactly
- every leaf is a part of speech (optionally qualified, as today) or a phrase symbol with no
  children, which is a hole

A tree that skips a level (`S → NP VP` with no `Clause`) returns 400.

## Realize

`POST /api/v1/sentences/realize` changes in three ways:

1. **Holes.** Each phrase leaf is expanded by weight before filling, using the grammar's existing
   expansion.
2. **Locks.** A leaf with `"locked": true` keeps its `lemma`. Every other leaf is cleared, as
   today. `locked` is a field on `grammar.Node` (omitted when false) and is echoed back.
   - A locked lemma must fit its slot: a noun lemma in the noun table, a verb lemma with the
     slot's frame, a determiner, pronoun or conjunction of the slot's kind. Otherwise the response
     is 422, naming the leaf's index.
   - Locked words are looked up by lemma, not drawn at random. That needs a lookup query per
     part of speech that returns the same row as its random query. They are re-inflected like any
     filled leaf.
   - The commonness floor doesn't apply to locked lemmas, so a remix can lock a rare word.
   - A locked singular determiner ("a", "this") restricts its NP's noun to lemmas that aren't
     plural-only ("Rastas"). Today the noun is always filled first and the determiner restricted
     to match it.
3. **Signature.** The response includes `signature`, an HMAC-SHA256 under `BUILD_SECRET` over the
   canonical JSON of the realized tree and an issue time. Keep checks it.

When no word fits a slot (an empty frame at the floor), the 422 names that leaf's index, so the
builder can highlight it.

`realize` still saves nothing and doesn't broadcast.

## Keep

`POST /api/v1/sentences` with `{tree, signature}` saves a built sentence and returns it as `GET
/sentences/{id}` would.

- The signature has to match the tree and be less than 24 hours old, or the response is 400. The
  signature is what stops anyone from posting a tree full of their own words. Without it, a
  permalink and the feed would show any text under RandSense's name. The server keeps no state
  between realize and keep.
- The text is formatted from the tree on the server, never taken from the client.
- It saves with `origin = 'built'` and broadcasts a `sentence` event like `random`.

`BUILD_SECRET` (32+ bytes) is a new setting beside `SESSION_SECRET`. Changing it invalidates
unkept signatures, which costs only a reroll.

**Data.** A migration adds `origin` (text, not null, default `'generated'`, values `generated`
and `built`) to `sentences`. Every endpoint that returns a sentence includes `origin`.

**Rate limits.** In nginx, `realize` is limited like generation, since every reroll calls it.
Keep is tight, like stars.

## Frontend

### Diagram

`Diagram.svelte` draws a tree. It's used read-only on the home and permalink pages and editable
in the builder.

- **HTML nodes over SVG lines.** Labels and words are HTML elements, buttons where they act,
  positioned by a computed layout. One SVG layer beneath them draws the connectors. Real elements
  keep focus, keyboard use, the font and the 44px targets working as elsewhere. SVG text can't
  be focused or wrapped the same way.
- **Semantics.** The DOM is a nested list in tree order, each item named by its grammar label.
  Screen readers get an outline and sighted users get the tree.
- **Layout** is a pure function in `lib/diagram.ts` from a tree and measured node sizes to
  positions:
  - leaves run left to right on the baseline, spaced by width
  - each phrase is centered over its first and last child
  - each depth is a row
  
  Nodes render once to be measured, then get positioned. d3's tree layout doesn't put the leaves
  on a baseline, and this is short enough to own.
- **Fit and zoom.** The tree opens scaled down to fit its container's width. A Zoom toggle shows
  it full size with sideways scrolling. The page's own pinch-zoom still works, so there's no
  gesture code.
- **Selection** comes from the sentence above it: selecting a word there highlights its branch
  and word in the diagram. On the read-only diagram, words aren't buttons, because a tree scaled
  down to fit would shrink them below 44px. The builder's editable mode settles its own targets.
- **Editable mode** draws holes as dashed "NP ?" buttons and shows each word's lock state.

On the home and permalink pages, "Show diagram" opens it. A "Remix" button there links to
`/build?from={id}`.

### Builder

`/build` is linked from the header.

- It starts as a single `S` hole. Tapping a hole opens `ExpansionSheet.svelte`, which lists that
  symbol's rules with their labels and descriptions. It's a bottom sheet on phones and a card on
  desktop, like the word card.
- Picking a rule replaces the hole with the rule's symbols. Phrases become holes and parts of
  speech become leaves.
- Tapping a filled phrase offers Change (pick another rule, replacing everything under it) and
  Clear (back to a hole). Either one drops the locks beneath it.
- **Fill** is always available. It posts the tree, holes and all, to `realize` and draws the
  words.
- After a fill, tapping a word toggles its lock. **Reroll** posts again with the locks.
- **Undo** steps back through structure changes, fills and locks. It's a stack of tree snapshots.
- **Keep this one** posts the last realize result and its signature, then goes to the new
  permalink.
- **Remix** (`?from={id}`) loads that sentence's tree and words with nothing locked. Keep stays
  disabled until the first reroll, because only `realize` output is signed.
- **Errors.** A 422 highlights the leaf it names and says what to do: change the slot, or unlock
  the word.

The state changes (expand, change, clear, toggle a lock, undo) are pure functions in
`lib/builder.ts`, so the page only wires them to the UI.

### Badge

Feed rows and permalinks of built sentences show a small "Homemade" label in the secondary text
color.

## Testing

- **Go:**
  - `Check`: valid trees, trees with holes, a wrong expansion, a wrong root
  - `Load` rejects a missing label
  - the grammar endpoint's shape
  - `realize`:
    - fills holes
    - keeps locked lemmas and re-inflects them
    - rejects a lock that doesn't fit
    - keeps a locked singular determiner off plural-only nouns
    - returns a signature
  - Keep:
    - saves with `origin` and broadcasts
    - rejects a tampered tree, an expired signature and a missing one
- **Vitest:**
  - layout: no overlaps, each parent centered over its children, leaves in sentence order
  - the builder's state functions, including undo
  - component tests for `Diagram` (selection, editable holes) and `ExpansionSheet`
- **Playwright:**
  - build a sentence by keyboard alone, fill it, lock a word, reroll (the locked lemma stays),
    and keep it; the permalink and the feed show the badge
  - remix from a permalink
  - axe on `/build` with the sheet open and on the diagram
  - reflow at 320px
