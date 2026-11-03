<template>
  <div class="page-shell">
    <!-- 上传卡：标题 + 存储/分类选择 + 设置按钮，下方是拖放区。
         把原来的「上传设置」面板并进来，减少一层卡片，设置项也不会被滚动到视线外。 -->
    <div class="min-w-0 surface">
      <div class="mb-3 flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between">
        <h2 class="surface-title shrink-0 whitespace-nowrap text-[15px]">
          <i class="ri-upload-cloud-2-line text-primary"></i>
          图片上传
        </h2>

        <!-- 只读胶囊：显示当前存储与分类，真正的切换在设置弹窗里。
             上传是高频动作，把「改配置」收进弹窗能减少误触，也让这行更短。 -->
        <div class="flex min-w-0 flex-1 basis-0 flex-wrap items-center gap-1.5 sm:justify-end">
          <span class="upload-chip" :title="`存储：${currentBucketLabel}`">
            <i class="ri-database-2-line" aria-hidden="true"></i>
            <span class="upload-chip-text">{{ currentBucketLabel }}</span>
          </span>

          <span v-if="foldersAuthenticated" class="upload-chip" :title="`分类：${currentFolderLabel}`">
            <i class="ri-folder-3-line" aria-hidden="true"></i>
            <span class="upload-chip-text">{{ currentFolderLabel }}</span>
          </span>

          <!-- 设置：纯图标按钮，点开是模态弹窗。
               质量这类「偶尔调一次」的选项不该常驻页面占宽度，
               用一个小圆点表示当前不是原图，点开才知道具体档位。 -->
          <button
            type="button"
            class="upload-icon-setting"
            :class="{ 'is-dirty': uploadQuality !== QUALITY_MAX }"
            :title="`上传设置（当前${uploadQuality === QUALITY_MAX ? '原图' : uploadQuality + '%'}）`"
            aria-label="上传设置"
            aria-haspopup="dialog"
            @click="uploadSettingsOpen = true"
          >
            <i class="ri-equalizer-2-line" aria-hidden="true"></i>
            <span v-if="uploadQuality !== QUALITY_MAX" class="upload-icon-setting-dot" aria-hidden="true"></span>
          </button>
        </div>
      </div>

          <div 
          class="imageflow-dropzone upload-area relative cursor-pointer overflow-hidden transition-all duration-300"
          :class="{ 
            'border-primary/30 bg-primary/5 dark:bg-primary/5': isDragOver,
            'border-slate-200 bg-slate-50 dark:border-white/10 dark:bg-slate-900/40': !isDragOver && !isUploading,
            'border-primary/50 bg-primary/10 dark:bg-primary/10': isUploading
          }"
          @drop="handleDrop"
          @dragover.prevent="handleDragOver"
          @dragenter.prevent="handleDragEnter"
          @dragleave="handleDragLeave"
          @click="triggerFileInput"
        >
          <div v-if="!isUploading" class="upload-content px-1 py-4 text-center">
            <div class="upload-icon mb-1.5 text-3xl text-slate-900 dark:text-white">
              <i class="ri-upload-cloud-line"></i>
            </div>
            <h3 class="mb-1 text-[15px] font-semibold leading-6 text-slate-900 dark:text-white">拖拽图片到此处，或点击上传</h3>
            <p class="mx-auto mb-3 max-w-md text-xs leading-5 text-slate-500 dark:text-slate-400">支持常见图片格式、剪贴板和 URL 上传。</p>
            <p v-if="isGuest" class="mx-auto mb-3 max-w-md text-xs text-secondary">游客图片依赖本浏览器的安全 Cookie。清除 Cookie 或更换浏览器后无法找回，请注册账户保存长期身份。</p>
            <div class="flex flex-col items-stretch justify-center gap-2 sm:flex-row sm:flex-wrap sm:items-center">
            <button class="primary-button w-full px-4 py-2 sm:w-auto">
              <i class="ri-file-image-line"></i>
              选择图片
            </button>
            <button 
            @click.stop="uploadbyurlmodal"
            class="soft-button w-full border-slate-200 px-4 py-2 sm:w-auto">
              <i class="ri-links-line"></i>
              从URL上传
            </button>
            </div>
            <p class="mt-2 text-center text-[11px] text-slate-400 dark:text-slate-500">
              支持 Ctrl+V 粘贴和直接拖入
            </p>
          </div>

          <!-- 上传进度状态 -->
          <div v-else class="upload-progress px-3 py-8 text-center sm:px-4 sm:py-10">
            <div class="spinner w-10 h-10 border-4 border-primary/30 border-t-primary rounded-full animate-spin mx-auto mb-3"></div>
            <p class="text-secondary text-sm mb-3">{{ uploadSummary }}</p>
            <template v-if="legacyUploading">
              <progress class="block h-2 w-full max-w-md mx-auto accent-blue-600" :value="uploadProgress == null ? undefined : uploadProgress" max="100" aria-label="文件传输进度"></progress>
              <button class="soft-button mt-3 px-3 py-1.5 text-xs" type="button" @click.stop="cancelLegacyUpload()">停止本次请求</button>
              <p class="mt-2 text-xs text-slate-500">停止请求不保证撤回服务器已接收的文件，请检查最近上传。</p>
            </template>
          </div>
        </div>

        <input 
          ref="fileInput"
          type="file"
          multiple
          :accept="uploadAccept"
          @change="handleFileSelect"
          class="hidden"
        />
      </div>

        <div v-if="unlinkedDirectTasks.length || directTasksError" class="surface space-y-2">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h2 class="text-sm font-semibold text-slate-900 dark:text-white">上传任务</h2>
            <button class="soft-button px-2 py-1 text-xs" type="button" :disabled="directRefreshing" @click="refreshDirectTasks">刷新状态</button>
          </div>
          <p class="text-[11px] leading-5 text-slate-500 dark:text-slate-400">显示最近 {{ MAX_RECENT_TASKS }} 个任务；文件传输、主图处理和缩略图分别显示。</p>
          <p v-if="directTasksError" class="text-xs text-amber-700 dark:text-amber-300" role="status">{{ directTasksError }}</p>
          <DirectUploadTaskStatus v-for="task in unlinkedDirectTasks" :key="task.client_id || task.id" :task="task"
            @retry="retryDirectTask" @cancel="cancelDirectTask" @confirm="confirmDirectTask" @reselect="reselectDirectFile" />
        </div>

      <!-- 最近上传：不再套外层大卡片，每张图自己是卡片，边界更清楚。
           标题行直接贴在页面背景上，和上方上传卡平级。 -->
      <div class="min-w-0">
        <div class="mb-2.5 flex items-center justify-between gap-2">
          <h2 class="surface-title text-[15px]">
            <i class="ri-gallery-line text-primary"></i>
            最近上传
          </h2>
          <div class="flex flex-wrap items-center gap-2">
            <!-- 复制格式只在标题处选一次，下面所有条目的复制按钮都跟随它 -->
            <div class="recent-format-group" role="group" aria-label="复制格式">
              <button
                v-for="fmt in COPY_FORMATS"
                :key="fmt.value"
                type="button"
                class="recent-format-pill"
                :class="{ 'is-active': copyFormat === fmt.value }"
                :aria-pressed="copyFormat === fmt.value"
                :title="`全部复制为 ${fmt.label}`"
                @click="copyFormat = fmt.value"
              >{{ fmt.short }}</button>
            </div>
            <span class="recent-count">{{ recentImages.length }} 张</span>
          </div>
        </div>

        <div v-if="recentImages.length > 0" class="home-result-list">
          <RecentImageCard v-for="image in recentImages" :key="image.id" v-bind="{ image, multiStorageSync, copyFormat, formatFileSize, getFullUrl, handleImageLoad, handleImageError, previewImage, downloadImage, deleteImage, directTaskForImage, retryDirectTask, cancelDirectTask, confirmDirectTask, reselectDirectFile, getStorageStatuses, getStorageStatusMeta, getStorageDisplayName, copyImageLink, getHtmlCode, getMarkdownCode }" />
        </div>

        <!-- 无图片状态 -->
        <div v-else class="flex min-h-[200px] flex-col items-center justify-center rounded-[var(--radius-md)] border border-dashed border-slate-200 py-8 text-center dark:border-white/10">
          <div class="mb-2.5 text-5xl text-slate-300 dark:text-slate-700">
            <i class="ri-image-line"></i>
          </div>
          <p class="text-sm text-slate-500 dark:text-slate-400">暂无上传的图片</p>
        </div>
      </div>
    <FolderDialog v-if="createFolderOpen && foldersAuthenticated" mode="create" :folders="folders" :account-key="folderAccountKey" @close="createFolderOpen = false" @saved="onFolderCreated" />

    <!-- 上传设置弹窗：模态、居中、带遮罩，与图库的文件夹弹窗同一套外壳 -->
    <DialogShell
      v-if="uploadSettingsOpen"
      ref="uploadSettingsRef"
      title="上传设置"
      subtitle="调整后对之后的上传生效，已上传的图片不受影响。"
      icon="ri-equalizer-2-line"
      presentation="folder"
      close-label="关闭上传设置"
      @close="uploadSettingsOpen = false"
    >
      <div class="px-5 pb-1 sm:px-6">
        <!-- 存储与分类：只有在这里才能真正切换 -->
        <div class="space-y-3">
          <div>
            <label for="upload-bucket" class="field-label">存储</label>
            <select
              v-if="!multiStorageSync"
              id="upload-bucket" aria-label="上传存储"
              class="input-modern"
              v-model="selectedBucket"
              :disabled="isGuest || isUploading"
              @change="handleBucketChange"
            >
              <option v-for="bucket in presetBuckets" :key="bucket.id" :value="bucket.id">{{ bucket.name }}</option>
            </select>
            <p v-else class="field-hint">{{ syncTargetLabel }}（多存储同步模式，暂不可切换）</p>
          </div>

          <div v-if="foldersAuthenticated">
            <label for="upload-folder" class="field-label">分类</label>
            <div class="flex gap-2">
              <select
                id="upload-folder" aria-label="上传分类"
                class="input-modern min-w-0 flex-1"
                v-model="selectedFolder"
                :disabled="isUploading || foldersLoading || !foldersLoaded"
              >
                <option value="0">未分类</option>
                <option v-for="folder in folders" :key="folder.id" :value="String(folder.id)" :disabled="folder.deleting">{{ folderLabel(folder) }}</option>
              </select>
              <button type="button" class="soft-button shrink-0 px-3" :disabled="isUploading || foldersLoading || !foldersLoaded || foldersAtLimit" @click="createFolderOpen = true" title="新建分类" aria-label="新建分类"><i class="ri-add-line" aria-hidden="true"></i></button>
            </div>
            <p v-if="foldersAtLimit" class="field-hint">已达到 200 个分类上限。</p>
            <p v-if="foldersError" role="alert" class="field-hint text-red-600">{{ foldersError }} <button type="button" class="underline" :disabled="isUploading" @click="reloadFolders">重试</button></p>
          </div>

          <!-- 标签：可从已有的里选，也可以直接输入新建。
               新建要 tag:create 权限，无权限时输入框禁用而不是隐藏——
               隐藏会让人以为「不能建」，禁用加说明才知道是没权限。 -->
          <div>
            <div class="flex items-baseline justify-between gap-2">
              <p class="field-label mb-0">标签</p>
              <span v-if="uploadTags.length" class="text-[11px] text-slate-400 dark:text-slate-500">已选 {{ uploadTags.length }} 个</span>
            </div>

            <!-- 已有标签：下拉选择 -->
            <select
              id="upload-tag"
              aria-label="选择已有标签"
              class="input-modern mt-1.5"
              :disabled="tagsLoading || !availableTags.length"
              @change="onPickTag($event)"
            >
              <option value="">{{ tagsLoading ? '正在加载标签…' : (availableTags.length ? '选择已有标签…' : '还没有标签') }}</option>
              <option v-for="tag in availableTags" :key="tag.id" :value="String(tag.id)">{{ tag.name }}</option>
            </select>

            <!-- 已选：胶囊，点一下移除 -->
            <div v-if="uploadTags.length" class="mt-2 flex flex-wrap gap-1.5">
              <button
                v-for="tag in uploadTags" :key="tag.id"
                type="button"
                class="upload-tag-chip is-active"
                :title="`移除 ${tag.name}`"
                @click="removeUploadTag(tag)"
              >
                {{ tag.name }}
                <i class="ri-close-line" aria-hidden="true"></i>
              </button>
            </div>

            <!-- 新建：输入即建，回车提交 -->
            <div class="mt-2 flex gap-2">
              <input
                id="upload-new-tag"
                v-model="newTagName"
                type="text"
                class="input-modern min-w-0 flex-1"
                :placeholder="canCreateTag ? '或输入新标签名，回车创建' : '没有新建标签的权限'"
                :disabled="!canCreateTag || creatingTag"
                :aria-invalid="!!newTagError"
                aria-describedby="upload-new-tag-hint"
                maxlength="10"
                @keydown.enter.prevent="createUploadTag"
              />
              <button
                type="button"
                class="soft-button shrink-0 px-3"
                :disabled="!canCreateTag || creatingTag || !newTagName.trim()"
                @click="createUploadTag"
              >
                <i :class="creatingTag ? 'ri-loader-4-line animate-spin' : 'ri-add-line'" aria-hidden="true"></i>
              </button>
            </div>
            <p id="upload-new-tag-hint" class="field-hint" :class="{ 'text-red-600 dark:text-red-400': newTagError }">
              {{ newTagError || (canCreateTag ? '最多 10 个字符；已存在会直接选中。' : '当前账号没有 tag:create 权限。') }}
            </p>

            <p v-if="uploadTags.length" class="field-hint">上传完成后自动打到新图片上。</p>
          </div>
        </div>

        <div class="mt-4 flex items-baseline justify-between gap-2 border-t border-slate-200 pt-3 dark:border-white/10">
          <label for="upload-quality" class="text-[13px] font-medium text-slate-700 dark:text-slate-300">上传质量</label>
          <span class="text-[13px] font-semibold text-primary dark:text-primary-light">{{ uploadQuality === QUALITY_MAX ? '原图' : uploadQuality + '%' }}</span>
        </div>

        <input
          id="upload-quality" type="range" min="40" max="100" step="5"
          v-model.number="uploadQuality"
          class="upload-quality-range mt-3 w-full"
        />
        <div class="mt-1 flex justify-between text-[10px] text-slate-400 dark:text-slate-500">
          <span>更小</span><span>原图</span>
        </div>

        <div class="mt-3 flex flex-wrap items-center gap-1.5">
          <button
            v-for="preset in QUALITY_PRESETS" :key="preset.value"
            type="button"
            class="upload-quality-preset"
            :class="{ 'is-active': uploadQuality === preset.value }"
            @click="uploadQuality = preset.value"
          >{{ preset.label }}{{ preset.value === QUALITY_MAX ? '' : ' · ' + preset.value + '%' }}</button>
        </div>

        <p class="mt-3 text-[11px] leading-5 text-slate-500 dark:text-slate-400">
          在浏览器内压缩后再上传，画质越低体积越小。小于 80KB 的图与动图（GIF）、矢量图（SVG）不做压缩，避免反而变大或丢失动效。
        </p>
      </div>

      <template #footer>
        <button type="button" class="soft-button" @click="uploadQuality = QUALITY_MAX">恢复原图</button>
        <!-- 走弹窗的关闭流程，而不是直接改开关：直接改会让内容在淡出动画前消失 -->
        <button type="button" class="primary-button" @click="uploadSettingsRef?.requestClose()">完成</button>
      </template>
    </DialogShell>
  </div>
