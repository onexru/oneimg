<template>
    <div class="flex min-h-[calc(100vh-140px)] items-center justify-center p-4">
        <!-- 全局加载遮罩 -->
        <div v-if="isLoading" class="fixed inset-0 bg-black/50 dark:bg-black/70 flex items-center justify-center z-50">
            <div class="loading-card bg-white dark:bg-gray-800 rounded-xl shadow-2xl p-6 max-w-md w-full m-[15px] lg:ml-[255px]">
                <!-- 加载动画 -->
                <div class="loading-spinner w-12 h-12 border-4 border-gray-200 dark:border-gray-700 border-t-primary dark:border-t-primary rounded-full animate-spin mx-auto mb-4"></div>
                <h3 class="loading-title text-lg font-bold text-center text-gray-800 dark:text-white mb-2">{{ loadingTitle }}</h3>
                <p class="loading-text text-center text-gray-600 dark:text-gray-300 mb-4">{{ loadingText }}</p>
                <!-- 进度条 -->
                <div class="loading-progress h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
                    <div class="progress-bar h-full bg-primary dark:bg-primary transition-all duration-300 ease-out" :style="{ width: loadingProgress + '%' }"></div>
                </div>
            </div>
        </div>

        <!-- 注册卡片 -->
        <div class="card glass-panel w-full max-w-md overflow-hidden transition-all duration-300" :class="{ 'opacity-50 pointer-events-none': isLoading }">
            <div class="card-body p-6">
                <div class="mb-8 text-center">
                    <div class="flex justify-end">
                        <router-link
                            to="/login"
                            class="text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 transition-colors"
                        >
                            返回登录
                        </router-link>
                    </div>
                    <div class="mx-auto mb-4 flex h-16 w-16 items-center justify-center rounded-3xl bg-gradient-to-br from-emerald-500 to-teal-700 text-2xl text-white shadow-lg shadow-emerald-500/20">
                        <i class="ri-user-add-line"></i>
                    </div>
                    <h5 class="card-title text-2xl font-bold text-gray-800 dark:text-white">创建账户</h5>
                    <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">注册新账户以开始使用</p>
                </div>

                <!-- 用户名输入 -->
                <div class="form-group mb-6">
                    <label for="reg-username" class="form-label block text-gray-700 dark:text-gray-300 mb-2">用户名</label>
                    <input
                        type="text"
                        id="reg-username"
                        ref="usernameInput"
                        v-model="username"
                        class="input-modern"
                        :class="{ 'border-red-400 dark:border-red-500 focus:ring-red-400 dark:focus:ring-red-500': usernameError }"
                        placeholder="3-50 个字符"
                        :disabled="isLoading"
                        maxlength="50"
                        @input="clearError('username')"
                        @keyup.enter="focusPassword"
                    />
                    <p v-if="usernameError" class="mt-1.5 text-xs text-red-500 dark:text-red-400">{{ usernameError }}</p>
                </div>

                <!-- 密码输入 -->
                <div class="form-group mb-6">
                    <label for="reg-password" class="form-label block text-gray-700 dark:text-gray-300 mb-2">密码</label>
                    <div class="relative">
                        <input
                            :type="showPassword ? 'text' : 'password'"
                            id="reg-password"
                            ref="passwordInput"
                            v-model="password"
                            class="input-modern pr-10"
                            :class="{ 'border-red-400 dark:border-red-500 focus:ring-red-400 dark:focus:ring-red-500': passwordError }"
                            placeholder="至少 6 个字符"
                            :disabled="isLoading"
                            maxlength="100"
                            @input="clearError('password'); onPasswordInput()"
                            @keyup.enter="focusConfirmPassword"
                        />
                        <button
                            type="button"
                            class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                            @click="showPassword = !showPassword"
                            tabindex="-1"
                        >
                            <i :class="showPassword ? 'ri-eye-off-line' : 'ri-eye-line'"></i>
                        </button>
                    </div>
                    <!-- 密码强度指示器 -->
                    <div v-if="password.length > 0" class="mt-2 flex gap-1">
                        <div
                            v-for="i in 4"
                            :key="i"
                            class="h-1 flex-1 rounded-full transition-colors duration-300"
                            :class="i <= passwordStrength.level ? passwordStrength.color : 'bg-gray-200 dark:bg-gray-700'"
                        ></div>
                    </div>
                    <p v-if="password.length > 0" class="mt-1 text-xs" :class="passwordStrength.textColor">
                        {{ passwordStrength.text }}
                    </p>
                    <p v-if="passwordError" class="mt-1.5 text-xs text-red-500 dark:text-red-400">{{ passwordError }}</p>
                </div>

                <!-- 确认密码输入 -->
                <div class="form-group mb-8">
                    <label for="reg-confirm-password" class="form-label block text-gray-700 dark:text-gray-300 mb-2">确认密码</label>
                    <div class="relative">
                        <input
                            :type="showConfirmPassword ? 'text' : 'password'"
                            id="reg-confirm-password"
                            ref="confirmPasswordInput"
                            v-model="confirmPassword"
                            class="input-modern pr-10"
                            :class="{ 'border-red-400 dark:border-red-500 focus:ring-red-400 dark:focus:ring-red-500': confirmError }"
                            placeholder="再次输入密码"
                            :disabled="isLoading"
                            maxlength="100"
                            @input="clearError('confirm')"
                            @keyup.enter="handleRegister"
                        />
                        <button
                            type="button"
                            class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors"
                            @click="showConfirmPassword = !showConfirmPassword"
                            tabindex="-1"
                        >
                            <i :class="showConfirmPassword ? 'ri-eye-off-line' : 'ri-eye-line'"></i>
                        </button>
                    </div>
                    <p v-if="confirmError" class="mt-1.5 text-xs text-red-500 dark:text-red-400">{{ confirmError }}</p>
                </div>

                <!-- 注册按钮 -->
                <div class="form-group">
                    <button
                        @click="handleRegister"
                        class="register-btn primary-button w-full py-3 text-base"
                        :class="{ 'opacity-70 cursor-not-allowed': isLoading }"
                        :disabled="isLoading"
                    >
                        注册
                    </button>
                </div>
            </div>
        </div>

        <!-- POW验证弹窗 -->
        <div
            v-if="showModal"
            class="fixed inset-0 bg-black/50 dark:bg-black/70 flex items-center justify-center z-50 transition-opacity duration-300"
            @click="closeModal"
            id="powModal"
            style="display: none;"
        >
            <div class="modal bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-md mx-4 transform transition-all duration-300 scale-100 lg:ml-[255px]" @click.stop>
                <div class="modal-header p-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
                    <h3 class="modal-title text-lg font-bold text-gray-800 dark:text-white">安全验证</h3>
                    <button
                        class="modal-close text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 text-xl font-bold transition-colors"
                        @click="closeModal"
                        :disabled="isLoading || !isPowReady"
                        :class="{ 'opacity-70 cursor-not-allowed': isLoading || !isPowReady }"
                    >
                        ×
                    </button>
                </div>
                <div class="pow p-6">
                    <div class="flex items-center justify-center">
                        <div id="pow-container" class="mx-auto min-w-[320px]"></div>
                    </div>
                    <p class="pow-tip text-center text-gray-600 dark:text-gray-300 mt-4">
                        请完成人机验证以继续注册
                    </p>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, reactive } from 'vue';
