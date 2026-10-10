<template>
  <DialogShell :title="isStorage ? '个人额外分配' : '用户权限'" :subtitle="user.username" close-label="关闭授权弹窗" @close="close">
    <form id="user-access-form" class="access-dialog-body p-4 space-y-4" @submit.prevent="save">
      <p v-if="loading" role="status" class="text-sm text-slate-500">正在读取最新授权与访问策略…</p>
      <template v-if="target && policy && actor">
        <p v-if="readOnlyReason" class="access-note">{{ readOnlyReason }}</p>
        <template v-if="isStorage">
          <div class="space-y-2">
            <label v-for="bucket in bucketPage.items" :key="bucket.id" class="access-choice" :class="{ 'opacity-60': bucket.disabled }">
              <input type="checkbox" :data-bucket-id="bucket.id" :checked="bucketChecked(bucket)" :disabled="locked || bucketLocked(bucket)" @change="toggleBucket(bucket.id, $event.target.checked)">
              <span class="min-w-0 break-words"><span class="block font-medium">{{ bucket.name }}</span><span class="block text-xs text-slate-500">{{ bucket.type }} · {{ bucketHint(bucket) }}</span></span>
            </label>
            <p v-if="!bucketPage.items.length" class="text-sm text-slate-500">暂无可分配的{{ policy.multi_storage_sync ? '远程同步' : '额外存储' }}源。</p>
          </div>
          <nav v-if="bucketPage.pages > 1" class="flex items-center justify-between gap-2" aria-label="存储源分页">
            <button type="button" class="soft-button" :disabled="page <= 1" @click="page--">上一页</button>
            <span class="text-xs">{{ bucketPage.page }} / {{ bucketPage.pages }}</span>
            <button type="button" class="soft-button" :disabled="page >= bucketPage.pages" @click="page++">下一页</button>
          </nav>
          <p v-if="disabledAssigned.length" class="text-xs text-amber-700 dark:text-amber-300">已停用：{{ disabledAssigned.join('、') }}</p>
          <label v-if="selfStorage && !readOnlyReason" class="access-choice border-amber-300">
            <input id="user-access-self-confirm" v-model="confirmSelf" type="checkbox" :disabled="locked">
            <span class="text-sm">我确认：保存将撤销当前账号会话，需要重新登录后核对结果。</span>
          </label>
        </template>
        <template v-else>
          <section class="access-note" data-base-permissions>
            <h4 class="font-semibold">基础权限</h4>
            <p class="mt-1 leading-6">本人上传、查看和删除图片、管理本人文件夹。</p>
          </section>
          <h4 class="font-semibold text-sm">额外功能授权</h4>
          <section v-for="group in primaryGroups" :key="group.title" class="space-y-2">
            <h5 class="text-xs font-semibold text-slate-500">{{ group.title }}</h5>
            <div class="grid grid-cols-2 gap-2">
              <label v-for="item in group.items" :key="item.code" class="access-choice text-xs">
                <input type="checkbox" :data-permission-code="item.code" :checked="codeChecked(item.code)" :disabled="locked" @change="toggleCode(item.code, $event.target.checked)">
                <span class="break-words">{{ item.name }}</span>
              </label>
            </div>
          </section>
          <details class="space-y-3">
            <summary class="cursor-pointer text-sm font-medium">其他功能授权（用户、存储、跨用户图片）</summary>
            <section v-for="group in otherGroups" :key="group.title" class="space-y-2">
              <h5 class="text-xs font-semibold text-slate-500">{{ group.title === '图片管理' ? '其他用户图片操作（另需管理员角色）' : group.title }}</h5>
              <div class="grid grid-cols-2 gap-2">
                <label v-for="item in group.items" :key="item.code" class="access-choice text-xs">
                  <input type="checkbox" :data-permission-code="item.code" :checked="codeChecked(item.code)" :disabled="locked || (item.code.startsWith('image:') && Number(target.role) !== 1)" @change="toggleCode(item.code, $event.target.checked)">
                  <span class="break-words">{{ item.name }}</span>
                </label>
              </div>
            </section>
          </details>
          <p v-if="unknownCodes.length" class="text-xs text-amber-700 dark:text-amber-300">存在当前界面不支持的历史授权，已保留且暂停编辑，避免保存时被过滤：{{ unknownCodes.join('、') }}</p>
        </template>
        <p class="text-xs leading-5 text-amber-700 dark:text-amber-300">保存后该用户需重新登录。</p>
      </template>
      <p v-if="error" id="user-access-error" role="alert" class="text-sm break-words text-red-600 dark:text-red-400">{{ error }}</p>
      <button v-if="!loading && !target" type="button" class="soft-button" @click="load">重新加载授权</button>
      <div v-if="pending" class="space-y-2">
        <button type="button" class="soft-button" :disabled="busy" @click="confirmSaved">重新核对保存结果</button>
        <a v-if="selfStorage" href="/login" class="block text-sm underline">重新登录后查看此账号的存储授权</a>
      </div>
    </form>
    <template #footer>
      <button v-if="target && !readOnlyReason" :id="isStorage ? 'user-storage-save' : 'user-permissions-save'" type="submit" form="user-access-form" class="primary-button" :disabled="locked || !dirty || (selfStorage && !confirmSelf)">{{ busy ? '正在核对…' : isStorage ? '保存存储分配' : '保存用户权限' }}</button>
      <button type="button" class="soft-button" :disabled="busy" @click="close">{{ readOnlyReason ? '关闭' : '取消' }}</button>
    </template>
  </DialogShell>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import DialogShell from '@/components/DialogShell.vue';