</template>

<script setup>
import { escapeHtml, safeResourceUrl } from '@/utils/html.js'

import { boundedPage, RECENT_IMAGE_LIMIT } from '@/utils/renderBounds.js';
import { uploadLimits, allowedRasterTypes } from '@/utils/uploadValidation.js';
import { readApiResponse } from '@/utils/apiFeedback.js';
import errorImg from '@/assets/images/error.webp';

import { ref, computed, onMounted, nextTick, onBeforeUnmount, watch } from 'vue'
import { useRoute } from 'vue-router';
import FolderDialog from '@/components/FolderDialog.vue';
import { useFolders } from '@/composables/useFolders.js';
import { folderFilter, folderAccount, folderLabel } from '@/utils/folders.js';
import DirectUploadTaskStatus from '@/components/DirectUploadTaskStatus.vue';
import { createHomeDirectTasks } from '@/utils/homeDirectTasks.js';
import RecentImageCard from '@/components/home/RecentImageCard.vue';
import DialogShell from '@/components/DialogShell.vue';
import { createHomeUploadActions } from '@/utils/homeUploadActions.js';
import { createHomeDialogs } from '@/utils/homeDialogs.js';
import Message from '@/utils/message.js';
import { createDialogScope } from '@/utils/dialogScope.js';
const { Dialog: PopupModal, dispose: disposeViewDialogs } = createDialogScope();

