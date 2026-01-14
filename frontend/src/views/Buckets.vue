<template>
  <div class="pt-6 md:px-4 xl:container xl:mx-auto">
    <!-- 顶部标题 + 添加存储按钮 -->
    <div class="flex items-center justify-between mb-6">
      <h2 class="section-title text-xl font-semibold flex items-center gap-2">
        <i class="ri-database-2-line text-primary"></i>
        存储管理
      </h2>
      <button
        @click="AddBucketModal"
        class="px-4 py-2 bg-primary text-white rounded-lg flex items-center gap-1 hover:bg-primary/80 transition-colors"
      >
        <i class="ri-add-line"></i>
        添加存储
      </button>
    </div>

    <!-- 多存储卡片列表 -->
    <div class="grid grid-cols-[repeat(auto-fit,minmax(320px,1fr))] gap-6">
      <div
        v-for="storage in buckets"
        :key="storage.key"
        class="bg-white dark:bg-dark-200 rounded-xl shadow-md dark:shadow-dark-md p-5 transition-all duration-300 hover:shadow-lg dark:hover:shadow-dark-lg relative"
      >
        <h3 class="section-title text-lg font-semibold mb-4 flex items-center gap-2">
          {{ storage.name }}
          <span class="text-xs bg-gray-100 dark:bg-dark-300 text-gray-500 dark:text-gray-300 px-2 py-0.5 rounded-full">
            {{ storage.type === 'default' ? '默认存储' : storage.type.toUpperCase() }}
          </span>
        </h3>

        <div class="grid grid-cols-3 gap-3 mb-5">
          <div class="bg-gray-50 dark:bg-dark-100 rounded-lg p-3 border border-gray-100 dark:border-dark-300">
            <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">总容量</p>
            <h4 class="text-lg font-bold text-gray-800 dark:text-white">{{ storage.total_readable || '--' }}</h4>
          </div>
          <div class="bg-gray-50 dark:bg-dark-100 rounded-lg p-3 border border-gray-100 dark:border-dark-300">
            <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">已使用</p>
            <h4 class="text-lg font-bold text-gray-800 dark:text-white">{{ storage.usage_readable }}</h4>
          </div>
          <div class="bg-gray-50 dark:bg-dark-100 rounded-lg p-3 border border-gray-100 dark:border-dark-300">
            <p class="text-xs text-gray-500 dark:text-gray-400 mb-1">剩余容量</p>
            <h4 class="text-lg font-bold text-gray-800 dark:text-white">{{ storage.usage_free || '--' }}</h4>
          </div>
        </div>

        <div class="mb-5">
          <div class="flex items-center justify-between mb-2">
            <p class="text-sm text-gray-600 dark:text-gray-300">使用率：{{ storage.usage_percent }}%</p>
            <p class="text-xs text-gray-500 dark:text-gray-400">
              {{ storage.usage_readable }} / {{ storage.total_readable }}
            </p>
          </div>
          <div class="w-full h-2 bg-gray-200 dark:bg-dark-300 rounded-full overflow-hidden">
            <div
              class="h-full bg-blue-500 dark:bg-blue-400 rounded-full transition-all duration-500"
              :style="{ width: `${storage.usage_percent}%` }"
            ></div>
          </div>
        </div>

        <div v-if="storage.type !== 'default'" class="flex items-center justify-end gap-3 pt-3 border-t border-gray-200 dark:border-dark-300">
          <button
          @click="UpdateBucketModal(storage)"
          class="px-3 py-1.5 bg-primary text-white rounded-lg hover:bg-primary/80 transition-colors flex items-center gap-1 text-sm">
            <i class="ri-edit-fill"></i>
            编辑
          </button>
          <button
            @click="DeleteBucketModal(storage.id)"
            class="px-3 py-1.5 bg-danger text-white dark:bg-danger-300 text-danger-700 dark:text-danger-200 rounded-lg hover:bg-danger/80 text-sm">
            <i class="ri-delete-bin-7-fill"></i>
            删除存储
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue';
import message from '@/utils/message.js';
const buckets = ref([]);

