# <div align="center">初春图床 OneImg</div>

<p align="center">
  现代化自托管图床系统，兼顾上传体验、存储灵活性与后台可控性。<br />
  基于 <code>Vue 3</code> + <code>Go</code> 构建，适合个人站点、博客、团队内容管理和多存储备份场景。
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Vue-3-42b883?style=for-the-badge&logo=vue.js&logoColor=white" />
  <img src="https://img.shields.io/badge/Go-Gin-00ADD8?style=for-the-badge&logo=go&logoColor=white" />
  <img src="https://img.shields.io/badge/SQLite-✔-003B57?style=for-the-badge&logo=sqlite&logoColor=white" />
  <img src="https://img.shields.io/badge/MySQL-✔-4479A1?style=for-the-badge&logo=mysql&logoColor=white" />
  <img src="https://img.shields.io/badge/PostgreSQL-✔-336791?style=for-the-badge&logo=postgresql&logoColor=white" />
  <img src="https://img.shields.io/badge/Docker-Ready-2496ED?style=for-the-badge&logo=docker&logoColor=white" />
  <img src="https://img.shields.io/badge/Multi_Storage-S3%20%7C%20R2%20%7C%20FTP%20%7C%20WebDAV%20%7C%20Telegram-6f42c1?style=for-the-badge" />
</p>

<p align="center">
  <img src="https://img.shields.io/github/stars/onexru/oneimg?style=flat-square" />
  <img src="https://img.shields.io/github/forks/onexru/oneimg?style=flat-square" />
  <img src="https://img.shields.io/github/issues/onexru/oneimg?style=flat-square" />
  <img src="https://img.shields.io/github/last-commit/onexru/oneimg?style=flat-square" />
  <img src="https://img.shields.io/github/repo-size/onexru/oneimg?style=flat-square" />
</p>

<p align="center">
  <a href="https://demo.eta.im">在线演示</a> ·
  <a href="https://www.tr0.cn/oneimgapi/">API 文档</a> ·
  <a href="https://www.tr0.cn">作者主页</a> ·
  <a href="https://www.cv0.cn/donate">赞助支持</a> ·
  <a href="https://qm.qq.com/q/lzT9IDkKVG">QQ 群</a>
</p>

<p align="center">
  <a href="#功能特性">功能特性</a> ·
  <a href="#界面预览">界面预览</a> ·
  <a href="#快速开始">快速开始</a> ·
  <a href="#部署说明">部署说明</a>
</p>

<p align="center">
  <img src="https://eta.im/uploads/2026/06/18bce5d81ad53298181.webp" width="92%" />
</p>

---

## 项目简介

OneImg 是一个功能完整的自托管图床与图片管理系统。它的核心目标不只是「能传图」，而是将**上传、管理、访问、同步和存储切换**整合成一套顺手的完整工作流。

相比传统图床方案，OneImg 在以下几个维度上做了更深的设计：

- **自托管可控**：数据完全掌握在自己手中，避免平台绑定与服务关停风险
- **多存储统一管理**：同时接入本地磁盘、对象存储和第三方通道，一套界面管理所有存储后端
- **链接稳定可迁移**：图片 URL 可长期保持不变，底层存储随时切换或扩展，对访问方完全透明
- **完整后台能力**：用户体系、权限控制、标签分类、容量统计、系统配置一应俱全

**适用场景**：个人博客与技术站点图床、团队文档配图管理、需要数据可控的自托管图片服务、以及希望将本地存储与对象存储/Telegram/FTP/WebDAV 统一纳入管理的环境。

---

## 功能特性

### 上传体验

支持多种上传方式，覆盖日常内容生产的全部习惯：

- **Ctrl+V 剪贴板粘贴**：截图后直接粘贴即传，无需额外操作
- **拖拽上传**：将图片拖入上传区域即可
- **批量选择**：一次选择多张图片，配合实时进度条
- **URL 远程上传**：输入图片链接，服务端拉取后入库
- 支持格式：JPEG / PNG / GIF / WebP（按实际图片字节校验，不信任浏览器声明的 MIME）

### 存储架构

支持 6 种存储后端，可按需组合使用：

| 存储类型 | 说明 |
|----------|------|
| 本地磁盘 | 默认存储，数据直接落盘 |
| Cloudflare R2 | 独立存储类型，支持专用端点与 CDN、浏览器直传 |
| S3 兼容 | 支持 Cloudflare R2、阿里云 OSS 等 S3 协议存储 |
| FTP | 传统 FTP 服务器 |
| WebDAV | 支持 Nextcloud、坚果云等 WebDAV 服务 |
| Telegram | 通过 Telegram Bot 将图片发送到频道/私聊 |

