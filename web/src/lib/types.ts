// The shapes the Go API returns. See the README's API section.

export interface Features {
	tense?: string;
	commonness?: number;
	form?: 'finite' | 'base' | 'gerund';
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
	features?: Features;
	children?: TreeNode[];
}

export interface Sentence {
	id: string;
	text: string;
	tree: TreeNode;
	star_count: number;
	created_at: string;
}
