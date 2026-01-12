<template>
  <div class="text-gray-800 dark:text-gray-200">
    <!-- 主要内容 -->
    <div class="gallery-content container mx-auto px-4 py-8">
      <!-- 顶部筛选栏 -->
      <div v-if="!loading" class="filter-bar mb-6 flex flex-wrap items-center justify-between gap-4">
        <div class="role-filter flex items-center gap-3">
          <div class="role-buttons flex rounded-lg border border-gray-300 dark:border-gray-700 overflow-hidden">
            <button
              @click="changeRole('admin')"
              class="px-4 py-2 text-sm transition-all"
              :class="[
                roleImage === 'admin' 
                  ? 'bg-primary text-white' 
                  : 'bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700'
              ]"
            >
              管理员
            </button>
            <button
              @click="changeRole('guest')"
              class="px-4 py-2 text-sm transition-all"
              :class="[
                roleImage === 'guest' 
                  ? 'bg-primary text-white' 
                  : 'bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700'
              ]"
            >
              游客
            </button>
          </div>
        </div>
        
        <!-- 批量操作 -->
        <div class="flex items-center gap-4">
          
          <div v-if="selectedImages.length > 0" class="batch-actions flex items-center gap-2">
            <button
            @click="handleBatchSetTag"
            class="px-4 py-2 text-sm bg-primary/10 hover:bg-primary/20 text-primary rounded-lg transition-all flex items-center gap-2">
                <i class="ri-bookmark-2-fill"></i>
                批量设置Tag
            </button>
            <!-- 批量删除按钮 - 游客和管理员都可见 -->
            <button
              @click="handleBatchDelete"
              class="px-4 py-2 text-sm bg-danger/10 hover:bg-danger/20 text-danger rounded-lg transition-all flex items-center gap-2"
            >
              <i class="ri-delete-bin-fill"></i>
              删除 ({{ selectedImages.length }})
            </button>
          </div>
        </div>
      </div>
      
      <!-- 全选复选框 - 游客和管理员都可见 -->
      <div v-if="!loading && images.length > 0" class="mb-4 flex items-center gap-2">
        <input
          type="checkbox"
          id="selectAll"
          class="w-4 h-4 rounded border-gray-300 text-primary focus:ring-primary"
          v-model="selectAll"
          @change="handleSelectAll"
        >
        <label for="selectAll" class="text-sm text-gray-600 dark:text-gray-400 cursor-pointer">
          全选
        </label>
      </div>

      <!-- 存储分类选择 -->
       <div class="max-w-[360px] mb-4 flex items-center gap-2">
        <div class="text-gray-600 dark:text-gray-400 text-sm">
            <span class="text-nowrap">存储分类：</span>
        </div>
        <select 
          class="w-full px-3 py-2 border border-light-300 dark:border-dark-100 rounded-lg bg-white dark:bg-dark-200 text-sm outline-none focus:ring-1 focus:ring-primary focus:border-primary transition-all"
          v-model="selectedBucket"
          @change="loadImages"
        >
          <option value="null">全部</option>
          <option 
            v-for="bucket in presetBuckets" 
            :key="bucket.id"
            :value="bucket.id"
          >{{ bucket.name }}</option>
        </select>
      </div>

      <!-- 标签分类选择 -->
       <div class="tags-container flex flex-wrap items-center gap-2 mb-4">
            <div class="text-gray-600 dark:text-gray-400 text-sm">
                <span class="text-nowrap">Tag分类：</span>
            </div>
            <div class="px-4 py-2 bg-primary/10 dark:bg-primary/20 text-primary rounded-lg text-sm cursor-pointer
            hover:ring-2 ring-primary ease-in-out duration-300 dark:ring-offset-gray-900"
            :class="{ 'ring-2 ring-primary dark:ring-offset-gray-900': isTagSelected(0) }"
            @click="handleTagSelection(0)">
                <span>默认</span>
            </div>
            <div
            v-if="presetTags.length > 0"
            v-for="tag in presetTags"
            class="px-4 py-2 bg-primary/10 dark:bg-primary/20 text-primary rounded-lg text-sm cursor-pointer
            hover:ring-2 ring-primary ease-in-out duration-300 dark:ring-offset-gray-900"
            :class="{ 'ring-2 ring-primary dark:ring-offset-gray-900': isTagSelected(tag.id) }"
            @click="handleTagSelection(tag.id)">
                <span>{{tag.name}}</span>
            </div>
       </div>
      
      <!-- 加载状态 -->
      <div v-if="loading" class="loading-container flex flex-col items-center justify-center py-20">
        <div class="spinner w-10 h-10 border-4 border-gray-200 dark:border-gray-700 border-t-primary dark:border-t-primary rounded-full animate-spin mb-4"></div>
        <p class="text-gray-600 dark:text-gray-400">加载中...</p>
      </div>
      
      <!-- 图片网格/列表 -->
      <div v-else-if="images.length > 0" class="images-container">
        <!-- 网格视图 -->
        <div v-if="viewMode === 'grid'" class="images-grid grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
          <div 
            v-for="image in images" 
            :key="image.id"
            class="image-card bg-white dark:bg-gray-800 rounded-xl shadow-md overflow-hidden hover:shadow-lg transition-all duration-300 cursor-pointer relative
            hover:ring-2 ring-primary ring-offset-2 ease-in-out duration-300 dark:ring-offset-gray-900"
            :class="{ 'ring-2 ring-primary ring-offset-2 dark:ring-offset-gray-900': isImageSelected(image.id) }"
          >
            <!-- 复选框 - 右上角位置 -->
            <div class="absolute top-3 right-3 z-10 p-0.5 rounded-full">
              <input
                type="checkbox"
                :id="`image-${image.id}`"
                class="w-5 h-5 rounded border-gray-300 text-primary focus:ring-primary bg-white dark:bg-gray-800 cursor-pointer"
                :checked="isImageSelected(image.id)"
                @change="(e) => handleImageSelection(image.id, e.target.checked)"
                @click.stop
              >
            </div>
            
            <div class="image-wrapper relative aspect-video overflow-hidden bg-gray-100 dark:bg-gray-900" @click="openPreview(image)">
              <!-- 显示图片所属角色 -->
              <span 
                class="image-role text-xs mt-1 px-2 py-0.5 rounded inline-block absolute left-[15px] top-[5px] z-[999]"
                :class="getRoleTagClass(image.user_id)"
              >
                {{ image.user_id == '1' ? '管理员' : '游客' }}
              </span>
              <span 
                class="image-role text-xs text-white mt-1 px-2 py-0.5 rounded-2xl inline-block absolute left-[75px] top-[5px] z-[999] bg-success"
              >
                {{ presetBuckets.find(bucket => bucket.id == image.bucket_id)?.name }}
              </span>
              <div class="loading absolute inset-0 flex items-center justify-center z-0 text-slate-300">
                <svg class="w-8 h-8 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="transform: scaleX(-1) scaleY(-1);">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
              </div>
              <img 
                :src="image.thumbnail || image.url" 
                :alt="image.filename"
                class="image-thumbnail w-full h-full object-cover transition-transform duration-500 hover:scale-105 opacity-0"
                loading="lazy"
                @load="handleImageLoad"
                @error="handleImageError"
              />
            </div>
            <div class="image-info p-3">
              <p class="image-filename font-medium text-sm truncate whitespace-nowrap overflow-hidden">{{ image.filename }}</p>
              <p class="image-meta text-xs text-gray-500 dark:text-gray-400 mt-1">
                {{ formatFileSize(image.file_size) }} • 
                {{ image.width }}×{{ image.height }}
              </p>
              <p class="image-date text-xs text-gray-500 dark:text-gray-400 mt-1">{{ formatDate(image.created_at) }}</p>
            </div>
          </div>
        </div>
        
        <!-- 分页 -->
        <div v-if="totalPages > 1" class="pagination flex flex-wrap items-center justify-center gap-2 py-8">
          <button 
            @click="changePage(currentPage - 1)"
            :disabled="currentPage <= 1"
            class="page-btn px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700 transition-all text-sm"
            :class="{ 'opacity-50 cursor-not-allowed': currentPage <= 1 }"
          >
            上一页
          </button>
          
          <div class="page-numbers flex gap-1">
            <button 
              v-for="page in visiblePages"
              :key="page"
              @click="changePage(page)"
              class="w-9 h-9 flex items-center justify-center rounded-lg border transition-all text-sm"
              :class="[
                page === currentPage 
                  ? 'bg-primary text-white border-primary' 
                  : 'border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700'
              ]"
            >
              {{ page }}
            </button>
          </div>
          
          <button 
            @click="changePage(currentPage + 1)"
            :disabled="currentPage >= totalPages"
            class="page-btn px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700 transition-all text-sm"
            :class="{ 'opacity-50 cursor-not-allowed': currentPage >= totalPages }"
          >
            下一页
          </button>
        </div>
      </div>
      
      <!-- 空状态 -->
      <div v-else class="empty-state flex flex-col items-center justify-center py-20 text-center">
        <div class="empty-icon text-6xl mb-4 text-gray-400 dark:text-gray-600">
          <i class="ri-image-ai-line"></i>
        </div>
        <h3 class="text-xl font-bold mb-2">暂无{{ roleImage === 'admin' ? '管理员' : '游客' }}图片</h3>
        <p class="text-gray-600 dark:text-gray-400 mb-6">
          还没有上传任何{{ roleImage === 'admin' ? '管理员' : '游客' }}图片，
          <router-link to="/" class="text-primary hover:underline">去上传一些吧</router-link>
        </p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed, onUnmounted, unref, watch } from 'vue'
