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
        toast.error('Користувача не знайдено');
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
      toast.success('Роль оновлено');
    } catch {
      toast.error('Помилка оновлення ролі');
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
      toast.success('Профіль збережено');
    } catch {
      toast.error('Помилка збереження профілю');
    } finally {
      savingProfile = false;
    }
  }
</script>

<TopBar crumbs={['OmniGate', 'Користувачі', user?.username ?? '…']}>
  {#snippet actions()}
    <Button variant="outline" size="sm" onclick={() => goto('/settings/users')}>
      <ChevronLeft size={14} /> Назад до користувачів
    </Button>
  {/snippet}
</TopBar>

{#if loading}
  <div class="flex-1 flex items-center justify-center text-muted-foreground">Завантаження…</div>
{:else if user}
  <main class="flex-1 p-6 max-w-[800px] space-y-5">
    <!-- User header -->
    <div class="flex items-center gap-3">
      <div class="w-10 h-10 rounded-full bg-primary flex items-center justify-center text-sm font-semibold text-primary-foreground">
        {user.username.slice(0, 2).toUpperCase()}
      </div>
      <div>
        <div class="font-semibold text-base">{user.username}</div>
        <div class="flex items-center gap-2 mt-0.5">
          {#if user.role}
            <Badge variant="secondary">{user.role.name}</Badge>
          {/if}
          <span class="text-[11px] text-muted-foreground">
            Зареєстровано {fmtDateTime(user.created_at)}
            {#if user.last_login} · Останній вхід {fmtDateTime(user.last_login)}{/if}
          </span>
        </div>
      </div>
    </div>

    <Separator />

    <div class="grid grid-cols-2 gap-6">
      <!-- Account -->
      <Card>
        <CardHeader><CardTitle>Акаунт</CardTitle></CardHeader>
        <CardContent class="space-y-4">
          <Field label="Логін">
            <Input value={user.username} disabled />
          </Field>
          <Field label="Роль">
            <Select type="single" bind:value={editRoleId}>
              <SelectTrigger>
                {roles.find(r => String(r.id) === editRoleId)?.name ?? 'Оберіть роль…'}
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
              {savingRole ? 'Збереження…' : 'Зберегти акаунт'}
            </Button>
          </div>
        </CardContent>
      </Card>

      <!-- Profile -->
      <Card>
        <CardHeader>
          <div class="flex items-center justify-between">
            <CardTitle>Профіль</CardTitle>
            {#if !profile}
              <Badge variant="outline" class="text-[10px]">Не створено</Badge>
            {/if}
          </div>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="grid grid-cols-2 gap-3">
            <Field label="Ім'я">
              <Input bind:value={fFirst} placeholder="Іван" />
            </Field>
            <Field label="Прізвище">
              <Input bind:value={fLast} placeholder="Петренко" />
            </Field>
          </div>
          <Field label="Телефон">
            <Input bind:value={fPhone} placeholder="+380991234567" />
          </Field>
          <Field label="Шлагбаум">
            <Select type="single" bind:value={fGate}>
              <SelectTrigger>
                {#if fGate}
                  <GateBadge gateId={fGate} />
                {:else}
                  Не призначено
                {/if}
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">Не обрано</SelectItem>
                {#each gates as g}
                  <SelectItem value={g.gate_id}>{g.name} ({g.gate_id})</SelectItem>
                {/each}
              </SelectContent>
            </Select>
          </Field>
          <Field label="Нотатки">
            <Textarea bind:value={fNotes} rows={2} placeholder="Необов'язкові нотатки…" />
          </Field>
          <div class="flex justify-end">
            <Button size="sm" onclick={saveProfile} disabled={savingProfile}>
              {savingProfile ? 'Збереження…' : profile ? 'Зберегти профіль' : 'Створити профіль'}
            </Button>
          </div>
        </CardContent>
      </Card>
    </div>
  </main>
{/if}
