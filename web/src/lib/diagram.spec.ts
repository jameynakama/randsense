import { describe, expect, it } from 'vitest';
import { sentence } from './testing/fixtures';
import { GAP, key, layout, ROW, short, type Box } from './diagram';
import { leaves } from './tree';
import type { TreeNode } from './types';

const measure = (text: string) => text.length * 10;
const drawn = layout(sentence.tree, measure);
const at = (path: number[]) => drawn.placed.get(key(path))!;

describe('layout', () => {
	it('places leaves left to right in sentence order, a gap apart', () => {
		const xs = leaves(sentence.tree).map((l) => at(l.path));
		const half = (p: (typeof xs)[number]) => Math.max(p.label.width, p.word?.width ?? 0) / 2;
		for (let i = 1; i < xs.length; i++) {
			expect(xs[i].label.x - half(xs[i]) - (xs[i - 1].label.x + half(xs[i - 1]))).toBe(GAP);
		}
		expect(xs.map((p) => p.word?.text)).toEqual([
			'the',
			'goose',
			'devoured',
			'her',
			',',
			'but',
			'she',
			'sang'
		]);
	});

	it('centers each phrase over its first and last child', () => {
		const clause = at([0]);
		expect(clause.label.x).toBe((at([0, 0]).label.x + at([0, 1]).label.x) / 2);
		const np = at([0, 0]);
		expect(np.label.x).toBe((at([0, 0, 0]).label.x + at([0, 0, 1]).label.x) / 2);
	});

	it('puts each node on its depth’s row and every word on one baseline below the deepest', () => {
		expect(at([]).label.y).toBe(0);
		expect(at([0, 1, 1, 0]).label.y).toBe(4 * ROW);
		const baselines = new Set(leaves(sentence.tree).map((l) => at(l.path).word!.y));
		expect([...baselines]).toEqual([5 * ROW]);
	});

	it('keeps boxes on the same row apart', () => {
		const rows = new Map<number, Box[]>();
		for (const p of drawn.placed.values()) {
			for (const b of [p.label, p.word].filter((b) => b !== undefined)) {
				rows.set(b.y, [...(rows.get(b.y) ?? []), b]);
			}
		}
		for (const boxes of rows.values()) {
			boxes.sort((a, b) => a.x - b.x);
			for (let i = 1; i < boxes.length; i++) {
				expect(boxes[i].x - boxes[i].width / 2).toBeGreaterThan(
					boxes[i - 1].x + boxes[i - 1].width / 2
				);
			}
		}
	});

	it('draws one solid edge per child and one dotted edge per word', () => {
		const solid = drawn.edges.filter((e) => !e.dotted);
		const dotted = drawn.edges.filter((e) => e.dotted);
		expect(solid).toHaveLength(drawn.placed.size - 1);
		expect(dotted).toHaveLength(leaves(sentence.tree).length);
	});

	it('sizes the drawing to its contents', () => {
		const last = at(leaves(sentence.tree).at(-1)!.path);
		expect(drawn.width).toBe(last.label.x + Math.max(last.label.width, last.word!.width) / 2);
		expect(drawn.height).toBe(5 * ROW + 24);
	});

	it('writes a separable verb as it reads in the sentence', () => {
		const tree: TreeNode = {
			symbol: 'S',
			children: [
				{ symbol: 'Verb:transitive', lemma: 'look up', word: 'looked up', display: 'looked' }
			]
		};
		expect(layout(tree, measure).placed.get(key([0]))!.word!.text).toBe('looked');
	});

	it('gives a comma its own column', () => {
		const comma = at([1]);
		expect(comma.word!.text).toBe(',');
		expect(comma.label.width).toBeGreaterThan(0);
		expect(comma.label.x).toBeGreaterThan(at([0]).label.x);
	});

	it('lays out a tree that is a single leaf under the root', () => {
		const one = layout({ symbol: 'S', children: [{ symbol: 'Verb', word: 'rains' }] }, measure);
		expect(one.width).toBe(Math.max(measure('Verb'), measure('rains')));
		expect(one.placed.get(key([]))!.label.x).toBe(one.placed.get(key([0]))!.label.x);
	});
});

describe('short', () => {
	it.each([
		['Determiner', 'Det'],
		['Verb:transitive', 'Verb'],
		['Comma', 'Punct'],
		['Conjunction:np', 'Conj'],
		['NP', 'NP'],
		['Mystery', 'Mystery']
	])('labels %s as %s', (symbol, want) => {
		expect(short({ symbol, children: symbol === 'NP' ? [{ symbol: 'Noun' }] : undefined })).toBe(
			want
		);
	});
});
