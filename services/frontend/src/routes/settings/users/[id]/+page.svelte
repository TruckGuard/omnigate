<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import Field from '$lib/components/Field.svelte';
  import GateBadge from '$lib/components/GateBadge.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Textarea } from '$lib/components/ui/textarea/index.js';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card/index.js';
  import { Separator } from '$lib/components/ui/separator/index.js';
  import {
    Select, SelectContent, SelectItem, SelectTrigger,
  } from '$lib/components/ui/select/index.js';
  import { api } from '$lib/api.js';
  import { fmtDateTime } from '$lib/utils.js';
  import type { AuthUser, AuthRole, UserProfile, Gate } from '$lib/types.js';
  import { ChevronLeft } from 'lucide-svelte';

  const userId = $derived(Number($page.params.id));

  let user    = $state<AuthUser | null>(null);
  let profile = $state<UserProfile | null>(null);
  let roles   = $state<AuthRole[]>([]);
  let gates   = $state<Gate[]>([]);
  let loading = $state(true);

  // Account form
  let editRoleId = $state('');
  let savingRole = $state(false);

  // Profile form
  let fFirst = $state('');
  let fLast  = $state('');
  let fPhone = $state('');
  let fGate  = $state('');
  let fNotes = $state('');
  let savingProfile = $state(false);

  $effect(() => {
    const id = userId;
    (async () => {
      loading = true;
      try {
        const [u, r, g, profiles] = await Promise.all([
          api.auth.getUser(id),
          api.auth.roles(),
          api.gates.list(),
          api.profiles.list(id),
        ]);
        user    = u;
        roles   = r;
        gates   = g;
        editRoleId = String(u.role_id);

        if (profiles.length > 0) {
          profile = profiles[0];
          fFirst = profile.first_name;
          fLast  = profile.last_name;
          fPhone = profile.phone;
          fGate  = profile.gate_id;
          fNotes = profile.notes;
        }
      } catch {
        toast.error('User not found');
        goto('/settings/users');
      } finally {
        loading = false;
      }
    })();
  });

  async function saveRole() {
    if (!user) return;
    savingRole = true;
    try {
      await api.auth.updateUserRole(user.id, Number(editRoleId));
      user = { ...user, role_id: Number(editRoleId), role: roles.find(r => r.id === Number(editRoleId)) };
      toast.success('Role updated');
    } catch {
      toast.error('Failed to update role');
    } finally {
      savingRole = false;
    }
  }

  async function saveProfile() {
    if (!user) return;
    savingProfile = true;
    try {
      const data = { first_name: fFirst, last_name: fLast, phone: fPhone, gate_id: fGate, notes: fNotes };
      if (profile) {
        profile = await api.profiles.update(profile.id, data);
      } else {
        profile = await api.profiles.create({ auth_id: user.id, ...data });
      }
      toast.success('Profile saved');
    } catch {
      toast.error('Failed to save profile');
    } finally {
      savingProfile = false;
    }
  }
</script>

<TopBar crumbs={['OmniGate', 'Users', user?.username ?? '…']}>
  {#snippet actions()}
    <Button variant="outline" size="sm" onclick={() => goto('/settings/users')}>
      <ChevronLeft size={14} /> Back to users
    </Button>
  {/snippet}
</TopBar>

{#if loading}
  <div class="flex-1 flex items-center justify-center text-muted-foreground">Loading…</div>
{:else if user}
  <main class="flex-1 p-6 max-w-[800px] space-y-5">
    <!-- User header -->
    <div class="flex items-center gap-3">
      <div class="w-10 h-10 rounded-full bg-primary flex items-center justify-center text-[14px] font-semibold text-primary-foreground">
        {user.username.slice(0, 2).toUpperCase()}
      </div>
      <div>
        <div class="font-semibold text-[16px]">{user.username}</div>
        <div class="flex items-center gap-2 mt-0.5">
          {#if user.role}
            <Badge variant="secondary">{user.role.name}</Badge>
          {/if}
          <span class="text-[11px] text-muted-foreground">
            Joined {fmtDateTime(user.created_at)}
            {#if user.last_login} · Last login {fmtDateTime(user.last_login)}{/if}
          </span>
        </div>
      </div>
    </div>

    <Separator />

    <div class="grid grid-cols-2 gap-6">
      <!-- Account -->
      <Card>
        <CardHeader><CardTitle>Account</CardTitle></CardHeader>
        <CardContent class="space-y-4">
          <Field label="Username">
            <Input value={user.username} disabled />
          </Field>
          <Field label="Role">
            <Select type="single" bind:value={editRoleId}>
              <SelectTrigger>
                {roles.find(r => String(r.id) === editRoleId)?.name ?? 'Select role…'}
              </SelectTrigger>
              <SelectContent>
                {#each roles as r}
                  <SelectItem value={String(r.id)}>{r.name}</SelectItem>
                {/each}
              </SelectContent>
            </Select>
          </Field>
          <div class="flex justify-end">
            <Button size="sm" onclick={saveRole} disabled={savingRole}>
              {savingRole ? 'Saving…' : 'Save account'}
            </Button>
          </div>
        </CardContent>
      </Card>

      <!-- Profile -->
      <Card>
        <CardHeader>
          <div class="flex items-center justify-between">
            <CardTitle>Profile</CardTitle>
            {#if !profile}
              <Badge variant="outline" class="text-[10px]">Not created</Badge>
            {/if}
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="grid grid-cols-2 gap-3">
            <Field label="First name">
              <Input bind:value={fFirst} placeholder="John" />
            </Field>
            <Field label="Last name">
              <Input bind:value={fLast} placeholder="Smith" />
            </Field>
          </div>
          <Field label="Phone">
            <Input bind:value={fPhone} placeholder="+1 555 000 0000" />
          </Field>
          <Field label="Assigned gate">
            <Select type="single" bind:value={fGate}>
              <SelectTrigger>
                {#if fGate}
                  <GateBadge gateId={fGate} />
                {:else}
                  No gate assigned
                {/if}
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">None</SelectItem>
                {#each gates as g}
                  <SelectItem value={g.gate_id}>{g.name} ({g.gate_id})</SelectItem>
                {/each}
              </SelectContent>
            </Select>
          </Field>
          <Field label="Notes">
            <Textarea bind:value={fNotes} rows={2} placeholder="Optional notes…" />
          </Field>
          <div class="flex justify-end">
            <Button size="sm" onclick={saveProfile} disabled={savingProfile}>
              {savingProfile ? 'Saving…' : profile ? 'Save profile' : 'Create profile'}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  </main>
{/if}
