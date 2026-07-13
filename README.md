# 初春图床系统

一个功能完整的现代化图床管理系统，基于 Vue.js 3 + Go 构建，支持POW验证、剪贴板上传等高级功能。

## 开发者

- [onexru](https://github.com/onexru)
- [雾创岛](https://www.tr0.cn)
- [打赏赞助](https://www.cv0.cn/donate)
- [QQ群](https://qm.qq.com/q/lzT9IDkKVG)

## API文档
- [API文档](https://www.tr0.cn/oneimgapi/)

## Demo
[初春图床v3.0](https://www.ip6s.com)

## 预览
![18bce5ad46f261fb288.webp](https://eta.im/uploads/2026/06/18bce5ad46f261fb288.webp)
![18bce5d81ad53298181.webp](https://eta.im/uploads/2026/06/18bce5d81ad53298181.webp)
![18bce5e035f58315808.webp](https://eta.im/uploads/2026/06/18bce5e035f58315808.webp)
![18c0e4f9e46e716b609.webp](https://eta.im/uploads/2026/07/18c0e4f9e46e716b609.webp)
![18bce6194f707b25401.webp](https://eta.im/uploads/2026/06/18bce6194f707b25401.webp)
![18bce624c55e939e703.webp](https://eta.im/uploads/2026/06/18bce624c55e939e703.webp)

**默认账号密码**
``` 默认账号
admin
```
``` 默认密码
123456
```
## 🐳 Docker 部署

### 环境要求
- Docker 20.10.0 或更高版本
- Docker Compose v2.0.0 或更高版本

### 使用 Docker Compose 部署

1. **克隆项目**
```bash
git clone https://github.com/onexru/oneimg.git
cd oneimg
```

2. **启动服务**
```bash
docker compose up -d
```
3. **访问系统**
- `http://localhost:8080`

5. **停止服务**
```bash
docker compose down
```

### 直接使用镜像
```bash
docker run -d \
--name oneimg \
-p 8080:8080 \
-v /data/oneimg:/app/data \
--restart unless-stopped \
onexru/oneimg-oneimg
```

### 获取TelegramID
使用机器人[@userinfobot](https://t.me/userinfobot) 发送/start 即可获取TelegramID

### 数据持久化
系统数据和上传的图片通过 Docker 数据卷保持持久化：
- 上传的图片存储在 `./uploads` 目录
- 数据库文件存储在 `./data` 目录

### 自定义配置
如需修改配置，可以通过环境变量或直接编辑 `.env` 文件：

## 功能特性

### 多存储支持
- 本地存储
- S3 兼容存储（R2、OSS等）
- WebDAV 存储
- FTP 存储
- Telegram 存储
- 图库中可为单张或批量图片选择访问链接实际读取的存储副本，默认读取本机，链接本身保持不变
- 远程存储源可临时停用且不会删除文件；停用期间停止新上传与同步，已选为访问源的图片自动回退本机

### 安全认证
- POW (工作量证明) 验证登录
- Session 会话管理
- 密码加密存储
- 可在系统设置中开启图片加密存储；本地、S3/R2、WebDAV、FTP 与 Telegram 均保存 AES-256-GCM 密文，访问时由程序统一解密
- 会话超时保护

> 加密存储只影响开启后新上传的图片，历史明文图片仍可正常访问。加密图片依赖 `.env` 中的 `CONFIG_SECRET`（未显式配置时会持久化到 `data/.config_secret`），请妥善备份且不要更换；开启后图片直链域名不可用。

### 图片上传
- **剪贴板粘贴直接上传** - 支持 Ctrl+V 粘贴上传
- 拖拽上传支持
- 批量文件选择上传
- 支持多种图片格式 (JPEG, PNG, GIF, WebP, SVG, BMP)
- 文件大小限制和格式验证
- 上传进度显示

### 图片管理
- 图片预览和详情查看
- 复制链接功能
- 图片信息展示

### 数据统计
- 仪表板概览
- 存储空间统计
- 实时数据更新

### 用户界面
- 现代化设计风格
- 响应式布局 (支持移动端)
- 深色/浅色主题
- 流畅的动画效果
- 直观的操作体验
