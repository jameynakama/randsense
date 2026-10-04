import type { Features, TreeNode } from './types';

export interface Leaf {
	node: TreeNode;
	// Child indexes from the root down to the leaf.
	path: number[];
}

// leaves lists the tree's leaves in sentence order. Their positions are the
// API's word_index.
export function leaves(tree: TreeNode, path: number[] = []): Leaf[] {
	if (!tree.children?.length) return [{ node: tree, path }];
	return tree.children.flatMap((c, i) => leaves(c, [...path, i]));
}

export function pos(node: TreeNode): string {
	return node.symbol.split(':')[0];
}

// frame is a verb slot's frame ("transitive" in "Verb:transitive").
export function frame(node: TreeNode): string | undefined {
	const [p, qualifier] = node.symbol.split(':');
	return p === 'Verb' ? qualifier : undefined;
}

export interface Token {
	index: number;
	text: string;
	punctuation: boolean;
}

// tokens is the sentence as written: one token per leaf, the first word
// capitalized. The final period isn't a leaf, so callers add it.
export function tokens(tree: TreeNode): Token[] {
	return leaves(tree).map(({ node }, index) => {
		const text = node.display ?? node.word ?? '';
		return {
			index,
			text: index === 0 ? text.charAt(0).toUpperCase() + text.slice(1) : text,
			punctuation: pos(node) === 'Comma'
		};
	});
}

const phraseNames: Record<string, string> = {
	S: 'sentence',
	Clause: 'clause',
	NP: 'noun phrase',
	VP: 'verb phrase',
	PP: 'prepositional phrase',
	InfVP: 'infinitive',
	GerVP: 'gerund phrase',
	ADJ: 'adjective phrase'
};

// symbolName reads a symbol out: "noun phrase" for NP, "verb" for
// Verb:transitive.
export function symbolName(node: TreeNode): string {
	return phraseNames[node.symbol] ?? pos(node).toLowerCase();
}

// role is where the leaf at path sits in its clause, from the nearest
// phrase that says: a prepositional phrase, an infinitive, a gerund phrase,
// or the subject or object. null when none applies.
export function role(tree: TreeNode, path: number[]): string | null {
	const chain = [tree];
	for (const i of path) chain.push(chain[chain.length - 1].children![i]);

	for (let depth = chain.length - 1; depth > 0; depth--) {
		const node = chain[depth];
		const parent = chain[depth - 1];
		switch (node.symbol) {
			case 'PP':
				return 'in a prepositional phrase';
			case 'InfVP':
				return 'in an infinitive';
			case 'GerVP':
				return 'in a gerund phrase';
		}
		const nominal = node.symbol === 'NP' || ['Noun', 'Pronoun'].includes(pos(node));
		if (!nominal || parent.symbol === 'NP') continue;
		const at = path[depth - 1];
		const siblings = parent.children!;
		if (siblings.slice(at + 1).some((s) => s.symbol === 'VP')) return 'in the subject';
		if (parent.symbol === 'VP' && siblings.slice(0, at).some((s) => pos(s) === 'Verb')) {
			return 'in the object';
		}
	}
	return null;
}

const ordinals: Record<number, string> = { 1: '1st', 2: '2nd', 3: '3rd' };
const forms: Record<string, string> = { finite: 'finite', base: 'base form', gerund: '-ing form' };
const genders: Record<string, string> = {
	fem: 'feminine',
	masc: 'masculine',
	neuter: 'neuter',
	epicene: 'any gender'
};

// featureLabels writes a node's features out for people: "past tense",
// "3rd person plural", "reflexive".
export function featureLabels(f: Features | undefined): string[] {
	if (!f) return [];
	const number = f.number === 'either' ? 'either number' : f.number;
	const labels: string[] = [];
	if (f.tense) labels.push(`${f.tense} tense`);
	if (f.form) labels.push(forms[f.form]);
	if (f.person) labels.push(`${ordinals[f.person]} person${number ? ` ${number}` : ''}`);
	else if (number) labels.push(number);
	if (f.case) labels.push(f.case);
	if (f.gender) labels.push(genders[f.gender] ?? f.gender);
	if (f.type) labels.push(f.type);
	if (f.frames?.length) labels.push(`frames: ${f.frames.join(', ')}`);
	if (f.separable) labels.push('separable');
	if (f.frequency !== undefined) labels.push(`frequency ${f.frequency} (Zipf)`);
	if (f.commonness !== undefined) labels.push(`commonness floor ${f.commonness}`);
	return labels;
}
