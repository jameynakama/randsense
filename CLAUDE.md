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
  `data/lexicon/verb_morphology.toml` comes from its `<variants>` lines (irreg, regd).
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
  let, make and have ("allowed him go"), so only the to-infinitive codes are used.
- **One test database per package.** `go test ./...` runs packages in parallel, and each
  database-backed package drops and recreates its database. Each derives its own name from
  `TEST_DATABASE_URL` with a package suffix (`_api`, `_oewn`); a new one needs its own.

## Roadmap

1. **Reflexives after prepositions** ("talked to himself"), once reflexive objects read well.
2. **Pronoun objects of separable phrasal verbs** go after the particle ("fought off itself",
   "tip off us") when they must go before it. The fix needs to know which multiword verbs are
   separable.
3. **Roman-numeral adjectives** (lxxxi, ixl). A naive regex would also hit "mix".
4. **Curation loop, after a frontend exists.**
   - **Voting:** an anonymous endpoint; anyone can vote, and votes only flag words for review.
   - **Admin:** a rudimentary UI for one admin to sort by votes and disable or remove entries.
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