import { useRouter } from 'vue-router'
import errorImg from '@/assets/images/error.webp';

// ====================== 常量定义 ======================
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';
const PAGE_SIZE = 20;
const ROLE_MAP = {
  admin: '管理员',
  guest: '游客'
};
const STORAGE_MAP = {
  default: '本地'
};

// ====================== 工具函数（抽离复用） ======================
/**
 * 获取完整的图片URL
 * @param {string} path - 图片相对路径
 * @returns {string} 完整URL
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
 * @param {number} bytes - 文件字节数
 * @returns {string} 格式化后的大小字符串
 */
const formatFileSize = (bytes) => {
  if (!bytes || isNaN(bytes)) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
};

/**
 * 格式化日期
 * @param {string} dateString - 日期字符串
 * @returns {string} 本地化日期字符串
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
 * 获取角色标签样式类
 * @param {string|number} userId - 用户ID
 * @returns {string} 样式类字符串
 */
const getRoleTagClass = (userId) => {
  return userId == '1' 
    ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' 
    : 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
};

/**
 * 序列化表单数据
 * @param {Object} modal - 弹窗实例
 * @returns {Object} 表单数据对象
 */
const serializeForm = (modal) => {
  const form = modal.content?.querySelector('form');
  if (!form) {
    console.warn('未找到表单元素');
    return {};
  }

  return Array.from(form.elements).reduce((acc, element) => {
    const { name, disabled, type, checked, value } = element;
    
    // 跳过无name、禁用的元素
    if (!name || disabled) return acc;
    
    // 处理复选框/单选框
    if ((type === 'checkbox' || type === 'radio') && !checked) return acc;
    
    // 处理文件输入
    if (type === 'file') {
      acc[name] = element.files.length > 0 ? element.files[0].name : '';
      return acc;
    }
    
    // 处理多值字段
    if (acc[name]) {
      acc[name] = Array.isArray(acc[name]) ? [...acc[name], value] : [acc[name], value];
    } else {
      acc[name] = value;
    }
    
    return acc;
  }, {});
};

