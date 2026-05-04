<script lang="ts">
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import Field from '$lib/components/Field.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card/index.js';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table/index.js';
  import {
    Dialog, DialogContent, DialogHeader, DialogTitle,
    DialogFooter, DialogDescription,
  } from '$lib/components/ui/dialog/index.js';
  import { api } from '$lib/api.js';
  import { authStore } from '$lib/stores/auth.svelte.js';
  import { fmtDateTime } from '$lib/utils.js';
  import type { Session } from '$lib/types.js';
  import { LogOut, KeyRound, Shield } from 'lucide-svelte';

  let sessions     = $state<Session[]>([]);
  let loadingSessions = $state(true);

  let currentPass  = $state('');
  let newPass      = $state('');
  let confirmPass  = $state('');
  let savingPw     = $state(false);

  let confirmRevokeAll = $state(false);

  async function loadSessions() {
    try { sessions = await api.auth.sessions(); }
    catch { toast.error('Failed to load sessions'); }
    finally { loadingSessions = false; }
  }

  $effect(() => { loadSessions(); });

  async function revokeSession(id: string) {
    try {
      await api.auth.revokeSession(id);
      toast.success('Session revoked');
      await loadSessions();
    } catch {
      toast.error('Failed to revoke session');
    }
  }

  async function revokeAll() {
    try {
      await api.auth.revokeAllSessions();
      toast.success('All sessions revoked');
      confirmRevokeAll = false;
      await loadSessions();
    } catch {
      toast.error('Failed to revoke sessions');
    }
  }

  async function changePassword() {
    if (!currentPass || !newPass) { toast.error('All password fields are required'); return; }
    if (newPass !== confirmPass) { toast.error('New passwords do not match'); return; }
    savingPw = true;
    try {
      await api.auth.changePassword(currentPass, newPass);
      toast.success('Password changed');
      currentPass = ''; newPass = ''; confirmPass = '';
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : '';
      toast.error(msg.includes('401') ? 'Incorrect current password' : 'Failed to change password');
    } finally {
      savingPw = false;
    }
  }
</script>

<TopBar crumbs={['OmniGate', 'Profile']} />

<main class="flex-1 p-6 max-w-[960px] grid grid-cols-[1fr_1.1fr] gap-6 items-start">
  <!-- Left: Account -->
  <div class="space-y-5">
    <Card>
      <CardHeader class="pb-2">
        <CardTitle class="text-[15px]">Account</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="grid grid-cols-[90px_1fr] gap-y-2 text-[13px] mb-4">
          <span class="text-muted-foreground">Username</span>
          <span class="font-mono">{authStore.username ?? '—'}</span>
          <span class="text-muted-foreground">Role</span>
          <Badge variant="secondary" class="w-fit capitalize">{authStore.role ?? '—'}</Badge>
        </div>
      </CardContent>
    </Card>

    <Card>
      <CardHeader class="pb-2">
        <CardTitle class="text-[15px] flex items-center gap-2"><KeyRound size={15} /> Change password</CardTitle>
      </CardHeader>
      <CardContent class="space-y-3">
        <Field label="Current password">
          <Input type="password" bind:value={currentPass} placeholder="Current password" />
        </Field>
        <Field label="New password">
          <Input type="password" bind:value={newPass} placeholder="New password" />
        </Field>
        <Field label="Confirm new password">
          <Input type="password" bind:value={confirmPass} placeholder="Repeat new password" />
        </Field>
        <div class="flex justify-end pt-1">
          <Button size="sm" onclick={changePassword} disabled={savingPw || !currentPass || !newPass || !confirmPass}>
            {savingPw ? 'Saving…' : 'Change password'}
          </Button>
        </div>
      </CardContent>
    </Card>
  </div>

  <!-- Right: Sessions -->
  <Card>
    <CardHeader class="pb-2">
      <div class="flex items-center justify-between">
        <CardTitle class="text-[15px] flex items-center gap-2"><Shield size={15} /> Active sessions</CardTitle>
        {#if sessions.length > 1}
          <Button variant="outline" size="sm" onclick={() => (confirmRevokeAll = true)}>
            <LogOut size={13} /> Revoke all others
          </Button>
        {/if}
      </div>
    </CardHeader>
    <CardContent class="p-0">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>User</TableHead>
            <TableHead>Role</TableHead>
            <TableHead class="w-[40px]"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {#each sessions as s (s.session_id)}
            {@const isCurrent = s.session_id === authStore.sessionId}
            <TableRow>
              <TableCell class="font-mono text-[12px]">
                {s.username}
                {#if isCurrent}
                  <Badge class="ml-2 text-[10px]">current</Badge>
                {/if}
              </TableCell>
              <TableCell class="text-[12px] text-muted-foreground capitalize">{s.role ?? '—'}</TableCell>
              <TableCell>
                {#if !isCurrent}
                  <Button variant="ghost" size="icon-sm" class="hover:text-destructive"
                    onclick={() => revokeSession(s.session_id)} title="Revoke this session">
                    <LogOut size={13} />
                  </Button>
                {/if}
              </TableCell>
            </TableRow>
          {/each}
          {#if !loadingSessions && sessions.length === 0}
            <TableRow>
              <TableCell colspan={3} class="py-8 text-center text-muted-foreground">No active sessions.</TableCell>
            </TableRow>
          {/if}
        </TableBody>
      </Table>
    </CardContent>
  </Card>
</main>

<!-- Confirm revoke all -->
<Dialog bind:open={confirmRevokeAll}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Revoke all other sessions?</DialogTitle>
      <DialogDescription>All sessions except your current one will be terminated immediately.</DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (confirmRevokeAll = false)}>Cancel</Button>
      <Button variant="destructive" onclick={revokeAll}>Revoke all</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
