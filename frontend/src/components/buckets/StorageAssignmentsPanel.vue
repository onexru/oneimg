<template>
  <DialogShell title="角色存储配置" close-label="关闭角色存储配置" @close="close">
    <div class="p-4 space-y-4">
      <div class="role-tabs" role="tablist" aria-label="角色">
        <button v-for="role in roleOrder" :id="`storage-role-tab-${role}`" :key="role" type="button" role="tab" :data-role-tab="role" :aria-selected="activeRole === role" :aria-controls="`storage-role-panel-${role}`" :tabindex="activeRole === role ? 0 : -1" @click="activeRole = role" @keydown="changeTab($event, role)">{{ roleNames[role] }}<span v-if="policy && dirty(role)" class="draft-dot" aria-label="未保存"></span></button>
      </div>
      <p v-if="loading && !policy" role="status">加载中…</p>
      <div v-if="error" role="alert"><p class="text-red-600">{{ error }}</p><button class="soft-button mt-2" :disabled="loading || anyBusy" @click="load">重试</button></div>
      <template v-if="policy">
        <div v-for="role in roleOrder" v-show="activeRole === role" :id="`storage-role-panel-${role}`" :key="role" role="tabpanel" :aria-labelledby="`storage-role-tab-${role}`" :data-role-storage="role" class="space-y-3">
          <p v-if="!policy.can_edit" role="status">只读</p>
          <div>
            <label v-for="bucket in bucketPage(role).items" :key="bucket.id" class="role-storage-option">
              <input type="checkbox" :data-bucket-id="bucket.id" :checked="drafts[role].bucket_ids.includes(bucket.id)" :disabled="locked(role) || (bucket.disabled && !drafts[role].bucket_ids.includes(bucket.id))" @change="toggle(role, bucket.id, $event.target.checked)">
              <span class="min-w-0 break-words">{{ bucket.name }}</span>
              <span v-if="bucket.disabled" class="role-storage-state">已停用</span>
              <span v-else-if="bucket.capacity > 0 && bucket.usage >= bucket.capacity" class="role-storage-state">已满</span>
            </label>
            <label v-for="id in missingIds(role)" :key="`missing-${id}`" class="role-storage-option">
              <input type="checkbox" :data-bucket-id="id" checked :disabled="locked(role)" @change="toggle(role, id, false)">
              <span>存储源 #{{ id }}</span><span class="role-storage-state">已删除</span>
            </label>
            <p v-if="!choices.length && !missingIds(role).length">暂无存储源</p>
          </div>
          <nav v-if="bucketPage(role).pages > 1" class="flex justify-between items-center gap-2" :aria-label="`${roleNames[role]}存储分页`">
            <button class="soft-button" :disabled="pages[role] <= 1" @click="pages[role]--">上一页</button>
            <span>{{ bucketPage(role).page }} / {{ bucketPage(role).pages }}</span>
            <button class="soft-button" :disabled="pages[role] >= bucketPage(role).pages" @click="pages[role]++">下一页</button>
          </nav>
          <label v-if="dirty(role) && needsEmptyConfirmation(role)" class="flex items-start gap-2 text-sm text-amber-700 dark:text-amber-300">
            <input v-model="emptyConfirmed[role]" type="checkbox" data-role-empty-confirm :disabled="locked(role)" class="mt-1 shrink-0">
            <span>{{ role === 3 ? '确认清空角色存储（个人额外授权不变）' : '确认清空，该角色将无法上传' }}</span>
          </label>
          <p v-if="drift[role]" role="alert" class="text-amber-700">配置已变化，请重新加载。</p>
          <button v-if="drift[role]" type="button" class="soft-button" :disabled="anyBusy" @click="resetRole(role)">重新加载此角色</button>
          <p v-if="errors[role]" role="alert" class="break-words text-red-600">{{ errors[role] }}</p>
          <p v-if="success[role]" role="status" class="text-emerald-700 dark:text-emerald-300">{{ success[role] }}</p>
          <button v-if="pending[role]" type="button" class="soft-button" :disabled="anyBusy" @click="confirmSaved(role)">核对保存结果</button>
        </div>
      </template>
    </div>
    <template #footer>
      <button type="button" class="soft-button" :disabled="anyBusy" @click="close">关闭</button>
      <button v-if="policy" id="role-storage-save" type="button" data-role-save class="primary-button" :disabled="locked(activeRole) || !dirty(activeRole) || (needsEmptyConfirmation(activeRole) && !emptyConfirmed[activeRole])" @click="save(activeRole)">{{ anyBusy ? '保存中…' : '保存' }}</button>
    </template>
  </DialogShell>
