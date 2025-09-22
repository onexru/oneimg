<template>
    <Navbar />
    <div class="settings-container">
        <div class="settings-header">
            <h1 class="page-title">
                <span class="title-icon">⚙️</span>
                设置
            </h1>
            <p class="page-description">管理您的账户设置和偏好</p>
        </div>

        <div class="settings-content">
            <div class="settings-panel">
                <div class="panel-content">
                    <h2 class="panel-title">
                        <span class="panel-icon">👤</span>
                        账户设置
                    </h2>
                    
                    <!-- 账户修改表单 -->
                    <form @submit.prevent="updateAccount" class="account-form">
                        <div class="setting-group">
                            <label class="setting-label" for="newUsername">新用户名（留空则不修改）</label>
                            <input 
                                id="newUsername"
                                v-model="accountForm.newUsername"
                                type="text" 
                                class="setting-input"
                                placeholder="留空则不修改用户名"
                                minlength="3"
                                maxlength="20"
                            />
                        </div>
                        
                        <div class="setting-group">
                            <label class="setting-label" for="currentPassword">当前密码</label>
                            <input 
                                id="currentPassword"
                                v-model="accountForm.currentPassword"
                                type="password" 
                                class="setting-input"
                                placeholder="请输入当前密码以确认修改"
                                required
                            />
                        </div>
                        
                        <div class="setting-group">
                            <label class="setting-label" for="newPassword">新密码（留空则不修改）</label>
                            <input 
                                id="newPassword"
                                v-model="accountForm.newPassword"
                                type="password" 
                                class="setting-input"
                                placeholder="留空则不修改密码（至少6位）"
                                minlength="6"
                            />
                        </div>
                        
                        <div class="setting-group">
                            <label class="setting-label" for="confirmPassword">确认新密码</label>
                            <input 
                                id="confirmPassword"
                                v-model="accountForm.confirmPassword"
                                type="password" 
                                class="setting-input"
                                placeholder="请再次输入新密码"
                            />
                        </div>
                        
                        <button 
                            type="submit" 
                            class="setting-btn accent"
                        >
                            <span>保存修改</span>
                        </button>
                    </form>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import Navbar from "@/components/Navbar.vue";
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import message from '@/utils/message.js'

const router = useRouter()
const currentUser = ref({})

const accountForm = ref({
    newUsername: '',
    currentPassword: '',
    newPassword: '',
    confirmPassword: ''
})

const isUpdatingAccount = ref(false)



// 更新账户信息（用户名和/或密码）
const updateAccount = async () => {
    const { newUsername, currentPassword, newPassword, confirmPassword } = accountForm.value
    
    // 检查是否有任何修改
    const hasUsernameChange = newUsername && newUsername.trim() !== ''
    const hasPasswordChange = newPassword && newPassword.trim() !== ''
    
    if (!hasUsernameChange && !hasPasswordChange) {
        message.error('请输入要修改的用户名或密码')
        return
    }
    
    // 验证用户名（如果要修改）
    if (hasUsernameChange) {
        if (newUsername.length < 3) {
            message.error('用户名长度至少为3位')
            return
        }
        
        if (newUsername.length > 20) {
            message.error('用户名长度不能超过20位')
            return
        }
        
        if (newUsername === currentUser.value.username) {
            message.error('新用户名不能与当前用户名相同')
            return
        }
    }
    
    // 验证密码（如果要修改）
    if (hasPasswordChange) {
        if (newPassword.length < 6) {
            message.error('新密码长度至少为6位')
            return
        }
        
        if (newPassword !== confirmPassword) {
            message.error('两次输入的新密码不一致')
            return
        }
    }
    
    // 验证当前密码
    if (!currentPassword) {
        message.error('请输入当前密码以确认修改')
        return
    }
    
    try {
        const response = await fetch('/api/account/change', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                new_username: newUsername,
                current_password: currentPassword,
                new_password: newPassword
            })
        })
        
        const result = await response.json()
        
        if (!response.ok || !result.success) {
            throw new Error(result.message || '密码更新失败')
        }
        message.success('修改成功')

        // 清空表单
        accountForm.value = {
            newUsername: '',
            currentPassword: '',
            newPassword: '',
            confirmPassword: ''
        }
        // 刷新页面
        setTimeout(() => {
            window.location.reload();
        }, 1000)

    } catch (error) {
        message.error(error.message || '更新失败')
    } finally {
        isUpdatingAccount.value = false
    }
}
</script>

