<template>
    <div class="text-gray-800 dark:text-gray-200">
        <!-- 页面头部 -->
        <div class="settings-header container mx-auto px-4 py-4">
            <h1 class="page-title flex items-center text-2xl md:text-3xl font-bold">
                设置
            </h1>
            <p class="page-description text-gray-600 dark:text-gray-400 mt-2">管理您的系统设置</p>
        </div>

        <!-- 主要内容 -->
        <div class="container mx-auto px-4 pb-16">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-8">
                <!-- 系统配置卡片 -->
                <div class="order-1 md:order-2 w-full p-0 mx-auto">
                    <div class="panel-content p-6 md:p-8 bg-white dark:bg-gray-800 rounded-xl shadow-md">
                        <h2 class="panel-title flex items-center text-xl font-semibold mb-8">
                            <span class="panel-icon mr-2 text-2xl">
                                <i class="ri-list-settings-line"></i>
                            </span>
                            系统配置
                        </h2>
                        
                        <div class="account-form space-y-6">
                            <!-- TG Bot Token：失去焦点保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="tg_bot_token">
                                    TG Bot Token
                                </label>
                                <input 
                                    id="tg_bot_token"
                                    v-model="systemSettings.tg_bot_token"
                                    type="text" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="请输入TG Bot Token"
                                    @blur="handleFieldBlur('tg_bot_token', systemSettings.tg_bot_token)"
                                />
                                <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                    存储选择Telegram时必填
                                </div>
                            </div>
                            
                            <!-- TG 通知接收者：失去焦点保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="tg_receivers">
                                    TG 通知接收者
                                </label>
                                <input 
                                    id="tg_receivers"
                                    v-model="systemSettings.tg_receivers"
                                    type="text" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="接收通知的TG用户ID"
                                    @blur="handleFieldBlur('tg_receivers', systemSettings.tg_receivers)"
                                />
                                <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                    存储选择Telegram时必填
                                </div>
                            </div>
                            
                            <!-- TG 通知文本：失去焦点保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="tg_notice_text">
                                    TG 通知文本
                                </label>
                                <input 
                                    id="tg_notice_text"
                                    v-model="systemSettings.tg_notice_text"
                                    type="text" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="自定义TG通知文本"
                                    @blur="handleFieldBlur('tg_notice_text', systemSettings.tg_notice_text)"
                                />
                                <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                    默认模板：{username} {date} 上传了图片 {filename}，存储容器[{StorageType}]
                                </div>
                            </div>
                            
                            <!-- 水印文本：失去焦点保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="watermark_text">
                                    图片水印文本
                                </label>
                                <input 
                                    id="watermark_text"
                                    v-model="systemSettings.watermark_text"
                                    type="text" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="图片水印文本"
                                    @blur="handleFieldBlur('watermark_text', systemSettings.watermark_text)"
                                />
                            </div>

                            <!-- 图片水印大小：失去焦点保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="watermark_size">
                                    图片水印大小
                                </label>
                                <input 
                                    id="watermark_size"
                                    v-model="systemSettings.watermark_size"
                                    type="text" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="图片水印大小"
                                    @blur="handleFieldBlur('watermark_size', systemSettings.watermark_size)"
                                />
                            </div>

                            <!-- 图片水印字体颜色 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="watermark_color">
                                    图片水印字体颜色
                                </label>
                                <input 
                                    id="watermark_color"
                                    v-model="systemSettings.watermark_color"
                                    type="text" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="图片水印字体颜色"
                                    @blur="handleFieldBlur('watermark_color', systemSettings.watermark_color)"
                                />
                                <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                    默认值为 #000000 黑色
                                </div>
                            </div>

                            <!-- 图片水印透明度：失去焦点保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="watermark_opac">
                                    图片水印透明度
                                </label>
                                <input 
                                    id="watermark_opac"
                                    v-model="systemSettings.watermark_opac"
                                    type="text" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="图片水印透明度"
                                    @blur="handleFieldBlur('watermark_opac', systemSettings.watermark_opac)"
                                />
                                <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                    默认值：0.5
                                </div>
                            </div>

                            <!-- 图片水印位置：下拉框变更保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="storage_type">
                                    图片水印位置
                                </label>
                                <select 
                                    id="watermark_pos"
                                    v-model="systemSettings.watermark_pos"
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    @change="handleSelectChange('watermark_pos', systemSettings.watermark_pos)"
                                >
                                    <option value="" disabled>请选择图片水印位置</option>
                                    <option value="top-left">左上角</option>
                                    <option value="top-right">右上角</option>
                                    <option value="bottom-left">左下角</option>
                                    <option value="bottom-right">右下角</option>
                                    <option value="center">居中</option>
                                </select>
                                <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                    系统默认右下角
                                </div>
                            </div>
                            <div class="setting-group"> 
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="referer_white_list">
                                    Referer来源白名单
                                </label>
                                <textarea 
                                    id="referer_white_list"
                                    v-model="systemSettings.referer_white_list"
                                    type="password"
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="Referer来源白名单，多个以英文逗号分隔"
                                    @blur="handleFieldBlur('referer_white_list', systemSettings.referer_white_list)"
                                    rows="4"
                                >
                                </textarea>
                                <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                    1. 仅需填写域名（支持主域名），多个以英文逗号分隔；<br>
                                    2. 无需填写协议（http://），无需填写端口（:80）；<br>
                                    3. 如果开启了来源白名单，那么仅能从这些来源访问图片资源（直接打开不受限制）
                                </div>
                            </div>

                            <div class="setting-group"> 
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="api_token">
                                    API Token
                                </label>
                                <div class="relative w-full">
                                    <input 
                                        id="api_token"
                                        v-model="systemSettings.api_token"
                                        type="text" 
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="API Token"
                                        @blur="handleFieldBlur('api_token', systemSettings.api_token)"
                                    />
                                    <button
                                        type="button"
                                        class="bg-primary absolute right-0 top-0 h-full hover:bg-primary-dark text-white px-3 py-[7px] rounded-r-lg transition-colors duration-200 flex items-center justify-center"
                                        @click="generateApiToken"
                                    >
                                        生成
                                    </button>
                                </div>
                                <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                    1. 用于调用 API 接口，在请求头 Authorization 字段中添加 oneimg_token={API Token}
                                </div>
                            </div>

                            <!-- 存储类型：下拉框变更保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="storage_type">
                                    存储类型
                                </label>
                                <select 
                                    id="storage_type"
                                    v-model="systemSettings.storage_type"
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    @change="handleSelectChange('storage_type', systemSettings.storage_type)"
                                >
                                    <option value="" disabled>请选择存储类型</option>
                                    <option value="default">本地存储</option>
                                    <option value="s3">S3</option>
                                    <option value="r2">R2</option>
                                    <option value="webdav">WebDav</option>
                                    <option value="ftp">FTP</option>
                                    <option value="telegram">Telegram</option>
                                </select>
                                <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                    选择Telegram存储必须使用海外服务器，否则无法正常上传、查看、删除图片
                                </div>
                            </div>

                            <!-- S3/R2配置：失去焦点保存 -->
                            <div v-if="['s3', 'r2'].includes(systemSettings.storage_type)" class="space-y-4 pt-2 border-t border-gray-200 dark:border-gray-700">
                                <h3 class="font-bold text-sm text-gray-800 dark:text-gray-200">S3/R2 配置</h3>
                                
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="s3_endpoint">
                                        S3 Endpoint
                                    </label>
                                    <input 
                                        id="s3_endpoint"
                                        v-model="systemSettings.s3_endpoint"
                                        type="text" 
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="如：s3.us-west-004.backblazeb2.com"
                                        @blur="handleFieldBlur('s3_endpoint', systemSettings.s3_endpoint)"
                                    />
                                </div>
                                
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="s3_access_key">
                                        S3 AccessKey
                                    </label>
                                    <input 
                                        id="s3_access_key"
                                        v-model="systemSettings.s3_access_key"
                                        type="text" 
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="S3访问密钥ID"
                                        @blur="handleFieldBlur('s3_access_key', systemSettings.s3_access_key)"
                                    />
                                </div>
                                
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="s3_secret_key">
                                        S3 SecretKey
                                    </label>
                                    <input 
                                        id="s3_secret_key"
                                        v-model="systemSettings.s3_secret_key"
                                        type="password" 
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="S3私有访问密钥"
                                        @blur="handleFieldBlur('s3_secret_key', systemSettings.s3_secret_key)"
                                    />
                                </div>
                                
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="S3Bucket">
                                        S3 Bucket
                                    </label>
                                    <input 
                                        id="S3Bucket"
                                        v-model="systemSettings.s3_bucket"
                                        type="text" 
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="存储桶名称"
                                        @blur="handleFieldBlur('s3_bucket', systemSettings.s3_bucket)"
                                    />
                                </div>
                            </div>

                            <!-- WebDAV配置：失去焦点保存 -->
                            <div v-if="systemSettings.storage_type === 'webdav'" class="space-y-4 pt-2 border-t border-gray-200 dark:border-gray-700"> 
                                <h3 class="font-bold text-sm text-gray-800 dark:text-gray-200">WebDav 配置</h3>

                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="webdav_url">
                                        WebDav URL
                                    </label>
                                    <input 
                                        id="webdav_url"
                                        v-model="systemSettings.webdav_url"
                                        type="text"
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="请填写 WebDav 地址"
                                        @blur="handleFieldBlur('webdav_url', systemSettings.webdav_url)"
                                    />
                                </div>
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="webdav_user">
                                        WebDav 用户名
                                    </label>
                                    <input 
                                        id="webdav_user"
                                        v-model="systemSettings.webdav_user"
                                        type="text"
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="请填写 WebDav 用户名"
                                        @blur="handleFieldBlur('webdav_user', systemSettings.webdav_user)"
                                    />
                                </div>
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="webdav_pass">
                                        WebDav 密码
                                    </label>
                                    <input 
                                        id="webdav_pass"
                                        v-model="systemSettings.webdav_pass"
                                        type="password"
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="请填写 WebDav 密码"
                                        @blur="handleFieldBlur('webdav_pass', systemSettings.webdav_pass)"
                                    />
                                </div>
                            </div>

                            <!-- FTP配置：失去焦点保存 -->
                            <div v-if="systemSettings.storage_type === 'ftp'" class="space-y-4 pt-2 border-t border-gray-200 dark:border-gray-700"> 
                                <h3 class="font-bold text-sm text-gray-800 dark:text-gray-200">FTP 配置</h3>

                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="ftp_host">
                                        FTP HOST
                                    </label>
                                    <input 
                                        id="ftp_host"
                                        v-model="systemSettings.ftp_host"
                                        type="text"
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="请填写 FTP IP或域名"
                                        @blur="handleFieldBlur('ftp_host', systemSettings.ftp_host)"
                                    />
                                </div>
                                <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                    直接填写IP或域名，无需填写 ftp:// 或者 sftp://
                                </div>
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="ftp_port">
                                        FTP 端口
                                    </label>
                                    <input 
                                        id="ftp_port"
                                        v-model="systemSettings.ftp_port"
                                        type="number"
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="FTP 默认端口号 21"
                                        @blur="handleFieldBlur('ftp_port', systemSettings.ftp_port)"
                                    />
                                </div>
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="ftp_user">
                                        FTP 用户名
                                    </label>
                                    <input 
                                        id="ftp_user"
                                        v-model="systemSettings.ftp_user"
                                        type="password"
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="请填写 FTP 用户名"
                                        @blur="handleFieldBlur('ftp_user', systemSettings.ftp_user)"
                                    />
                                </div>
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="ftp_pass">
                                        FTP 密码
                                    </label>
                                    <input 
                                        id="ftp_pass"
                                        v-model="systemSettings.ftp_pass"
                                        type="password"
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="请填写 FTP 登录密码"
                                        @blur="handleFieldBlur('ftp_pass', systemSettings.ftp_pass)"
                                    />
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- 系统设置卡片（开关部分不变） -->
                <div class="order-2 md:order-1 w-full p-0 mx-auto">
                    <!-- SEO设置卡片 -->
                    <div class="panel-content p-6 mb-4 md:p-8 bg-white dark:bg-gray-800 rounded-xl shadow-md space-y-6">
                        <h2 class="panel-title flex items-center text-xl font-semibold mb-8">
                            <span class="panel-icon mr-2 text-2xl">
                                <i class="ri-seo-line"></i>
                            </span>
                            SEO 设置
                        </h2>
                        <div class="setting-group"> 
                            <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="seo_title">
                                网站标题
                            </label>
                            <input 
                                id="seo_title"
                                v-model="systemSettings.seo_title"
                                type="text"
                                class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                placeholder="请输入网站标题"
                                @blur="handleFieldBlur('seo_title', systemSettings.seo_title)"
                            />
                        </div>
                        <div class="setting-group"> 
                            <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="seo_description">
                                网站标题
                            </label>
                            <textarea
                                id="seo_description"
                                v-model="systemSettings.seo_description"
                                type="text"
                                class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                rows="3"
                                placeholder="请输入网站描述"
                                @blur="handleFieldBlur('seo_description', systemSettings.seo_description)"
                            ></textarea>
                        </div>
                        <div class="setting-group"> 
                            <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="seo_keywords">
                                网站关键词
                            </label>
                            <textarea
                                id="seo_keywords"
                                v-model="systemSettings.seo_keywords"
                                type="text"
                                class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                rows="3"
                                placeholder="请输入网站关键词"
                                @blur="handleFieldBlur('seo_keywords', systemSettings.seo_keywords)"
                            ></textarea>
                        </div>
                        <div class="setting-group"> 
                            <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="seo_icp">
                                网站备案号
                            </label>
                            <input 
                                id="seo_icp"
                                v-model="systemSettings.seo_icp"
                                type="text"
                                class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                placeholder="请输入网站备案号"
                                @blur="handleFieldBlur('seo_icp', systemSettings.seo_icp)"
                            />
                            <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                输入网站备案号会在页面底部显示备案信息
                            </div>
                        </div>
                        <div class="setting-group"> 
                            <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="public_security">
                                网站公安备案号
                            </label>
                            <input 
                                id="public_security"
                                v-model="systemSettings.public_security"
                                type="text"
                                class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                placeholder="请输入网站公安备案号"
                                @blur="handleFieldBlur('public_security', systemSettings.public_security)"
                            />
                            <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                输入网站公安备案号会在页面底部显示公安备案信息
                            </div>
                        </div>
                        <div class="setting-group"> 
                            <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="seo_icon">
                                网站小图标
                            </label>
                            <input 
                                id="seo_icon"
                                v-model="systemSettings.seo_icon"
                                type="text"
                                class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                placeholder="请输入网站小图标"
                                @blur="handleFieldBlur('seo_icon', systemSettings.seo_icon)"
                            />
                            <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                输入网站小图标URL会替换默认的小图标
                            </div>
                        </div>
                    </div>


                    <div class="panel-content p-6 md:p-8 bg-white dark:bg-gray-800 rounded-xl shadow-md">
                        <h2 class="panel-title flex items-center text-xl font-semibold mb-8">
                            <span class="panel-icon mr-2 text-2xl">
                                <i class="ri-settings-2-line"></i>
                            </span>
                            系统设置
                        </h2>
                        
                        <div class="account-form space-y-6">
                            <!-- 是否保存原图 -->
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    保存原图
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.original_image"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('original_image', systemSettings.original_image)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-green-500 dark:peer-checked:bg-green-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            
                            <!-- 其他开关省略（和之前一致） -->
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    保存WEBP格式
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.save_webp"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('save_webp', systemSettings.save_webp)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-green-500 dark:peer-checked:bg-green-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    生成缩略图
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.thumbnail"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('thumbnail', systemSettings.thumbnail)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-green-500 dark:peer-checked:bg-green-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">生成缩略图，可提升后台预览速度，上传速度稍慢</div>
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    允许游客上传
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.tourist"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('tourist', systemSettings.tourist)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-green-500 dark:peer-checked:bg-green-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    启用TG通知
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.tg_notice"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('tg_notice', systemSettings.tg_notice)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-blue-500 dark:peer-checked:bg-blue-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">国内服务器不要开启TG通知</div>
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    启用POW验证
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.pow_verify"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('pow_verify', systemSettings.pow_verify)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-purple-500 dark:peer-checked:bg-purple-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    开启图片水印
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.watermark_enable"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('watermark_enable', systemSettings.watermark_enable)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-green-500 dark:peer-checked:bg-green-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">新上传的图片自动添加水印，已上传的图片不会添加水印，可以通过图片外链传入GET参数添加水印。</div>
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    开启来源白名单
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.referer_white_enable"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('referer_white_enable', systemSettings.referer_white_enable)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-green-500 dark:peer-checked:bg-green-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    启用API
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.start_api"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('start_api', systemSettings.start_api)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-green-500 dark:peer-checked:bg-green-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">启用API必须设置API Token。</div>
                        </div>
                    </div>
                </div>

            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import message from '@/utils/message.js'

