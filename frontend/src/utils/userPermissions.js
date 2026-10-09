export const PERMISSION_GROUPS = [
  {
    title: '用户管理',
    items: [
      { code: 'user:create', name: '添加用户' },
      { code: 'user:delete', name: '删除用户' },
      { code: 'user:role:update', name: '修改角色' },
      { code: 'user:permission:update', name: '编辑权限' },
      { code: 'user:password:reset', name: '重置密码' },
    ]
  },
  {
    title: '内容与标签',
    items: [
      { code: 'tag:create', name: '新增Tag' },
      { code: 'tag:delete', name: '删除Tag' },
      { code: 'tag:update', name: '编辑Tag' },
    ]
  },
  {
    title: '存储管理',
    items: [
      { code: 'storage:create', name: '新增存储' },
      { code: 'storage:update', name: '编辑存储' },
      { code: 'storage:delete', name: '删除存储' },
    ]
  },
  {
    title: '图片管理',
    items: [
      { code: 'image:delete', name: '删除图片' },
      { code: 'image:tag:add', name: '添加图片标签' },
      { code: 'image:tag:delete', name: '删除图片标签' },
      { code: 'image:access:source', name: '图片存储源' },
    ]
  },
  {
    title: '系统设置',
    items: [
      { code: 'setting:list', name: '查看设置' },
      { code: 'setting:upload', name: '上传与存储' },
      { code: 'setting:image', name: '图片处理' },
      { code: 'setting:security', name: '安全与登录' },
      { code: 'setting:notification', name: '通知' },
      { code: 'setting:api', name: 'API' },
      { code: 'setting:seo', name: '站点SEO' },
    ]
  }
]
