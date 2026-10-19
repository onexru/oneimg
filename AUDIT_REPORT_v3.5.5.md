# oneImg v3.5.5 安全审计总报告

- 审计对象：oneImg 自托管图床（Go gin+gorm 后端 130+ 文件 / Vue3 前端），基线 commit `1b52dad`（PR#32 merge，含 "v3.5.5"）
- 审计方式：6 个并行子系统专项审计（认证会话、访问控制/数据层、加密与配置、SSRF/网络面、上传管线、文件服务/存储层），随后对全部 High/Medium 与可行 Low 落地修复
- 审计结论：**整体质量显著高于同类自托管图床项目**。无 Critical；发现 3 个 High（1 个提权链、1 个权限缺失 SSRF、1 个弱密钥派生），全部已修复。修复后 `go build`、`go vet` 全绿，`go test` 仅存 2 个与本次修改无关的 Windows TempDir 清理句柄问题（原代码同样失败，已用 git stash 对照验证）。

---

## 一、已修复清单（本次全部落地并编译验证）

### High

| # | 位置 | 问题 | 修复 |
|---|------|------|------|
| H1 | `backend/controllers/Users.go:329` UpdateUserPermission | `multi_storage_sync=true` 时"不能改超管权限""不能改自己权限"两道防线被整体跳过；持 `user:permission:update` 委托码的普通用户可给自己加全部 22 个权限码、改超管/管理员的 Buckets/Codes，配合 `user:password:reset` 构成完整提权链 | 保护条件改为 `if req.Codes != nil \|\| !setting.MultiStorageSync`：权限码修改无条件执行超管/自身保护，同步模式仅放宽 Buckets |
| H2 | `backend/routes/routes.go:155` POST /api/buckets/test | 无权限码：任意登录用户（游客模式下含游客）可提交任意内网 S3 endpoint/FTP host/WebDAV URL 令服务器主动出站连接，错误回显区分拒绝/超时/认证失败 → 内网端口扫描+指纹（SSRF） | 挂 `RequireAnyPermission("storage:create","storage:update")`（OR 语义，前端创建/编辑两流程都调用"测试连接"故需 OR）；同时 GET /api/buckets 挂 storage 三码任一，堵住存储拓扑与本地路径泄露 |
| H3 | `backend/utils/secureconfig/secureconfig.go:425` getSecretKey + `backend/config/config.go:234` | CONFIG_SECRET 单轮无盐 sha256 派生 AES-256 密钥且无长度校验：弱口令 → DB 泄露后离线爆破全部存储凭证 | 启动时显式设置的 CONFIG_SECRET <32 字符直接 `log.Fatal`（留空仍走自动生成强随机密钥路径） |

### Medium