// 复制格式是「最近上传」这个区块级的选择：改一次，下面所有条目都跟着变。
// 加新格式只改这里；条目组件通过 copyFormat prop 接收，不再各自持有状态。
const COPY_FORMATS = [
  { value: 'url', short: 'URL', label: 'URL' },
  { value: 'html', short: 'HTML', label: 'HTML' },
  { value: 'markdown', short: 'MD', label: 'Markdown' },
  { value: 'bbcode', short: 'BB', label: 'BBCode' },
]
const copyFormat = ref('url')

// ===== 上传质量 =====
// 后端只做存储、没有图像编码能力，所以质量档位在浏览器端用 canvas 重编码实现。
// 100 表示原图（不重编码）；40 以下压缩痕迹明显，所以下限设在 40。
const QUALITY_MAX = 100
const QUALITY_PRESETS = [
  { label: '小体积', value: 60 },
  { label: '均衡', value: 80 },
  { label: '高清', value: 92 },
  { label: '原图', value: QUALITY_MAX },
]
const STORAGE_KEY = 'upload-quality'

// 记住上次选择，下次上传不用重新调。
// 默认 85 而不是 90：实测 90 以上重编码后常比原图还大，会被保护逻辑退回原图，
// 用户会觉得「调了没反应」。85 能稳定拿到 20% 左右的体积收益。
const uploadQuality = ref((() => {
  const saved = Number(localStorage.getItem(STORAGE_KEY))
  return Number.isFinite(saved) && saved >= 40 && saved <= QUALITY_MAX ? saved : 85
})())
watch(uploadQuality, (value) => { try { localStorage.setItem(STORAGE_KEY, String(value)) } catch { /* 隐私模式下忽略 */ } })