// ====================== 响应式数据 ======================
const images = ref([]);
const loading = ref(false);
const viewMode = ref('grid');
const currentPage = ref(1);
const totalPages = ref(1);
const roleImage = ref("admin");
const isAdmin = ref(false);
const presetTags = ref([]);
const presetBuckets = ref([]);
const selectedBucket = ref(null);
const selectedImages = ref([]); // 选中的图片ID数组
const selectedTags = ref([]); // 选中的标签ID数组
const selectAll = ref(false); // 全选状态
const currentPreviewImage = ref(null);

// 路由实例
const router = useRouter();

// ====================== 计算属性 ======================
/**
 * 分页可见页码
 */
const visiblePages = computed(() => {
  const pages = [];
  const start = Math.max(1, currentPage.value - 2);
  const end = Math.min(totalPages.value, currentPage.value + 2);
  
  for (let i = start; i <= end; i++) {
    pages.push(i);
  }
  
  return pages;
});

// ====================== 监听器 ======================
/**
 * 监听全选状态变化（优化：双向绑定同步）
 */
watch(
  () => [selectedImages.value.length, images.value.length],
  ([selectedLen, imageLen]) => {
    selectAll.value = imageLen > 0 && selectedLen === imageLen;
  },
  { immediate: true }
);

// ====================== API 请求函数 ======================
/**
 * 获取标签列表
 */
const getTagsList = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/tags`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    
    const result = await response.json();
    if (response.ok && result.code === 200) {
      presetTags.value = result.data?.list || [];
    } else {
      throw new Error(result.message || '获取标签列表失败');
    }
  } catch (error) {
    console.error('获取标签失败:', error);
    Message.error(error.message || '获取标签列表失败');
  }
};

/**
 * 获取存储列表
 */
const getBucketsList = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/buckets/list`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    
    const result = await response.json();
    if (response.ok && result.code === 200) {
      presetBuckets.value = result.data || [];
    } else {
      throw new Error(result.message || '获取存储列表失败');
    }
  } catch (error) {
    console.error('获取存储列表失败:', error);
    Message.error(error.message || '获取存储列表失败');
  }
};  

/**
 * 加载图片列表
 */
