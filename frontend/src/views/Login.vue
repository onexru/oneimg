<template>
    <div class="login">
        <!-- 全局加载遮罩 -->
        <div v-if="isLoading" class="global-loading">
            <div class="loading-card">
                <div class="loading-spinner"></div>
                <h3 class="loading-title">{{ loadingTitle }}</h3>
                <p class="loading-text">{{ loadingText }}</p>
                <div class="loading-progress">
                    <div class="progress-bar" :style="{ width: loadingProgress + '%' }"></div>
                </div>
            </div>
        </div>

        <!-- 登录卡片 -->
        <div class="card" :class="{ 'card-disabled': isLoading }">
            <div class="card-body">
                <h5 class="card-title">登录</h5>
                <div class="form-group">
                    <label for="username" class="form-label">用户名</label>
                    <input 
                        type="text" 
                        v-model="username" 
                        class="form-input" 
                        placeholder="用户名"
                        :disabled="isLoading"
                    />
                </div>
                <div class="form-group">
                    <label for="password" class="form-label">密码</label>
                    <input 
                        type="password" 
                        v-model="password" 
                        class="form-input" 
                        placeholder="密码"
                        :disabled="isLoading"
                    />
                </div>
                <div class="form-group">
                    <button @click="handleLogin" class="login-btn" :disabled="isLoading">
                        登录
                    </button>
                </div>
            </div>
        </div>

        <!-- POW验证弹窗 -->
        <div 
            v-if="showModal" 
            class="modal-backdrop" 
            :class="{ 'modal-visible': showModal }"
            @click="closeModal"
        >
            <div class="modal" @click.stop>
                <div class="modal-header">
                    <h3 class="modal-title">安全验证</h3>
                    <button class="modal-close" @click="closeModal" :disabled="isLoading">×</button>
                </div>
                <div class="pow">
                    <div id="pow-container"></div>
                    <p class="pow-tip">请完成人机验证以继续登录</p>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue';
import message from '@/utils/message.js';

const showModal = ref(false);
const username = ref('');
const password = ref('');
const isEventBound = ref(false);
const isLoading = ref(false);
const loadingTitle = ref('');
const loadingText = ref('');
const loadingProgress = ref(0);

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

const handleLogin = () => {
    if (isLoading.value) return;
    
    if (!username.value || !password.value) {
        message.warning('请输入用户名和密码');
        return;
    }
    
    setLoadingState('正在启动', '准备安全验证...', 10);
    setTimeout(() => {
        showModal.value = true;
    }, 500);
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
    }
});

const createPowWidget = () => {
    if (isEventBound.value) return;
    
    const container = document.getElementById('pow-container');
    if (!container) {
        setTimeout(createPowWidget, 200);
        return;
    }

    // 创建POW组件
    container.innerHTML = '<pow-widget id="pow" data-pow-api-endpoint="https://cha.eta.im/"></pow-widget>';
    
    setLoadingState('等待验证', '请完成人机验证...', 50);
    clearLoadingState(); // 清除加载状态，让用户进行验证
    
    // 开始检查token
    setTimeout(checkForToken, 500);
};

const checkForToken = () => {
    const widget = document.querySelector("#pow");
    widget.addEventListener('solve', function(event) {
        const token = event.detail.token;
        handlePowSuccess({ detail: { token: token } });
    });
    widget.addEventListener('error', function(event) {
        message.error("验证失败，请重试！")
    });
};

const closeModal = () => {
    showModal.value = false;
    clearLoadingState();
    cleanupPowEvent();
};

const cleanupPowEvent = () => {
    if (isEventBound.value) {
        isEventBound.value = false;
    }
    
    const container = document.getElementById('pow-container');
    if (container) {
        // 先尝试优雅地移除POW组件
        const widget = container.querySelector('#pow');
        if (widget && typeof widget.remove === 'function') {
            try {
                widget.remove();
            } catch (e) {
                // 如果移除失败，直接清空容器
                container.innerHTML = '';
            }
        } else {
            container.innerHTML = '';
        }
    }
};