// 弹窗由 DialogShell 负责遮罩、焦点与 Esc 关闭，这里只管开关
const uploadSettingsOpen = ref(false)
// 给「完成」按钮用的弹窗实例引用，关闭要走它自己的流程才能播完动画
const uploadSettingsRef = ref(null)

// ===== 上传标签 =====
// 后端上传接口不接收 tag 参数，只能在上传成功后逐张调 /api/images/tags 追加，
// 所以这里选中的标签是一个「待打标」清单，上传完成时生效。
const uploadTags = ref([])
const availableTags = ref([])
const tagsLoading = ref(false)
const newTagName = ref('')
const newTagError = ref('')
const creatingTag = ref(false)

// 新建标签需要 tag:create 权限。管理员(role 1)默认拥有，
// 其他角色看 permission.codes；两者都没有就只让选不让建。
//
// 用 ref 而不是 computed 直接读 localStorage：localStorage 不是响应式源，
// computed 只会在首次求值时算一次，那一刻路由守卫可能还没把 userInfo 写进去，
// 结果就会被永久缓存成错误的值。
const canCreateTag = ref(false)
const refreshTagPermission = () => {
  try {
    const user = JSON.parse(localStorage.getItem('userInfo') || '{}')
    canCreateTag.value = Number(user.role) === 1
      || (Array.isArray(user?.permission?.codes) && user.permission.codes.includes('tag:create'))
  } catch { canCreateTag.value = false }
}
onMounted(refreshTagPermission)

const loadAvailableTags = async () => {
  if (tagsLoading.value) return
  tagsLoading.value = true
  try {
    const res = await fetch(`${API_BASE_URL}/api/tags`, {
      headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' },
    })
    const body = await res.json().catch(() => ({}))
    const list = body?.data?.list || body?.data || []
    availableTags.value = Array.isArray(list) ? list.filter(t => t && t.name) : []
  } catch {
    availableTags.value = []
  } finally {
    tagsLoading.value = false
  }
}

