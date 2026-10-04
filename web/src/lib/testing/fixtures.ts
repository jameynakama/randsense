import type { Sentence, TreeNode } from '#lib/types.js';

const leaf = (symbol: string, word: string, extra: Partial<TreeNode> = {}): TreeNode => ({
	symbol,
	lemma: word,
	word,
	...extra
});
const node = (symbol: string, ...children: TreeNode[]): TreeNode => ({ symbol, children });

// sentence is "The goose devoured her, but she sang." as the API returns it.
export const sentence: Sentence = {
	id: 'aaaaaaaa',
	text: 'The goose devoured her, but she sang.',
	star_count: 2,
	created_at: '2026-10-03T12:00:00Z',
	tree: {
		...node(
			'S',
			node(
				'Clause',
				node(
					'NP',
					leaf('Determiner', 'the', { features: { type: 'definite', number: 'either' } }),
					leaf('Noun', 'goose', { features: { number: 'singular', frequency: 3.5 } })
				),
				node(
					'VP',
					leaf('Verb:transitive', 'devoured', {
						lemma: 'devour',
						features: {
							tense: 'past',
							form: 'finite',
							person: 3,
							number: 'singular',
							frames: ['transitive']
						}
					}),
					node(
						'NP',
						leaf('Pronoun', 'her', {
							features: { case: 'accusative', person: 3, number: 'singular', gender: 'fem' }
						})
					)
				)
			),
			leaf('Comma', ','),
			leaf('Conjunction:coordinating', 'but'),
			node(
				'Clause',
				node('NP', leaf('Pronoun', 'she')),
				node('VP', leaf('Verb:intransitive', 'sang', { lemma: 'sing' }))
			)
		),
		features: { tense: 'past', commonness: 0 }
	}
};

// another is the fixture sentence under a different id and text.
export function another(id: string, text: string): Sentence {
	return { ...structuredClone(sentence), id, text };
}
