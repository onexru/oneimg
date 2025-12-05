<template>
    <div class="login flex items-center justify-center p-4">
        <!-- 全局加载遮罩 -->
        <div v-if="isLoading" class="fixed inset-0 bg-black/50 dark:bg-black/70 flex items-center justify-center z-50">
            <div class="loading-card bg-white dark:bg-gray-800 rounded-xl shadow-2xl p-6 max-w-md w-full m-[15px]">
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

        <!-- 登录卡片 -->
        <div class="card bg-white dark:bg-gray-800 rounded-xl shadow-lg w-full max-w-md transition-all duration-300" :class="{ 'opacity-50 pointer-events-none': isLoading }">
            <div class="card-body p-6">
                <h5 class="card-title text-2xl font-bold text-center text-gray-800 dark:text-white mb-8">登录</h5>
                <!-- 用户名输入 -->
                <div class="form-group mb-6">
                    <label for="username" class="form-label block text-gray-700 dark:text-gray-300 mb-2">用户名</label>
                    <input 
                        type="text" 
                        v-model="username" 
                        class="form-input w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white transition-all outline-none"
                        placeholder="用户名"
                        :disabled="isLoading"
                        @keyup.enter="handleLogin"
                    />
                </div>
                <!-- 密码输入 -->
                <div class="form-group mb-8">
                    <label for="password" class="form-label block text-gray-700 dark:text-gray-300 mb-2">密码</label>
                    <input 
                        type="password" 
                        v-model="password" 
                        class="form-input w-full px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-primary focus:border-primary dark:bg-gray-700 dark:text-white transition-all outline-none"
                        placeholder="密码"
                        :disabled="isLoading"
                        @keyup.enter="handleLogin"
                    />
                </div>
                <!-- 登录按钮 -->
                <div class="form-group">
                    <button 
                        @click="handleLogin" 
                        class="login-btn w-full py-3 bg-primary hover:bg-primary/90 text-white font-medium rounded-lg transition-all duration-200 flex items-center justify-center"
                        :class="{ 'opacity-70 cursor-not-allowed': isLoading }"
                        :disabled="isLoading"
                    >
                        登录
                    </button>
                </div>
                <!-- 游客登录按钮 -->
                 <div v-if="loginConfig.tourist" class="form-group">
                    <button 
                        @click="handleTouristLogin" 
                        class="tourist-login-btn w-full py-3 hover:bg-purple-500/90 bg-purple-500 dark:bg-purple-600 mt-4 text-white font-medium rounded-lg transition-all duration-200 flex items-center justify-center"
                        :class="{ 'opacity-70 cursor-not-allowed': isLoading }"
                        :disabled="isLoading"
                    > 
                    游客登录
                    </button>
                 </div>
            </div>
        </div>

        <!-- POW验证弹窗 -->
        <div 
            v-if="showModal" 
            class="fixed inset-0 bg-black/50 dark:bg-black/70 flex items-center justify-center z-50 transition-opacity duration-300"
            @click="closeModal" id="powModal" style="display: none;"
        >
            <div class="modal bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-md mx-4 transform transition-all duration-300 scale-100" @click.stop>
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
                        请完成人机验证以继续登录
                    </p>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, reactive } from 'vue';
import message from '@/utils/message.js';

// 响应式数据
const showModal = ref(false);
const username = ref('');
const password = ref('');
const isLoading = ref(false);
const loadingTitle = ref('');
const loadingText = ref('');
const loadingProgress = ref(0);
const isPowReady = ref(false);
let powCheckInterval = null; // 轮询检测定时器

// 登录配置
const loginConfig = reactive({
    pow_verify: false,
    tourist: false
})

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

// 游客登录处理
const handleTouristLogin = () => {
    if (isLoading.value) return;

    username.value = 'guest';
    password.value = 'guest';

    if (loginConfig.pow_verify) {
        // 启动POW验证
        setLoadingState('正在启动', '准备安全验证...', 10);
        setTimeout(() => {
            // 优化进度提示
            setLoadingState('加载验证', '正在加载验证界面...', 60);
            showModal.value = true;
        }, 500);
    } else {
        // 直接登录
        putLogin("000");
    }
};

