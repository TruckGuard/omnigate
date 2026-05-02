<script lang="ts">
  import { goto } from '$app/navigation';
  import { authStore } from '$lib/stores/auth.svelte.js';
  import { api } from '$lib/api.js';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Label } from '$lib/components/ui/label/index.js';
  import { Card, CardContent } from '$lib/components/ui/card/index.js';

  let username = $state('');
  let password = $state('');
  let loading  = $state(false);
  let error    = $state('');

  if (authStore.isAuthenticated) goto('/');

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    loading = true;
    try {
      const res = await api.auth.login(username, password);
      // Fetch full user info + permissions after login
      authStore.login(res.session_id, username, '');
      const me = await api.auth.validate();
      authStore.login(res.session_id, me.username, me.role, me.permissions);
      goto('/');
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      error = msg.startsWith('401') ? 'Invalid username or password.' : 'Sign in failed. Please try again.';
    } finally {
      loading = false;
    }
  }
</script>

<div class="min-h-screen flex items-center justify-center bg-background">
  <div class="w-full max-w-[360px] px-4">
    <!-- Logo -->
    <div class="flex flex-col items-center mb-8">
      <div class="w-10 h-10 rounded-xl bg-primary flex items-center justify-center mb-3">
        <svg viewBox="0 0 40 40" width="22" height="22">
          <rect x="4" y="6" width="32" height="28" rx="4" fill="none" stroke="white" stroke-width="2.5"/>
          <line x1="4" y1="14" x2="36" y2="14" stroke="white" stroke-width="2.5"/>
          <line x1="14" y1="14" x2="14" y2="34" stroke="white" stroke-width="2.5"/>
          <line x1="26" y1="14" x2="26" y2="34" stroke="white" stroke-width="2.5"/>
          <circle cx="20" cy="10" r="1.8" fill="white"/>
        </svg>
      </div>
      <h1 class="text-[20px] font-bold tracking-[-0.01em]">OmniGate</h1>
      <p class="text-[13px] text-muted-foreground mt-0.5">Sign in to your account</p>
    </div>

    <Card>
      <CardContent class="pt-6">
        <form onsubmit={handleSubmit} class="space-y-4">
          <div class="space-y-1.5">
            <Label for="username">Username</Label>
            <Input
              id="username"
              type="text"
              bind:value={username}
              autocomplete="username"
              required
              placeholder="Enter your username"
            />
          </div>

          <div class="space-y-1.5">
            <Label for="password">Password</Label>
            <Input
              id="password"
              type="password"
              bind:value={password}
              autocomplete="current-password"
              required
              placeholder="Enter your password"
            />
          </div>

          {#if error}
            <div class="rounded-md bg-destructive/10 border border-destructive/20 px-3 py-2 text-[12px] text-destructive">
              {error}
            </div>
          {/if}

          <Button type="submit" class="w-full" disabled={loading}>
            {loading ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>
      </CardContent>
    </Card>

    <p class="text-center text-[11px] text-muted-foreground mt-6">
      OmniGate IoT Gateway · {new Date().getFullYear()}
    </p>
  </div>
</div>