**核心设计理念**：

- **稳定链接**：每张图片拥有固定 URL，不因存储切换而改变
- **访问源切换**：不换链接即可切换实际读取的存储副本，方便在多个存储间调度
- **多存储同步**：本机与远程存储可协同工作，一张图可同时存在于多个存储后端
- **故障自动回退**：远程源停用时，已选该源的图片自动回退到本机读取，不中断服务、不删除文件

### 安全机制

从登录到存储，多层防护组合使用：

- **POW 工作量证明登录**：暴力破解成本指数级提升，有效抵御自动化攻击
- **AES-256-GCM 图片加密**：支持全部存储后端，加密后的文件即使泄露也无法直接查看
- **Session 会话管理**：超时自动失效，防止会话劫持
- **密码 bcrypt 加密**：拒绝明文存储，符合安全最佳实践
- **配置加密**：敏感配置项（如存储密钥）支持加密存储

> 如果启用了图片加密存储，请务必妥善保管 `CONFIG_SECRET`。更换或丢失该密钥会导致历史加密内容无法正常解密。

### 管理后台

提供完整的 Web 管理界面，所有操作集中在一处完成：

- **图片管理**：列表浏览、详情查看、标签分类、批量删除
- **用户与权限**：多用户体系，按角色分配操作权限
- **存储管理**：各存储后端状态监控、容量统计、同步状态追踪
- **系统设置**：全局配置、安全策略、上传限制等
- **数据概览**：仪表板实时展示上传量、存储用量等关键指标

### 界面设计

- 深色 / 浅色主题无缝切换
- 全响应式布局，移动端友好
- 流畅过渡动画，操作反馈明确

---

## 界面预览

> 现代化后台布局，覆盖上传、图库、标签、存储管理、用户权限与系统设置等核心场景。

<p align="center">
  <img src="https://eta.im/uploads/2026/06/18bce5ad46f261fb288.webp" width="48%" />
  <img src="https://eta.im/uploads/2026/06/18bce5d81ad53298181.webp" width="48%" />
</p>
<p align="center">
  <img src="https://eta.im/uploads/2026/06/18bce5e035f58315808.webp" width="48%" />
  <img src="https://eta.im/uploads/2026/07/18c0e4f9e46e716b609.webp" width="48%" />
</p>
<p align="center">
  <img src="https://eta.im/uploads/2026/06/18bce6194f707b25401.webp" width="48%" />
  <img src="https://eta.im/uploads/2026/06/18bce624c55e939e703.webp" width="48%" />
</p>

---

## 快速开始

> 几分钟即可跑起来，适合先本地体验，再逐步接入对象存储或远程存储。

### Docker Compose

```bash
git clone https://github.com/onexru/oneimg.git
cd oneimg
docker compose up -d
docker compose exec oneimg cat /app/data/.initial_admin_password
```

启动后访问 `http://localhost:8080`

### Docker 直接运行

```bash
docker run -d \
  --name oneimg \
  -p 8080:8080 \
  -e APP_URL=https://img.example.com \
  -v /data/oneimg:/app/data \
  -v /data/oneimg-uploads:/app/uploads \
  --restart unless-stopped \
  onexru/oneimg-oneimg

# 查看首次启动生成的随机管理员密码
docker exec oneimg cat /app/data/.initial_admin_password
```

### 初始账号与密码

```text
账号：admin（由 DEFAULT_USER 决定）
密码：首次启动自动生成，见 data/.initial_admin_password
```

密码生成规则：

| `DEFAULT_PASS` | 行为 |
|---|---|
| 留空（推荐） | 仅在数据库中还没有任何用户时，生成 43 字符随机密码写入 `data/.initial_admin_password`（权限 `0600`），**不打印到日志** |
| 非空 | 直接使用该值作为初始管理员密码 |

- 该文件只在首次创建管理员时写入；之后重启不会覆盖，也不会重置已有账号的密码。
- 密码**不会**出现在启动日志里，必须读取该文件获取。
- 登录后请立即修改密码，并删除该文件。

> ⚠️ 历史 `.env` 模板里曾有 `DEFAULT_PASS=123456`，若你的 `.env` 还保留这一行，它会被当作初始密码注入。请删除该行或改成强密码。仓库现在提供的模板已改为留空。

---

## 部署说明

### 环境要求

- Docker 20.10.0+
- Docker Compose v2.0.0+

### 数据持久化

建议挂载以下目录以保证数据安全：

| 目录 | 用途 |
|------|------|
| `./data` | 数据库与系统配置 |
| `./uploads` | 本地上传文件 |

支持三种数据库：
1. sqlite （默认）
2. mysql
3. postgres

