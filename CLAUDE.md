# randsense notes

Project decisions, planned work and known quirks. Setup, commands and API details are in
`README.md`.

## Direction

The goal is grammatically sound nonsense from a deterministic grammar and lexicon ("the dog
devoured the philosophy"). The absurdity comes from correct syntax without semantic restrictions:
verb frames constrain syntax only, never what a verb's subject or object may be. Never replace the
generator with an LLM. LLMs are fine for offline curation only, such as batch-labeling words.

## Settled decisions

Don't reopen these without new evidence.

- **Word source.** OEWN 2025 supplies the words. Small lexicons repeat the same words.
- **Morphology source.** The NLM SPECIALIST lexicon from the previous Django app has a structural
  medical bias that filtering can't remove, so it supplies only morphology:
  `service/data/lexicon/verb_morphology.toml` comes from its `<variants>` lines (irreg, regd).
- **Curation never edits source data.** OEWN, SUBTLEX-US and the SPECIALIST extraction stay as
  shipped. Hand curation lives in its own discoverable places, so the compiled lexicon can tell
  sourced from curated: `verb_morphology_curated.toml`, `separable_verbs.toml`,
  `closed_class.toml`, and `mislabeled` in `oewn/frames.go`.
- **Re-extraction.** If more morphology is needed, re-extract from SPECIALIST; the extraction
  script was throwaway.
- **No ProperNoun slot.** OEWN has dropped nearly all named entities, so capitalized nouns are
  mostly common nouns (Hopi, Amati, Rastas) and take determiners.
- **Grammar-first verb frames.** The grammar picks `Verb:<frame>` and a verb with that frame is
  fetched. Picking the verb first and expanding from its frames would take structure and weights
  away from `grammar.toml`.
- **Commas.** A comma appears only before a clause-coordinating conjunction; without one, "for"
  and "so" read as prepositions. Subordinate clauses only follow the main clause, so they need no
  comma.
- **Coordination agreement.** "or" and "nor" agree with their last part; "and" is plural, in the
  lowest person among its parts ("you and I ... ourselves").
- **Rare words are a feature.** `commonness` is a per-request floor, never an ingest filter.
  Frequency comes from SUBTLEX-US counts per part of speech; plain word-form counts let common
  spellings pass in rare roles (verb "baby", noun "meet"). When a frame has no verbs at the floor,
  `Generate` expands a fresh tree. Capping the floor per frame was rejected because it would
  quietly break the floor's guarantee.
- **Mislabeled OEWN frames.** OEWN gives many lemmas frames they can't take ("hoped singing",
  "broke even ugly"). Exclude those lemma-frame pairs using `mislabeled` in `oewn/frames.go`; each
  lemma keeps its other frames. A frame code with too many mislabeled lemmas stays unmapped: the
  bare-infinitive codes (`via-inf`, `vtaa-inf`, `vii-inf`) list allow, permit and induce alongside
  let, make and have ("allowed him go"), so only the to-infinitive codes are used. The bar for
  `mislabeled` is no grammatical reading at all, counting "in order to" readings and archaic or
  dialect uses: "found her to devour", "learned us to", "pray me to" and "tipped her to" stay.
- **Separable verbs are syntax only.** `service/data/lexicon/separable_verbs.toml` lists multiword
  verbs whose object goes after the head verb ("look it up", "set the goose on fire"). An idiom
  is listed only when the other order breaks the syntax, never to keep its meaning.
- **One test database per package.** `go test ./...` runs packages in parallel, and each
  database-backed package drops and recreates its database. Each derives its own name from
  `TEST_DATABASE_URL` with a package suffix (`_api`, `_oewn`); a new one needs its own.

## Roadmap

1. **Frontend and saved sentences.** Designed in
   `docs/superpowers/specs/2026-10-03-frontend-design.md`: monorepo move, saved sentences, live
   feed over SSE, permalinks, stars, flags and a single-admin flag viewer. The backend
   and the public app are built; the spec's "Build order" lists what remains. Later, try other
   entrance animations for sentences arriving in the feed.
2. **Sentence diagram and builder.** Designed in
   `docs/superpowers/specs/2026-10-05-diagram-builder-design.md`: a drawn tree for every
   sentence, a top-down builder with word locks and rerolls, kept sentences with a Homemade
   badge, and Remix. What's left is its "Non-goals" list (a diagram game, drag and drop, picking
   words, a commonness slider, a Reed-Kellogg view). The API's default floor is 1, the best for
   sentences, until a slider exposes it.
   - **Keep can be replayed:** a signed tree can be kept again for 24 hours, each time a new row
     in the feed. The tight nginx limit on keep is the only guard, so it has to be in the deploy
     config.
3. **Idioms with a broken object frame.** OEWN marks "give birth", "find fault" and "pull wires"
   transitive, which gives "gave birth the goose". Each idiom needs a label: keep the frame,
   drop object frames (keeping "she gave birth"), or move it to a fixed-preposition frame ("gave
   birth to mud"). Some prepositions ("find fault with", "take kindly to") would need new
   frames. Keep the idioms themselves.
4. **Curation loop, after the frontend.**
   - **Flags are the votes:** the frontend's flags, with their copied lemma and part of speech,
     are what flag words for review.
   - **Admin:** grow the frontend's flag viewer into a UI for disabling or removing entries.
     No user accounts. psql on the server was rejected as the admin UX.
   - **Prerequisite:** ingest currently truncates and reloads, which would wipe `active`,
     `vote_count` and corrections to the heuristic `nouns.plural` flag. That flag misfires on
     Taos, Sauternes and tabes.
5. **LLM batch labeling** for curation.
6. **More sentence types:**
   - passive voice, from transitive frames only; `verb_morphology.toml` already has past
     participles
   - questions
   - conditionals
7. **Compiled lexicon, once a base version feels done.** Build every source (OEWN, SUBTLEX-US,
   `mislabeled`, the `service/data/lexicon/` lists) into one versioned file that ingest loads
   and that can be shared as research. It could also become where curation corrections live, so they
   survive re-ingest. Gzipped JSON Lines diffs well; WN-LMF XML fits poorly once synsets are
   gone. Check whether SUBTLEX-US and SPECIALIST allow redistribution before publishing; OEWN is
   CC BY 4.0.

## Known quirks

These are deliberately left alone.

- **Accepted as funny, not bugs:** pluralized mass nouns and names ("soccers", "smokings",
  "Morgans"), -man compounds ("waterwomans"), and reflexives of any gender after a noun subject
  ("the philosophy devoured himself").
- **Rare homographs at `commonness=0`:** noun "few", adjective "meet" and verb "baby" make
  grammatical sentences read as broken. If revisited, either drop lemmas whose spelling is mostly
  another part of speech (SUBTLEX's dominant-POS data), or keep closed-class spellings out of
  open-class tables.
- **High floors (5+):** verbs spelled like modals ("wills") and first names (Al, Jack) still get
  through. Small frames (weather, dummy that-clause, transitive-into-gerund) run out of verbs, so
  those sentence shapes stop appearing.
- **Unmatched words count as 0:** single-word genus names and drug brands (Cryptoprocta,
  Mevacor) stay in the noun table, but any floor drops them because SUBTLEX-US lacks them.
- **a/an misses:** abbreviations (mph, nth) and a few other words (Oneida, yttrium).
- **"neither X or Y":** the determiner "neither" in the first part of an "or" coordination reads
  like a broken "neither...nor". Under evaluation; a possible fix is removing "neither" from the
  determiners.
- **Genitive "his" as a subject** can be mistaken for a determiner when the verb is also a
  noun ("His look deficient").