const typeSpecificFields = {
  s3: [
    { name: 's3_endpoint', label: 'Endpoint', type: 'text', placeholder: '请输入 Endpoint', required: true},
    { name: 's3_access_key', label: 'AccessKey', type: 'text', placeholder: '请输入 AccessKey', required: true},
    { name: 's3_secret_key', label: 'SecretKey', type: 'text', placeholder: '请输入 SecretKey', required: true},
    { name: 's3_bucket', label: 'Bucket', type: 'text', placeholder: '请输入 Bucket', required: true},
    { name: 'capacity', label: '容量大小', type: 'number', placeholder: '请输入容量大小，单位 GB', required: true}
  ],
  r2: [
    { name: 'r2_endpoint', label: 'Endpoint', type: 'text', placeholder: '请输入 Endpoint', required: true},
    { name: 'r2_access_key', label: 'AccessKey', type: 'text', placeholder: '请输入 AccessKey', required: true},
    { name: 'r2_secret_key', label: 'SecretKey', type: 'text', placeholder: '请输入 SecretKey', required: true},
    { name: 'r2_bucket', label: 'Bucket', type: 'text', placeholder: '请输入 Bucket', required: true},
    { name: 'capacity', label: '容量大小', type: 'number', placeholder: '请输入容量大小，单位 GB', required: true}
  ],
  ftp: [
    { name: 'ftp_host', label: 'Host', type: 'text', placeholder: '请输入 Host', required: true, tip: '无需填写 ftp:// 或者 sftp://'},
    { name: 'ftp_port', label: 'Port', type: 'number', placeholder: 'FTP 默认端口号 21', required: true, defaultValue: 21 },
    { name: 'ftp_user', label: 'Username', type: 'text', placeholder: '请输入 Username', required: true},
    { name: 'ftp_pass', label: 'Password', type: 'password', placeholder: '请输入 Password', required: true},
    { name: 'capacity', label: '容量大小', type: 'number', placeholder: '请输入容量大小，单位 GB', required: true}
  ],
  webdav: [
    { name: 'webdav_url', label: 'URL', type: 'text', placeholder: '请填写 WebDav 地址', required: true},
    { name: 'webdav_user', label: 'Username', type: 'text', placeholder: '请输入 Username', required: true},
    { name: 'webdav_pass', label: 'Password', type: 'password', placeholder: '请输入 Password', required: true},
    { name: 'capacity', label: '容量大小', type: 'number', placeholder: '请输入容量大小，单位 GB', required: true}
  ],
  telegram: [
    { name: 'tg_bot_token', label: 'Bot Token', type: 'text', placeholder: '请输入 Token', required: true},
    { name: 'tg_receivers', label: 'Chat ID', type: 'text', placeholder: '请输入 Chat ID', required: true}
  ]
};