const systemSettings = reactive({
    id: 1,
    original_image: false,
    save_webp: false,
    thumbnail: false,
    tourist: false,
    tg_notice: false,
    pow_verify: false,
    tg_bot_token: '',
    tg_receivers: '',
    tg_notice_text: '',
    storage_type: '',
    s3_endpoint: '',
    s3_access_key: '',
    s3_secret_key: '',
    s3_bucket: '',
    webdav_url: '',
    webdav_user: '',
    webdav_pass: '',
    ftp_host: '',
    ftp_port: 21,
    ftp_user: '',
    ftp_pass: '',
    watermark_enable: '',
    watermark_text: '',
    watermark_pos: '',
    watermark_size: '',
    watermark_color: '',
    watermark_opac: '',
    referer_white_list: '',
    referer_white_enable: false,
    seo_title: '',
    seo_description: '',
    seo_keywords: '',
    seo_icp: '',
    public_security: '',
    seo_icon: '',
    api_token: '',
    start_api: false
})

const updateSetting = reactive({})

// 加载状态
const isUpdating = ref(false)
let debounceTimer = null

// 统一请求头配置（复用）
const getRequestHeaders = () => {
    return {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
    }
}

const saveSetting = async (key, value) => {
    if (updateSetting?.[key] === value) {
        return
    }
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(async () => {
        try {
            if (isUpdating.value) return
            isUpdating.value = true

            const response = await fetch('/api/settings/update', {
                method: 'POST',
                headers: getRequestHeaders(),
                body: JSON.stringify({
                    key: key,
                    value: value
                })
            })
            
            const result = await response.json()
            
            if (response.ok && result.code === 200) {
                message.success(`更新成功`)
                updateSetting[key] = value
            } else {
                // 更新失败自动回滚
                if (updateSetting[key]) {
                    systemSettings[key] = updateSetting[key]
                }
                message.error(`更新失败：${result.message || '未知错误'}`)
            }
        } catch (error) {
            console.error(`保存失败:`, error)
            message.error(`更新失败：网络异常`)
        } finally {
            isUpdating.value = false
        }
    }, 300)
}

