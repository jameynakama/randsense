import { leaves, pos, tokens } from './tree';
import type { Grammar, TreeNode } from './types';

// Realized is what realize returns for a draft.
export interface Realized {
	text: string;
	tree: TreeNode;
	// Keep posts this with the tree, untouched.
	signature: string;
}

// Draft is the tree being built, and the last fill if only locks have
// changed since.
export interface Draft {
	tree: TreeNode;
	filled: Realized | null;
}

// Builder is the draft and every earlier draft, latest last. Functions here
// return new state and never change what they're given.
export interface Builder {
	draft: Draft;
	undo: Draft[];
}

export function start(grammar: Grammar): Builder {
	return { draft: { tree: { symbol: grammar.start }, filled: null }, undo: [] };
}

export function nodeAt(tree: TreeNode, path: number[]): TreeNode {
	return path.reduce((node, i) => node.children![i], tree);
}

// edit changes a copy of the draft's tree, keeping the draft for undo. Only
// a change that leaves the words alone keeps the fill.
function edit(b: Builder, change: (tree: TreeNode) => void, keepsFill = false): Builder {
	const tree = structuredClone(b.draft.tree);
	change(tree);
	return {
		draft: { tree, filled: keepsFill ? b.draft.filled : null },
		undo: [...b.undo, b.draft]
	};
}

// choose gives the phrase at path the rule's symbols as children: phrases
// become holes and parts of speech empty slots. Whatever was under it goes,
// locks and all.
export function choose(b: Builder, path: number[], rule: string[]): Builder {
	return edit(b, (tree) => {
		nodeAt(tree, path).children = rule.map((symbol) => ({ symbol }));
	});
}

// clear empties the phrase at path back to a hole.
export function clear(b: Builder, path: number[]): Builder {
	return edit(b, (tree) => {
		delete nodeAt(tree, path).children;
	});
}

export function toggleLock(b: Builder, path: number[]): Builder {
	return edit(
		b,
		(tree) => {
			const node = nodeAt(tree, path);
			if (node.locked) delete node.locked;
			else node.locked = true;
		},
		true
	);
}

// fill shows a realize response, keeping the locks it echoes.
export function fill(b: Builder, realized: Realized): Builder {
	return {
		draft: { tree: realized.tree, filled: realized },
		undo: [...b.undo, b.draft]
	};
}

function unlock(node: TreeNode) {
	delete node.locked;
	node.children?.forEach(unlock);
}

// remix starts from a saved tree and its words, with nothing locked. Only
// realize's own output is signed, so Keep waits for a reroll.
export function remix(tree: TreeNode): Builder {
	const copy = structuredClone(tree);
	unlock(copy);
	return { draft: { tree: copy, filled: null }, undo: [] };
}

export function undo(b: Builder): Builder {
	if (!b.undo.length) return b;
	return { draft: b.undo[b.undo.length - 1], undo: b.undo.slice(0, -1) };
}

// Slots whose word comes from the grammar rather than the lexicon: fixed
// words, and reflexives, which follow their subject.
const fixed = new Set([
	'Comma',
	'To',
	'Complementizer',
	'Pronoun:it',
	'Pronoun:reflexive',
	'Conjunction:neither',
	'Conjunction:nor'
]);

// lockable says whether a leaf's word could be locked. realize ignores
// locks on the others.
export function lockable(node: TreeNode): boolean {
	if (fixed.has(node.symbol) || fixed.has(pos(node))) return false;
	return !(pos(node) === 'Preposition' && node.symbol.includes(':'));
}

// draftText writes out the tree, or '' while any slot has no word.
export function draftText(tree: TreeNode): string {
	if (leaves(tree).some(({ node }) => !node.word)) return '';
	const words = tokens(tree).map((t) => (t.index > 0 && !t.punctuation ? ' ' : '') + t.text);
	return words.join('') + '.';
}
