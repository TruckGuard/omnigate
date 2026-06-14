<script lang="ts">
  import { toast } from 'svelte-sonner';
  import TopBar from '$lib/components/TopBar.svelte';
  import Field from '$lib/components/Field.svelte';
  import { Button } from '$lib/components/ui/button/index.js';
  import { Badge } from '$lib/components/ui/badge/index.js';
  import { Input } from '$lib/components/ui/input/index.js';
  import { Textarea } from '$lib/components/ui/textarea/index.js';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card/index.js';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table/index.js';
  import {
    Dialog, DialogContent, DialogHeader, DialogTitle,
    DialogFooter, DialogDescription,
  } from '$lib/components/ui/dialog/index.js';
  import {
    Select, SelectContent, SelectItem, SelectTrigger,
  } from '$lib/components/ui/select/index.js';
  import { api } from '$lib/api.js';
  import { authStore } from '$lib/stores/auth.svelte.js';
  import type { Gate, Session, UserProfile } from '$lib/types.js';
  import { LogOut, KeyRound, Shield, User } from 'lucide-svelte';

  let sessions        = $state<Session[]>([]);
  let loadingSessions = $state(true);
  let gates           = $state<Gate[]>([]);

  // Password change
  let currentPass  = $state('');
  let newPass      = $state('');
  let confirmPass  = $state('');
  let savingPw     = $state(false);

  // Profile editing
  let profile        = $state<UserProfile | null>(null);
  let profileLoading = $state(true);
  let savingProfile  = $state(false);
  let pfFirst = $state('');
  let pfLast  = $state('');
  let pfPhone = $state('');
  let pfGate  = $state('');
  let pfNotes = $state('');

  let confirmRevokeAll = $state(false);

  async function loadSessions() {
    try { sessions = await api.auth.sessions(); }
    catch { toast.error('Помилка завантаження сесій'); }
    finally { loadingSessions = false; }
  }

  async function loadProfile() {
    try {
      gates = await api.gates.list();
      const validate = await api.auth.validate();
      const authId = Number(validate.id);
      if (!isNaN(authId)) {
        const res = await api.profiles.list(authId);
        // Гарантуємо, що працюємо з масивом
        const profiles = Array.isArray(res) ? res : [res];

        if (profiles.length > 0 && profiles[0].id) {
          profile = profiles[0];
          pfFirst = profile.first_name ?? '';
          pfLast  = profile.last_name ?? '';
          pfPhone = profile.phone ?? '';
          pfGate  = profile.gate_id ?? '';
          pfNotes = profile.notes ?? '';
        }
      }
    } catch { /* ignore */ }
    finally { profileLoading = false; }
  }

  $effect(() => { loadSessions(); loadProfile(); });

  async function revokeSession(id: string) {
    try {
      await api.auth.revokeSession(id);
      toast.success('Сесію відкликано');
      await loadSessions();
    } catch {
      toast.error('Помилка відкликання сесії');
    }
  }

  async function revokeAll() {
    try {
      await api.auth.revokeAllSessions();
      toast.success('Всі сесії відкликано');
      confirmRevokeAll = false;
      await loadSessions();
    } catch {
      toast.error('Помилка відкликання сесій');
    }
  }

  async function changePassword() {
    if (!currentPass || !newPass) { toast.error('Всі поля пароля обов\'язкові'); return; }
    if (newPass !== confirmPass) { toast.error('Нові паролі не збігаються'); return; }
    savingPw = true;
    try {
      await api.auth.changePassword(currentPass, newPass);
      toast.success('Пароль змінено');
      currentPass = ''; newPass = ''; confirmPass = '';
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : '';
      toast.error(msg.includes('401') ? 'Невірний поточний пароль' : 'Помилка зміни пароля');
    } finally {
      savingPw = false;
    }
  }

  async function saveProfile() {
    savingProfile = true;
    try {
      const data = {
        first_name: pfFirst,
        last_name: pfLast,
        phone: pfPhone,
        gate_id: pfGate,
        notes: pfNotes,
      };
      if (profile) {
        profile = await api.profiles.update(profile.id, data);
      } else {
        const validate = await api.auth.validate();
        profile = await api.profiles.create({
          auth_id: Number(validate.id),
          ...data,
        });
      }
      toast.success('Профіль збережено');
    } catch {
      toast.error('Помилка збереження профілю');
    } finally {
      savingProfile = false;
    }
  }
</script>

<TopBar crumbs={[{label:'OmniGate',href:'/'},{label:'Профіль'}]} />