const generateApiToken = () => {
    const token = generate32BitTokenMixCase();
    systemSettings.api_token = token;
    saveSetting('api_token', token)
}

const generate32BitTokenMixCase = () => {
  const chars = '0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz';
  let token = '';
  for (let i = 0; i < 32; i++) {
    const randomIndex = Math.floor(Math.random() * chars.length);
    token += chars[randomIndex];
  }
  return token;
}

// 开关状态变更统一处理方法
const handleSwitchChange = (key, value) => {
    if (key == "start_api") {
        if (systemSettings.api_token == '') {
            message.warning('请先填写API Token')
            systemSettings.start_api = false
            return
        }
    }
    saveSetting(key, value)
}

// 输入框失去焦点处理
const handleFieldBlur = (key, value) => {
    if (key == 'tg_bot_token' || key == 'tg_receivers') {
        if (systemSettings.tg_bot_token == '' || systemSettings.tg_receivers == '') {
            if(systemSettings.storage_type === 'telegram'){
                // 未设置机器人令牌和接收者列表时，自动切回默认存储方式
                message.warning('配置不完整，切回默认存储方式')
                setTimeout(() => {
                    systemSettings.storage_type = 'default'
                    saveSetting("storage_type", 'default')
                }, 1500)
                
                return
            }
        }
    }
    if (key == 'api_token' && value == '') {
        if (systemSettings.start_api) {
            setTimeout(() => {
                saveSetting("start_api", false)
                systemSettings.start_api = false
            }, 500);
        }
    }
    saveSetting(key, value)
}