const loadImages = async () => {
  if (loading.value) return; // 防止重复请求
  loading.value = true;
  
  try {
    const params = new URLSearchParams({
      page: currentPage.value,
      limit: PAGE_SIZE,
      sort_by: 'created_at',
      sort_order: 'desc',
      role: roleImage.value,
      tags: selectedTags.value,
      bucket: selectedBucket.value
    });
    
    const response = await fetch(`${API_BASE_URL}/api/images?${params}`, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    
    if (response.ok) {
      const result = await response.json();
      images.value = result.data.images || [];
      totalPages.value = result.data.total_pages || 1;
      selectedImages.value = []; // 重置选择状态
    } else {
      if (response.status === 401) {
        localStorage.removeItem('authToken');
        router.push('/login');
        Message.error('登录已过期，请重新登录');
        return;
      }
      throw new Error('加载图片失败');
    }
  } catch (error) {
    console.error('加载图片错误:', error);
    Message.error(`加载图片失败: ${error.message}`);
  } finally {
    loading.value = false;
  }
};

/**
 * 批量删除图片
 * @param {Array} deleteIds - 要删除的图片ID数组
 */
const batchDeleteImages = async (deleteIds) => {
  // 优化：并行删除（控制并发数）
  const promises = deleteIds.map(id => deleteAsync(id));
  await Promise.allSettled(promises);
  // 重新加载列表
  loadImages();
};

/**
 * 删除单张图片
 * @param {string|number} id - 图片ID
 * @returns {boolean} 是否删除成功
 */
const deleteAsync = async (id) => {    
  const loadingInstance = Loading.show({
    text: '删除中...',
    color: '#ff4d4f',
    mask: true
  });
  
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/${id}`, {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    
    if (response.ok) {
      Message.success('图片删除成功');
      selectedImages.value = selectedImages.value.filter(imageId => imageId !== id);
      // 重新加载列表
      loadImages();
      return true;
    } else {
      const result = await response.json();
      throw new Error(result.message || '删除失败');
    }
  } catch (error) {
    console.error('删除图片错误:', error);
    Message.error(`删除图片失败: ${error.message}`);
    return false;
  } finally {
    await loadingInstance.hide();
  }
};

/**
 * 给图片添加标签
 * @param {string|number} imageId - 图片ID
 * @param {Object} values - 表单值
 */
const pustImageTag = async (imageId, values) => {
  const { tag } = values;
  if (tag === '0') {
    Message.warning('请选择Tag标签');
    return;
  }
  
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/tag`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify({ id: imageId, tag })
    });

    const result = await response.json();
    if (response.ok && result.code === 200) {
      // 更新本地数据
      const image = images.value.find(item => item.id === imageId);
      if (image) {
        // 移除默认Tag
        image.tags = image.tags.filter(item => item.id !== 0);
        // 添加新Tag
        const newTag = presetTags.value.find(item => item.id === Number(tag));
        if (newTag) image.tags.push(newTag);
        // 更新预览图片
        currentPreviewImage.value = image;
        if (image) openPreview(image);
      }
      Message.success(result.message || '添加成功');
    } else {
      Message.error(result.message || '添加失败');
      openPreview(currentPreviewImage.value);
    }
  } catch (err) {
    Message.error(`出错了：${err.message}`);
    console.warn(err);
  }
};