// 打开设置面板时才去拉标签，避免首页初次加载多一次请求。
// 同时重读一次权限：登录态可能在这之后才由路由守卫落定。
watch(uploadSettingsOpen, (open) => {
  if (!open) return
  refreshTagPermission()
  if (!availableTags.value.length) loadAvailableTags()
})

const isTagPicked = (tag) => uploadTags.value.some(t => String(t.id) === String(tag.id))

const addUploadTag = (tag) => {
  if (!tag || isTagPicked(tag)) return
  uploadTags.value.push(tag)
}

const removeUploadTag = (tag) => {
  const idx = uploadTags.value.findIndex(t => String(t.id) === String(tag.id))
  if (idx >= 0) uploadTags.value.splice(idx, 1)
}

// 下拉选择：选完立刻回到占位项，这样同一个标签移除后还能再选一次
const onPickTag = (event) => {
  const id = event.target.value
  event.target.value = ''
  const tag = availableTags.value.find(t => String(t.id) === String(id))
  if (tag) addUploadTag(tag)
}

// 输入新建：成功后直接选中，并回填到可选列表，避免下次还要重新拉接口
const createUploadTag = async () => {
  newTagError.value = ''
  const name = newTagName.value.trim()
  if (!name) { newTagError.value = '请输入标签名称'; return }
  if (name.length > 10) { newTagError.value = '标签名称不能超过 10 个字符'; return }

  // 已存在就直接选中，不重复创建
  const existing = availableTags.value.find(t => t.name === name)
  if (existing) {
    addUploadTag(existing)
    newTagName.value = ''
    return
  }

  creatingTag.value = true
  try {
    const res = await fetch(`${API_BASE_URL}/api/tags`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' },
      body: JSON.stringify({ name }),
    })
    const body = await res.json().catch(() => ({}))
    if (res.status === 409) { newTagError.value = '标签已存在，请从上方下拉中选择'; return }
    if (!res.ok || body.code !== 200) throw new Error(body.message || '创建标签失败')
    const tag = body.data
    if (!tag?.id) throw new Error('服务端未返回标签信息')
    availableTags.value = [...availableTags.value, tag]
    addUploadTag(tag)
    newTagName.value = ''
  } catch (error) {
    newTagError.value = error.message || '创建标签失败'
  } finally {
    creatingTag.value = false
  }
}

// 给刚上传成功的图片补打标签。逐张逐标签串行，避免并发打爆接口；
// 失败只记日志不影响上传结果本身。
const applyTagsToImages = async (images) => {
  if (!uploadTags.value.length || !images.length) return
  const tagIds = uploadTags.value.map(t => t.id)
  for (const image of images) {
    if (!image?.id) continue
    for (const tagId of tagIds) {
      try {
        await fetch(`${API_BASE_URL}/api/images/tags`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' },
          body: JSON.stringify({ id: image.id, tag: String(tagId) }),
        })
      } catch { /* 单个标签失败不阻断后续 */ }
    }
  }
}

// 多存储同步时不显示下拉，用一句话说明去向
const syncTargetLabel = computed(() =>
  syncBuckets.value.length ? `本机 + ${syncBuckets.value.length} 个同步目标` : '本机（未配置远程同步）'
)

// 上传卡上只读展示的当前值（真正的切换在设置弹窗里）
const currentBucketLabel = computed(() => {
  if (multiStorageSync.value) return syncTargetLabel.value
  const hit = presetBuckets.value.find(b => b.id == selectedBucket.value)
  return hit?.name || '未选择存储'
})

const currentFolderLabel = computed(() => {
  const id = String(selectedFolder.value ?? '0')
  if (id === '0') return '未分类'
  const hit = folders.value.find(f => String(f.id) === id)
  return hit ? folderLabel(hit) : '未分类'
})
import Loading from '@/utils/loading.js';
import { MAX_RECENT_TASKS, mergeImages } from '@/utils/directUpload.js'
import { getStorageDisplayName, getStorageStatuses, getStorageStatusMeta, hasActiveStorageSync } from '@/utils/storageStatus.js'

// ====================== 常量定义 ======================
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';
const authHeaders = () => ({});
const isGuest = computed(() => Number(JSON.parse(localStorage.getItem('userInfo') || '{}').role) === 2);


// ====================== 响应式数据 ======================
// 上传相关
const isDragOver = ref(false);
const legacyUploading = ref(false);
const directTasks = ref([]);
const isUploading = computed(() => legacyUploading.value || directTasks.value.some(task => task.busy));
const uploadConfig = ref({});
const clientUploadLimits = computed(() => uploadLimits(uploadConfig.value));
const uploadAccept = computed(() => allowedRasterTypes(uploadConfig.value).join(','));
const uploadingCount = ref(0);
const uploadProgress = ref(0);
const recentImages = ref([]);
const fileInput = ref(null);

// Folder state belongs to this mounted account only; never persist upload destinations.
const route = useRoute();
const { account: folderAccountKey, authenticated: foldersAuthenticated, folders, loading: foldersLoading,
  loaded: foldersLoaded, error: foldersError, atLimit: foldersAtLimit, reload: reloadFolders } = useFolders(API_BASE_URL);