import { useRouter } from 'vue-router';
import message from '@/utils/message.js';

const router = useRouter();

// ref 引用
const usernameInput = ref(null);
const passwordInput = ref(null);
const confirmPasswordInput = ref(null);

// 响应式数据
const username = ref('');
const password = ref('');
const confirmPassword = ref('');
const showPassword = ref(false);
const showConfirmPassword = ref(false);
const isLoading = ref(false);
const loadingTitle = ref('');
const loadingText = ref('');
const loadingProgress = ref(0);
const showModal = ref(false);
const isPowReady = ref(false);
let powCheckInterval = null;

// 登录配置（复用登录接口的配置）
const loginConfig = reactive({
    pow_verify: false,
    start_register: false
});

// 错误信息
const usernameError = ref('');
const passwordError = ref('');
const confirmError = ref('');

// 清除对应字段的错误
const clearError = (field) => {
    if (field === 'username') usernameError.value = '';
    if (field === 'password') passwordError.value = '';
    if (field === 'confirm') confirmError.value = '';
};

// 聚焦逻辑
const focusPassword = () => {
    passwordInput.value?.focus();
};
const focusConfirmPassword = () => {
    confirmPasswordInput.value?.focus();
};

// 密码强度计算
const passwordStrength = computed(() => {
    const pwd = password.value;
    if (!pwd) return { level: 0, text: '', color: '', textColor: 'text-gray-400 dark:text-gray-500' };

    let score = 0;
    if (pwd.length >= 6) score++;
    if (pwd.length >= 10) score++;
    if (/[A-Z]/.test(pwd) && /[a-z]/.test(pwd)) score++;
    if (/\d/.test(pwd)) score++;
    if (/[^A-Za-z0-9]/.test(pwd)) score++;

    if (score <= 1) return { level: 1, text: '弱', color: 'bg-red-500', textColor: 'text-red-500 dark:text-red-400' };
    if (score <= 2) return { level: 2, text: '较弱', color: 'bg-orange-500', textColor: 'text-orange-500 dark:text-orange-400' };
    if (score <= 3) return { level: 3, text: '中等', color: 'bg-yellow-500', textColor: 'text-yellow-500 dark:text-yellow-400' };
    return { level: 4, text: '强', color: 'bg-emerald-500', textColor: 'text-emerald-500 dark:text-emerald-400' };
});

