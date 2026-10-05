import { describe, expect, it } from 'vitest';
import { featureLabels, frame, leaves, role, tokens } from './tree';
import type { TreeNode } from './types';

const leaf = (symbol: string, word: string, extra: Partial<TreeNode> = {}): TreeNode => ({
	symbol,
	lemma: word,
	word,
	...extra
});
const node = (symbol: string, ...children: TreeNode[]): TreeNode => ({ symbol, children });

// "the goose under the moon devoured her, but she urged us to sing"
const tree = node(
	'S',
	node(
		'Clause',
		node(
			'NP',
			leaf('Determiner', 'the'),
			leaf('Noun', 'goose'),
			node(
				'PP',
				leaf('Preposition', 'under'),
				node('NP', leaf('Determiner', 'the'), leaf('Noun', 'moon'))
			)
		),
		node('VP', leaf('Verb:transitive', 'devoured'), node('NP', leaf('Pronoun', 'her')))
	),
	leaf('Comma', ','),
	leaf('Conjunction:coordinating', 'but'),
	node(
		'Clause',
		node('NP', leaf('Pronoun', 'she')),
		node(
			'VP',
			leaf('Verb:transitive-to-infinitive', 'urged'),
			node('NP', leaf('Pronoun', 'us')),
			node('InfVP', leaf('To', 'to'), node('VP', leaf('Verb:intransitive', 'sing')))
		)
	)
);

describe('leaves', () => {
	it('lists leaves in sentence order with their paths', () => {
		const got = leaves(tree);
		expect(got.map((l) => l.node.word).join(' ')).toBe(
			'the goose under the moon devoured her , but she urged us to sing'
		);
		expect(got[4].path).toEqual([0, 0, 2, 1, 1]);
	});
});

describe('tokens', () => {
	it('capitalizes the first word and marks commas', () => {
		const got = tokens(tree);
		expect(got[0].text).toBe('The');
		expect(got[7]).toEqual({ index: 7, text: ',', punctuation: true });
		expect(got.filter((t) => t.punctuation)).toHaveLength(1);
	});

	it('writes a split separable verb as the sentence does', () => {
		const split = node(
			'S',
			node('NP', leaf('Pronoun', 'she')),
			node(
				'VP',
				leaf('Verb:transitive', 'looked up', { lemma: 'look up', display: 'looked' }),
				node('NP', leaf('Pronoun', 'her', { display: 'her up' }))
			)
		);

		expect(tokens(split).map((t) => t.text)).toEqual(['She', 'looked', 'her up']);
	});
});

describe('role', () => {
	const roleOf = (word: string) => {
		const l = leaves(tree).find((l) => l.node.word === word)!;
		return role(tree, l.path);
	};

	it.each([
		['the', 'in the subject'],
		['goose', 'in the subject'],
		['moon', 'in a prepositional phrase'],
		['under', 'in a prepositional phrase'],
		['her', 'in the object'],
		['she', 'in the subject'],
		['us', 'in the object'],
		['sing', 'in an infinitive']
	])('%s is %s', (word, want) => {
		expect(roleOf(word)).toBe(want);
	});

	it.each(['devoured', ',', 'but', 'urged'])('%s has no role', (word) => {
		expect(roleOf(word)).toBeNull();
	});
});

describe('frame', () => {
	it('reads frames only from verbs', () => {
		expect(frame(leaf('Verb:transitive', 'x'))).toBe('transitive');
		expect(frame(leaf('Pronoun:reflexive', 'x'))).toBeUndefined();
	});
});

describe('featureLabels', () => {
	it('labels a finite verb', () => {
		expect(
			featureLabels({
				tense: 'past',
				form: 'finite',
				person: 3,
				number: 'plural',
				frames: ['transitive', 'intransitive'],
				separable: true,
				frequency: 3.25
			})
		).toEqual([
			'past tense',
			'finite',
			'3rd person plural',
			'frames: transitive, intransitive',
			'separable',
			'frequency 3.25 (Zipf)'
		]);
	});

	it('labels pronouns, determiners and the root', () => {
		expect(
			featureLabels({ case: 'reflexive', person: 1, number: 'singular', gender: 'epicene' })
		).toEqual(['1st person singular', 'reflexive', 'any gender']);
		expect(featureLabels({ type: 'definite', number: 'either' })).toEqual([
			'either number',
			'definite'
		]);
		expect(featureLabels({ tense: 'present', commonness: 0 })).toEqual([
			'present tense',
			'commonness floor 0'
		]);
	});

	it('has nothing to say without features', () => {
		expect(featureLabels(undefined)).toEqual([]);
	});
});