<main class="flex-1 p-4 sm:p-6 max-w-[1080px] grid grid-cols-1 lg:grid-cols-[1fr_1.1fr] gap-6 items-start">
  <!-- Left column -->
  <div class="space-y-5">
    <!-- Account info -->
    <Card>
      <CardHeader class="pb-2">
        <CardTitle class="text-base">Акаунт</CardTitle>
      </CardHeader>
      <CardContent>
        <div class="grid grid-cols-[100px_1fr] gap-y-2 text-sm mb-4">
          <span class="text-muted-foreground">Логін</span>
          <span class="font-mono">{authStore.username ?? '—'}</span>
          <span class="text-muted-foreground">Роль</span>
          <Badge variant="secondary" class="w-fit capitalize">{authStore.role ?? '—'}</Badge>
        </div>
      </CardContent>
    </Card>

    <!-- Profile editing -->
    <Card>
      <CardHeader class="pb-2">
        <CardTitle class="text-base flex items-center gap-2"><User size={15} /> Персональні дані</CardTitle>
      </CardHeader>
      <CardContent class="space-y-3">
        {#if profileLoading}
          <p class="text-sm text-muted-foreground">Завантаження…</p>
        {:else}
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <Field label="Ім'я">
              <Input bind:value={pfFirst} placeholder="Іван" />
            </Field>
            <Field label="Прізвище">
              <Input bind:value={pfLast} placeholder="Петренко" />
            </Field>
          </div>
          <Field label="Телефон">
            <Input bind:value={pfPhone} placeholder="+380991234567" />
          </Field>
          <Field label="КПП">
            <Select type="single" bind:value={pfGate}>
              <SelectTrigger>
                {gates.find(g => g.gate_id === pfGate)?.name ?? (pfGate || 'Не обрано')}
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
            <Textarea bind:value={pfNotes} rows={2} placeholder="Необов'язкові нотатки…" />
          </Field>
          <div class="flex justify-end pt-1">
            <Button size="sm" onclick={saveProfile} disabled={savingProfile}>
              {savingProfile ? 'Збереження…' : 'Зберегти профіль'}
            </Button>
          </div>
        {/if}
      </CardContent>
    </Card>

    <!-- Change password -->
    <Card>
      <CardHeader class="pb-2">
        <CardTitle class="text-base flex items-center gap-2"><KeyRound size={15} /> Змінити пароль</CardTitle>
      </CardHeader>
      <CardContent class="space-y-3">
        <Field label="Поточний пароль">
          <Input type="password" bind:value={currentPass} placeholder="Поточний пароль" />
        </Field>
        <Field label="Новий пароль">
          <Input type="password" bind:value={newPass} placeholder="Новий пароль" />
        </Field>
        <Field label="Підтвердити новий пароль">
          <Input type="password" bind:value={confirmPass} placeholder="Повторіть новий пароль" />
        </Field>
        <div class="flex justify-end pt-1">
          <Button size="sm" onclick={changePassword} disabled={savingPw || !currentPass || !newPass || !confirmPass}>
            {savingPw ? 'Збереження…' : 'Змінити пароль'}
          </Button>
        </div>
      </CardContent>
    </Card>
  </div>

  <!-- Right column: Sessions -->
  <Card>
    <CardHeader class="pb-2">
      <div class="flex items-center justify-between">
        <CardTitle class="text-base flex items-center gap-2"><Shield size={15} /> Активні сесії</CardTitle>
        {#if sessions.length > 1}
          <Button variant="outline" size="sm" onclick={() => (confirmRevokeAll = true)}>
            <LogOut size={13} /> Відкликати всі інші
          </Button>
        {/if}
      </div>
    </CardHeader>
    <CardContent class="p-0">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Користувач</TableHead>
            <TableHead>Роль</TableHead>
            <TableHead class="w-[40px]"></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {#each sessions as s (s.session_id)}
            {@const isCurrent = s.session_id === authStore.sessionId}
            <TableRow>
              <TableCell class="font-mono text-sm">
                {s.username}
                {#if isCurrent}
                  <Badge class="ml-2 text-xs">поточна</Badge>
                {/if}
              </TableCell>
              <TableCell class="text-sm text-muted-foreground capitalize">{s.role ?? '—'}</TableCell>
              <TableCell>
                {#if !isCurrent}
                  <Button variant="ghost" size="icon-sm" class="hover:text-destructive"
                    onclick={() => revokeSession(s.session_id)} title="Відкликати сесію">
                    <LogOut size={13} />
                  </Button>
                {/if}
              </TableCell>
            </TableRow>
          {/each}
          {#if !loadingSessions && sessions.length === 0}
            <TableRow>
              <TableCell colspan={3} class="py-8 text-center text-muted-foreground">Активних сесій немає.</TableCell>
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
      <DialogTitle>Відкликати всі інші сесії?</DialogTitle>
      <DialogDescription>Всі сесії, окрім поточної, будуть негайно завершені.</DialogDescription>
    </DialogHeader>
    <DialogFooter>
      <Button variant="outline" onclick={() => (confirmRevokeAll = false)}>Скасувати</Button>
      <Button variant="destructive" onclick={revokeAll}>Відкликати всі</Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
