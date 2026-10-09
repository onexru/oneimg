import { readApiResponse } from './apiFeedback.js';
import { createDialogScope } from './dialogScope.js';
import message from './message.js';
import { escapeHtml } from './html.js';
const SuperAdminID = 1;
const RoleAdmin = 1;
const RoleUser = 3;
export function createUserManagementActions({ closeDropdown, fetchUsers }) {
const { Dialog: PopupModal, dispose: disposeDialogs } = createDialogScope();
function openCreateModal() {
  const modal = new PopupModal({
    title: '新增用户',
    type: 'form',
    formFields: [
      {
        name: 'username',
        label: '用户名',
        type: 'text',
        placeholder: '请输入用户名（3-50字符）',
        required: true,
      },
      {
        name: 'password',
        label: '密码',
        type: 'password',
        placeholder: '请输入密码（6-100字符）',
        required: true,
      },
      {
        name: 'role',
        label: '角色',
        type: 'select',
        options: [
          { label: '请选择角色', value: '', disabled: true },
          { label: '管理员', value: '1' },
          { label: '普通用户', value: '3' }, // 这里原本是对的
        ],
        required: true,
      },
    ],
    formSubmit: async (modal, formData) => {
      if (!formData.username || formData.username.length < 3) {
        message.warning('用户名至少3个字符')
        return
      }
      if (!formData.password || formData.password.length < 6) {
        message.warning('密码至少6个字符')
        return
      }

      try {
        const res = await fetch('/api/users/Add', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'X-Requested-With': 'XMLHttpRequest',
          },
          body: JSON.stringify({
            username: formData.username,
            password: formData.password,
            role: parseInt(formData.role) || RoleUser,
          }),
        })
        const result = await readApiResponse(res, '请求失败')

        if (res.ok && result.code === 200) {
          message.success('用户创建成功')
          modal.close()
          fetchUsers()
        } else {
          message.error(result.message || '创建失败')
        }
      } catch (err) {
        console.error('创建用户失败:', err)
        message.error(err.message || '网络错误，请重试')
      }
    },
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '创建',
        type: 'primary',
        callback: (modal) => {
          modal.content.querySelector('form').dispatchEvent(
            new Event('submit', { bubbles: true })
          )
        },
      },
    ],
  })
  modal.open()
}

function openDeleteModal(user) {
  closeDropdown()
  const modal = new PopupModal({
    title: '确认删除用户',
    content: `
      <div class="flex items-start gap-3">
        <div class="shrink-0 w-10 h-10 flex items-center justify-center rounded-full bg-red-100 dark:bg-red-900/30">
          <i class="ri-error-warning-fill text-red-500 text-xl"></i>
        </div>
        <div>
          <p class="text-sm text-slate-700 dark:text-slate-200">
            你确定要删除用户 <strong>${escapeHtml(user.username)}</strong> 吗？
          </p>
          <p class="mt-1.5 text-xs text-slate-500 dark:text-slate-400">
            此操作无法撤销，该用户的所有关联数据将会丢失。
          </p>
        </div>
      </div>
    `,
    type: 'confirm',
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '确认删除',
        type: 'danger',
        callback: async (modal) => {
          modal.close()
          try {
            const res = await fetch(`/api/users/${user.id}`, {
              method: 'DELETE',
              headers: {
                'X-Requested-With': 'XMLHttpRequest',
              },
            })
            const result = await readApiResponse(res, '请求失败')

            if (res.ok && result.code === 200) {
              message.success('用户已删除')
              fetchUsers()
            } else {
              message.error(result.message || '删除失败')
            }
          } catch (err) {
            console.error('删除用户失败:', err)
            message.error(err.message || '网络错误，请重试')
          }
        },
      },
    ],
  })
  modal.open()
}



