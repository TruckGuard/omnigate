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
  import { UserCog, Trash2, KeyRound, UserPlus } from 'lucide-svelte';

  let users    = $state<AuthUser[]>([]);
  let profiles = $state<Map<number, UserProfile>>(new Map());
  let roles    = $state<AuthRole[]>([]);
  let loading  = $state(true);
  let saving   = $state(false);

  let roleOpen      = $state(false);
  let deleteOpen    = $state(false);
  let pwOpen        = $state(false);
  let createOpen    = $state(false);
  let selected      = $state<AuthUser | null>(null);
  let editRoleId    = $state('');
  let newPassword   = $state('');

  let newUsername   = $state('');
  let newUserPass   = $state('');
  let newUserRoleId = $state('');

  async function load() {
    try {
      const [u, r, p] = await Promise.all([api.auth.users(), api.auth.roles(), api.profiles.list()]);
      users    = u;
      roles    = r;
      profiles = new Map(p.map(pr => [pr.auth_id, pr]));
    } catch {
      toast.error('Помилка завантаження користувачів');
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
      toast.success('Роль оновлено');
      roleOpen = false;
      await load();
    } catch {
      toast.error('Помилка оновлення ролі');
    } finally {
      saving = false;
    }
  }

  async function handleDelete() {
    if (!selected) return;
    try {
      await api.auth.deleteUser(selected.id);
      toast.success('Користувача видалено');
      deleteOpen = false;
      await load();
    } catch {
      toast.error('Помилка видалення користувача');
    }
  }

  async function handleCreateUser() {
    if (!newUsername || !newUserPass) { toast.error('Логін та пароль обов\'язкові'); return; }
    saving = true;
    try {
      await api.auth.createUser({
        username: newUsername,
        password: newUserPass,
        role_id: newUserRoleId ? Number(newUserRoleId) : undefined,
      });
      toast.success('Користувача створено');
      createOpen = false;
      newUsername = ''; newUserPass = ''; newUserRoleId = '';
      await load();
    } catch {
      toast.error('Помилка створення користувача');
    } finally {
      saving = false;
    }
  }

  async function handleResetPw() {
    if (!selected || !newPassword) return;
    saving = true;
    try {
      await api.auth.resetPassword(selected.id, newPassword);
      toast.success('Пароль скинуто');
      pwOpen = false;
      newPassword = '';
    } catch {
      toast.error('Помилка скидання пароля');
    } finally {
      saving = false;
    }
  }
</script>

