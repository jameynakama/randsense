import { defineEnvVars } from '@sveltejs/kit/env';

export const variables = defineEnvVars({
	API_ORIGIN: {
		description: 'Where the Go API listens, for server-side requests.',
		schema: (value) => value ?? 'http://localhost:8080'
	}
});