// 删除图片标签
const deleteImageTagAsync = async (imageId, tagId) => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/tag`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify({ id: imageId, tag: tagId })
    });

    const result = await response.json();
    if (response.ok && result.code === 200) {
      // 更新本地数据
      const image = images.value.find(item => item.id === imageId);
      if (image) {
        // 移除Tag
        image.tags = image.tags.filter(item => item.id !== tagId);
        // 如果全部移除则添加默认Tag
        if(image.tags.length === 0){
            image.tags.push({ id: 0, name: '默认' });
        }
        // 更新预览图片
        currentPreviewImage.value = image;
      }
      Message.success(result.message || '删除成功');
      return true;
    } else {
      Message.error(result.message || '删除失败');
      return false;
    }
  } catch (err) {
    Message.error(`出错了：${err.message}`);
    console.warn(err);
    return false;
  }
};

// ====================== 事件处理函数 ======================
/**
 * 切换角色筛选
 * @param {string} role - 角色类型（admin/guest）
 */
const changeRole = (role) => {
  if (roleImage.value !== role) {
    roleImage.value = role;
    currentPage.value = 1;
    selectedImages.value = [];
    selectAll.value = false;
    loadImages();
  }
};

/**
 * 切换分页
 * @param {number} page - 目标页码
 */
const changePage = (page) => {
  if (page >= 1 && page <= totalPages.value) {
    currentPage.value = page;
    selectedImages.value = [];
    selectAll.value = false;
    loadImages();
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }
};

/**
 * 检查图片是否被选中
 * @param {string|number} imageId - 图片ID
 * @returns {boolean} 是否选中
 */
const isImageSelected = (imageId) => {
  return selectedImages.value.includes(imageId);
};

/**
 * 检查Tag是否被选中
 * @param {string|number} tagId - Tag ID
 * @returns {boolean} 是否选中
 */
const isTagSelected = (tagId) => {
  return selectedTags.value.includes(tagId);
};

/**
 * 处理单个Tag选择
 * @param {string|number} tagId - Tag ID
 * @param {boolean} isChecked - 是否选中
 */
const handleTagSelection = (tagId) => {
    if (!selectedTags.value.includes(tagId)) {
        selectedTags.value.push(tagId);
    }else{
        selectedTags.value = selectedTags.value.filter(id => id !== tagId);
    }
    // 加载图片
    loadImages();
};

/**
 * 处理单个图片选择
 * @param {string|number} imageId - 图片ID
 * @param {boolean} isChecked - 是否选中
 */
const handleImageSelection = (imageId, isChecked) => {
  if (isChecked) {
    if (!selectedImages.value.includes(imageId)) {
      selectedImages.value.push(imageId);
    }
  } else {
    selectedImages.value = selectedImages.value.filter(id => id !== imageId);
  }
};

/**
 * 处理全选
 * @param {Event} e - 事件对象
 */
const handleSelectAll = (e) => {
  const isChecked = e.target.checked;
  selectedImages.value = isChecked 
    ? images.value.map(image => image.id) 
    : [];
};

/**
 * 处理批量删除确认
 */
const handleBatchDelete = () => {
  if (selectedImages.value.length === 0) {
    Message.warning('请选择要删除的图片');
    return;
  }

  // 权限过滤
  const userInfo = JSON.parse(localStorage.getItem('userInfo') || '{}');
  let filterIds = selectedImages.value;
  if (!isAdmin.value && userInfo.id) {
    filterIds = selectedImages.value.filter(id => {
      const image = images.value.find(item => item.id === id);
      return image && image.user_id === userInfo.id;
    });
  }

  if (filterIds.length === 0) {
    Message.warning('你没有权限删除选中的图片');
    return;
  }
  
  const modal = new PopupModal({
    title: '批量删除确认',
    content: `
      <div class="flex gap-3">
        <i class="fa fa-exclamation-triangle text-warning text-xl mt-1"></i>
        <div>
          <p>确定要删除选中的 ${filterIds.length} 张图片吗？</p>
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
          await batchDeleteImages(filterIds);
        }
      }
    ],
    maskClose: true
  });
  modal.open();
};

/**
 * 批量设置标签
 */
const handleBatchSetTag = () => {
    if (selectedImages.value.length === 0) {
        Message.warning('请选择要编辑的图片');
        return;
    }

    // 已选择的图片
    const imageId = selectedImages.value;

    // 构建标签选项
    const tagList = [
        { value: "0", label: "请选择Tag", disabled: true }
    ];
    presetTags.value.forEach(tag => {
        tagList.push({ value: tag.id, label: tag.name });
    });

    const modal = new showFormModal({
        title: '批量编辑Tag',
        formFields: [
        {
            name: 'tag',
            label: 'Tag标签',
            type: 'select',
            required: true,
            defaultValue: "0",
            options: tagList,
            tip: "已选择的图片：<br>" + images.value.filter(item => imageId.includes(item.id)).map(item => item.filename).join("<br>")
        },
        ],
        buttons: [
        {
            text: '取消',
            type: 'default',
            callback: (modal) => {
            modal.close();
            }
        },
        {
            text: '删除Tag',
            type: 'danger',
            callback: (modal) => {
            const formData = serializeForm(modal);
            batchDeleteTag(formData);
            modal.close();
            }
        },
        {
            text: '添加Tag',
            type: 'primary',
            callback: (modal) => {
            const formData = serializeForm(modal);
            batchAddTag(formData);
            modal.close();
            }
        }
        ]
    });
    modal.open();
}