// 处理pow验证成功的逻辑
const handlePowSuccess = (e) => {
    const token = e.detail.token;
    console.log('🎯 获取到POW token:', token);
    
    setTimeout(() => {
        putLogin(token);
    }, 500);
};

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
        
        if (response.ok && result.success) {
            // 保存userdata
            localStorage.setItem('userInfo', JSON.stringify({"username": username.value}));
            setLoadingState('登录成功', '即将跳转到主页...', 100);
            
            setTimeout(() => {
                clearLoadingState();
                showModal.value = false;
                
                // 这里可以添加路由跳转逻辑
                // router.push('/dashboard');
                
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
}

// 加载POW脚本
onMounted(() => {
    // 修复POW脚本的URL方法错误
    if (!URL.revokeObjectUrl && URL.revokeObjectURL) {
        URL.revokeObjectUrl = URL.revokeObjectURL;
    }
    
    if (!document.querySelector('script[src="https://cha.eta.im/static/js/pow.min.js"]')) {
        const script = document.createElement('script');
        script.src = 'https://cha.eta.im/static/js/pow.min.js';
        document.head.appendChild(script);
    }
});

// 清理资源
onUnmounted(() => {
    cleanupPowEvent();
});
</script>

<style lang="scss" scoped>
.login {
    display: flex;
    justify-content: center;
    align-items: center;
    min-height: 100vh;
    padding: 20px;
    position: relative;
    background-color: var(--bg-color);

    /* 全局加载遮罩 */
    .global-loading {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0, 0, 0, 0.8);
        backdrop-filter: blur(8px);
        z-index: 9999;
        display: flex;
        justify-content: center;
        align-items: center;
        animation: fadeIn 0.3s ease-out;

        .loading-card {
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 16px;
            padding: 40px;
            text-align: center;
            box-shadow: 0 20px 60px var(--shadow-color);
            max-width: 400px;
            width: 90%;
            animation: slideUp 0.4s ease-out;

            .loading-spinner {
                width: 60px;
                height: 60px;
                border: 4px solid var(--border-color);
                border-top: 4px solid var(--accent-color);
                border-radius: 50%;
                animation: spin 1s linear infinite;
                margin: 0 auto 20px;
            }

            .loading-title {
                font-size: 1.5rem;
                color: var(--text-color);
                margin: 0 0 10px 0;
                font-weight: 600;
            }

            .loading-text {
                color: var(--muted-color);
                margin: 0 0 20px 0;
                font-size: 1rem;
            }

            .loading-progress {
                width: 100%;
                height: 6px;
                background: var(--border-color);
                border-radius: 3px;
                overflow: hidden;
                margin-top: 20px;

                .progress-bar {
                    height: 100%;
                    background: linear-gradient(90deg, var(--accent-color), #2ecc71);
                    border-radius: 3px;
                    transition: width 0.3s ease;
                    animation: progressGlow 2s ease-in-out infinite alternate;
                }
            }
        }
    }

    .card {
        border-radius: 12px;
        border: 1px solid var(--border-color);
        max-width: 400px;
        width: 100%;
        background-color: var(--card-bg);
        box-shadow: 0 12px 32px var(--shadow-color);
        transition: all 0.3s ease;
        backdrop-filter: blur(10px);

        &.card-disabled {
            opacity: 0.6;
            pointer-events: none;
            transform: scale(0.98);
        }

        .card-body {
            padding: 2.5em;

            .card-title {
                font-size: 2rem;
                color: var(--accent-color);
                margin-bottom: 1.5rem;
                text-align: center;
                font-weight: 700;
            }

            .form-group {
                position: relative;
                margin-bottom: 1.5rem;

                .form-label {
                    display: block;
                    margin-bottom: 0.5rem;
                    font-weight: 500;
                    color: var(--text-color);
                    transition: color 0.3s ease;
                }

                .form-input {
                    width: 100%;
                    padding: 14px 16px;
                    border: 1px solid var(--input-border);
                    border-radius: 8px;
                    font-size: 1rem;
                    transition: all 0.3s ease;
                    background-color: var(--input-bg);
                    color: var(--text-color);

                    &:focus {
                        outline: none;
                        border-color: var(--accent-color);
                        box-shadow: 0 0 0 3px rgba(96, 165, 250, 0.1);
                    }

                    &::placeholder {
                        color: var(--muted-color);
                    }

                    &:disabled {
                        opacity: 0.6;
                        cursor: not-allowed;
                        background-color: var(--hover-bg);
                    }
                }

                .login-btn {
                    margin-top: 0.5rem;
                    width: 100%;
                    padding: 14px 16px;
                    background-color: var(--accent-color);
                    color: #fff;
                    border: none;
                    border-radius: 8px;
                    font-size: 1rem;
                    font-weight: 600;
                    cursor: pointer;
                    transition: all 0.3s ease;

                    &:hover:not(:disabled) {
                        opacity: 0.9;
                        transform: translateY(-2px);
                        box-shadow: 0 8px 25px rgba(96, 165, 250, 0.3);
                    }

                    &:active:not(:disabled) {
                        transform: translateY(0);
                    }

                    &:disabled {
                        opacity: 0.7;
                        cursor: not-allowed;
                        transform: none;
                    }
                }
            }
        }
    }

    /* POW验证弹窗 */
    .modal-backdrop {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(0, 0, 0, 0.6);
        z-index: 100;
        backdrop-filter: blur(4px);
        display: flex;
        justify-content: center;
        align-items: center;
        opacity: 0;
        visibility: hidden;
        transition: all 0.3s ease;

        &.modal-visible {
            opacity: 1;
            visibility: visible;
        }

        .modal {
            position: relative;
            width: 90%;
            max-width: 500px;
            background: var(--card-bg);
            border: 1px solid var(--border-color);
            border-radius: 12px;
            box-shadow: 0 20px 60px var(--shadow-color);
            overflow: hidden;
            transform: translateY(20px) scale(0.95);
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);

            .modal-header {
                padding: 24px 24px 0;
                border-bottom: 1px solid var(--border-color);
                
                .modal-title {
                    font-size: 1.25rem;
                    color: var(--text-color);
                    font-weight: 600;
                    margin: 0 0 16px 0;
                }
            }

            .pow {
                padding: 24px;
                display: flex;
                align-items: center;
                justify-content: center;
                flex-direction: column;
                
                #pow-container {
                    min-width: 300px;
                    #pow {
                        min-width: 300px;
                    }
                }
                
                .pow-tip {
                    padding: 16px 0 0;
                    color: var(--muted-color);
                    font-size: 0.9rem;
                    margin: 0;
                    text-align: center;
                }
            }

            .modal-close {
                position: absolute;
                top: 16px;
                right: 16px;
                background: none;
                border: none;
                font-size: 1.5rem;
                cursor: pointer;
                color: var(--muted-color);
                width: 36px;
                height: 36px;
                border-radius: 50%;
                display: flex;
                align-items: center;
                justify-content: center;
                transition: all 0.2s ease;

                &:hover:not(:disabled) {
                    background-color: var(--hover-bg);
                    color: var(--text-color);
                }

                &:disabled {
                    opacity: 0.5;
                    cursor: not-allowed;
                }
            }
        }

        &.modal-visible .modal {
            transform: translateY(0) scale(1);
        }
    }
}

/* 动画定义 */
@keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
}

@keyframes slideUp {
    from { 
        opacity: 0;
        transform: translateY(30px) scale(0.9);
    }
    to { 
        opacity: 1;
        transform: translateY(0) scale(1);
    }
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

@keyframes progressGlow {
    0% { box-shadow: 0 0 5px rgba(52, 152, 219, 0.3); }
    100% { box-shadow: 0 0 20px rgba(52, 152, 219, 0.6); }
}
</style>