import { PERMISSION_GROUPS } from '@/utils/userPermissions.js';
import { selectorPage } from '@/utils/renderBounds.js';
import { loadAccessPolicy, loadAccessUser, loadAccessActor, writeUserAccess, verifyUserAccess, accessReadOnlyReason, sameGrants, inheritedUserBucketIds } from '@/utils/accessManagement.js';
const props = defineProps({ user: { type: Object, required: true }, mode: { type: String, required: true } });
const emit = defineEmits(['close', 'saved']);
const target = ref(null), policy = ref(null), actor = ref(null), loading = ref(true), busy = ref(false), error = ref('');
const selected = ref([]), page = ref(1), pending = ref(null), confirmSelf = ref(false);
const controller = new AbortController();
let disposed = false;
const isStorage = computed(() => props.mode === 'storage');
const supportedCodes = new Set(PERMISSION_GROUPS.flatMap(group => group.items.map(item => item.code)));
const primaryGroups = PERMISSION_GROUPS.filter(group => ['系统设置', '内容与标签'].includes(group.title)).reverse();
const otherGroups = PERMISSION_GROUPS.filter(group => !primaryGroups.includes(group));
const unknownCodes = computed(() => (target.value?.permission?.codes || []).filter(code => !supportedCodes.has(code)));
const readOnlyReason = computed(() => !target.value || !actor.value ? '正在核对当前身份与授权…' : accessReadOnlyReason(props.mode, target.value, policy.value, actor.value) || (!isStorage.value && unknownCodes.value.length ? '历史授权只读，请先确认服务端支持范围。' : ''));
const locked = computed(() => loading.value || busy.value || !!pending.value || !!readOnlyReason.value);
const selfStorage = computed(() => isStorage.value && target.value?.id === actor.value?.id && !!policy.value?.multi_storage_sync);
const original = computed(() => {
  const values = (isStorage.value ? target.value?.permission?.buckets : target.value?.permission?.codes) || [];
  return isStorage.value && policy.value?.multi_storage_sync ? values.filter(id => !policy.value.buckets.some(bucket => bucket.id === id && bucket.type === 'default')) : values;
});
const dirty = computed(() => !sameGrants(selected.value, original.value));
const selectableBuckets = computed(() => {
  if (!policy.value || !target.value) return [];
  return policy.value.buckets.filter(bucket => !policy.value.multi_storage_sync || bucket.type !== 'default');
});
const bucketPage = computed(() => selectorPage(selectableBuckets.value, page.value));
const disabledAssigned = computed(() => policy.value?.buckets.filter(bucket => bucket.disabled && selected.value.includes(bucket.id)).map(bucket => bucket.name) || []);
function inherited(bucket) { return inheritedUserBucketIds(target.value, policy.value).includes(bucket.id); }
function bucketLocked(bucket) { return bucket.disabled || inherited(bucket); }
function bucketChecked(bucket) { return selected.value.includes(bucket.id) || (!bucket.disabled && inherited(bucket)); }
function bucketHint(bucket) {
  if (bucket.disabled) return '已停用';
  if (inherited(bucket)) return '已分配';
  if (bucket.capacity > 0 && bucket.usage >= bucket.capacity) return '已满';
  return '';
}
function toggleBucket(id, checked) { if (!locked.value) selected.value = checked ? [...new Set([...selected.value, id])] : selected.value.filter(value => value !== id); }
function codeChecked(code) { return !!target.value?.access_summary?.all_management_permissions || selected.value.includes(code); }
function toggleCode(code, checked) { if (!locked.value) selected.value = checked ? [...new Set([...selected.value, code])] : selected.value.filter(value => value !== code); }
function close() { if (!busy.value) emit('close'); }
async function load() {
  loading.value = true; error.value = '';
  try {
    const [nextUser, nextPolicy, nextActor] = await Promise.all([loadAccessUser(props.user.id, fetch, controller.signal), loadAccessPolicy(fetch, controller.signal), loadAccessActor(fetch, controller.signal)]);
    if (disposed) return;
    target.value = nextUser; policy.value = nextPolicy; actor.value = nextActor;
    // Local storage is fixed in multi mode; never send it as a remote target.
    selected.value = [...(isStorage.value ? nextUser.permission.buckets || [] : nextUser.permission.codes || [])];
    if (isStorage.value && nextPolicy.multi_storage_sync) selected.value = selected.value.filter(id => !nextPolicy.buckets.some(bucket => bucket.id === id && bucket.type === 'default'));
  } catch (caught) { if (!disposed) error.value = caught.message || '加载授权失败'; }
  finally { if (!disposed) loading.value = false; }
}
async function readBack() {
  const updated = await verifyUserAccess(props.user.id, props.mode, pending.value, fetch, controller.signal);
  if (!disposed) emit('saved', updated);
}
async function confirmSaved() {
  if (busy.value || !pending.value) return;
  busy.value = true; error.value = '';
  try { await readBack(); }
  catch (caught) { if (!disposed) error.value = `${caught.message}；尚未确认保存，请重新登录或核对后再操作。`; }
  finally { if (!disposed) busy.value = false; }
}
async function save() {
  if (locked.value || !dirty.value || (selfStorage.value && !confirmSelf.value)) return;
  busy.value = true; error.value = '';
  const values = [...selected.value];
  try {
    // Recheck the cookie identity and capability before writing, without replacing drafts.
    const [currentPolicy, currentActor] = await Promise.all([loadAccessPolicy(fetch, controller.signal), loadAccessActor(fetch, controller.signal)]);
    if (currentActor.id !== actor.value.id) throw new Error('登录账号已变更，请重新打开弹窗');
    if (currentPolicy.multi_storage_sync !== policy.value.multi_storage_sync || currentPolicy.default_storage_id !== policy.value.default_storage_id || JSON.stringify(currentPolicy.roles) !== JSON.stringify(policy.value.roles)) throw new Error('存储策略已变更，请重新打开弹窗核对');
    const reason = accessReadOnlyReason(props.mode, target.value, currentPolicy, currentActor);
    if (reason) throw new Error(reason);
    await writeUserAccess(props.user.id, props.mode, values, fetch, controller.signal);
    if (disposed) return;
    pending.value = values;
    await readBack();
  } catch (caught) { if (!disposed) error.value = `${caught.message}${pending.value ? '；写入可能已生效，尚未确认，请先核对结果。' : ''}`; }
  finally { if (!disposed) busy.value = false; }
}
onMounted(load);
onBeforeUnmount(() => { disposed = true; controller.abort(); });
</script>

<style scoped>
.access-dialog-body { color: #334155; }
.access-choice { display: flex; align-items: flex-start; gap: 8px; border: 1px solid rgb(148 163 184 / .25); border-radius: 10px; padding: 10px; min-width: 0; }
.access-choice input { flex: 0 0 auto; margin-top: 3px; width: 16px; height: 16px; }
.access-note { padding: 12px; border-radius: 10px; background: rgb(148 163 184 / .1); font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; }
button:disabled { cursor: not-allowed; opacity: .55; }
:global(.dark) .access-dialog-body { color: #cbd5e1; }
</style>