| # | 位置 | 问题 | 修复 |
|---|------|------|------|
| M1 | `backend/middlewares/SecurityHeaders.go` | CSP `script-src 'self' https:` 过宽（XSS 后可拉任意外部脚本），connect-src/frame-src 同样放行 | 重写为 `buildCSP()` 动态生成：script-src 收敛到 'self' + challenges.cloudflare.com（Turnstile）+ PoW 源；connect-src 'self' + api.github.com（版本检查）+ 同前；powProviderOrigins() 从设置读取管理员自定义 pow_script_url/pow_widget_url（仅接受 https 无凭据源），默认 cha.eta.im 恒在白名单；5s 缓存 |
| M2 | `backend/routes/routes.go` CORS | AllowOriginFunc 放行任意 `http://localhost:*/127.0.0.1:*` 且 AllowCredentials:true → 受害者 localhost 任意端口恶意页可带 cookie 读全部已登录 GET 接口 | localhost 放行仅当 APP_URL 本身是 localhost/127.0.0.1/::1 时（新增 isLocalhostURL helper） |
| M3 | TG 通知（ImageUpload/ImageURLUpload/LegacyUpload 4 处调用点） | MigrateSecrets 已把 TGBotToken 加密为 `enc:v1:...`，但 4 处直接拿密文调 telegram API → 必然 401 静默失败（功能坏死） | 新建 `backend/controllers/notify.go` `sendTGNotification(settingRow, data)`：GetSecret 解密后发送，空/错记日志返回；4 调用点全部替换 |
| M4 | 上传 DoS：`main.go` 无 ReadTimeout + `UploadLimits.go` 全局仅 2 槽 | 慢速 multipart body 无限占槽，2 连接即全局上传 DoS；FallbackDirectUpload 完全不受槽约束 | `wrapUploadBody(c, max)` = MaxBytesReader + stallAbortBody（60s 空闲中断，经 http.ResponseController 设 body deadline，gin v1.11 responseWriter Unwrap 已验证可穿透）+ 槽持有 15 分钟 time.AfterFunc 兜底自动释放；三个入口（ImageUpload/ImageURLUpload/FallbackDirectUpload）全部接入；FallbackDirectUpload 补 acquireUploadRequest 槽 |
| M5 | `.env` DEFAULT_PASS=123456 模板默认 | 部署沿用即 admin/123456 | **已彻底移除**：v3.5.6 起代码模板 `DEFAULT_PASS=` 留空、README/README.txt/.env.example 全部改为"随机生成"口径，本机 `.env` 中的历史值也已清除；代码层 DefaultPass 为空时走 `data/.initial_admin_password` 随机生成路径 |

### Low

