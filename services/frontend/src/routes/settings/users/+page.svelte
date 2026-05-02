<script lang="ts">
  import { goto } from '$app/navigation';
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import Field from '$lib/components/Field.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table/index.js';
  import {
    Dialog, DialogContent, DialogHeader, DialogTitle,
    DialogFooter, DialogDescription,
  } from '$lib/components/ui/dialog/index.js';
  import {
    Select, SelectContent, SelectItem, SelectTrigger,
  } from '$lib/components/ui/select/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { api } from '$lib/api.js';
  import { timeAgo } from '$lib/utils.js';
  import type { AuthRole, AuthUser, UserProfile } from '$lib/types.js';
  import { UserCog, Trash2, KeyRound } from 'lucide-svelte';

  let users    = $state<AuthUser[]>([]);
  let profiles = $state<Map<number, UserProfile>>(new Map());
  let roles    = $state<AuthRole[]>([]);
  let loading  = $state(true);
  let saving   = $state(false);

  let roleOpen    = $state(false);
  let deleteOpen  = $state(false);
  let pwOpen      = $state(false);
  let selected    = $state<AuthUser | null>(null);
  let editRoleId  = $state('');
  let newPassword = $state('');

  async function load() {
    try {
      const [u, r, p] = await Promise.all([api.auth.users(), api.auth.roles(), api.profiles.list()]);
      users    = u;
      roles    = r;
      profiles = new Map(p.map(pr => [pr.auth_id, pr]));
    } catch {
      toast.error('Failed to load users');
    } finally {
      loading = false;
    }
  }

  $effect(() => { load(); });

  async function saveRole() {
    if (!selected) return;
    saving = true;
    try {
      await api.auth.updateUserRole(selected.id, Number(editRoleId));
      toast.success('Role updated');
      roleOpen = false;
      await load();
    } catch {
      toast.error('Failed to update role');
    } finally {
      saving = false;
    }
  }

  async function handleDelete() {
    if (!selected) return;
    try {
      await api.auth.deleteUser(selected.id);
      toast.success('User deleted');
      deleteOpen = false;
      await load();
    } catch {
      toast.error('Failed to delete user');
    }
  }

  async function handleResetPw() {
    if (!selected || !newPassword) return;
    saving = true;
    try {
      await api.auth.resetPassword(selected.id, newPassword);
      toast.success('Password reset');
      pwOpen = false;
      newPassword = '';
    } catch {
      toast.error('Failed to reset password');
    } finally {
      saving = false;
    }
  }
</script>

<TopBar crumbs={['OmniGate', 'Users']} title="Users" />

<main class="flex-1 p-6">
  <div class="rounded-md border border-border overflow-hidden">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Username</TableHead>
          <TableHead class="w-[180px]">Name</TableHead>
          <TableHead class="w-[120px]">Role</TableHead>
          <TableHead class="w-[130px]">Last login</TableHead>
          <TableHead class="w-[100px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each users as user (user.id)}
          {@const profile = profiles.get(user.id)}
          <TableRow class="cursor-pointer" onclick={() => goto(`/settings/users/${user.id}`)}>
            <TableCell class="font-mono text-[12px]">{user.username}</TableCell>
            <TableCell class="text-muted-foreground">
              {profile ? `${profile.first_name} ${profile.last_name}`.trim() || '—' : '—'}
            </TableCell>
            <TableCell>
              {#if user.role}
                <Badge variant="secondary">{user.role.name}</Badge>
              {:else}
                <span class="text-[12px] text-muted-foreground">—</span>
              {/if}
            </TableCell>
            <TableCell class="text-[12px] text-muted-foreground">
              {user.last_login ? timeAgo(user.last_login) : 'Never'}
            </TableCell>
            <TableCell>
              <div role="presentation" class="flex gap-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
                <Button variant="ghost" size="icon-sm" title="Edit role"
                  onclick={() => { selected = user; editRoleId = String(user.role_id); roleOpen = true; }}>
                  <UserCog size={14} />
                </Button>
                <Button variant="ghost" size="icon-sm" title="Reset password"
                  onclick={() => { selected = user; newPassword = ''; pwOpen = true; }}>
                  <KeyRound size={14} />
                </Button>
                <Button variant="ghost" size="icon-sm" title="Delete" class="hover:text-destructive"
                  onclick={() => { selected = user; deleteOpen = true; }}>
                  <Trash2 size={14} />
                </Button>
              </div>
            </TableCell>
          </TableRow>
        {/each}
        {#if !loading && users.length === 0}
          <TableRow>
            <TableCell colspan={5} class="py-10 text-center text-muted-foreground">No users found.</TableCell>
          </TableRow>
        {/if}
      </TableBody>
    </Table>
  </div>
</main>

<!-- Edit role dialog -->
<Dialog bind:open={roleOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Edit role — <span class="font-mono font-normal">{selected?.username}</span></DialogTitle>
    </DialogHeader>
    <Field label="Assigned role">
      <Select type="single" bind:value={editRoleId}>
        <SelectTrigger>{roles.find(r => String(r.id) === editRoleId)?.name ?? 'Select role…'}</SelectTrigger>
        <SelectContent>
          {#each roles as r}
            <SelectItem value={String(r.id)}>{r.name}</SelectItem>
          {/each}
        </SelectContent>
      </Select>
    </Field>
    <DialogFooter class="mt-4">
      <Button variant="outline" onclick={() => (roleOpen = false)}>Cancel</Button>
      <Button onclick={saveRole} disabled={saving}>Save</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Reset password dialog -->
<Dialog bind:open={pwOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Reset password — <span class="font-mono font-normal">{selected?.username}</span></DialogTitle>
    </DialogHeader>
    <Field label="New password">
      <Input type="password" bind:value={newPassword} placeholder="Enter new password" />
    </Field>
    <DialogFooter class="mt-4">
      <Button variant="outline" onclick={() => (pwOpen = false)}>Cancel</Button>
      <Button onclick={handleResetPw} disabled={saving || !newPassword}>
        {saving ? 'Resetting…' : 'Reset password'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Delete dialog -->
<Dialog bind:open={deleteOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Delete user?</DialogTitle>
      <DialogDescription>
        <span class="font-mono">{selected?.username}</span> will be permanently removed.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (deleteOpen = false)}>Cancel</Button>
      <Button variant="destructive" onclick={handleDelete}>Delete</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