const selectedFolder = ref('0');
const createFolderOpen = ref(false);
let initialFolderPending = true;
const uploadFolderName = computed(() => folders.value.find(folder => String(folder.id) === selectedFolder.value)?.name || '未分类');
watch(folderAccountKey, () => { selectedFolder.value = '0'; createFolderOpen.value = false; initialFolderPending = false; });
watch([folders, foldersLoaded], () => {
  if (!foldersLoaded.value) return;
  const requested = initialFolderPending ? folderFilter(route.query.folder_id, '0') : selectedFolder.value;
  initialFolderPending = false;
  selectedFolder.value = folders.value.some(folder => !folder.deleting && String(folder.id) === requested) ? requested : '0';
});
// Resolve at upload start, also rejecting stale IDs if another tab changes account.
const getUploadFolderId = () => foldersAuthenticated.value && folderAccount() === folderAccountKey.value && folders.value.some(folder => !folder.deleting && String(folder.id) === selectedFolder.value) ? Number(selectedFolder.value) : 0;
async function onFolderCreated({ folder }) {
  createFolderOpen.value = false;
  await reloadFolders();
  if (!disposed && folders.value.some(item => !item.deleting && item.id === folder?.id)) selectedFolder.value = String(folder.id);
}

// 存储相关
const multiStorageSync = ref(false);
const storageConfigLoaded = ref(false);
const presetBuckets = ref([
  { id: "1", name: '默认存储', type: "default" },
]);
const selectedBucket = ref("1");
const syncBuckets = ref([]);

// 预览相关
const activeCopyMenu = ref(null);
let previewCopyMenu = false;

let syncPollTimer = null;
let disposed = false;
const { directRefreshing, directTasksError, resumeTask, deletedImageIds, currentUser, realUser,
  directEnabled, uploadSummary, directTaskForImage, unlinkedDirectTasks, usableTasks,
  refreshDirectTasks, startDirectFiles, retryDirectTask, cancelDirectTask, confirmDirectTask,
  reselectDirectFile, handleResumeFile, disposeDirectTasks } = createHomeDirectTasks({
  API_BASE_URL, directTasks, recentImages, legacyUploading, uploadProgress, uploadingCount,
  multiStorageSync, storageConfigLoaded, presetBuckets, selectedBucket, getUploadFolderId, fileInput,
  validateFiles: files => validateFiles(files), loadRecentImages: () => loadRecentImages(),
  isDisposed: () => disposed,
  onSingleUpload: showUploadResult,
});

// ====================== 工具函数 ======================
/**
 * 获取完整的图片URL
 */
const getFullUrl = (path) => {
  if (!path) return '';
  if (typeof window === 'undefined') return path;
  
  // 处理绝对路径和相对路径
  if (path.startsWith('http')) return path;
  return `${window.location.origin}${path}`;
};

/**
 * 格式化文件大小
 */