> 修改数据库在`.env`内`DB_TYPE`配置

MySQL / PostgreSQL 的 TLS 由 `DB_TLS_MODE` 控制：

| 取值 | 行为 |
|---|---|
| `auto`（默认） | `DB_HOST` 解析为本机/私有地址（含 Docker 网络）时按 `preferred`，公网地址时按 `verify-full` |
| `disable` | 不使用 TLS |
| `preferred` | 服务端支持则使用 TLS，不校验证书；不支持则明文 |
| `required` | 必须使用 TLS，不校验证书 |
| `verify-ca` | 必须使用 TLS，校验 CA（`DB_CA_CERT_PATH`，留空使用系统 CA），不校验主机名 |
| `verify-full` | 必须使用 TLS，校验 CA 和主机名（`DB_TLS_SERVER_NAME`，留空等于 `DB_HOST`） |

同一 Docker 网络内的自建数据库（例如 `DB_HOST=mysql`）无需额外配置；连接云厂商 VPC 内的托管数据库时，如需严格校验请显式设置 `DB_TLS_MODE=verify-full`。启动日志会输出最终生效的模式。

### Telegram 存储配置

使用 Telegram 作为存储后端时，需要获取 Telegram User ID：

向机器人 [@userinfobot](https://t.me/userinfobot) 发送 `/start`，即可获取你的 ID。

### 配置方式

所有配置都可以**直接在容器编排里设置**，不需要进容器改 `/app/.env`。

**Docker Compose**：编辑 `docker-compose.yml` 的 `environment` 段，或把 `.env.example` 复制为 `.env`（compose 会自动读取并替换 `${VAR}`）：

```bash
cp .env.example .env
vim .env    # 至少改 APP_URL
docker compose up -d
```

**docker run**：用 `-e` 逐个传入，例如：

```bash
docker run -d --name oneimg -p 8080:8080 \
  -e APP_URL=https://img.example.com \
  -e DEFAULT_USER=admin \
  -e MAX_FILE_SIZE=20971520 \
  -e TRUSTED_PROXIES=172.17.0.0/16 \
  -v /data/oneimg:/app/data \
  -v /data/oneimg-uploads:/app/uploads \
  onexru/oneimg-oneimg
```

**优先级**：编排注入的环境变量 > 容器内自动生成的 `/app/.env`。

应用首次启动时若发现工作目录没有 `.env` 会自动生成一份模板，但加载时（godotenv）**不会覆盖已存在的环境变量**，所以容器注入的值始终生效，容器内 `.env` 只用来补全未注入的默认项。这也意味着容器里那份 `.env` 可以完全不管。

> 早期版本会把 `.env` 文件放在容器内，重建容器即丢失；现在密钥类的自动生成值都落在 `./data` 挂载卷里（`data/.config_secret`、`data/.initial_admin_password`），持久化与重建都不会变。

常用变量：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `APP_URL` | `http://localhost:8080` | **公网必改**。API 返回的图片 URL 取自该值 |
| `SERVER_PORT` | `8080` | 监听端口 |
| `DB_TYPE` | `sqlite` | `sqlite` / `mysql` / `postgres` |
| `DB_TLS_MODE` | `auto` | 数据库 TLS 模式，见上文「数据库」 |
| `DEFAULT_USER` / `DEFAULT_PASS` | `admin` / 空 | 初始管理员；`DEFAULT_PASS` 留空则随机生成 |
| `SESSION_SECRET` | 空（自动生成） | 会话签名密钥 |
| `CONFIG_SECRET` | 空（自动生成） | 配置与图片加密密钥，显式设置必须 ≥32 字符且不可更换 |
| `MAX_FILE_SIZE` | `10485760` | 单文件上限（字节），另有 32MB 硬上限 |
| `ALLOWED_TYPES` | jpeg/png/gif/webp | 允许的 MIME，逗号分隔 |
| `TRUSTED_PROXIES` | 空 | 反代信任的 IP/CIDR，留空表示不信任任何代理 |

完整清单见 [`.env.example`](.env.example)。

---

## 支持作者

项目全程免费开源、持续稳定维护，若本项目对你有帮助，欢迎点亮 ⭐ Star 支持开源，也可通过[打赏通道](https://www.cv0.cn/donate)助力作者持续优化迭代！

欢迎加入官方技术交流群，反馈使用问题、提出优化建议、交流部署与二次开发经验！

<!-- Verified on DartNode: DNOS-F6A1ACA1 -->
[![Powered by DartNode](https://dartnode.com/branding/DN-Open-Source-sm.png)](https://dartnode.com "Powered by DartNode - Free VPS for Open Source")