| # | 位置 | 问题 | 修复 |
|---|------|------|------|
| L1 | `backend/controllers/Login.go:110` | 用户不存在直接 401 不跑 bcrypt → 时序枚举账号 | 不存在分支对 DummyBcryptHash（cost 10 与真实一致）做假比较抹平时序 |
| L2 | `backend/middlewares/SecurityMiddleware.go` | /api/account/change 不在限流名单 → 会话劫持者可无限试当前密码 | 单独预算 "acct:"+ip 10/min |
| L3 | `backend/utils/images/images.go` generateUniqueFileName | {random} 用 math/rand 且 rand.New 返回值被丢弃（死代码） | 改 crypto/rand 1 字节 %900+100，失败退化纳秒；UUID 后缀兜底不变 |
| L4 | `backend/utils/images/images.go` ReplaceMagicVariables | {filename} 未消毒客户端可控原始文件名 → 可注入 "/" 造嵌套目录（storage.Key 兜底在校验后） | 替换值过 safeFilenameStem |
| L5 | `backend/utils/images/validation.go` safeFilenameStem | 保留全部 r>127 → U+202E 等 bidi 视觉欺骗 | 剥离 Unicode Cf 类 |
| L6 | `backend/storage/local.go` resolve | 逐分隔符从 `/` 重建路径，Windows 盘符根得 `\D:` 而非 `D:\`（功能性 bug，Win 版受影响） | 重写为 filepath.Join(root, FromSlash(key)) + filepath.Rel 逃逸检查 + 逐组件 Lstat（Unix 从分隔符、Windows 从 VolumeName、UNC 保持 \\server\share 前缀） |
| L7 | `backend/controllers/LegacyUpload.go` saveLegacyURLUpload | URL 上传 tag 无存在性预检，依赖外键回滚 | 事务内 First 预检，与新版路径一致 |
| L8 | `backend/utils/safefetch/safefetch.go` ReadLimited | max+1 超限哨兵从未被调用方检查 → 超限静默截断报误导错误 | 超限返回 ErrBodyTooLarge |
| L9 | `backend/utils/webdav/webdav.go` | 无 CheckRedirect，30x 被跟随（storage/webdav.go 已禁，utils 版漏） | 新增 httpClient()（Timeout + ErrUseLastResponse 禁重定向），5 处统一替换 |
| L10 | `backend/middlewares/AuthMiddleware.go` | （配套 H2）只有单码 RequirePermission | 新增 RequireAnyPermission（OR 语义、API token 一律拒绝、超管/"*" 放行） |

### 配套基础设施

- `backend/utils/authsecurity/credentials.go`：新增 DummyBcryptHash 常量（cost 10，与真实哈希一致）。

---

## 二、未修复/文档化项（有意接受的取舍）

1. **`.env` 模板 DEFAULT_PASS 现已留空**（v3.5.6 起）：留空时仅在数据库还没有任何用户时生成 43 字符随机密码（32 字节随机数 base64url）写入 `data/.initial_admin_password`（0600，不落日志）——这是推荐路径；非空则直接作为初始密码。该文件只写一次，重启不覆盖，也不会重置已有账号密码。历史模板里的 `DEFAULT_PASS=123456` 若仍留在部署的 `.env` 中会被当作初始密码，需删除。
2. **CONFIG_SECRET 部署要求**：文件加密密钥（securestorage）在 CONFIG_SECRET 为空时回退 SESSION_SECRET（域分离前缀是公开常量）。当前版本 CONFIG_SECRET 总非空（自动生成持久化 `data/.config_secret`），回退不可达；但生产建议显式独立配置两者，且 CONFIG_SECRET 现已强制 ≥32 字符。
3. **公开图片代理无速率限制**（Proxy.go 流式路径）：带宽/上游配额 DoS 向量。建议反代层 limit_req；代码层不加（公开端点限流易误伤合法流量）。
4. **safefetch ProxyFromEnvironment**：配置了 HTTP(S)_PROXY 环境变量时拨号级 IP 校验校验的是代理地址而非目标。powverify 已显式 Proxy:nil；其他出站点在代理部署下需知悉此语义。
5. **限流器/cappow Store 单进程内存态**：多实例部署会放大限额、重启清零；单实例部署无影响。
6. **TRUSTED_PROXIES 信任模型**：默认信任零代理（安全默认）；配置后 XFF 可控，仅在有真实反代时配置。
7. **OIDC/CAS superAdminUsername 映射**：IdP 侧若允许自注册同名用户即映射超管（恒定时间比较）。部署外部认证时需在 IdP 锁定该用户名。
8. **SQLite 单连接**（pool.go MaxOpen=MaxIdle=1）：防 locked 但慢查询阻塞全部请求；高并发场景建议 PG。
9. **bcrypt cost 10**：全部哈希点 cost 10（含 DummyHash，为时序一致必须同 cost）。离线爆破成本可再提升到 12，但登录延迟翻倍——容量取舍，未改。
10. **注册无邮箱/邀请约束**：仅 30/min/IP，可批量造 Role=3 用户（资源污染，无越权）。
11. **legacy API token bcrypt CPU 放大器**：配置 APITokenHash 时任意随机 Authorization 头触发 ~10ms bcrypt（auth 路径外不限流）。低危，未改。
12. **Docker 镜像以 root 运行**（原 Dockerfile 无 USER）：容器内进程 root。建议 K8s/Docker run 时 `--user` 或加 USER 指令；本次发布镜像与原版行为保持一致（见四）。
13. **storage_sync_state 错误原文入库并经 API 返回**：已核实现有驱动错误不含凭据；未来新增驱动需注意。
14. **FTP 不支持 FTPS、S3/WebDAV 允许 http:// endpoint**：功能取舍，凭据明文风险由部署者承担。
15. **直传容量预留可扣留 24h（12×32MiB）**：设计容量，配合配额对账幂等。

---

## 三、各子系统审计摘要

### 1. 认证/会话（结论：可上线，无 Critical/High）
会话令牌 crypto/rand 32B、DB 仅存 SHA-256 摘要、登录轮换会话 ID、auth_version+全局 epoch 双重撤销、CSRF 双层防护（SameOriginWrites + SameSite）、首启随机密码。发现 F12(CORS)/F13(CSP)/F1(时序)/F2(限流) 等，均已修复或文档化。

### 2. 访问控制/数据层（结论：质量高，1 个 High 提权链——已修复）
IDOR 全按 user_id 过滤、批量操作逐张校验、SQL 全参数化+排序白名单+列名反射白名单、设置 key→权限码 fail-closed、删用户事务内完整撤销。High 提权链（multi_storage_sync 旁路）已修。

### 3. 加密与配置（结论：无 Critical，质量高于同类）
AES-256-GCM+随机 nonce 全敏感配置加密入库、API 响应掩码、oi_ token bcrypt+常量时间比较。High（弱密钥派生无校验）已修；TG 通知密文 bug（功能坏死）已修。

### 4. SSRF/网络面（结论：防护高于绝大多数同类）
safefetch 拨号级 IP 校验（教科书级 anti-DNS-rebinding，覆盖 CGNAT/链路本地/0.x/IPv6 映射）、重定向逐跳复验、图片代理非开放代理、OIDC/CAS 完整（state+PKCE+回调钉死+XXE 防护）、全项目零 InsecureSkipVerify。唯一必修（buckets/test 权限）已修。

### 5. 上传管线（结论：无 Critical/High，成熟度高）
魔数+MIME 双向校验只认 4 种栅格、像素炸弹防护（≤8192 边长 ≤16M 像素、GIF 帧扫描）、全入口 MaxBytesReader、配额 TOCTOU 原子防护、直传签名覆盖 Content-Type+Length、SSRF 全链。两个 Medium DoS（慢速 body 占槽、fallback 无槽）已修。

### 6. 文件服务/存储层（结论：无 Critical/High）
路径穿越四层防护（DB 精确参数化匹配 + Key() 拒 `\\,:,..,//,#?` 控制字符 + 逐组件 Lstat 拒 symlink + filepath.Rel）、SVG 强制 attachment+CSP sandbox、删除幂等+配额防负数。Windows 盘符 resolve bug 已修。