const batchDeleteTag = async (formData) => {
    if (formData.tag === "0") {
        Message.warning("请选择Tag");
        return;
    }

    try {
        const response = await fetch(`${API_BASE_URL}/api/images/tags`, {
        method: 'DELETE',
        headers: {
            'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            },
            body: JSON.stringify({
                image_ids: selectedImages.value,
                tag_id: formData.tag
            })
        });
        
        const result = await response.json();
        if (response.ok && result.code === 200) {
            Message.success('删除Tag成功');
            // 刷新列表
            await loadImages();
        } else {
            throw new Error(result.message || '删除Tag失败');
        }
    } catch (error) {
        console.error('删除Tag失败:', error);
        Message.error(error.message || '删除Tag失败');
    }
}

const batchAddTag = async (formData) => {
    if (formData.tag === "0") {
        Message.warning("请选择Tag");
        return;
    }
    try {
        const response = await fetch(`${API_BASE_URL}/api/images/tags`, {
        method: 'POST',
        headers: {
            'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            },
            body: JSON.stringify({
                image_ids: selectedImages.value,
                tag_id: formData.tag
            })
        });
        
        const result = await response.json();
        if (response.ok && result.code === 200) {
            Message.success('添加Tag成功');
            // 刷新列表
            await loadImages();
        } else {
            throw new Error(result.message || '添加Tag失败');
        }
    } catch (error) {
        console.error('添加Tag失败:', error);
        Message.error(error.message || '添加Tag失败');
    }
}

/**
 * 图片加载完成处理
 * @param {Event} e - 事件对象
 */
const handleImageLoad = (e) => {
  e.target.classList.remove('opacity-0');
  const loadingEl = e.target.parentElement.querySelector('.loading');
  if (loadingEl) loadingEl.classList.add('hidden');
};

/**
 * 图片加载错误处理
 * @param {Event} e - 事件对象
 */
const handleImageError = (e) => {
  // 使用导入的错误图片，而非base64硬编码
  e.target.src = errorImg;
  const loadingEl = e.target.parentElement.querySelector('.loading');
  if (loadingEl) loadingEl.classList.add('hidden');
};

/**
 * 打开图片预览
 * @param {Object} image - 图片对象
 */
const openPreview = (image) => {
  currentPreviewImage.value = image;
  
  // 生成预览弹窗内容
  const previewContent = generatePreviewContent(image);
  
  const customModal = new PopupModal({
    title: image.filename,
    content: previewContent,
    type: 'default',
    buttons: [
      {
        text: '确定',
        type: 'default',
        callback: (modal) => {
          modal.close();
          // 清理全局函数
          cleanPreviewGlobalFunctions();
        }
      }
    ],
    maskClose: true,
    zIndex: 10000,
    maxHeight: '90vh'
  });

  // 注册弹窗操作函数
  registerPreviewGlobalFunctions(customModal, image.id);

  customModal.open();
};

/**
 * 生成预览弹窗内容
 * @param {Object} image - 图片对象
 * @returns {string} HTML字符串
 */
const generatePreviewContent = (image) => {
  const roleClass = image.user_id == '1' 
    ? 'background-color: #e0f2fe; color: #0369a1; dark:background-color: #075985; dark:color: #bae6fd;' 
    : 'background-color: #dcfce7; color: #166534; dark:background-color: #14532d; dark:color: #bbf7d0;';
  
  // 生成标签HTML
  const tagsHtml = image.tags?.map(tag => `
    <div class="px-2 py-0.5 rounded bg-primary/10 dark:bg-primary/20 text-primary text-xs" data-tag-id="${tag.id}" data-image-id="${image.id}">
      <span>${tag.name}</span>
      <button
        onclick="window.deleteImageTag(event, ${image.id}, ${tag.id})"
        class="ml-1 text-primary/70 hover:text-primary/30">
        <i class="ri-close-line text-xs"></i>
      </button>
    </div>
  `).join('') || '';

  return `
    <div class="image-preview-popup w-full max-w-5xl max-h-[85vh] flex flex-col overflow-hidden bg-white dark:bg-dark-200">
      <!-- 顶部操作栏 -->
      <div class="preview-header bg-light-50 pb-2 flex justify-between items-center">
        <div class="flex items-center gap-2">
          <span class="text-xs px-2 py-0.5 rounded" style="${roleClass}">
            ${image.user_id == '1' ? '管理员' : '游客'}
          </span>
          <span class="text-xs text-white px-2 py-0.5 rounded bg-success">
            ${presetBuckets.value.find(bucket => bucket.id == image.bucket_id)?.name}
          </span>
        </div>
        <div class="flex gap-1">
          <!-- 下载按钮 -->
          <button 
            class="px-3 py-1.5 text-xs bg-light-100 dark:bg-dark-300 hover:bg-light-200 whitespace-nowrap dark:hover:bg-dark-400 text-secondary rounded-md transition-colors duration-200 flex items-center gap-1"
            onclick="event.stopPropagation(); window.downloadPreviewImage()"
          >
            <i class="ri-download-fill text-xs"></i>
            下载
          </button>
          <!-- 删除按钮 -->
          <button 
            class="px-3 py-1.5 text-xs bg-danger/10 hover:bg-danger/20 whitespace-nowrap text-danger rounded-md transition-colors duration-200 flex items-center gap-1"
            onclick="event.stopPropagation(); window.deletePreviewImage(${image.id})"
          >
            <i class="ri-delete-bin-fill text-xs"></i>
            删除
          </button>
        </div>
      </div>
      
      <!-- 预览图片区域 -->
      <div class="max-h-[360px] flex-1 overflow-auto flex items-center justify-center">
        <a 
          class="spotlight min-w-full max-w-full min-h-[260px] block" 
          href="${getFullUrl(image.url)}" 
          data-description="尺寸: ${image.width || '未知'}×${image.height || '未知'} | 大小: ${formatFileSize(image.file_size || 0)} | 上传日期：${formatDate(image.created_at)} | 角色：${image.user_id == '1' ? '管理员' : '游客'}"
        >
          <div class="relative max-w-full w-fill max-h-[360px] min-h-[260px] rounded-lg overflow-hidden animate-pulse flex items-center justify-center">
            <div class="absolute inset-0 flex items-center justify-center">
              <svg class="w-10 h-10 text-slate-300 animate-spin loading-svg" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="transform: scaleX(-1) scaleY(-1);">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </div>
            <img 
              src="${getFullUrl(image.url)}"
              alt="${image.filename}" 
              class="max-w-full w-fill max-h-[360px] min-h-[260px] object-contain rounded-lg relative z-10 opacity-0 transition-opacity duration-300"
              onload="this.classList.remove('opacity-0'); this.parentElement.classList.remove('animate-pulse'); this.parentElement.querySelector('.loading-svg').classList.add('hidden');"
              onerror="this.parentElement.classList.remove('animate-pulse'); this.classList.remove('opacity-0'); this.src='${errorImg}';"
            />
          </div>
        </a>
      </div>

      <!-- 图片复制区域 -->
      <div class="flex gap-1 flex-wrap items-center w-full mt-3 mb-3">
        <p class="mr-1 text-xs text-secondary font-semibold">复制：</p>
        <button 
          onclick="window.copyPreviewImageLink('url')"
          class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
          <i class="ri-link text-xs w-4 text-center text-white"></i> URL
        </button>
        <button 
          onclick="window.copyPreviewImageLink('html')"
          class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
          <i class="ri-code-fill text-xs w-4 text-center text-white"></i> HTML
        </button>
        <button 
          onclick="window.copyPreviewImageLink('markdown')"
          class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
          <i class="ri-markdown-fill text-xs w-4 text-center text-white"></i> Markdown
        </button>
      </div>

      <!-- Tags -->
      <div class="pt-2 flex flex-wrap gap-2 items-center">
        <p class="mr-1 text-xs text-secondary font-semibold">Tags：</p>
        ${tagsHtml}
        <button 
          onclick="window.addImageTag(${image.id})"
          class="flex items-center px-2 py-1 bg-success/10 dark:bg-success/20 text-success rounded-full text-xs hover:text-success/30 transition-colors">
          <i class="ri-add-line"></i>
        </button>
      </div>
      
      <!-- 底部信息栏 -->
      <div class="pt-2 flex flex-wrap gap-2 text-xs text-secondary">
        <div class="flex items-center gap-1.5">
          <i class="ri-ruler-line w-3.5 text-center"></i>
          尺寸: ${image.width || '未知'}×${image.height || '未知'}
        </div>
        <div class="flex items-center gap-1.5">
          <i class="ri-image-line w-3.5 text-center"></i>
          大小: ${formatFileSize(image.file_size || 0)}
        </div>
        <div class="flex items-center gap-1.5">
          <i class="ri-hard-drive-3-line"></i>
          存储: ${STORAGE_MAP[image.storage] || image.storage || '未知'}
        </div>
        <div class="flex items-center gap-1.5">
          <i class="ri-user-line"></i>
          角色: ${image.user_id == '1' ? '管理员' : '游客'}
        </div>
      </div>
    </div>
  `;
};

/**
 * 注册预览弹窗全局函数
 * @param {Object} modal - 弹窗实例
 * @param {string|number} imageId - 图片ID
 */
const registerPreviewGlobalFunctions = (modal, imageId) => {
  // 复制图片链接
  window.copyPreviewImageLink = (type) => {
    if (!currentPreviewImage.value) return;
    const image = currentPreviewImage.value;
    const fullUrl = getFullUrl(image.url);
    let copyText = '';
    
    switch (type) {
      case 'url': copyText = fullUrl; break;
      case 'html': copyText = `<img src="${fullUrl}" alt="${image.filename}" width="${image.width || ''}" height="${image.height || ''}">`; break;
      case 'markdown': copyText = `![${image.filename}](${fullUrl})`; break;
      default: copyText = fullUrl;
    }
    
    try {
      navigator.clipboard.writeText(copyText);
      Message.success(`已复制${type.toUpperCase()}格式链接`);
    } catch (error) {
      // 降级处理
      const textArea = document.createElement('textarea');
      textArea.value = copyText;
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand('copy');
      document.body.removeChild(textArea);
      Message.success(`已复制${type.toUpperCase()}格式链接`);
    }
  };

  // 下载图片
  window.downloadPreviewImage = () => {
    if (!currentPreviewImage.value) return;
    const image = currentPreviewImage.value;
    const link = document.createElement('a');
    link.href = getFullUrl(image.url);
    link.download = image.filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    Message.success('下载已开始');
  };

  // 删除预览图片
  window.deletePreviewImage = async (id) => {
    modal.close();
    await deleteImage(id);
  };

  // 添加标签
  window.addImageTag = (id) => {
    modal.close();
    addImageTagModal(id);
  };

  // 删除标签
  window.deleteImageTag = (event, imgId, tagId) => {
    if (tagId == 0) {
      Message.warning("无法删除默认Tag");
      return;
    }
    event.preventDefault();
    if(deleteImageTagAsync(imgId, tagId)){
        const tagDiv = document.querySelector(`[data-image-id="${imageId}"][data-tag-id="${tagId}"]`);
        // 如果当前标签是最后一个，则修改为默认
        if(currentPreviewImage.value.tags.length <= 1){
            tagDiv.querySelector('span').innerHTML = "默认";
            tagDiv.setAttribute('data-tag-id', '0');
            // 修改删除调用函数
            tagDiv.querySelector('button').setAttribute("onclick", `window.deleteImageTag(event, ${imageId}, 0)`);
        } else {
            if (tagDiv) tagDiv.remove();
        }
    }
  };

  // 切换复制菜单（预留）
  window.togglePreviewCopyMenu = () => {
    const dropdown = document.getElementById('previewCopyDropdown');
    const icon = document.getElementById('copyMenuIcon');
    if (dropdown && icon) {
      const isHidden = dropdown.classList.contains('hidden');
      if (isHidden) {
        dropdown.classList.remove('hidden', 'opacity-0', 'translate-y-[-5px]');
        dropdown.classList.add('block', 'opacity-100', 'translate-y-0');
        icon.classList.add('rotate-180');
      } else {
        dropdown.classList.add('hidden', 'opacity-0', 'translate-y-[-5px]');
        dropdown.classList.remove('block', 'opacity-100', 'translate-y-0');
        icon.classList.remove('rotate-180');
      }
    }
  };
};

/**
 * 清理预览弹窗全局函数
 */
const cleanPreviewGlobalFunctions = () => {
  [
    'togglePreviewCopyMenu',
    'copyPreviewImageLink',
    'downloadPreviewImage',
    'deletePreviewImage',
    'addImageTag',
    'deleteImageTag'
  ].forEach(fnName => delete window[fnName]);
};

/**
 * 打开删除图片确认弹窗
 * @param {string|number} imageId - 图片ID
 */
const deleteImage = async (imageId) => {
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
          deleteAsync(imageId);
        }
      }
    ],
    maskClose: true
  });
  modal.open();
};