// 下拉框变更处理
const handleSelectChange = (key, value) => {
    if(key == 'storage_type' && value === 'telegram'){
        if(!systemSettings.tg_bot_token || !systemSettings.tg_receivers){
            message.warning('请先填写机器人令牌和接收者列表')
            // 重置下拉
            if(updateSetting?.storage_type) {
                systemSettings.storage_type = updateSetting.storage_type
            } else {
                systemSettings.storage_type = 'default'
            }
            return
        }
    }
    saveSetting(key, value)
}

// 获取系统设置
const getSettings = async () => { 
    try {
        const response = await fetch('/api/settings/get', {
            method: 'GET',
            headers: getRequestHeaders(),
        })
        
        if (!response.ok) {
            throw new Error(`请求失败：${response.status}`)
        }
        
        const result = await response.json()
        
        if (result.code === 200 && result.data) {
            Object.assign(systemSettings, result.data)
            Object.assign(updateSetting, result.data)
        } else {
            message.error(result.message || '获取设置失败：无数据')
        }
    } catch (error) {
        console.error('获取设置失败:', error)
        message.error(error.message || '获取设置失败：网络异常')
    }
}

onMounted(() => {
    getSettings()
})
</script>

<style scoped>
.switch-transition {
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}
.switch-antialias {
    transform: translateZ(0);
}
</style>