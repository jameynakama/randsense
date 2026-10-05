import { pos } from './tree';
import type { TreeNode } from './types';

// Pixel heights of a row, a label and a word, and the gap between columns.
export interface Sizes {
	row: number;
	label: number;
	word: number;
	gap: number;
}

// Kept in step with Diagram.svelte's styles.
export const READ: Sizes = { row: 40, label: 20, word: 24, gap: 16 };
// An editable tree's labels and words are 44px buttons.
export const EDIT: Sizes = { row: 64, label: 44, word: 44, gap: 8 };

// Editing makes phrases open their rules and words toggle their locks.
export interface Editing {
	onphrase: (path: number[]) => void;
	onword: (path: number[]) => void;
	// The slot no word fits, to highlight.
	problemPath: number[] | null;
}

// A box's x is its center and y its top.
export interface Box {
	x: number;
	y: number;
	width: number;
}

export interface Placed {
	node: TreeNode;
	path: number[];
	label: Box;
	word?: Box & { text: string };
}

// An edge's path is its child's, or for a dotted edge to a word, its leaf's.
export interface Edge {
	from: [number, number];
	to: [number, number];
	dotted: boolean;
	path: number[];
}

export interface Layout {
	placed: Map<string, Placed>;
	edges: Edge[];
	width: number;
	height: number;
}

export type Measure = (text: string, kind: 'label' | 'word') => number;

export function key(path: number[]): string {
	return path.join('.');
}

const abbreviations: Record<string, string> = {
	Determiner: 'Det',
	Adjective: 'Adj',
	Adverb: 'Adv',
	Preposition: 'Prep',
	Pronoun: 'Pron',
	Conjunction: 'Conj',
	Complementizer: 'Comp',
	Comma: 'Punct'
};

// short is the label drawn for a node: a phrase's symbol, or a leaf's part
// of speech, abbreviated the way trees usually are.
export function short(node: TreeNode): string {
	if (node.children?.length) return node.symbol;
	const p = pos(node);
	return abbreviations[p] ?? p;
}

function deepestLeaf(node: TreeNode, depth = 0): number {
	if (!node.children?.length) return depth;
	return Math.max(...node.children.map((c) => deepestLeaf(c, depth + 1)));
}

// layout places a tree for drawing. Leaves get columns left to right, as
// wide as their label or word; each phrase centers over its first and last
// child; each depth is a row; and every word sits on one baseline under
// the deepest leaf.
export function layout(tree: TreeNode, measure: Measure, sizes: Sizes = READ): Layout {
	const placed = new Map<string, Placed>();
	const edges: Edge[] = [];
	const baseline = (deepestLeaf(tree) + 1) * sizes.row;
	let cursor = 0;

	function place(node: TreeNode, path: number[]): number {
		const y = path.length * sizes.row;
		const text = short(node);
		const labelWidth = measure(text, 'label');

		if (!node.children?.length) {
			const written = node.display ?? node.word ?? '';
			const wordWidth = written ? measure(written, 'word') : 0;
			const column = Math.max(labelWidth, wordWidth);
			const x = cursor + column / 2;
			cursor += column + sizes.gap;
			const p: Placed = { node, path, label: { x, y, width: labelWidth } };
			if (written) {
				p.word = { x, y: baseline, width: wordWidth, text: written };
				edges.push({ from: [x, y + sizes.label], to: [x, baseline], dotted: true, path });
			}
			placed.set(key(path), p);
			return x;
		}

		const xs = node.children.map((c, i) => place(c, [...path, i]));
		const x = (xs[0] + xs[xs.length - 1]) / 2;
		placed.set(key(path), { node, path, label: { x, y, width: labelWidth } });
		xs.forEach((cx, i) =>
			edges.push({
				from: [x, y + sizes.label],
				to: [cx, y + sizes.row],
				dotted: false,
				path: [...path, i]
			})
		);
		return x;
	}

	place(tree, []);
	return { placed, edges, width: cursor - sizes.gap, height: baseline + sizes.word };
}