const onPasswordInput = () => {
    if (confirmPassword.value.length > 0) {
        if (confirmPassword.value !== password.value) {
            confirmError.value = '两次输入的密码不一致';
        } else {
            confirmError.value = '';
        }
    }
};

// 加载状态管理
const setLoadingState = (title, text, progress = 0) => {
    isLoading.value = true;
    loadingTitle.value = title;
    loadingText.value = text;
    loadingProgress.value = progress;
};

const clearLoadingState = () => {
    isLoading.value = false;
    loadingTitle.value = '';
    loadingText.value = '';
    loadingProgress.value = 0;
};

// 前端表单校验
const validateForm = () => {
    let valid = true;

    if (!username.value.trim()) {
        usernameError.value = '请输入用户名';
        valid = false;
    } else if (username.value.trim().length < 3) {
        usernameError.value = '用户名至少需要 3 个字符';
        valid = false;
    } else if (username.value.trim().length > 50) {
        usernameError.value = '用户名不能超过 50 个字符';
        valid = false;
    } else {
        usernameError.value = '';
    }

    if (!password.value) {
        passwordError.value = '请输入密码';
        valid = false;
    } else if (password.value.length < 6) {
        passwordError.value = '密码至少需要 6 个字符';
        valid = false;
    } else if (password.value.length > 100) {
        passwordError.value = '密码不能超过 100 个字符';
        valid = false;
    } else {
        passwordError.value = '';
    }

    if (!confirmPassword.value) {
        confirmError.value = '请再次输入密码';
        valid = false;
    } else if (confirmPassword.value !== password.value) {
        confirmError.value = '两次输入的密码不一致';
        valid = false;
    } else {
        confirmError.value = '';
    }

    return valid;
};

// 注册处理
const handleRegister = () => {
    if (isLoading.value) return;

    if (!validateForm()) return;

    if (loginConfig.pow_verify) {
        setLoadingState('正在启动', '准备安全验证...', 10);
        setTimeout(() => {
            setLoadingState('加载验证', '正在加载验证界面...', 20);
            showModal.value = true;
        }, 500);
    } else {
        putRegister("000");
    }
};

// 监听弹窗状态变化
watch(showModal, (newVal) => {
    if (newVal) {
        setTimeout(() => {
            setLoadingState('加载验证', '正在初始化验证组件...', 30);
            createPowWidget();
        }, 800);
    } else {
        cleanupPowEvent();
        isPowReady.value = false;
    }
});

// 创建POW验证组件
const createPowWidget = () => {
    const container = document.getElementById('pow-container');
    if (!container) {
        setTimeout(createPowWidget, 200);
        return;
    }

    container.innerHTML = '';
    const powWidget = document.createElement('pow-widget');
    powWidget.id = 'pow';
    powWidget.setAttribute('data-pow-api-endpoint', 'https://cha.eta.im/');
    container.appendChild(powWidget);

    powWidget.addEventListener('load', handlePowLoaded);
    powWidget.addEventListener('ready', handlePowLoaded);
    powWidget.addEventListener('solve', handlePowSuccess);
    powWidget.addEventListener('error', (e) => {
        message.error("验证失败，请重试！" + (e.detail?.message || ''));
        closeModal();
    });
};