// 登录处理
const handleLogin = () => {
    if (isLoading.value) return;
    
    if (!username.value || !password.value) {
        message.warning('请输入用户名和密码');
        return;
    }
    
    if (loginConfig.pow_verify) {
        // 启动POW验证
        setLoadingState('正在启动', '准备安全验证...', 10);
        setTimeout(() => {
            // 优化进度提示
            setLoadingState('加载验证', '正在加载验证界面...', 60);
            showModal.value = true;
        }, 500);
    } else {
        // 直接登录
        putLogin("000");
    }
};

// 监听弹窗状态变化
watch(showModal, (newVal) => {
    if (newVal) {
        // 弹窗显示后，初始化POW组件
        setTimeout(() => {
            setLoadingState('加载验证', '正在初始化验证组件...', 30);
            createPowWidget();
        }, 800);
    } else {
        // 弹窗关闭，清理资源
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

    // 清空容器并创建POW组件
    const powWidget = document.createElement('pow-widget');
    powWidget.id = 'pow';
    powWidget.setAttribute('data-pow-api-endpoint', 'https://cha.eta.im/');
    container.appendChild(powWidget);

    handlePowLoaded();

    // 绑定验证事件（提前绑定，确保不遗漏）
    powWidget.addEventListener('solve', handlePowSuccess);
    powWidget.addEventListener('error', () => {
        message.error("验证失败，请重试！");
    });
};

// POW组件加载就绪处理
const handlePowLoaded = () => {
    clearInterval(powCheckInterval); // 清除轮询
    isPowReady.value = true; // 标记组件就绪
    setLoadingProgress(80); // 进度条更新为80%（等待用户验证）
    clearLoadingState(); // 清除全局加载状态，允许用户操作
    document.getElementById('powModal').style.display = '';
};

// 检查验证token
const handlePowSuccess = (e) => {
    closeModal();
    const token = e.detail.token;
    setLoadingState('验证通过', '正在提交登录请求...', 90);
    setTimeout(() => {
        putLogin(token);
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
    clearInterval(powCheckInterval); // 清除轮询
    const container = document.getElementById('pow-container');
    if (container) {
        const widget = container.querySelector('#pow');
        if (widget) {
            widget.removeEventListener('solve', handlePowSuccess);
            widget.removeEventListener('load', handlePowLoaded);
            widget.removeEventListener('ready', handlePowLoaded);
            try {
                widget.remove();
            } catch (e) {
                container.innerHTML = '';
            }
        }
    }
    isPowReady.value = false;
};

// 辅助：更新进度条（可选）
const setLoadingProgress = (progress) => {
    loadingProgress.value = progress;
};

// 提交登录请求
const putLogin = async (token) => {
    setLoadingState('登录中', '正在验证用户信息...', 90);
    
    try {
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                username: username.value,
                password: password.value,
                powToken: token
            })
        });
        
        const result = await response.json();
        
        if (response.ok && result.code === 200) {
            // 保存用户信息
            localStorage.setItem('userInfo', JSON.stringify({"username": username.value}));
            setLoadingState((result.message || '登录成功'), '即将跳转到主页...', 100);
            
            setTimeout(() => {
                clearLoadingState();
                showModal.value = false;
                // 跳转到主页
                window.location.href = '/';
            }, 1500);
        } else {
            clearLoadingState();
            message.error('登录失败: ' + (result.message || '未知错误'));
            closeModal();
        }
    } catch (error) {
        clearLoadingState();
        message.error('登录请求失败，请检查网络连接: ' + error.message);
        closeModal();
    }
};

const getLoginSettings = async () => { 
    try {
        const response = await fetch('/api/settings/login', {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json'
            }
        })
        const result = await response.json();
        if (response.ok && result.code === 200) {
            Object.assign(loginConfig, result.data)
        } else {
            message.error('获取登录配置失败');
        }
    } catch (error) {
        message.error('获取登录配置失败');
    }
};

// 加载POW脚本
onMounted(() => {
    // 修复URL方法兼容问题
    if (!URL.revokeObjectUrl && URL.revokeObjectURL) {
        URL.revokeObjectUrl = URL.revokeObjectURL;
    }

    // 获取登录配置
    getLoginSettings()
    
    // 避免重复加载脚本
    if (!document.querySelector('script[src="https://cha.eta.im/static/js/pow.min.js"]')) {
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
});

// 清理资源
onUnmounted(() => {
    cleanupPowEvent();
});
</script>