<TopBar crumbs={['OmniGate', 'Користувачі']} title="Користувачі">
  {#snippet actions()}
    <Button size="sm" onclick={() => (createOpen = true)}>
      <UserPlus size={14} /> Новий користувач
    </Button>
  {/snippet}
</TopBar>

<main class="flex-1 p-6">
  <div class="rounded-md border border-border overflow-hidden">
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Логін</TableHead>
          <TableHead class="w-[180px]">Ім'я</TableHead>
          <TableHead class="w-[120px]">Роль</TableHead>
          <TableHead class="w-[130px]">Останній вхід</TableHead>
          <TableHead class="w-[100px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {#each users as user (user.id)}
          {@const profile = profiles.get(user.id)}
          <TableRow class="cursor-pointer" onclick={() => goto(`/settings/users/${user.id}`)}>
            <TableCell class="font-mono text-sm">{user.username}</TableCell>
            <TableCell class="text-muted-foreground">
              {profile ? `${profile.first_name} ${profile.last_name}`.trim() || '—' : '—'}
            </TableCell>
            <TableCell>
              {#if user.role}
                <Badge variant="secondary">{user.role.name}</Badge>
              {:else}
                <span class="text-sm text-muted-foreground">—</span>
              {/if}
            </TableCell>
            <TableCell class="text-sm text-muted-foreground">
              {user.last_login ? timeAgo(user.last_login) : 'Ніколи'}
            </TableCell>
            <TableCell>
              <div role="presentation" class="flex gap-1" onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.stopPropagation()}>
                <Button variant="ghost" size="icon-sm" title="Редагувати роль"
                  onclick={() => { selected = user; editRoleId = String(user.role_id); roleOpen = true; }}>
                  <UserCog size={14} />
                </Button>
                <Button variant="ghost" size="icon-sm" title="Скинути пароль"
                  onclick={() => { selected = user; newPassword = ''; pwOpen = true; }}>
                  <KeyRound size={14} />
                </Button>
                <Button variant="ghost" size="icon-sm" title="Видалити" class="hover:text-destructive"
                  onclick={() => { selected = user; deleteOpen = true; }}>
                  <Trash2 size={14} />
                </Button>
              </div>
            </TableCell>
          </TableRow>
        {/each}
        {#if !loading && users.length === 0}
          <TableRow>
            <TableCell colspan={5} class="py-10 text-center text-muted-foreground">Користувачів не знайдено.</TableCell>
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
      <DialogTitle>Редагувати роль — <span class="font-mono font-normal">{selected?.username}</span></DialogTitle>
    </DialogHeader>
    <Field label="Призначена роль">
      <Select type="single" bind:value={editRoleId}>
        <SelectTrigger>{roles.find(r => String(r.id) === editRoleId)?.name ?? 'Оберіть роль…'}</SelectTrigger>
        <SelectContent>
          {#each roles as r}
            <SelectItem value={String(r.id)}>{r.name}</SelectItem>
          {/each}
        </SelectContent>
      </Select>
    </Field>
    <DialogFooter class="mt-4">
      <Button variant="outline" onclick={() => (roleOpen = false)}>Скасувати</Button>
      <Button onclick={saveRole} disabled={saving}>Зберегти</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Reset password dialog -->
<Dialog bind:open={pwOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Скинути пароль — <span class="font-mono font-normal">{selected?.username}</span></DialogTitle>
    </DialogHeader>
    <Field label="Новий пароль">
      <Input type="password" bind:value={newPassword} placeholder="Введіть новий пароль" />
    </Field>
    <DialogFooter class="mt-4">
      <Button variant="outline" onclick={() => (pwOpen = false)}>Скасувати</Button>
      <Button onclick={handleResetPw} disabled={saving || !newPassword}>
        {saving ? 'Скидання…' : 'Скинути пароль'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Delete dialog -->
<Dialog bind:open={deleteOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Видалити користувача?</DialogTitle>
      <DialogDescription>
        <span class="font-mono">{selected?.username}</span> буде назавжди видалено.
      </DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (deleteOpen = false)}>Скасувати</Button>
      <Button variant="destructive" onclick={handleDelete}>Видалити</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>

<!-- Create user dialog -->
<Dialog bind:open={createOpen}>
  <DialogContent class="max-w-sm">
    <DialogHeader>
      <DialogTitle>Новий користувач</DialogTitle>
      <DialogDescription>Створіть новий акаунт. Після входу користувач зможе змінити пароль.</DialogDescription>
    </DialogHeader>
    <div class="space-y-3 py-2">
      <Field label="Логін">
        <Input bind:value={newUsername} placeholder="john.doe" class="font-mono" />
      </Field>
      <Field label="Пароль">
        <Input type="password" bind:value={newUserPass} placeholder="Тимчасовий пароль" />
      </Field>
      <Field label="Роль">
        <Select type="single" bind:value={newUserRoleId}>
          <SelectTrigger>{roles.find(r => String(r.id) === newUserRoleId)?.name ?? 'Оберіть роль…'}</SelectTrigger>
          <SelectContent>
            {#each roles as r}
              <SelectItem value={String(r.id)}>{r.name}</SelectItem>
            {/each}
          </SelectContent>
        </Select>
      </Field>
    </div>
    <DialogFooter>
      <Button variant="outline" onclick={() => (createOpen = false)}>Скасувати</Button>
      <Button onclick={handleCreateUser} disabled={saving || !newUsername || !newUserPass}>
        {saving ? 'Створення…' : 'Створити користувача'}
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
