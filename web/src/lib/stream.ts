import type { Sentence } from './types';

export interface StarsEvent {
	id: string;
	count: number;
}

export interface StreamHandlers {
	// Runs on every connection, reconnects included. The stream doesn't
	// replay what was missed, so this is where a page catches up.
	onOpen?: () => void;
	onSentence?: (s: Sentence) => void;
	onStars?: (e: StarsEvent) => void;
}

// subscribe listens to the live stream until the returned function is
// called. The browser reconnects by itself when the connection drops.
export function subscribe(handlers: StreamHandlers): () => void {
	const source = new EventSource('/api/v1/sentences/stream');
	source.addEventListener('open', () => handlers.onOpen?.());
	source.addEventListener('sentence', (e) => handlers.onSentence?.(JSON.parse(e.data)));
	source.addEventListener('stars', (e) => handlers.onStars?.(JSON.parse(e.data)));
	return () => source.close();
}
