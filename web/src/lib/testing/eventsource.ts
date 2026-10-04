// FakeEventSource stands in for the browser's EventSource in component
// tests: vi.stubGlobal('EventSource', FakeEventSource).
export class FakeEventSource extends EventTarget {
	static latest: FakeEventSource | undefined;
	url: string;
	closed = false;

	constructor(url: string) {
		super();
		this.url = url;
		FakeEventSource.latest = this;
	}

	close() {
		this.closed = true;
	}

	// open is the stream connecting or reconnecting.
	open() {
		this.dispatchEvent(new Event('open'));
	}

	emit(name: string, data: unknown) {
		this.dispatchEvent(new MessageEvent(name, { data: JSON.stringify(data) }));
	}
}
