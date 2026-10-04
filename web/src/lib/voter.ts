// The voter token is a random UUID this browser stars with. It isn't an
// account. The browser also remembers which sentences it starred, since
// the API only lists them a page at a time.

const TOKEN_KEY = 'randsense:voter';
const STARRED_KEY = 'randsense:starred';

type Store = Pick<Storage, 'getItem' | 'setItem'>;

// Values storage refused to keep. They last as long as the page.
const memory = new Map<string, string>();

// Blocked site data makes localStorage itself throw. That works like
// storage that refuses every write.
const blocked: Store = {
	getItem: () => null,
	setItem: () => {
		throw new Error('storage is blocked');
	}
};

function browserStore(): Store {
	try {
		return window.localStorage;
	} catch {
		return blocked;
	}
}

export function voterToken(store: Store = browserStore()): string {
	let token = read(store, TOKEN_KEY);
	if (!token) {
		token = crypto.randomUUID();
		write(store, TOKEN_KEY, token);
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
	write(store, STARRED_KEY, JSON.stringify([...ids]));
}

function starredIds(store: Store): Set<string> {
	try {
		return new Set(JSON.parse(read(store, STARRED_KEY) ?? '[]'));
	} catch {
		return new Set();
	}
}

// Storage that is full or refuses writes keeps values in memory instead,
// for the life of the page. Memory is read first because it holds whatever
// storage refused.
function read(store: Store, key: string): string | null {
	return memory.get(key) ?? store.getItem(key);
}

function write(store: Store, key: string, value: string): void {
	try {
		store.setItem(key, value);
		memory.delete(key);
	} catch {
		memory.set(key, value);
	}
}
