/**
 * 前端路由：页面映射、登录守卫与 SEO 配置加载。
 */
import { createRouter, createWebHistory } from 'vue-router'
import Message from '@/utils/message.js'

let seoStting = {
  seo_title: 'OneImg',
  seo_description: '',
  seo_keywords: '',
  seo_icp: '',
  public_security: '',
  seo_icon: ''
}

const seoBus = {
  callbacks: [],
  onUpdate: (cb) => seoBus.callbacks.push(cb),
  triggerUpdate: (data) => seoBus.callbacks.forEach(cb => cb(data))
}

let seoRequestPromise = null

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/Login.vue'),
    meta: { 
      title: '登录', 
      public: true 
    }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/Register.vue'),
    meta: { 
      title: '注册', 
      public: true 
       }
  },
  {
    path: '/',
    name: 'Home',
    component: () => import('@/views/Home.vue'),
    meta: {
      title: '首页'
    }
  },
  {
    path: '/gallery',
    name: 'Gallery',
    component: () => import('@/components/gallery/GalleryRoute.vue'),
    props: { management: false },
    meta: { title: '我的图库' }
  },
  {
    path: '/admin/gallery',
    name: 'AdminGallery',
    component: () => import('@/components/gallery/GalleryRoute.vue'),
    props: { management: true },
    meta: { title: '总图库', adminOnly: true }
  },
  {
    path: '/tags',
    name: 'Tags',
    component: () => import('@/views/Tags.vue'),
    meta: {
      title: '标签'
    }
  },
  {
    path: '/stats',
    name: 'Stats',
    component: () => import('@/views/Stats.vue'),
    meta: {
      title: '系统统计'
    }
  },
  {
    path: '/buckets',
    name: 'Buckets',
    component: () => import('@/views/Buckets.vue'),
    meta: {
      title: '存储列表'
    }
  },
  {
    path: '/users',
    name: 'Users',
    component: () => import('@/views/Users.vue'),
    meta: { 
      title: '用户管理', adminOnly: true
    }
  },
  {
    path: '/account',
    name: 'Account',
    component: () => import('@/views/Account.vue'),
    meta: { 
      title: '账户设置' 
    }
  },
  {
    path: '/settings',
    name: 'Settings',
    component: () => import('@/views/Settings.vue'),
    meta: { 
      title: '系统设置' 
    }
  }
]

// 创建路由实例
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes
})

// 获取SEO配置
const getSeoSetting = async () => {
  if (seoRequestPromise) return seoRequestPromise;

  seoRequestPromise = new Promise(async (resolve) => {
    try {
      const response = await fetch('/api/settings/seo', {
        method: 'GET',
        headers: { 'Content-Type': 'application/json' }
      });

      if (!response.ok) {
        throw new Error(`请求失败：${response.status} ${response.statusText}`);
      }

      const result = await response.json();

      if (result.code === 200 && result.data) {
        seoStting = { ...seoStting, ...result.data };
        window.seoStting = seoStting; // 挂载到全局
        seoBus.triggerUpdate(seoStting); // 更新SEO设置

        // 设置网站图标
        if (seoStting.seo_icon) {
          let favicon = document.querySelector('link[rel="icon"]');
          if (!favicon) {
            favicon = document.createElement('link');
            favicon.rel = 'icon';
            favicon.type = 'image/x-icon';
            document.head.appendChild(favicon);
          }
          favicon.href = seoStting.seo_icon;
        }
      } else {
        Message.error(result.message || '获取SEO设置失败：无数据');
      }
    } catch (error) {
      console.error('获取SEO设置失败:', error);
      Message.error(error.message || '获取SEO设置失败：网络异常');
    } finally {
      resolve(seoStting);
    }
  });

  return seoRequestPromise;
};

// 封装动态标题计算函数
const getPageTitle = (to) => {
  if (to.meta.title === '首页') {
    return seoStting.seo_title;
  }
  return to.meta.title ? `${to.meta.title} - ${seoStting.seo_title}` : seoStting.seo_title;
};

//全局前置守卫
router.beforeEach(async (to, from, next) => {
  try {
    // 等待SEO接口完成
    await getSeoSetting();

    // 设置页面标题
    document.title = getPageTitle(to);

    // 处理公开路由
    const isPublic = to.meta.public;
    if (isPublic) {
      return next();
    }

    // Browser metadata is only a cache; the cookie-authenticated status is authoritative.
    // 验证登录状态
    const response = await fetch('/api/user/status');
    if (!response.ok) {
      // 删除本地用户信息
      localStorage.removeItem('userInfo');
      window.refreshNavItems && window.refreshNavItems();
      throw new Error(`登录状态验证失败：${response.status}`);
    }

    const result = await response.json();
    if (result.code !== 200 || !result.data.logged_in) {
      localStorage.removeItem('userInfo');
      window.refreshNavItems && window.refreshNavItems();
      return next('/login');
    }

    const userInfo = {
      id: result.data.user_id ?? result.data.id,
      username: result.data.username,
      role: result.data.user_role ?? result.data.role,
      isTourist: Number(result.data.user_role ?? result.data.role) === 2,
      permission: { codes: result.data.permissions || result.data.permission?.codes || [] },
    };
    localStorage.setItem('userInfo', JSON.stringify(userInfo));
    window.refreshNavItems?.();

    // Role comes from this request, never from cached navigation metadata.
    if (to.meta.adminOnly && Number(userInfo.role) !== 1) {
      Message.error(to.name === 'Users' ? '用户管理仅管理员可访问' : '总图库仅管理员可访问');
      return next('/gallery');
    }

    // 所有验证通过，放行
    next();
  } catch (error) {
    // 避免多次调用next的警告
    if (!to.fullPath.includes('/login')) {
      next('/login');
    } else {
      next();
    }
  }
});

// 全局后置守卫
router.afterEach((to) => {
  document.title = getPageTitle(to);
});

window.seoBus = seoBus;

export default router
