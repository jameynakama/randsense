<script lang="ts">
	import { page } from '$app/state';
</script>

<svelte:head>
	<title>{page.status === 404 ? 'Not found' : 'Something went wrong'} · RandSense</title>
</svelte:head>

<h1>{page.status === 404 ? 'No sentence here' : 'Something went wrong'}</h1>
<p>{page.error?.message}</p>
<p><a href="/">Back to RandSense</a></p>
