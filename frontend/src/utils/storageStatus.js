const STATUS_META = {
  pending: {
    label: '等待同步',
    icon: 'ri-time-line',
    badgeClass: 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-300',
  },
  uploading: {
    label: '同步中',
    icon: 'ri-loader-4-line animate-spin',
    badgeClass: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-500/20 dark:bg-blue-500/10 dark:text-blue-300',
  },
  success: {
    label: '已同步',
    icon: 'ri-checkbox-circle-line',
    badgeClass: 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/20 dark:bg-emerald-500/10 dark:text-emerald-300',
  },
  failed: {
    label: '同步失败',
    icon: 'ri-error-warning-line',
    badgeClass: 'border-red-200 bg-red-50 text-red-700 dark:border-red-500/20 dark:bg-red-500/10 dark:text-red-300',
  },
  unknown: {
    label: '状态未知',
    icon: 'ri-question-line',
    badgeClass: 'border-slate-200 bg-slate-50 text-slate-600 dark:border-white/10 dark:bg-white/5 dark:text-slate-300',
  },
}

const OVERALL_META = {
  local: {
    label: '仅保存在本机',
    icon: 'ri-computer-line',
    badgeClass: 'border-slate-200 bg-slate-50 text-slate-600 dark:border-white/10 dark:bg-white/5 dark:text-slate-300',
  },
  pending: STATUS_META.pending,
  uploading: STATUS_META.uploading,
  success: STATUS_META.success,
  partial: {
    label: '部分同步失败',
    icon: 'ri-alert-line',
    badgeClass: 'border-orange-200 bg-orange-50 text-orange-700 dark:border-orange-500/20 dark:bg-orange-500/10 dark:text-orange-300',
  },
  failed: STATUS_META.failed,
}

export const getStorageStatuses = (image) => {
  if (!Array.isArray(image?.storage_statuses)) return []
  // 本机落盘在界面中单独展示，这里只返回需要跟踪的后台同步目标。
  return image.storage_statuses.filter(item => item && item.bucket_type !== 'default')
}

export const getStorageStatusMeta = (status) => {
  return STATUS_META[status] || STATUS_META.unknown
}

export const getStorageDisplayName = (storage) => {
  if (storage?.bucket_name) return storage.bucket_name
  if (storage?.bucket_type) return storage.bucket_type.toUpperCase()
  return storage?.bucket_id ? `存储源 ${storage.bucket_id}` : '未知存储源'
}

export const hasActiveStorageSync = (image) => {
  return getStorageStatuses(image).some(item => item.status === 'pending' || item.status === 'uploading')
}

export const getStorageSyncSummary = (image) => {
  const statuses = getStorageStatuses(image)
  if (statuses.length === 0) {
    return { ...OVERALL_META.local, status: 'local', total: 0, success: 0, failed: 0, active: 0 }
  }

  const success = statuses.filter(item => item.status === 'success').length
  const failed = statuses.filter(item => item.status === 'failed').length
  const uploading = statuses.filter(item => item.status === 'uploading').length
  const pending = statuses.filter(item => item.status === 'pending').length
  const active = uploading + pending

  let status = 'pending'
  if (success === statuses.length) status = 'success'
  else if (failed === statuses.length) status = 'failed'
  else if (failed > 0) status = 'partial'
  else if (uploading > 0) status = 'uploading'

  const meta = OVERALL_META[status]
  const progressLabel = status === 'success'
    ? `已同步 ${success}/${statuses.length}`
    : status === 'uploading' || status === 'pending'
      ? `同步中 ${success}/${statuses.length}`
      : `${meta.label} ${success}/${statuses.length}`

  return {
    ...meta,
    label: progressLabel,
    status,
    total: statuses.length,
    success,
    failed,
    active,
  }
}

const escapeHtml = (value) => String(value ?? '')
  .replaceAll('&', '&amp;')
  .replaceAll('<', '&lt;')
  .replaceAll('>', '&gt;')
  .replaceAll('"', '&quot;')
  .replaceAll("'", '&#039;')

export const renderStorageStatusesHtml = (image, emptyText = '未配置远程同步源') => {
  const statuses = getStorageStatuses(image)
  if (statuses.length === 0) {
    return `
      <div class="rounded-xl border border-dashed border-slate-200 px-3 py-2 text-xs text-slate-500 dark:border-white/10 dark:text-slate-400">
        ${escapeHtml(emptyText)}
      </div>
    `
  }

  return statuses.map((storage) => {
    const meta = getStorageStatusMeta(storage.status)
    const errorHtml = storage.status === 'failed' && storage.error
      ? `<p class="mt-1 break-words text-[11px] leading-4 text-red-600 dark:text-red-300" title="${escapeHtml(storage.error)}">${escapeHtml(storage.error)}</p>`
      : ''

    return `
      <div class="rounded-xl border border-slate-200/80 bg-slate-50 px-3 py-2 dark:border-white/10 dark:bg-slate-900">
        <div class="flex items-center justify-between gap-2">
          <span class="min-w-0 truncate text-xs font-medium text-slate-700 dark:text-slate-200" title="${escapeHtml(getStorageDisplayName(storage))}">
            ${escapeHtml(getStorageDisplayName(storage))}
          </span>
          <span class="inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[11px] ${meta.badgeClass}">
            <i class="${meta.icon}"></i>${meta.label}
          </span>
        </div>
        ${errorHtml}
      </div>
    `
  }).join('')
}