// 添加存储弹窗
const AddBucketModal = () => {
  const baseFields = [
    {
      name: 'name',
      label: '存储名称',
      type: 'text',
      placeholder: '请输入存储名称',
      required: true,
      tip: '存储名称不能超过10个字符',
    },
    {
      name: 'type',
      label: '存储类型',
      type: 'select',
      options: [
        { label: '请选择存储类型', value: '', disabled: true },
        { label: 'S3', value: 's3' },
        { label: 'R2', value: 'r2' },
        { label: 'FTP', value: 'ftp' },
        { label: 'WebDav', value: 'webdav' },
        { label: 'Telegram', value: 'telegram' },
      ],
      required: true,
      onChange: (_, type) => {
        modal.appendFormFields(typeSpecificFields[type] || [], ['name', 'type']);
      }
    }
  ];

  const modal = new PopupModal({
    title: '添加存储',
    type: 'form',
    formFields: baseFields,
    formSubmit: async (modal, formData) => {
      try {
        if (!formData.name || !formData.type) {
          message.warning('请填写存储名称和选择存储类型');
          return;
        }
        const response = await fetch('/api/buckets', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('authToken')}`
          },
          body: JSON.stringify(formData)
        });
        const result = await response.json();
        if (response.ok && result.code === 200) {
          message.success('存储添加成功');
          modal.close();
          GetBuckets();
        } else {
          message.error(result.message || '添加存储失败');
        }
      } catch (error) {
        console.error('添加存储失败:', error);
        message.error('添加存储失败，请稍后重试');
      }
    },
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: () => {
          modal.close();
        }
      },
      {
        text: '确认添加',
        type: 'primary',
        callback: (modal) => {
          modal.content.querySelector('form').dispatchEvent(
            new Event('submit', { bubbles: true })
          );
        }
      }
    ]
  });

  modal.open();
};

// 更新存储弹窗
const UpdateBucketModal = (bucket) => {
  const setValue = typeSpecificFields[bucket.type].map(field => ({
    ...field,
    defaultValue: field.name == 'capacity' ? formatCapacity(bucket[field.name]) : bucket.config[field.name]
  }));
  const modal = new PopupModal({
    title: '编辑存储',
    type: 'form',
    formFields: [
      { name: 'name', label: '存储名称', type: 'text', placeholder: '请输入存储名称', required: true, defaultValue: bucket.name },
      { name: 'type', label: '存储类型', type: 'select', disabled: true,
      tip: '存储类型不可修改；<br><b class="text-red-500">修改配置会导致已有的图片无法访问，请谨慎操作</b>',
      options: [
        { label: '请选择存储类型', value: '', disabled: true },
        { label: 'S3', value: 's3' },
        { label: 'R2', value: 'r2' },
        { label: 'FTP', value: 'ftp' },
        { label: 'WebDav', value: 'webdav' },
        { label: 'Telegram', value: 'telegram' },
      ], required: true, defaultValue: bucket.type },
      ...setValue
    ],
    formSubmit: async (modal, formData) => {
      try {
        if (!formData.name || !formData.type) {
          message.warning('请填写存储名称和选择存储类型');
          return;
        }
        const response = await fetch(`/api/buckets/update/${bucket.id}`, {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('authToken')}`
          },
          body: JSON.stringify(formData)
        });
        const result = await response.json();
        if (response.ok && result.code === 200) {
          message.success('存储更新成功');
          modal.close();
          GetBuckets();
        } else {
          message.error(result.message || '更新存储失败');
        }
      } catch (error) {
        console.error('更新存储失败:', error);
        message.error('更新存储失败，请稍后重试');
      }
    },
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: () => {
          modal.close();
        }
      },
      {
        text: '确认更新',
        type: 'primary',
        callback: (modal) => {
          modal.content.querySelector('form').dispatchEvent(
            new Event('submit', { bubbles: true })
          );
        }
      }
    ]
  });
  modal.open();
}

// 删除存储弹窗
const DeleteBucketModal = (id) => {
  const modal = new PopupModal({
    title: '删除存储',
    content: `
      <p>确定要删除该存储吗？</p>
      <p class="text-red-500">注意：存储下的图片存储信息也会一并删除（源文件除外），请谨慎操作！</p>
    `,
    type: 'confirm',
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
          try {
            const response = await fetch(`/api/buckets/${id}`, {
              method: 'DELETE',
              headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
              }
            });
            const result = await response.json();
            if (response.ok && result.code === 200) {
              message.success('存储删除成功');
              modal.close();
              GetBuckets();
            } else {
              message.error(result.message || '删除存储失败');
            }
          } catch (error) {
            console.error('删除存储失败:', error);
            message.error('删除存储失败，请稍后重试');
          }
        }
      }
    ]
  });
  modal.open();
};

// 获取存储列表
const GetBuckets = async () => {
  try {
    const response = await fetch('/api/buckets', {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    const result = await response.json();
    if (response.ok && result.code === 200) {
      buckets.value = result.data;
    } else {
      message.error(result.message || '获取存储列表失败');
    }
  } catch (error) {
    console.error('获取存储列表失败:', error);
    message.error('获取存储列表失败，请稍后重试');
  }
};

// 辅助函数，存储容量转换 B -> GB
const formatCapacity = (value) => {
  if (!value) return '0';
  return (value / 1024 / 1024 / 1024).toFixed(2);
};

onMounted(() => {
  GetBuckets();
});
</script>