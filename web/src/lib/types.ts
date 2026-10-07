// The shapes the Go API returns. See the README's API section.

export interface Features {
	tense?: string;
	commonness?: number;
	form?: 'finite' | 'base' | 'gerund' | 'participle';
	person?: number;
	number?: string;
	case?: string;
	gender?: string;
	type?: string;
	frames?: string[];
	separable?: boolean;
	frequency?: number;
}

export interface TreeNode {
	symbol: string;
	lemma?: string;
	word?: string;
	// How the leaf is written when it differs from word: a separable verb
	// split around its object ("looked" ... "her up").
	display?: string;
	// Keeps the leaf's lemma when the tree is filled again.
	locked?: boolean;
	features?: Features;
	children?: TreeNode[];
}

export interface Sentence {
	// built: kept from the builder; generated: from random.
	origin: 'generated' | 'built';
	id: string;
	text: string;
	tree: TreeNode;
	star_count: number;
	created_at: string;
}

export interface Label {
	label: string;
	description: string;
	example?: string;
}

export interface Phrase extends Label {
	// Each rule is the symbols a phrase can expand to, in grammar.toml order.
	rules: string[][];
}

export interface Grammar {
	start: string;
	phrases: Record<string, Phrase>;
	slots: Record<string, Label>;
}

// A complaint from the admin API's flags list.
export interface Flag {
	id: number;
	// The flagged leaf's position among the tree's leaves, commas counted.
	// word_index, lemma and pos are null when the whole sentence was flagged.
	word_index: number | null;
	lemma: string | null;
	pos: string | null;
	comment: string;
	created_at: string;
	sentence: Pick<Sentence, 'id' | 'text' | 'tree'>;
}

export interface FlaggedWord {
	lemma: string;
	pos: string;
	count: number;
}
