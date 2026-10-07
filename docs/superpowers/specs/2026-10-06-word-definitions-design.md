# Word definitions design

The word card shows what a word means. Open English WordNet already ships a gloss for every
synset; ingest currently drops them. The card shows a content word's OEWN senses, and every card
links to Wiktionary.

## Goals

- Show a content word's OEWN definitions on its card, reader first: a few senses, then the rest
  on request.
- Give every word, closed-class included, a way to read more: a Wiktionary link.
- Keep trees, stored sentences and keep tokens the size they are now.

## Non-goals

- Hand-written glosses for closed-class words. That is new curation, and the Wiktionary link
  covers those words.
- Picking the sense a sentence "uses". The generator never chooses a sense, and every reading is
  grammatical.
- Cleaning up glosses. Usage notes like "(usually followed by 'to')" stay as shipped.

## Decisions

- **Every sense, not the first.** A sentence is grammatical under every sense and absurd under
  most of them, so no one sense is the right one to show.
- **A lazy endpoint, not definitions in the tree.** Embedding glosses would add dozens of strings
  to every generated sentence, every `sentences.tree` row and every signed keep payload, for text
  read only after a tap. It would also freeze old feed rows to the glosses of the day they were made.
  A static gloss file for the frontend was rejected for size (several megabytes).
- **Wiktionary for the link.** Its URL scheme has been stable for two decades, it covers closed-class
  words and the long tail, and titles are case-sensitive the way lemmas are ("Hopi", "hopi").
  Commercial dictionaries are thin on rare words and carry ads.
- **`active` is ignored.** A deactivated word in an old feed sentence still shows its meaning.

## Data and ingest

- A migration adds `definitions JSONB NOT NULL DEFAULT '[]'` to `nouns`, `verbs`, `adjectives`
  and `adverbs`.
- The parser keeps each `<Sense>`'s `synset` attribute, in order, on `Entry`.
- Synsets come after every `<LexicalEntry>` in the file, so ingest reads it twice: the first pass
  maps synset ID to its `<Definition>` text, the second ingests entries and resolves their synset IDs
  to glosses, in sense order.
- Several LexicalEntries can share a lemma and table: the `a` and `s` adjective codes, and
  entries split by etymology. Ingest pools them into one row with every entry's glosses, in entry
  order; verb frames come from the first entry. The ingest upsert design covers how.

## API

`GET /api/v1/words/{pos}/{lemma}`

- `pos` is `noun`, `verb`, `adjective` or `adverb`. The client URL-encodes the lemma ("sea%20anemone").
- `200 {"definitions": ["...", ...]}` in OEWN sense order. The array can be empty.
- `404` when no row matches, or for any other `pos`.
- `Cache-Control: public, max-age=86400`. Glosses change only on reingest.
- The lookup uses the `UNIQUE (lemma, source)` index.
- `deploy/randsense.nginx` gets a `location ~ ^/api/v1/words/` in the `randsense_generate` zone
  (10 r/s per IP, burst 30), so the endpoint isn't left under the unlimited `/api/` block.

## Frontend

- `web/src/lib/definitions.ts`, shaped like `stars.ts`: `definitions(fetch, pos, lemma)` returns
  the array, `[]` on a 404, and throws on any other failure.
- `WordCard` fetches only for nouns, verbs, adjectives and adverbs, and fetches again when `index`
  changes, since tapping another word reuses the open card.
- While loading, and after an error, the card has no definitions section: everything else renders
  at once and the glosses appear when they arrive.
- When glosses exist, the card shows a "Definitions" heading over an `<ol>` of the first three.
  With more than three, a "Show all N" button (`aria-expanded`) reveals the rest. A new word starts collapsed.
- Below the list: "Definitions from Open English WordNet (CC BY 4.0)", linking to
  `https://en-word.net/`. OEWN's license requires the credit.
- Every card gets an "Open in Wiktionary" link above Close:
  `https://en.wiktionary.org/wiki/<lemma>#English`, built from the lemma, not the inflected word,
  with spaces as underscores and the rest URL-encoded. It opens in a new tab with `rel="noopener"`.

## Testing

- **Parser:** `testdata/sample.xml` gains `<Synset>` elements with `<Definition>`s; entries carry
  their synset IDs in sense order.
- **Ingest:** definitions land in sense order; an `a` and an `s` entry with the same lemma end up
  in one row with both entries' glosses.
- **Handler:** 200 with glosses, 404 for an unknown lemma, 404 for a closed-class `pos`, the cache
  header.
- **`definitions.spec.ts`:** a 404 returns `[]`; other failures throw.
- **`WordCard.svelte.spec.ts`:** no fetch for a closed-class word; three senses, then "Show all N"; no
  section without glosses; the Wiktionary href for a multiword and a capitalized lemma.
- **e2e:** opening a noun's card shows a definition, against CI's freshly ingested database.
