<script lang="ts">
	import { goto } from '$app/navigation';
	import { login } from '#lib/admin.js';

	let password = $state('');
	let problem = $state('');
	let sending = $state(false);
	let field: HTMLInputElement | undefined = $state();

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		problem = '';
		if (!password) {
			problem = 'Enter the password.';
			field?.focus();
			return;
		}
		sending = true;
		try {
			if (await login(fetch, password)) {
				await goto('/admin');
				return;
			}
			problem = 'That password isn’t right.';
			field?.select();
		} catch {
			// Also covers the rate limit on login.
			problem = 'Couldn’t log in. Try again in a minute.';
		} finally {
			sending = false;
		}
	}
</script>

<svelte:head>
	<title>Admin login · RandSense</title>
</svelte:head>

<form onsubmit={submit} novalidate>
	<label for="password">Password</label>
	<input
		id="password"
		type="password"
		autocomplete="current-password"
		bind:value={password}
		bind:this={field}
		aria-invalid={problem ? 'true' : undefined}
		aria-describedby="password-problem"
	/>
	<p id="password-problem" class="problem" aria-live="polite">{problem}</p>
	<button type="submit" class="button primary" disabled={sending}>Log in</button>
</form>

<style>
	form {
		display: grid;
		gap: 0.5rem;
		max-inline-size: 24rem;
		margin: 2rem auto;
		text-align: start;
	}

	label {
		font-size: var(--text-sm);
		font-weight: 600;
		color: var(--secondary);
	}

	input {
		font: inherit;
		min-block-size: 44px;
		min-inline-size: 0;
		background: var(--bg);
		border: 1px solid var(--separator);
		border-radius: 10px;
		padding: 0.5rem 0.75rem;
	}

	.problem {
		margin: 0;
		font-size: var(--text-sm);
	}

	button {
		justify-self: start;
	}
</style>