// POW组件加载就绪处理
const handlePowLoaded = () => {
    clearInterval(powCheckInterval);
    isPowReady.value = true;
    loadingProgress.value = 80;
    clearLoadingState();
    document.getElementById('powModal')?.style.removeProperty('display');
};

// POW验证成功
const handlePowSuccess = async (e) => {
    closeModal();
    const token = e.detail.token;
    setLoadingState('验证通过', '正在提交注册请求...', 90);

    setTimeout(() => {
        putRegister(token);
    }, 500);
};

// 关闭弹窗
const closeModal = () => {
    showModal.value = false;
    clearLoadingState();
    cleanupPowEvent();
};

// 清理POW组件和事件
const cleanupPowEvent = () => {
    clearInterval(powCheckInterval);
    const container = document.getElementById('pow-container');
    if (container) {
        const widget = container.querySelector('#pow');
        if (widget) {
            widget.removeEventListener('solve', handlePowSuccess);
            widget.removeEventListener('load', handlePowLoaded);
            widget.removeEventListener('ready', handlePowLoaded);
            widget.removeEventListener('error', () => {});
            widget.remove();
        }
    }
    isPowReady.value = false;
};

// 提交注册请求
const putRegister = async (token) => {
    setLoadingState('注册中', '正在创建账户...', 90);

    try {
        const response = await fetch('/api/register', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                username: username.value.trim(),
                password: password.value,
                powToken: token
            })
        });

        const result = await response.json();

        if (response.ok && result.code === 200) {
            setLoadingState(result.message || '注册成功', '即将跳转到登录页面...', 100);
            message.success(result.message || '注册成功，请登录');

            setTimeout(() => {
                clearLoadingState();
                showModal.value = false;
                router.push('/login');
            }, 1500);
        } else {
            clearLoadingState();
            const errMsg = result.message || '注册失败，请稍后重试';

            if (errMsg.includes('用户名') || errMsg.includes('已存在')) {
                usernameError.value = errMsg;
            } else if (errMsg.includes('密码')) {
                passwordError.value = errMsg;
            } else if (errMsg.includes('pow') || errMsg.includes('POW')) {
                message.error(errMsg);
            } else if (errMsg.includes('未开放') || errMsg.includes('禁止')) {
                message.error(errMsg);
                setTimeout(() => {
                    router.push('/login');
                }, 1500);
                return;
            } else {
                message.error(errMsg);
            }
        }
    } catch (error) {
        clearLoadingState();
        message.error('注册请求失败，请检查网络连接: ' + error.message);
    }
};

// 获取登录配置（检查 pow_verify 和 start_register）
const getLoginSettings = async () => {
    try {
        const response = await fetch('/api/settings/login', {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json'
            }
        });
        const result = await response.json();
        if (response.ok && result.code === 200) {
            loginConfig.pow_verify = result.data.pow_verify || false;
            loginConfig.start_register = result.data.start_register || false;
        }
    } catch (error) {
        console.warn('获取注册配置失败:', error);
    }
};

onMounted(async () => {
    // 修复URL方法兼容问题
    if (!URL.revokeObjectUrl && URL.revokeObjectURL) {
        URL.revokeObjectUrl = URL.revokeObjectURL;
    }

    await getLoginSettings();

    // 检查注册是否开放
    if (!loginConfig.start_register) {
        message.warning('暂未开放注册');
        router.replace('/login');
        return;
    }

    // 加载POW脚本（避免重复加载）
    if (loginConfig.pow_verify && !document.querySelector('script[src="https://cha.eta.im/static/js/pow.min.js"]')) {
        const script = document.createElement('script');
        script.src = 'https://cha.eta.im/static/js/pow.min.js';
        script.onload = () => {
            console.log('POW脚本加载完成');
        };
        script.onerror = () => {
            message.error('验证脚本加载失败，请刷新页面重试');
            clearLoadingState();
            closeModal();
        };
        document.head.appendChild(script);
    }

    usernameInput.value?.focus();
});

// 清理资源
onUnmounted(() => {
    cleanupPowEvent();
});
</script>