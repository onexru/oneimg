<template>
    <div class="pt-6 md:px-4 xl:container xl:mx-auto">
        <!-- 标签管理卡片 -->
        <div class="bg-white dark:bg-dark-200 rounded-xl shadow-md dark:shadow-dark-md p-5 transition-all duration-300 hover:shadow-lg dark:hover:shadow-dark-lg">
            <h2 class="section-title text-lg font-semibold mb-4 flex items-center gap-2">
                <i class="ri-bookmark-line text-primary"></i>
                标签管理
            </h2>
            
            <!-- 标签添加区域 -->
            <div class="flex justify-between items-center mb-6">
                <input type="text" 
                    v-model="tagInput"
                    @keyup.enter="handleAddTag"
                    class="flex-1 px-6 py-4 border min-w-[100px] border-light-300 dark:border-dark-100 rounded-lg bg-white dark:bg-dark-200 text-sm outline-none focus:ring-1 focus:ring-primary focus:border-primary transition-all"
                    placeholder="请输入标签(最多10个字符)..." 
                    maxlength="10"/>
                <button class="px-6 py-4 ml-3 text-sm font-semibold text-white bg-primary rounded-lg hover:bg-primary/80 transition-all" 
                        @click="handleAddTag"
                        :disabled="isAdding">
                    <span v-if="!isAdding" class="truncate text-overflow">添加</span>
                    <span v-else class="flex items-center gap-1">
                        <i class="ri-loader-2-line animate-spin"></i>
                        提交中
                    </span>
                </button>
            </div>
            
            <!-- 错误提示 -->
            <div v-if="errorMsg" class="mb-4 px-4 py-2 text-sm text-red-500 bg-red-50 dark:bg-red-900/20 rounded-lg">
                {{ errorMsg }}
            </div>
            
            <!-- 标签列表区域 -->
            <div class="tag-list-container">
                <h3 class="text-sm font-medium text-gray-600 dark:text-gray-300 mb-3">已创建标签</h3>
                
                <!-- 标签列表 -->
                <div class="flex flex-wrap gap-3">
                    <!-- 默认标签 -->
                    <div class="flex items-center px-4 py-2 bg-primary/10 dark:bg-primary/20 text-primary rounded-lg text-sm">
                        <span>默认</span>
                        <button class="ml-2 text-primary/70 hover:text-red-500 transition-colors" 
                                @click="handleDeleteTag(0, 0)"
                                :disabled="isDeleting">
                            <i class="ri-close-line"></i>
                        </button>
                    </div>

                    <!-- 标签项 -->
                    <div v-for="(tag, index) in tagList" :key="index" class="flex items-center px-4 py-2 bg-primary/10 dark:bg-primary/20 text-primary rounded-lg text-sm">
                        <span>{{ tag?.name || '未知'}}</span>
                        <button class="ml-2 text-primary/70 hover:text-red-500 transition-colors" 
                                @click="handleDeleteTag(tag.id, index)"
                                :disabled="isDeleting">
                            <i class="ri-close-line"></i>
                        </button>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
// 响应式数据
const tagInput = ref('');          // 标签输入框
const tagList = ref([]);           // 标签列表
const errorMsg = ref('');          // 错误提示
const isAdding = ref(false);       // 添加标签加载状态
const isDeleting = ref(false);     // 删除标签加载状态

// 初始化：加载已有标签
onMounted(() => {
    fetchTagList();
});

// 获取标签列表
const fetchTagList = async () => {
    try {
        // 替换为你的实际接口地址
        const response = await fetch('/api/tags', {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            }
        });
        
        const result = await response.json();
        if (response.ok && result.code === 200) {
            tagList.value = result.data?.list || [];
        } else {
            throw new Error(result.message || '获取标签列表失败');
        }
    } catch (error) {
        console.error('获取标签失败:', error);
        Message.error(error.message || '获取标签列表失败');
    }
};

// 处理添加标签
const handleAddTag = async () => {
    // 清空之前的错误提示
    errorMsg.value = '';
    
    // 1. 输入校验
    const tagName = tagInput.value.trim();
    if (!tagName) {
        errorMsg.value = '标签名称不能为空';
        return;
    }
    
    if (tagName.length > 10) {
        errorMsg.value = '标签名称不能超过10个字符';
        return;
    }
    
    // 2. 重复校验
    const isDuplicate = tagList.value.some(tag => tag.name === tagName);
    if (isDuplicate) {
        errorMsg.value = '该标签已存在，请勿重复添加';
        return;
    }
    
    // 3. 提交添加
    try {
        isAdding.value = true;
        
        // 替换为你的实际接口地址
        const response = await fetch('/api/tags', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            },
            body: JSON.stringify({ name: tagName })
        });
        
        const result = await response.json();
        if (response.ok && result.code === 200) {
            // 添加成功，更新列表
            tagList.value.push(result.data);
            tagInput.value = ''; // 清空输入框
            Message.success('标签添加成功');
        } else {
            throw new Error(result.message || '添加标签失败');
        }
    } catch (error) {
        console.error('添加标签失败:', error);
        errorMsg.value = error.message || '添加标签失败';
        Message.error(error.message || '添加标签失败');
    } finally {
        isAdding.value = false;
    }
};

// 处理删除标签
const handleDeleteTag = async (tagId, index) => {
    // 确认删除
    const modal = new PopupModal({
        title: '确认删除',
        content: `
        <div class="flex gap-3">
            <i class="fa fa-exclamation-triangle text-warning text-xl mt-1"></i>
            <div>
            <p>确定要删除这个标签吗？</p>
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
            modal.close()
            await deleteAsync(tagId, index)
            }
        }
        ],
        maskClose: true
    })
    modal.open()
};

const deleteAsync = async (tagId, index) => {
    try {
        isDeleting.value = true;
        
        // 替换为你的实际接口地址
        const response = await fetch(`/api/tags/${tagId}`, {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            }
        });
        
        const result = await response.json();
        if (response.ok && result.code === 200) {
            // 删除成功，更新列表
            tagList.value.splice(index, 1);
            Message.success('标签删除成功');
        } else {
            throw new Error(result.message || '删除标签失败');
        }
    } catch (error) {
        console.error('删除标签失败:', error);
        Message.error(error.message || '删除标签失败');
    } finally {
        isDeleting.value = false;
    }
}
</script>