<style scoped lang="scss">
.settings-container {
    min-height: 100vh;
    background: var(--bg-color);
    padding: 2rem;
}

.settings-header {
    text-align: center;
    margin-bottom: 3rem;
}

.page-title {
    font-size: 2.5rem;
    font-weight: 700;
    color: var(--text-color);
    margin-bottom: 0.5rem;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
}

.title-icon {
    font-size: 2rem;
}

.page-description {
    color: var(--muted-color);
    font-size: 1.1rem;
}

.settings-content {
    max-width: 600px;
    margin: 0 auto;
}

.settings-panel {
    background: var(--card-bg);
    border-radius: 16px;
    box-shadow: 0 2px 8px var(--shadow-color);
    overflow: hidden;
    border: 1px solid var(--border-color);
}

.panel-content {
    padding: 2rem;
}

.panel-title {
    font-size: 1.5rem;
    font-weight: 600;
    color: var(--text-color);
    margin-bottom: 2rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

.panel-icon {
    font-size: 1.25rem;
}

.account-form {
    .setting-group {
        margin-bottom: 1.5rem;
    }
}

.setting-group {
    margin-bottom: 1.5rem;
}

.setting-label {
    display: block;
    font-weight: 500;
    color: var(--text-color);
    margin-bottom: 0.5rem;
    font-size: 0.95rem;
}

.setting-input {
    width: 100%;
    padding: 12px 15px;
    border: 2px solid var(--input-border);
    border-radius: 8px;
    background: var(--input-bg);
    color: var(--text-color);
    font-size: 1rem;
    transition: all 0.3s ease;
    
    &:focus {
        outline: none;
        border-color: var(--accent-color);
        box-shadow: 0 0 0 2px rgba(26, 115, 232, 0.2);
    }
    
    &::placeholder {
        color: var(--muted-color);
    }
}

.current-username {
    .username-display {
        display: inline-block;
        padding: 12px 15px;
        background: var(--input-bg);
        border: 2px solid var(--input-border);
        border-radius: 8px;
        color: var(--text-color);
        font-weight: 500;
    }
}

.setting-btn {
    width: 100%;
    padding: 12px 24px;
    border: none;
    border-radius: 8px;
    font-size: 1rem;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.3s ease;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    background: var(--accent-color);
    color: #fff;
    
    &.primary {
        background: var(--accent-color);
        color: var(--bg-color);
        
        &:hover:not(:disabled) {
            opacity: 0.9;
            transform: translateY(-1px);
        }
        
        &:disabled {
            opacity: 0.6;
            cursor: not-allowed;
            transform: none;
        }
    }
}

.btn-spinner {
    width: 16px;
    height: 16px;
    border: 2px solid transparent;
    border-top: 2px solid currentColor;
    border-radius: 50%;
    animation: spin 1s linear infinite;
}

@keyframes spin {
    to {
        transform: rotate(360deg);
    }
}

@media (max-width: 768px) {
    .settings-container {
        padding: 1rem;
    }
    
    .page-title {
        font-size: 2rem;
    }
    
    .panel-content {
        padding: 1.5rem;
    }
}

@media (max-width: 480px) {
    .settings-container {
        padding: 0.5rem;
    }
    
    .page-title {
        font-size: 1.75rem;
    }
    
    .panel-content {
        padding: 1rem;
    }
    
    .setting-input {
        padding: 10px 12px;
    }
}
</style>