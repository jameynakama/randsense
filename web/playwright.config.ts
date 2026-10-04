import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testMatch: '**/*.e2e.{ts,js}',
	webServer: [
		{
			// Uses DATABASE_URL and the admin settings from the root .env, which
			// `just test-fe` loads.
			command: 'go run ./cmd/server',
			cwd: '../service',
			url: 'http://localhost:8080/health',
			reuseExistingServer: true
		},
		{ command: 'npm run build && npm run preview', port: 4173, reuseExistingServer: true }
	],
	use: { baseURL: 'http://localhost:4173' },
	projects: [
		{ name: 'desktop', use: { ...devices['Desktop Chrome'] } },
		{
			name: 'phone',
			use: {
				...devices['Desktop Chrome'],
				viewport: { width: 390, height: 844 },
				isMobile: true,
				hasTouch: true
			}
		}
	]
});
