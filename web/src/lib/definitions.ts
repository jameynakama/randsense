// CONTENT_POS are the parts of speech OEWN defines. The rest are closed-class.
export const CONTENT_POS = ['noun', 'verb', 'adjective', 'adverb'];

// definitions fetches a content word's OEWN glosses in sense order, none when
// the lexicon lacks it. It throws when the API can't answer.
export async function definitions(
	fetch: typeof globalThis.fetch,
	pos: string,
	lemma: string
): Promise<string[]> {
	const res = await fetch(`/api/v1/words/${pos}/${encodeURIComponent(lemma)}`);
	if (res.status === 404) return [];
	if (!res.ok) throw new Error(`status ${res.status}`);
	return (await res.json()).definitions;
}

// wiktionaryUrl is lemma's English Wiktionary entry. Titles keep their case,
// as lemmas do ("Hopi").
export function wiktionaryUrl(lemma: string): string {
	return `https://en.wiktionary.org/wiki/${encodeURIComponent(lemma.replaceAll(' ', '_'))}#English`;
}
