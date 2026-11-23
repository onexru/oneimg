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
- **用户上传配额**（v4.0.0）：支持个体配额、站点默认配额（默认 10 GiB）与不限额三档，管理员不参与限额
- **SSRF 防护**（v4.0.0）：存储连接测试与保存拒绝指向内网/元数据地址，并防 DNS rebinding
- **转码限流与缓存隔离**（v4.0.0）：水印转码请求限流 + 槽位超时，转码结果私有缓存不污染 CDN

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

### 离线安装镜像（不需要访问 Docker Hub）

部分网络环境拉不动 Docker Hub，所以每次发布也会把镜像按架构导出成压缩包，挂在 [Releases](https://github.com/onexru/oneimg/releases) 里：

| 平台 | 文件名 |
|---|---|
| Linux x86_64 | `oneimg-vX.Y.Z-docker-amd64.tar.gz` |
| Linux arm64 | `oneimg-vX.Y.Z-docker-arm64.tar.gz` |

```bash
docker load -i oneimg-v4.0.2-docker-amd64.tar.gz
docker images onexru/oneimg-oneimg        # 加载后自带 4.0.2 标签

docker run -d \
  --name oneimg \
  -p 8080:8080 \
  -e APP_URL=https://img.example.com \
  -v /data/oneimg:/app/data \
  -v /data/oneimg-uploads:/app/uploads \
  onexru/oneimg-oneimg:4.0.2
```

导出的是标准 docker-archive，加载后的镜像名与 tag 和从仓库拉取完全一致，需要改名时 `docker tag` 即可。这些包与 Docker Hub 上是同一次构建的产物，`SHA256SUMS.txt` 放在一起便于校验。

### 二进制包（Linux / Windows）

不想用容器时，直接下载对应平台的压缩包即可，前端已通过 `go:embed` 打进可执行文件，解压就能跑。

| 平台 | 文件名 |
|---|---|
| Linux x86_64 | `oneimg-vX.Y.Z-linux-amd64.tar.gz` |
| Linux arm64 | `oneimg-vX.Y.Z-linux-arm64.tar.gz` |
| Windows x64 | `oneimg-vX.Y.Z-windows-amd64.zip` |
| Windows arm64 | `oneimg-vX.Y.Z-windows-arm64.zip` |

到 [Releases](https://github.com/onexru/oneimg/releases) 下载，同一次发布里附带的 `SHA256SUMS.txt` 可用于校验完整性。

```bash
tar -xzf oneimg-v4.0.2-linux-amd64.tar.gz
cd oneimg-v4.0.2-linux-amd64
APP_URL=https://img.example.com ./oneimg
```

- 工作目录下会自动创建 `data/`（数据库、初始密码）和 `uploads/`（本地存储），升级时保留这两个目录。
- `APP_URL` 建议显式指定，否则返回的图片链接会以 `http://localhost:8080` 为前缀。
- Linux 版基于 musl 静态链接，不依赖发行版 glibc 版本；Windows 版是单文件 exe，无需额外运行库。

#### 关于 UPX 加壳（可选）

发布流程支持用 [UPX](https://upx.github.io/) 压缩可执行文件，**默认关闭**。开启后二进制体积约为原来的 28%（如 `linux/amd64` 59 MB → 16.5 MB），下载包约小 25%——因为压缩包本身已经在压，收益没有二进制体积看起来那么大。

代价是运行时要把整个程序解压到匿名内存：

| 项目 | 未加壳 | UPX 后 |
|---|---|---|
| 常驻内存（RSS） | 约 10 MiB | 约 56 MiB |
| 启动到可响应 | 约 1.2 s | 约 2.2 s |
| Windows 杀软 | — | 有误报 / 隔离风险 |

所以默认不压。确实需要更小的分发体积时，可以在仓库的 Actions 变量里设置 `UPX=true`，或在手动触发发布时把 `upx` 选为 `true`；加壳与不加壳功能完全一致，只是资源占用不同。手上的包是否加过壳，看可执行文件大小即可（加壳后明显更小），也可以用 `upx -d` 尝试还原。

### 初始账号与密码

> **版本差异**：`v3.5.5` 及更早版本，初始管理员密码固定为 `123456`；从 `v3.5.6` 起改为首次启动随机生成。

```text
账号：admin（由 DEFAULT_USER 决定）
密码：v3.5.6 起首次启动自动生成，见 data/.initial_admin_password
      v3.5.5 及更早版本固定为 123456
```

**v3.5.6 及之后**（随机密码）的生成规则：

| `DEFAULT_PASS` | 行为 |
|---|---|
| 留空（推荐） | 仅在数据库中还没有任何用户时，生成 43 字符随机密码写入 `data/.initial_admin_password`（权限 `0600`），**不打印到日志** |
| 非空 | 直接使用该值作为初始管理员密码 |

- 该文件只在首次创建管理员时写入；之后重启不会覆盖，也不会重置已有账号的密码。
- 密码**不会**出现在启动日志里，必须读取该文件获取。
- 登录后请立即修改密码，并删除该文件。

**v3.5.5 及更早版本**：内置的 `.env` 模板为 `DEFAULT_PASS=123456`，首次启动直接以该值创建管理员，**不存在** `data/.initial_admin_password` 文件，日志里也不会提示初始密码。请用 `admin` / `123456` 登录后立即修改密码。

> ⚠️ 从旧版本升级时，如果你保留了旧的 `.env` 文件，其中的 `DEFAULT_PASS=123456` 会被继续当作初始密码使用（仅影响尚未创建过管理员的空库）。新版本仓库提供的模板已改为留空，建议删除该行或改成强密码。

---

## 部署说明

### 环境要求

- Docker 20.10.0+（用容器部署时）
- Docker Compose v2.0.0+（用 Compose 编排时）
- 用二进制包部署时无额外依赖，见上一节

### 数据持久化

建议挂载以下目录以保证数据安全：

| 目录 | 用途 |
|------|------|
| `./data` | 数据库、系统配置与自动生成的密钥（`.session_secret`、`.config_secret`） |
| `./uploads` | 本地上传文件 |

> 只要 `./data` 卷保留，`SESSION_SECRET` / `CONFIG_SECRET` 留空时会自动生成并持久化到卷内，重启进程或重建容器都不会导致用户掉线、也不会让已加密的内容无法解密。反过来说，删除 `./data` 会同时丢失这些密钥。

支持三种数据库：
1. sqlite （默认）
2. mysql
3. postgres

> 修改数据库在`.env`内`DB_TYPE`配置

### 反向代理与限流（TRUSTED_PROXIES）

v4.0.0 起**默认不信任任何代理头**：`X-Forwarded-For` 仅当请求来源在
`TRUSTED_PROXIES` 列表内才被采信。登录/注册等接口按客户端真实 IP 限流
（30 次/分钟），配置错误会让全站共用反代 IP 的预算。

| 部署形态 | 建议值 |
|---|---|
| 无反代，直接暴露 | 留空（默认） |
| Docker 网络内反代（Nginx Proxy Manager 等） | `172.16.0.0/12` |
| 本机反代 | `127.0.0.1` |
| 前置 Cloudflare | 补充 CF 官方 IP 段 |

> ⚠️ 配置了非法条目（不是合法 IP/CIDR）时应用会**拒绝启动**，而非静默忽略——这是有意设计，避免限流在毫不知情的情况下失效。

### 用户上传配额

每个用户的可上传总量支持三档语义（v4.0.0）：

| 配置 | 行为 |
|---|---|
| 个体配额设为具体值 | 严格按该值限制（不与默认值取小） |
| 未设置（跟随默认） | 使用站点默认配额 `default_user_quota`（默认 10 GiB） |
| 设为 0 | 不限量 |

- 管理员与超级管理员**不参与配额**；
- 配额判定以**落盘后的真实大小**为准（压缩转码后更小的图不会被误判）；
- 在「用户管理」弹窗中设置，用户卡片实时显示进度条（<70% 主色 / 70–90% 琥珀 / ≥90% 红色）。

### 保存源文件名

v4.0.0 修复了 [#45](https://github.com/onexru/oneimg/issues/45) 反馈的问题：

- 开启「保存源文件名」后**不再追加随机后缀**，`风景.jpg` 上传后就是 `风景.webp`；
- 可读字符（含中文、空格）原样保留，不可读字符折叠为单个 `-`；
- 重名自动编号：`风景.webp`、`风景-2.webp`、`风景-3.webp`……探测占用失败时回退随机命名，绝不覆盖已有图片；
- 关闭开关时保持原有「模板 + 随机串」命名逻辑不变。

### 上传限制

限制值不在界面里，也不全在设置项里 —— 下面这张表就是全部，服务端按它判定：

| 限制 | 值 | 是否可调 |
|---|---|---|
| 单个文件体积 | 32 MiB 硬上限；`MaxFileSize`（默认 10 MiB，按**字节**配置）只能往下调 | 后台设置 |
| 单边像素 | ≤ 8192 px | 固定 |
| 总像素 | ≤ 16,000,000（1600 万） | 固定 |
| 动图（GIF） | ≤ 200 帧，帧像素合计 ≤ 6400 万 | 固定 |
| 图片格式 | JPEG / PNG / GIF / WebP；不支持 SVG、BMP、TIFF、HEIC | 后台白名单 |

判定顺序是**体积 → 格式（读文件头）→ 尺寸**，三步都在解码之前完成：
超限的请求不会先吃一次解码内存，也不会因为「体积小」就放过一张 4000 万像素的图。

> **为什么像素上限写死**：解码后的位图约占 `像素 × 4 字节`，1600 万像素 ≈ 64 MiB；
> 图片处理并发默认 2 个槽，峰值约 128 MiB。做成设置项等于把 OOM 开关交给使用者，
> 所以它保持常量。需要传更大的图时，请先缩放再上传。

这三个数值也能从 `GET /api/uploadConfig` 读到（`max_file_size` / `max_image_pixels` /
`max_image_dimension`），客户端可以提前拦掉，不必等传完才被拒。

### API Token 的权限范围（scopes）

API Token（`Authorization: Bearer oi_...`）只能访问下表中的接口，其余路由一律 403
「API 凭据无此操作权限」——包括设置、用户、存储、标签增删与存储源切换。浏览器会话不受此限制。

| scope | 覆盖的接口 |
|---|---|
| `image:read` | 图片列表与详情、`/api/user/status`、`/api/uploadConfig`、`/api/buckets/list`、`/api/tags` |
| `upload:write` | `POST /api/upload`、`/api/upload/images`、`/api/images/upload`、`/api/images/url` |
| `image:delete` | `DELETE /api/images/:id`、`POST /api/images/batch-delete` |

两点需要注意：

- **v4.0.2 及更早签发的 token 不含 `image:delete`**（当时默认值是 `image:read,upload:write`），
  拿它调删除接口会返回 403。到后台重新生成一次 token 即可（同一时间只保留一个有效 token，
  重新生成会吊销旧的）。
- token 的权限码由 scopes 推导，**不随账号角色提升**；图片归属由控制器逐张校验，普通账号
  的 token 只能删自己上传的图。（超管本人签发的 token 等价于他本人的浏览器会话，可管理全部
  图片——这一点与删除按钮在后台的行为一致。）

### 上传接口（API）

`POST /api/upload`（单张）与 `POST /api/upload/images`（批量）走**同一套**解析逻辑，
参数与返回完全一致，请求必须是 `multipart/form-data`（鉴权需在后台「启用 API」，
并携带 `Authorization: Bearer <token>`）。

| 字段 | 必填 | 说明 |
|---|---|---|
| `images[]` | 是 | 文件字段。**规范字段名就是 `images[]`，单张上传也用它**；同时兼容 `file` / `file[]` / `images` |
| `folder_id` | 否 | 目标文件夹 ID，缺省 `0`（根目录） |
| `tags` | 否 | JSON 数组字符串，如 `["风景","壁纸"]`，只会关联**已存在**的标签 |
| `bucket_id` | 否 | 仅单存储模式有效，指定目标存储源 |

```bash
curl -X POST "$HOST/api/upload/images" \
  -H "Authorization: Bearer $TOKEN" \
  -F "images[]=@a.png" \
  -F "images[]=@b.png" \
  -F 'tags=["测试"]'
```

成功响应（`failed_count` 为失败张数，`sync_targets` 为后台同步目标数）：

```json
{
  "code": 200,
  "message": "文件已保存到本机，正在后台同步",
  "data": {
    "count": 2,
    "failed_count": 0,
    "sync_targets": 1,
    "files": [
      {
        "success": true,
        "id": 128,
        "folder_id": 0,
        "url": "https://img.example.com/uploads/2026/11/xxx.webp",
        "thumbnail_url": "https://img.example.com/uploads/2026/11/xxx_thumb.webp",
        "storage": "default",
        "filename": "xxx.webp",
        "file_size": 51200,
        "mime_type": "image/webp",
        "width": 1200,
        "height": 800
      }
    ]
  }
}
```

> 开启「多存储同步」时 `message` 为「文件已保存到本机，正在后台同步」：
> 返回成功**只代表本机副本已就绪**，远端副本由后台任务继续上传，进度见图片详情。

失败响应体固定为 `{"code": <状态码>, "message": "...", "data": null}`：

| code | 场景 | message 示例 |
|---|---|---|
| 400 | Content-Type 不是 multipart | `请求 Content-Type 必须是 multipart/form-data，文件字段名使用 images[]` |
| 400 | 文件字段名不对 | `未找到上传文件：multipart 字段名应为 images[]（兼容 file / file[]）` |
| 400 | 超过数量上限 | `最多只能上传10个文件` |
| 401 | token 缺失/过期，或后台未启用 API | `用户未登录或会话已过期` / `API 凭据无效` |
| 403 | token 缺少上传 scope，或游客身份无效 | `API 凭据无此操作权限` |
| 413 | 请求体超过上限 | `上传内容超过请求体上限：请减少文件数量或缩小单张图片` |
| 429 | 全局上传槽被占满 | `服务器正在处理其他上传（并发上限 2），请稍后重试`，带 `Retry-After: 2` |
| 503 | 水印转码繁忙 | `图片处理繁忙，请稍后重试`，带 `Retry-After: 2` |

单张失败不会拖垮整批：`data.files[]` 中该项 `success` 为 `false`，并带 `error_code`
（稳定分类，供程序判断）与 `message`（给人看的说明）：

| `error_code` | 含义 | 该怎么做 |
|---|---|---|
| `file_too_large` | 超过体积上限 | 压缩或缩小体积 |
| `image_dimensions` | 超过像素 / 单边上限 | 先缩小尺寸再传 |
| `unsupported_format` | 格式不支持，或内容与扩展名/声明类型不一致 | 转成 JPEG / PNG / GIF / WebP |
| `invalid_image` | 解码失败，文件可能已损坏 | 换一个文件 |
| `upload_timeout` | 处理超时 | 重试 |
| `storage_failed` | 存储源写入失败 | 检查存储配置或换存储源 |
| `record_failed` | 图片记录写入或容量更新失败 | 重试 |
| `quota_exceeded` | 账号配额用尽 | 删图或申请更多额度 |

> 更早的版本里这 8 类在响应中都是同一句「文件上传失败」，体积、像素、格式、存储
> 完全分不出来，只能靠二分试尺寸反推限制值（[#58](https://github.com/onexru/oneimg/issues/58)）。
> 现在请以 `error_code` 为准，`message` 可能随时润色。

**并发与退避**：服务端同时只处理 `UPLOAD_SLOTS`（默认 2）个上传请求；槽位占满时会先排队
最多 5 秒，仍未拿到才返回 429。脚本化批量上传请按 `Retry-After` 退避重试（例如 2 秒后
重试 1~3 次），不要立即重发；确实需要更高并发再调 `UPLOAD_SLOTS`。

#### URL 远程上传（`POST /api/images/url`）

```bash
curl -X POST "$HOST/api/images/url" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/a.jpg","tag_id":0,"folder_id":0}'
```

| 字段 | 必填 | 说明 |
|---|---|---|
| `url` | 是 | 远端图片地址，仅 `http/https`；目标解析到内网/回环/链路本地/云 metadata 会被拒绝 |
| `tag_id` | 否 | 已存在的标签 ID，`0` 或不传表示不加标签 |
| `folder_id` | 否 | 目标文件夹，缺省 `0`（根目录） |

**成功响应带 `data`，不是 `null`** —— `data.file.url` 就是新图片地址（单存储模式）：

```json
{
  "code": 200,
  "message": "URL 图片上传成功",
  "data": {
    "file": {
      "success": true,
      "id": 131,
      "folder_id": 0,
      "url": "/uploads/2026/11/a.webp",
      "thumbnail_url": "/uploads/2026/11/thumbnails/a.webp",
      "storage": "default",
      "filename": "a.webp",
      "file_size": 51200,
      "thumbnail_size": 8192,
      "mime_type": "image/webp",
      "width": 1200,
      "height": 800,
      "created_at": "2026-11-20 17:52:15"
    }
  }
}
```

开启「多存储同步」时 `message` 变为「URL 图片已保存到本机，正在后台同步」，并多一个
`data.sync_targets`（后台同步的存储源数量，`> 0` 表示远端副本仍在同步，进度见图片详情）。

- `file.url` 是相对路径，按 `APP_URL` 拼成绝对地址再对外使用。
- 失败时（`url` 非法、下载失败、格式不允许、超限、配额不足）`data` 为 `null`，`message` 说明原因。
- 下载失败的错误会列出**每个**尝试过的地址及原因，不再是只留最后一个：

```json
{"code":400,"message":"图片下载失败：连接 pic1.zhimg.com 失败，尝试 2 个地址均失败：[240e:c3:2800:6:800:0:d00:10]:443: connect: network is unreachable；156.59.198.226:443: connect: connection refused","data":null}
```

> 拨号顺序为 **IPv4 优先**。纯 IPv6 出网与双栈环境都不受影响；只有 IPv4 出网时不会先撞
> 一通 `ENETUNREACH`。若 DNS 只返回了 AAAA（A 查询失败或被按线路过滤），会显式补查一次
> A 记录再试 —— 不会因为「解析结果里没有 IPv4」就直接失败。

#### 删除接口（单张 / 批量）

| 接口 | 说明 |
|---|---|
| `DELETE /api/images/:id` | 删除单张。成功返回 200；不存在 404、越权 403、副本删失败 502 |
| `POST /api/images/batch-delete` | 批量删除，**一次请求带全部 ID** |

```bash
curl -X POST "$HOST/api/images/batch-delete" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":[128,129,130]}'
```

```json
{
  "code": 200,
  "message": "已删除 2 张，1 张未完成",
  "data": {
    "requested": 3,
    "deleted": [128, 129],
    "failed": [{ "id": 130, "message": "部分存储源删除失败，文件记录已保留，可稍后重试" }],
    "deleted_count": 2,
    "failed_count": 1
  }
}
```

约定：

- 两个接口都需要 token 具备 `image:delete`（见上一节）；v4.0.2 及更早签发的 token 需要重新生成。
- `ids` 单次最多 **200** 个（图库一页 30 张，正常一次选中就是一页）；会自动去重、丢弃非法 ID。
- 服务端**顺序**删完再返回，逐张给结果：一张失败不影响其它张，`failed[]` 里的 `message` 可直接展示。
- 删除「已不存在」的 ID 记为**成功**，所以重试是幂等的 —— 反代超时后整批重发不会误报失败。
- 批量删除用 `POST` 而不是带请求体的 `DELETE`：部分反代 / CDN 会丢弃 `DELETE` 的 body。

> 为什么不做成「一个 ID 一条 `DELETE`」：删除副本要拿同一把全局存储锁，服务端本来就是串行
> 执行的。请求拆得越多，排在队尾的越容易在反代里空等到读超时（Nginx 默认 60s、Cloudflare
> 100s）被掐断，表现出来就是「每次只删掉一部分，再点一次又删掉一部分」。一条请求反而更快、
> 结果也更准确。

#### SVG 为什么是「下载」而不是直接显示

SVG 可以内嵌 `<script>`。为避免恶意 SVG 在同源下执行，`/uploads/*.svg` 一律：

- `Content-Disposition: attachment`（浏览器下载而不是渲染）；
- `Content-Security-Policy: sandbox; default-src 'none'; ...`；
- `Cache-Control: private, no-store`。

这是**刻意的防 XSS 设计，不是缺陷**。所以不要把上传后的 SVG 直接 `<img>` 内嵌到页面当普通
图片用；需要矢量图展示时，请自行转成 PNG/WebP 再上传。

#### 为什么单文件有 32MiB 硬上限

`MAX_FILE_SIZE` 只能往下调，不能突破 32MiB：上传是少数会把整张图读进内存再解码的接口，
若把上界完全交给配置（允许填 0 或 500MB），一个请求就能把进程打爆；而 `/uploads` 的转码
路径还会在未鉴权时触发解码。真正限制「1MB 的 PNG 解出 1.6GB 位图」的是解码前后的
16MP / 8192px 上限与全局处理槽，32MiB 只是外层闸门。

### 数据库 TLS 模式

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
| `SESSION_SECRET` | 空（自动持久化） | 会话与 OIDC/CAS state 签名密钥，留空时生成到 `data/.session_secret` |
| `CONFIG_SECRET` | 空（自动持久化） | 配置与图片加密密钥，留空时生成到 `data/.config_secret`；显式设置必须 ≥32 字符且不可更换 |
| `MAX_FILE_SIZE` | `10485760` | 单文件上限（字节），另有 32MB 硬上限 |
| `ALLOWED_TYPES` | jpeg/png/gif/webp | 允许的 MIME，逗号分隔 |
| `UPLOAD_SLOTS` | `2` | 同时处理的上传请求数，上限 16。每个槽位峰值内存约 100~200MB，调大前按机器内存核算；占满时返回 `429` + `Retry-After: 2` |
| `TRUSTED_PROXIES` | 空 | 反代信任的 IP/CIDR，逗号分隔；留空不信任任何代理，非法条目拒绝启动（见上文「反向代理与限流」） |

> 用户默认配额 `default_user_quota`（默认 10 GiB）不是环境变量，在「系统设置 → 上传」中配置；个体配额优先于该值。

完整清单见 [`.env.example`](.env.example)。

---

## 支持作者

项目全程免费开源、持续稳定维护，若本项目对你有帮助，欢迎点亮 ⭐ Star 支持开源，也可通过[打赏通道](https://www.cv0.cn/donate)助力作者持续优化迭代！

欢迎加入官方技术交流群，反馈使用问题、提出优化建议、交流部署与二次开发经验！

---

## 开源协议

本项目基于 [MIT License](LICENSE) 开源。您可以自由地使用、复制、修改、合并、发布和分发本软件及其文档，唯一的要求是保留原始的版权声明与许可声明。

> 本项目按「现状」提供，作者不对软件的适用性、正确性或由此产生的任何损失承担责任，详见 [LICENSE](LICENSE) 全文。