/**
 * 打开添加标签弹窗
 * @param {string|number} imageId - 图片ID
 */
const addImageTagModal = async (imageId) => {
  // 构建标签选项
  const tagList = [
    { value: "0", label: "请选择Tag", disabled: true }
  ];
  presetTags.value.forEach(tag => {
    tagList.push({ value: tag.id, label: tag.name });
  });

  const modal = new showFormModal({
    title: '添加Tag',
    formFields: [
      {
        name: 'tag',
        label: 'Tag标签',
        type: 'select',
        required: true,
        defaultValue: "0",
        options: tagList
      },
    ],
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => {
          modal.close();
          // 重新打开预览
          const image = currentPreviewImage.value;
          if (image) openPreview(image);
        }
      },
      {
        text: '添加',
        type: 'primary',
        callback: (modal) => {
          const formData = serializeForm(modal);
          pustImageTag(imageId, formData);
          modal.close();
        }
      }
    ]
  });
  modal.open();
};

// ====================== 生命周期 ======================
onMounted(() => {
  // 初始化用户角色
  const userInfo = JSON.parse(localStorage.getItem('userInfo') || '{}');
  if (userInfo?.isTourist === true) {
    roleImage.value = "guest";
  } else {
    isAdmin.value = true;
  }
  
  // 加载数据
  getTagsList();
  getBucketsList();
  loadImages();
});

onUnmounted(() => {
  // 清理全局函数和资源
  cleanPreviewGlobalFunctions();
});
</script>