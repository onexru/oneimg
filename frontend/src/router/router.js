import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { 
      title: '登录', 
      public: true  // 标记为公开路由，不需要登录即可访问
    }
  },
  {
    path: '/',
    name: 'Home',
    component: () => import('@/views/Home.vue'),
    meta: {
      title: '首页 - OneImg图床系统'
    }
  },
  {
    path: '/gallery',
    name: 'Gallery',
    component: () => import('@/views/Gallery.vue'),
    meta: {
      title: '图片画廊 - OneImg图床系统'
    }
  },
  {
    path: '/stats',
    name: 'Stats',
    component: () => import('@/views/Stats.vue'),
    meta: {
      title: '统计信息 - OneImg图床系统'
    }
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('@/views/Settings.vue'),
    meta: { 
      title: '系统设置 - OneImg图床系统' 
    }
  }
]

// 创建路由实例
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL), // 使用环境变量中的基础URL
  routes
})

// 全局前置守卫 - 处理页面标题和登录验证
router.beforeEach(async (to, from, next) => {
  document.title = to.meta.title || 'OneImg图床系统';
  const isPublic = to.meta.public;
  if (isPublic) {
    return next();
  }
  try {
    const response = await fetch('/api/user/status', {
      method: 'GET'
    });
    const result = await response.json()
    if (result.code === 200 && result.data.logged_in == true) {
      return next();
    }
    console.log(result);
    next('/login');
  } catch (error) {
    console.error('验证登录状态失败:', error);
    next('/login');
  }
});

export default router