const formatFileSize = (bytes) => {
  if (!bytes || bytes < 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

/**
 * 格式化日期
 */
const formatDate = (dateString) => {
  if (!dateString) return '';
  try {
    const date = new Date(dateString);
    return date.toLocaleString('zh-CN');
  } catch (error) {
    console.error('日期格式化失败:', error);
    return dateString;
  }
};

/**
 * 获取复制类型文本
 */
const getTypeText = (type) => {
  const typeMap = {
    url: 'URL',
    html: 'HTML',
    markdown: 'Markdown',
    bbcode: 'BBCode'
  };
  return typeMap[type] || '';
};

/**
 * 生成HTML代码
 */
const getHtmlCode = (image) => {
  const url = getFullUrl(image.url);
  const alt = image.filename || '图片预览';
  return `<img loading="lazy" decoding="async" src="${escapeHtml(safeResourceUrl(url))}" alt="${escapeHtml(alt)}"/>`;
};

/**
 * 生成Markdown代码
 */
const getMarkdownCode = (image) => {
  const url = getFullUrl(image.url);
  const filename = image.filename || '图片';
  return `![${filename}](${url})`;
};

/**
 * 生成BBCode代码（论坛常用，与图库批量复制保持同一写法）
 */
const getBBCode = (image) => {
  const url = getFullUrl(image.url);
  return `[img]${url}[/img]`;
};

// ====================== API 请求函数 ======================
/**
 * 获取上传配置
 */
const getUploadConfig = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/uploadConfig`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest'
      }
    });
    
    const result = await readApiResponse(response, '请求失败');
    if (response.ok && result.code === 200) {
      uploadConfig.value = result.data || {};
      presetBuckets.value = Array.isArray(result.data?.buckets) ? result.data.buckets : [];
      multiStorageSync.value = result.data?.multi_storage_sync === true;
      const configuredSyncBuckets = result.data?.sync_buckets ?? result.data?.buckets ?? [];
      // 本机作为固定的第一落点，不重复列入远程同步目标。
      syncBuckets.value = Array.isArray(configuredSyncBuckets)
        ? configuredSyncBuckets.filter(bucket => bucket?.type !== 'default')
        : [];

      if (!multiStorageSync.value) {
        const defaultBucket = result.data?.default_bucket || '1';
        const storedBucketId = localStorage.getItem('currentBucket');
        const storedBucket = storedBucketId == null
          ? null
          : presetBuckets.value.find(bucket => bucket.id === Number(storedBucketId));
        selectedBucket.value = storedBucket?.id || defaultBucket;
      }
      storageConfigLoaded.value = true;
      scheduleSyncRefresh();
    } else {
      throw new Error(result.message || '获取上传配置失败');
    }
  } catch (error) {
    console.error('获取上传配置失败:', error);
    Message.error(error.message || '获取上传配置失败');
  }
};

/**
 * 加载最近上传的图片
 */
const loadRecentImages = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/images?scope=mine&limit=12`, {
      headers: {
        'X-Requested-With': 'XMLHttpRequest'
      }
    });
    
    const result = await readApiResponse(response, '加载图片失败');
    if (disposed) return;
    recentImages.value = boundedPage(mergeImages(Array.isArray(result.data?.images) ? result.data.images : [], usableTasks()), RECENT_IMAGE_LIMIT);
    scheduleSyncRefresh();
    if (foldersAuthenticated.value) reloadFolders();
  } catch (error) {
    if (disposed) return;
    console.error('加载图片失败:', error);
    Message.error(`加载图片失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    });
  }
};

const scheduleSyncRefresh = () => {
  if (disposed || !multiStorageSync.value) {
    if (syncPollTimer) clearTimeout(syncPollTimer);
    syncPollTimer = null;
    return;
  }
  const hasActiveSync = recentImages.value.some(hasActiveStorageSync);
  if (!hasActiveSync) {
    if (syncPollTimer) clearTimeout(syncPollTimer);
    syncPollTimer = null;
    return;
  }
  if (syncPollTimer) return;

  syncPollTimer = setTimeout(async () => {
    syncPollTimer = null;
    await loadRecentImages();
  }, 2500);
};

/**
 * 删除单张图片
 */
const deleteAsync = async (imageId) => { 
  let loadingInstance;
  try {
    loadingInstance = Loading.show({
      text: '删除中...',
      color: '#ff4d4f',
      mask: true
    });
    
    const response = await fetch(`${API_BASE_URL}/api/images/${imageId}`, {
      method: 'DELETE',
      headers: {
        'X-Requested-With': 'XMLHttpRequest',
        'Content-Type': 'application/json'
      }
    });
    
    const result = await readApiResponse(response, '请求失败');
    if (response.ok && result.code === 200) {
      deletedImageIds.add(String(imageId));
      while (deletedImageIds.size > 64) deletedImageIds.delete(deletedImageIds.values().next().value);
      recentImages.value = recentImages.value.filter(image => String(image.id) !== String(imageId));
      Message.success('图片删除成功', {
        duration: 1500,
        position: 'top-right'
      });
      
      closeDeletedPreview(imageId);
      previewCopyMenu = false;
      activeCopyMenu.value = null;
      await loadRecentImages();
    } else {
      throw new Error(result.message || '删除失败；部分存储源可能已删除，请重试');
    }
  } catch (error) {
    console.error('删除图片错误:', error);
    Message.error(`删除失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    });
  } finally {
    if (loadingInstance) await loadingInstance.hide();
  }
};

// ====================== 事件处理函数 ======================
/**
 * 拖拽相关处理
 */
const { handleDragOver, handleDragEnter, handleDragLeave, handleDrop, triggerFileInput,
  handleFileSelect, handlePaste, validateFiles, uploadFiles, cancelLegacyUpload } = createHomeUploadActions({
  API_BASE_URL, isDragOver, isUploading, fileInput, resumeTask, handleResumeFile, clientUploadLimits,
  uploadConfig, storageConfigLoaded, directEnabled, startDirectFiles, legacyUploading, uploadingCount,
  uploadProgress, getUploadFolderId, multiStorageSync, selectedBucket, recentImages, loadRecentImages,
  uploadQuality,
  isDisposed: () => disposed,
  onSingleUpload: showUploadResult,
  onUploaded: applyTagsToImages,
});

/**
 * 图片相关操作
 */
const handleImageLoad = (e) => {
  e.target.classList.remove('opacity-0');
  const loadingEl = e.target.parentElement.querySelector('.loading');
  if (loadingEl) loadingEl.classList.add('hidden');
};

const handleImageError = (e) => {
  if (e.target.dataset.fallbackApplied) return;
  e.target.dataset.fallbackApplied = 'true';
  e.target.src = errorImg;
  const loadingEl = e.target.parentElement.querySelector('.loading');
  if (loadingEl) loadingEl.classList.add('hidden');
};

