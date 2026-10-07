import { describe, expect, it } from 'vitest';
import {
	choose,
	clear,
	draftText,
	fill,
	lockable,
	nodeAt,
	remix,
	start,
	toggleLock,
	undo,
	type Realized
} from './builder';
import { grammar, sentence } from './testing/fixtures';

const realized: Realized = { text: sentence.text, tree: sentence.tree, signature: 's' };

describe('builder', () => {
	it('starts from a lone start hole with nothing to undo', () => {
		const b = start(grammar);
		expect(b.draft).toEqual({ tree: { symbol: 'S' }, filled: null });
		expect(b.undo).toEqual([]);
	});

	it("expands a hole into the rule's symbols, and undoes it", () => {
		const b = choose(start(grammar), [], ['Clause']);
		expect(b.draft.tree).toEqual({ symbol: 'S', children: [{ symbol: 'Clause' }] });
		expect(undo(b).draft.tree).toEqual({ symbol: 'S' });
	});

	it('changes a filled phrase, dropping its words, its locks and the fill', () => {
		const locked = toggleLock(fill(start(grammar), realized), [0, 0, 1]);
		const b = choose(locked, [0, 0], ['Pronoun']);
		expect(nodeAt(b.draft.tree, [0, 0])).toEqual({
			symbol: 'NP',
			children: [{ symbol: 'Pronoun' }]
		});
		expect(nodeAt(b.draft.tree, [0, 1, 0]).word).toBe('devoured');
		expect(b.draft.filled).toBeNull();
	});

	it('clears a phrase back to a hole', () => {
		const b = clear(fill(start(grammar), realized), [0, 1]);
		expect(nodeAt(b.draft.tree, [0, 1])).toEqual({ symbol: 'VP' });
		expect(b.draft.filled).toBeNull();
	});

	it('toggles a lock without losing the fill', () => {
		const filled = fill(start(grammar), realized);
		const once = toggleLock(filled, [0, 0, 1]);
		expect(nodeAt(once.draft.tree, [0, 0, 1]).locked).toBe(true);
		expect(once.draft.filled).toBe(realized);
		expect(nodeAt(toggleLock(once, [0, 0, 1]).draft.tree, [0, 0, 1]).locked).toBeUndefined();
	});

	it('undoes a fill back to the unfilled draft', () => {
		const b = choose(start(grammar), [], ['Clause']);
		expect(undo(fill(b, realized)).draft).toBe(b.draft);
	});

	it('leaves the state alone when there is nothing to undo', () => {
		const b = start(grammar);
		expect(undo(b)).toBe(b);
	});

	it.each([
		['Noun', true],
		['Verb:transitive', true],
		['Preposition', true],
		['Pronoun', true],
		['Conjunction:coordinating', true],
		['Comma', false],
		['To', false],
		['Be', false],
		['Complementizer:whether', false],
		['Preposition:with', false],
		['Pronoun:it', false],
		['Pronoun:reflexive', false],
		['Conjunction:nor', false]
	])('says whether %s can be locked: %s', (symbol, want) => {
		expect(lockable({ symbol })).toBe(want);
	});

	it('writes out a draft with every word, and nothing while a slot is empty', () => {
		expect(draftText(sentence.tree)).toBe('The goose devoured her, but she sang.');
		expect(draftText(choose(start(grammar), [], ['Clause']).draft.tree)).toBe('');
	});

	it('remixes a saved tree with its words, no locks and nothing to keep', () => {
		const saved = structuredClone(sentence.tree);
		saved.children![0].children![0].children![1].locked = true;

		const b = remix(saved);
		expect(nodeAt(b.draft.tree, [0, 0, 1])).toEqual(nodeAt(sentence.tree, [0, 0, 1]));
		expect(nodeAt(b.draft.tree, [0, 0, 1]).locked).toBeUndefined();
		expect(b.draft.filled).toBeNull();
		expect(b.undo).toEqual([]);
		expect(nodeAt(saved, [0, 0, 1]).locked).toBe(true);
	});
});