function openRoleModal(user) {
  closeDropdown()
  const currentRole = String(user.role)

  const modal = new PopupModal({
    title: '修改用户角色',
    content: `
      <div class="py-1">
        <p class="text-sm text-slate-600 dark:text-slate-300 mb-1">
          修改用户 <strong class="text-slate-900 dark:text-white">${escapeHtml(user.username)}</strong> 的角色
        </p>
        <div class="mt-3">
          <label class="field-label block mb-1.5">选择角色</label>
          <select
            name="newRole"
            class="input-modern w-full py-2.5"
          >
            <option value="1" ${currentRole === '1' ? 'selected' : ''}>管理员</option>
            <!-- 修复点：修改普通用户对应的 value 为 3，防止 400 校验错误 -->
            <option value="3" ${currentRole === '3' ? 'selected' : ''}>普通用户</option>
          </select>
        </div>
      </div>
    `,
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '重置密码',
        type: 'default',
        callback: () => {
          handleResetPassword(user)
        },
      },
      {
        text: '保存',
        type: 'primary',
        callback: async (modal) => {
          const newRoleSelect = modal.content.querySelector('select[name="newRole"]')
          // 修复点：默认缺省也应降级为 3
          const newRole = parseInt(newRoleSelect?.value || '3')

          try {
            const res = await fetch('/api/users/updateRole', {
              method: 'POST',
              headers: {
                'Content-Type': 'application/json',
                'X-Requested-With': 'XMLHttpRequest',
              },
              body: JSON.stringify({ id: user.id, role: newRole }),
            })
            const result = await readApiResponse(res, '请求失败')

            if (res.ok && result.code === 200) {
              message.success('角色更新成功')
              modal.close()
              fetchUsers()
            } else {
              message.error(result.message || '更新失败')
            }
          } catch (err) {
            console.error('更新角色失败:', err)
            message.error(err.message || '网络错误，请重试')
          }
        },
      },
    ],
  })
  modal.open()
}

async function handleResetPassword(user) {
    const modal = new PopupModal({
    title: '重置用户密码',
    content: `
      <div class="flex items-start gap-3">
        <div class="shrink-0 w-10 h-10 flex items-center justify-center rounded-full bg-red-100 dark:bg-red-900/30">
          <i class="ri-error-warning-fill text-red-500 text-xl"></i>
        </div>
        <div>
          <p class="text-sm text-slate-700 dark:text-slate-200">
            你确定要重置用户 <strong>${escapeHtml(user.username)}</strong> 的密码吗？
          </p>
          <p class="mt-1.5 text-xs text-slate-500 dark:text-slate-400">
            此操作无法撤销，请谨慎操作。
          </p>
        </div>
      </div>
    `,
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '确定',
        type: 'primary',
        callback: async () => {
          await resetPassword(user)
        },
      }
    ]
    })
    modal.open()
}

const resetPassword = async (user) => {
  closeDropdown()
  try {
    const res = await fetch(`/api/users/resetPassword/${user.id}`, {
      method: 'POST',
      headers: {
        'X-Requested-With': 'XMLHttpRequest',
      },
    })
    const result = await readApiResponse(res, '请求失败')

    if (res.ok && result.code === 200 && result.data?.new_password) {
      const newPassword = result.data.new_password
      const modal = new PopupModal({
        title: '密码重置成功',
        content: `
          <div class="py-1">
            <p class="text-sm text-slate-600 dark:text-slate-300 mb-3">
              用户 <strong class="text-slate-900 dark:text-white">${escapeHtml(user.username)}</strong> 的新密码已生成，请妥善保管。
            </p>
            <div>
              <label class="field-label block mb-1.5">新密码</label>
              <div class="flex items-center gap-2">
                <code class="flex-1 rounded-xl border border-slate-200 dark:border-white/10 bg-slate-50 dark:bg-slate-800 px-3.5 py-2.5 text-sm font-mono tracking-wider break-all text-slate-900 dark:text-white">
                  ${escapeHtml(newPassword)}
                </code>
                <button
                  id="copy-pwd-btn"
                  class="soft-button min-h-0 py-2.5 px-3 shrink-0"
                  title="复制密码"
                >
                  <i class="ri-file-copy-line text-base"></i>
                </button>
              </div>
            </div>
          </div>
        `,
        buttons: [
          {
            text: '知道了',
            type: 'primary',
            callback: (modal) => modal.close(),
          },
        ],
      })
      modal.open()

      await nextTick()
      const copyBtn = document.getElementById('copy-pwd-btn')
      if (copyBtn) {
        copyBtn.addEventListener('click', async () => {
          try {
            await navigator.clipboard.writeText(newPassword)
            message.success('密码已复制到剪贴板')
            copyBtn.innerHTML = '<i class="ri-check-line text-base text-emerald-500"></i>'
            setTimeout(() => {
              copyBtn.innerHTML = '<i class="ri-file-copy-line text-base"></i>'
            }, 2000)
          } catch {
            message.warning('复制失败，请手动复制')
          }
        })
      }

      fetchUsers()
    } else {
      message.error(result.message || '重置失败')
    }
  } catch (err) {
    console.error('重置密码失败:', err)
    message.error(err.message || '网络错误，请重试')
  }
}


return { disposeDialogs, openCreateModal, openDeleteModal, openRoleModal, handleResetPassword };
}
