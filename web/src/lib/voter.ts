// The voter token is a random UUID this browser stars with. It isn't an
// account. The browser also remembers which sentences it starred, since
// the API only lists them a page at a time.

const TOKEN_KEY = 'randsense:voter';
const STARRED_KEY = 'randsense:starred';

// Blocked site data makes localStorage throw, so stars then last only as
// long as the page.
const memory = new Map<string, string>();
const fallback: Pick<Storage, 'getItem' | 'setItem'> = {
	getItem: (k) => memory.get(k) ?? null,
	setItem: (k, v) => void memory.set(k, v)
};

type Store = Pick<Storage, 'getItem' | 'setItem'>;

function browserStore(): Store {
	try {
		return window.localStorage;
	} catch {
		return fallback;
	}
}

export function voterToken(store: Store = browserStore()): string {
	let token = store.getItem(TOKEN_KEY);
	if (!token) {
		token = crypto.randomUUID();
		store.setItem(TOKEN_KEY, token);
	}
	return token;
}

export function isStarred(id: string, store: Store = browserStore()): boolean {
	return starredIds(store).has(id);
}

export function setStarred(id: string, starred: boolean, store: Store = browserStore()): void {
	const ids = starredIds(store);
	if (starred) ids.add(id);
	else ids.delete(id);
	store.setItem(STARRED_KEY, JSON.stringify([...ids]));
}

function starredIds(store: Store): Set<string> {
	try {
		return new Set(JSON.parse(store.getItem(STARRED_KEY) ?? '[]'));
	} catch {
		return new Set();
	}
}