---

## 四、构建与交付（v3.5.5）

- 产物（命名规则 `oneImg_系统_架构_v3.5.5`）：
  - `oneImg_Win_amd64_v3.5.5.zip`（主机 TDM-GCC CGO 构建）
  - `oneImg_Linux_amd64_v3.5.5.tar.gz`（golang:1.26-alpine 容器静态构建）
  - `oneImg_Linux_arm64_v3.5.5.tar.gz`（QEMU 模拟 golang:1.26-alpine 构建）
  - `oneImg_Docker_amd64_v3.5.5.tar`（docker save）
  - `oneImg_Docker_arm64_v3.5.5.tar`（docker save）
- 全部二进制 UPX `--ultra-brute` 级（-9）压缩。
- 静态链接可行性已确认：chai2010/webp@v1.4.0 内嵌全部 libwebp C 源码由 cgo 直接编译（LDFLAGS 仅 `-lm`），mattn/go-sqlite3 自带 amalgamation → 无需系统 libwebp/sqlite。
- Docker 镜像与原 Dockerfile runtime 层保持一致（alpine:3.23 + ca-certificates + tzdata + libwebp + dist + ca/ + data/uploads 目录），仅把编译产物换为预压缩二进制。
- 前端 dist 已用与 Dockerfile 一致的 pnpm@10.17.1 重建（vite built in 5.57s）。

---

## 五、遗留建议（下次迭代）

1. bcrypt cost 10 → 12（全部哈希点同步，含 DummyHash）。
2. secureconfig 密钥派生改 HKDF+用途标签，移除 SESSION_SECRET 回退。
3. 限流器/cappow Store 外置（Redis）以支持多实例。
4. Dockerfile 增加 `USER` 非 root + HEALTHCHECK。
5. 注册增加邮箱验证/邀请码/总量上限。
6. legacy API token bcrypt 回退按 IP 限流或移除。