const copyImageLink = async (image, type) => {
  if (!image) return;
  
  const fullUrl = getFullUrl(image.url);
  let copyText = '';
  
  switch (type) {
    case 'url':
      copyText = fullUrl;
      break;
    case 'html':
      copyText = `<img loading="lazy" decoding="async" src="${fullUrl}" alt="${escapeHtml(image.filename)}" width="${image.width || ''}" height="${image.height || ''}">`;
      break;
    case 'markdown':
      copyText = `![${escapeHtml(image.filename)}](${fullUrl})`;
      break;
    case 'bbcode':
      copyText = `[img]${fullUrl}[/img]`;
      break;
    default:
      copyText = fullUrl;
  }
  
  try {
    await navigator.clipboard.writeText(copyText);
    Message.success(`已复制${getTypeText(type)}格式`, {
      duration: 1500,
      position: 'top-right'
    });
  } catch (error) {
    // 降级处理
    const textArea = document.createElement('textarea');
    textArea.value = copyText;
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand('copy');
    document.body.removeChild(textArea);
    Message.success(`已复制${getTypeText(type)}格式`, {
      duration: 1500,
      position: 'top-right'
    });
  } finally {
    // 关闭所有下拉菜单
    nextTick(() => {
      previewCopyMenu = false;
      activeCopyMenu.value = null;
      
      // 关闭预览弹窗内的复制下拉框
      const dropdown = document.getElementById('previewCopyDropdown');
      const icon = document.getElementById('copyMenuIcon');
      if (dropdown && icon) {
        dropdown.classList.add('hidden', 'opacity-0', 'translate-y-[-5px]');
        dropdown.classList.remove('block', 'opacity-100', 'translate-y-0');
        icon.classList.remove('rotate-180');
      }
    });
  }
};

/**
 * 单存储模式下记住用户选择的存储桶。
 */
const handleBucketChange = () => {
  const bucketId = selectedBucket.value;
  if (!bucketId) return;
  localStorage.setItem('currentBucket', bucketId);
};

const deleteImage = (imageId) => {
  const modal = new PopupModal({
    title: '确认删除',
    content: `
      <div class="flex gap-3">
        <i class="fa fa-exclamation-triangle text-warning text-xl mt-1"></i>
        <div>
          <p>确定要删除这张图片吗？</p>
          <p class="mt-1 text-secondary text-sm">删除后无法恢复，请谨慎操作</p>
        </div>
      </div>
    `,
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close()
      },
      {
        text: '确认删除',
        type: 'danger',
        callback: async (modal) => {
          modal.close();
          await deleteAsync(imageId);
        }
      }
    ],
    maskClose: true
  });
  modal.open();
};

const downloadImage = (image) => {
  if (!image || !image.url) {
    Message.error('图片信息不完整，无法下载', {
      duration: 2000,
      position: 'top-right'
    });
    return;
  }
  
  const fullUrl = getFullUrl(image.url);
  const link = document.createElement('a');
  const downloadUrl = new URL(fullUrl, window.location.origin);
  downloadUrl.searchParams.set('download', '1');
  link.href = downloadUrl.toString();
  link.download = image.filename || `image-${Date.now()}.png`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  
  Message.info('开始下载图片', {
    duration: 1500,
    position: 'top-right'
  });
  
  previewCopyMenu = false;
  activeCopyMenu.value = null;
};

/**
 * 图片预览功能
 */
/**
 * 从URL上传图片
 */
const { disposeDialogs, previewImage, uploadbyurlmodal, cleanupPreview, closeDeletedPreview } = createHomeDialogs({
  storageConfigLoaded, getUploadFolderId, folders, foldersAuthenticated, presetBuckets, selectedBucket, multiStorageSync, getFullUrl,
  errorImg, formatFileSize, formatDate, copyImageLink, downloadImage, deleteImage, loadRecentImages,
  onSingleUpload: showUploadResult, isDisposed: () => disposed,
});

function showUploadResult(image) {
  if (disposed || !image?.id || !image.url) return;
  // Resolve the uploaded ID, never whichever image happens to be first in the recent list.
  const recent = recentImages.value.find(item => String(item.id) === String(image.id));
  previewImage({
    ...image, ...recent,
    thumbnail: recent?.thumbnail || image.thumbnail || image.thumbnail_url || '',
    bucket_id: recent?.bucket_id ?? image.bucket_id ?? selectedBucket.value,
    uploader_role: recent?.uploader_role ?? image.uploader_role ?? currentUser().role,
    tags: recent?.tags ?? image.tags ?? [],
  }, { title: '上传成功', showLink: true });
}

/**
 * 全局点击处理（关闭下拉菜单）
 */
const handleGlobalClick = (e) => {
  if (activeCopyMenu.value !== null) {
    const cardCopyMenus = document.querySelectorAll('.recent-item .relative.z-50');
    let isClickInside = false;
    
    cardCopyMenus.forEach(menu => {
      if (menu.contains(e.target)) {
        isClickInside = true;
      }
    });
    
    if (!isClickInside) {
      activeCopyMenu.value = null;
    }
  }
};

// ====================== 生命周期 ======================
onMounted(() => {
  // 初始化数据
  getUploadConfig();
  loadRecentImages();
  refreshDirectTasks();
  
  // 注册全局事件
  document.addEventListener('paste', handlePaste);
  document.addEventListener('click', handleGlobalClick);
});

onBeforeUnmount(() => {
  disposed = true;
  disposeDirectTasks();
  cancelLegacyUpload();
  if (syncPollTimer) clearTimeout(syncPollTimer);
  
  // 移除事件监听
  document.removeEventListener('paste', handlePaste);
  document.removeEventListener('click', handleGlobalClick);
  
  // 清理预览资源
  cleanupPreview();
  disposeDialogs();
  disposeViewDialogs();
  
  // 关闭所有消息提示
  if (window.Message) {
    Message.closeAll();
  }
});
</script>