</template>
<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import DialogShell from '@/components/DialogShell.vue';
import { selectorPage } from '@/utils/renderBounds.js';
import { sameGrants } from '@/utils/accessManagement.js';
import { loadStorageAssignments, writeRoleAssignment, verifyRoleAssignment, sameRoleAssignment } from '@/utils/storageAssignments.js';
const emit = defineEmits(['close']);
const roleOrder = [1, 3, 2], roleNames = { 1: '管理员', 3: '普通用户', 2: '游客' };
const activeRole = ref(1), policy = ref(null), loading = ref(true), error = ref(''), busyRole = ref(null);
const drafts = ref({}), pages = ref({ 1: 1, 3: 1, 2: 1 }), emptyConfirmed = ref({}), errors = ref({}), success = ref({}), drift = ref({}), pending = ref({});
const controller = new AbortController();
let disposed = false;
const anyBusy = computed(() => busyRole.value !== null);
const choices = computed(() => policy.value?.buckets.filter(bucket => !policy.value.multi_storage_sync || bucket.type !== 'default') || []);
function close() { if (!anyBusy.value) emit('close'); }
function changeTab(event, role) {
  if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
  event.preventDefault();
  const index = roleOrder.indexOf(role);
  activeRole.value = event.key === 'Home' ? roleOrder[0] : event.key === 'End' ? roleOrder.at(-1) : roleOrder[(index + (event.key === 'ArrowRight' ? 1 : -1) + roleOrder.length) % roleOrder.length];
  document.getElementById(`storage-role-tab-${activeRole.value}`)?.focus();
}
function baseline(role) { return policy.value.roles.find(item => item.role === role); }
function savedIds(role) {
  const row = baseline(role);
  const ids = row.mode === 'custom' ? row.bucket_ids : row.effective_bucket_ids;
  return ids.filter(id => !policy.value.multi_storage_sync || !policy.value.buckets.some(bucket => bucket.id === id && bucket.type === 'default'));
}
// Compatibility rows are read only; every user edit saves an explicit pool.
// Simply opening or switching tabs must never convert or write existing policy.
function payload(role) { return { mode: 'custom', bucket_ids: [...drafts.value[role].bucket_ids] }; }
function dirty(role) { return !sameGrants(drafts.value[role].bucket_ids, savedIds(role)); }
function locked(role) { return loading.value || anyBusy.value || !!error.value || !policy.value?.can_edit || !!pending.value[role] || !!drift.value[role]; }
function bucketPage(role) { return selectorPage(choices.value, pages.value[role]); }
function needsEmptyConfirmation(role) { return !policy.value.multi_storage_sync && !drafts.value[role].bucket_ids.length; }
function resetFeedback(role) { errors.value[role] = ''; success.value[role] = ''; emptyConfirmed.value[role] = false; }
function missingIds(role) { return drafts.value[role].bucket_ids.filter(id => !policy.value.buckets.some(bucket => bucket.id === id)); }
function resetRole(role) {
  drafts.value[role] = { bucket_ids: [...savedIds(role)] };
  drift.value[role] = false; resetFeedback(role);
}
function toggle(role, id, checked) {
  if (locked(role)) return;
  const values = drafts.value[role].bucket_ids;
  drafts.value[role].bucket_ids = checked ? [...new Set([...values, id])] : values.filter(value => value !== id);
  resetFeedback(role);
}
function accept(next, savedRole = null) {
  const previous = policy.value;
  const metadataChanged = previous && (previous.multi_storage_sync !== next.multi_storage_sync ||
    JSON.stringify(previous.buckets.map(bucket => [bucket.id, bucket.type, bucket.disabled])) !== JSON.stringify(next.buckets.map(bucket => [bucket.id, bucket.type, bucket.disabled])));
  const keep = {};
  for (const role of roleOrder) {
    keep[role] = previous && role !== savedRole && dirty(role);
    const nextRole = next.roles.find(item => item.role === role);
    if (keep[role] && (metadataChanged || !sameRoleAssignment(baseline(role), nextRole) || !sameGrants(baseline(role).effective_bucket_ids, nextRole.effective_bucket_ids))) drift.value[role] = true;
  }
  policy.value = next;
  for (const role of roleOrder) if (!keep[role]) resetRole(role);
}
async function load() {
  if (anyBusy.value) return;
  loading.value = true; error.value = '';
  try { const next = await loadStorageAssignments(fetch, controller.signal); if (!disposed) accept(next); }
  catch (caught) { if (!disposed) error.value = caught.message || '加载失败'; }
  finally { if (!disposed) loading.value = false; }
}
async function readBack(role) {
  const next = await verifyRoleAssignment(role, pending.value[role], fetch, controller.signal);
  if (disposed) return;
  pending.value[role] = null; accept(next, role); success.value[role] = '已保存';
}
async function confirmSaved(role) {
  if (anyBusy.value) return;
  busyRole.value = role; errors.value[role] = '';
  try { await readBack(role); }
  catch (caught) { if (!disposed) errors.value[role] = caught.message; }
  finally { if (!disposed) busyRole.value = null; }
}
async function save(role) {
  if (locked(role) || !dirty(role) || (needsEmptyConfirmation(role) && !emptyConfirmed.value[role])) return;
  busyRole.value = role; errors.value[role] = ''; success.value[role] = '';
  const expected = payload(role);
  try {
    const current = await loadStorageAssignments(fetch, controller.signal);
    if (disposed) return;
    const previous = baseline(role);
    accept(current);
    if (drift.value[role] || !sameRoleAssignment(previous, baseline(role))) throw new Error('配置已变化，请重新加载');
    if (!current.can_edit) throw new Error('无保存权限');
    await writeRoleAssignment(role, expected, fetch, controller.signal);
    if (disposed) return;
    pending.value[role] = expected;
    await readBack(role);
  } catch (caught) { if (!disposed) errors.value[role] = pending.value[role] ? '保存结果未确认，请核对。' : caught.message; }
  finally { if (!disposed) busyRole.value = null; }
}
onMounted(load);
onBeforeUnmount(() => { disposed = true; controller.abort(); });
</script>
<style scoped>
button:disabled { cursor: not-allowed; opacity: .55; }
.role-tabs { display: flex; gap: 4px; padding: 4px; background: rgb(148 163 184 / .12); border-radius: 10px; }
.role-tabs button { flex: 1; min-width: 0; padding: 9px 4px; font-size: 14px; border-radius: 7px; }
.role-tabs button[aria-selected='true'] { background: white; color: #2563eb; box-shadow: 0 1px 3px #0001; }
.draft-dot { display: inline-block; width: 5px; height: 5px; margin-left: 5px; background: #2563eb; border-radius: 50%; vertical-align: middle; }
.role-storage-option { display: flex; align-items: center; gap: 10px; padding: 12px 0; border-bottom: 1px solid rgb(148 163 184 / .18); font-size: 14px; }
.role-storage-option input { width: 16px; height: 16px; flex-shrink: 0; }
.role-storage-state { margin-left: auto; white-space: nowrap; font-size: 12px; color: #64748b; }
:global(.dark) .role-tabs button[aria-selected='true'] { background: #334155; color: #93c5fd; }
</style>
