# Passive voice design

Every verb frame with an object gets a passive, with an optional "by" agent: "the goose
was devoured", "the geese were handed to themselves by the philosophy", "she wanted to be
considered ugly". The direction in `CLAUDE.md` still holds: frames constrain syntax only.

## Goals

- Passives for every object frame: transitive, transitive-pp, ditransitive, the
  fixed-preposition frames (from, of, on, to, with), transitive-to-infinitive,
  transitive-adjective and transitive-into-gerund.
- An optional agent: "by" and a noun phrase or a reflexive.
- Passives inside infinitives and gerunds ("to be devoured", "being devoured").
- "Be" shows as its own node in the diagram and the builder.

## Non-goals

- Progressive and perfect ("was devouring", "has devoured"). They reuse `Be` and the past
  participle, but each needs its own aspect rules.
- Adverb placement around the auxiliary (see Known quirks).

## Decisions

- **An explicit `Be` leaf and a `PassVP` phrase.** Verbs under `PassVP` take the past participle,
  keyed off the symbol the way `InfVP` and `GerVP` are. Two alternatives were rejected:
  - A passive flag on the verb slot hides "was" from the tree, so the builder can't show it,
    and a slot holds only one qualifier.
  - `VP → Be Verb:transitive`, with the participle keyed off a `Be` sibling, adds a one-off
    rule to agreement instead of following the symbol pattern.
- **`Agent` is its own phrase, not a `PP` rule.** A `PP → Preposition:by NP` rule would put a
  fixed "by" into every prepositional phrase slot.

## Grammar

New rules in `service/data/grammar/grammar.toml`:

```toml
VP     → Be PassVP                     # weight 0.6
VP     → Be PassVP Agent               # weight 0.5
PassVP → Verb:transitive               # "was devoured"
PassVP → Verb:transitive-pp PP         # "was put under the bridge"
PassVP → Verb:ditransitive NP          # "was given the goose"
PassVP → Verb:transitive-to Preposition:to NP
PassVP → Verb:transitive-to Preposition:to Pronoun:reflexive
         (likewise from, of, on and with)
PassVP → Verb:transitive-to-infinitive InfVP   # "was urged to sing"
PassVP → Verb:transitive-adjective Adjective   # "was considered ugly"
PassVP → Verb:transitive-into-gerund Preposition:into GerVP  # "was coaxed into singing"
Agent  → Preposition:by NP                     # weight 1
Agent  → Preposition:by Pronoun:reflexive      # weight 0.1
```

Together, the two `VP` rules make about 7% of verb phrases passive. `PassVP` rules are weighted like their
active twins, with each reflexive variant at a fifth of its noun phrase variant.

New labels: `[phrase.PassVP]` ("passive verb phrase"), `[phrase.Agent]` ("agent": the "by"
phrase naming who did it), `[slot.Be]` ("\"be\"") and `[slot."Preposition:by"]`. The header
comment lists `PassVP` and `Agent` among the symbols agreement depends on.

In `service/internal/grammar/grammar.go`, `Be` joins `allPOS` as a fixed terminal like `To`, and
"by" joins the Preposition qualifiers.

## Generator

In `service/internal/sentence/sentence.go`:

- `chooseWord` returns "be" for `Be`.
- `agreeWithSubjects` inflects a `Be` leaf like a verb: finite ("am", "is", "are", "was",
  "were"), base under `InfVP`, -ing under `GerVP`.
- A new `participle` verb form applies under `PassVP` and sets `Form: "participle"`.
- `PassVP` and `Agent` pass the subject's agreement down, as `VP` and `PP` do, so reflexives in
  either agree with the subject ("the geese were devoured by themselves").
- Pronouns in the agent and in a ditransitive's remaining object come out accusative already,
  since no VP follows them.
- Separable verbs need nothing: with no object after the verb, they stay whole ("was looked up",
  "was set on fire").

## Morphology

In `service/internal/morph/morph.go`, `irregular` gains `PastParticiple`
(`toml:"past_participle"`). Every irregular in `verb_morphology.toml` and the curated file
has one.

`Verbs.PastParticiple(lemma)` inflects the same head word as `Conjugate`: an irregular whole
lemma is inflected as a whole ("wined and dined"), an irregular head word uses its past participle
("taken"), a hyphenated verb inflects its last part ("spoon-fed"), doubled verbs double
("stopped"), and the rest follow the regular past rules. Those rules (-d, -ied, -ed) move into a
helper shared with the past tense.

## Frontend

In `web/src/lib/tree.ts` and `types.ts`:

- `forms` gains `participle: 'past participle'`; the `form` type gains `'participle'`.
- `role()` returns "in the agent" for a word under `Agent`. A word in the noun phrase after a
  verb under `PassVP` reads as "in the object", as it does under `VP`.

The builder and diagram take the new rules and labels from `/api/v1/grammar`.

## Testing

- `morph_test.go`: past participles of irregular, regular, doubled, hyphenated, multi-word
  separable, compound and whole-lemma irregular verbs.
- `sentence_test.go`, on fixed trees:
  - "be" agrees with singular, plural, coordinated, first person and second person subjects
  - "to be devoured" and "being devoured"
  - accusative agent pronouns, and agent reflexives that agree with the subject
  - a ditransitive passive keeps its object
  - passive verbs carry the `participle` form
- `grammar_test.go`: the project grammar loads with every slot labeled, and `Preposition:by`
  is accepted.
- `tree.spec.ts`: the agent role, the object role under `PassVP`, and the past participle label.

## Known quirks

- `VP → Adverb VP` puts adverbs before the auxiliary: "the goose quickly was devoured".
  Grammatical but stilted. A fix would place adverbs between `Be` and `PassVP`.
- Idioms OEWN marks transitive passivize badly ("the goose was given birth"). They belong to the
  roadmap item for idioms with broken object frames.
