# Memos 外部 AI 调用接口文档

本文面向外部 AI、Agent、自动化脚本或第三方服务，目标是让调用方可以直接按 HTTP 请求访问 Memos 的核心能力。

## 1. 基本信息

- Base URL: `https://<你的 Memos 域名>`
- API 前缀: `/api/v1`
- 默认内容类型: `application/json; charset=utf-8`
- 文件下载前缀: `/file`
- 推荐鉴权方式: `Authorization: Bearer <access_token>`
- 兼容鉴权方式: `Cookie: memos.access-token=<access_token>`

## 2. 鉴权说明

### 2.1 推荐流程

1. 管理员或用户先通过登录接口获取会话。
2. 进入设置页创建一个用户访问令牌。
3. 外部 AI 后续请求统一携带这个令牌。

### 2.2 Access Token

- Token 本质上是 JWT。
- 普通登录会话默认有效期为 7 天；`neverExpire=true` 设置约 100 年有效期。
- 专用访问令牌由 `expiresAt` 指定过期时间；未提供时不设置过期时间。
- 服务端还会检查用户状态及令牌是否仍在该用户的访问令牌列表中，签名正确不代表一定可用。
- 创建访问令牌接口会返回完整 token 字符串，请只在安全位置保存一次。
- 使用令牌时，优先放到 `Authorization` 请求头。

### 2.3 权限模型

- 未登录时，只能访问允许匿名访问的读接口，且结果会被自动收窄到公开数据。
- 登录后，可访问与当前账号权限匹配的数据。
- 部分接口有额外权限限制，例如：
  - `CreateUser`
  - `SetWorkspaceSetting`
  - `runtime/memory-limit`
- 接口被拒绝时，通常会返回 `401` 或 `403`。

## 3. 请求格式约定

- 路径参数使用资源名，例如 `users/1`、`memos/abc123`、`resources/xyz`。
- 查询参数使用 gRPC-Gateway 的命名风格，嵌套字段通常使用点号，例如 `passwordCredentials.username`。
- `body: "*"` 的接口：请求体直接传 JSON 对象。
- `body: "memo"`、`body: "resource"` 这类接口：请求体传对应字段的对象本身，不需要再额外包一层外壳。
- `bytes` 类型字段在 JSON 中使用 base64 编码。
- 时间字段使用 RFC3339 / protobuf JSON 时间格式。
- 枚举值按枚举名传递，例如 `NORMAL`、`ARCHIVED`、`PUBLIC`、`PRIVATE`。

## 4. 最常用调用路径

### 4.1 获取登录状态

```http
POST /api/v1/auth/status
```

说明：
- 用于判断当前 token 是否有效。
- 该接口不需要请求体。

### 4.2 用户密码登录

```http
POST /api/v1/auth/signin?passwordCredentials.username=<username>&passwordCredentials.password=<password>&neverExpire=true
```

说明：
- 这是网页登录与脚本登录的统一接口。
- 成功后服务端会下发 `Set-Cookie`。
- 如果需要长期使用给外部 AI，建议登录后再创建专用 access token。

### 4.3 用户注册

```http
POST /api/v1/auth/signup?username=<username>&password=<password>
```

说明：
- 当工作区禁止注册时会返回 `403`。

### 4.4 创建用户访问令牌

```http
POST /api/v1/users/1/access_tokens
Authorization: Bearer <当前账号自己的 token>
Content-Type: application/json

{
  "description": "openai-agent",
  "expiresAt": "2026-06-30T00:00:00Z"
}
```

返回值包含：
- `accessToken`
- `description`
- `issuedAt`
- `expiresAt`

说明：
- `name` 必须是当前登录用户自己的 `users/{id}`。
- 该接口不能替其他用户创建 token。

### 4.5 创建 memo

```http
POST /api/v1/memos
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "content": "今天完成了接口文档整理",
  "visibility": "PRIVATE",
  "pinned": false
}
```

说明：
- `content` 是核心字段。
- `visibility` 可选值通常为 `PRIVATE`、`PROTECTED`、`PUBLIC`。
- 如果不传 `visibility`，服务端转换为 `PRIVATE`；网页编辑器会另外显式传入用户偏好。

### 4.6 查询 memo 列表

```http
GET /api/v1/memos?pageSize=20&state=NORMAL&direction=DESC
Authorization: Bearer <access_token>
```

说明：
- `pageToken` 用于翻页。
- `parent=users/1` 时可只看某个用户的 memo。
- `filter` 支持 CEL 表达式。

### 4.7 上传资源

```http
POST /api/v1/resources
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "filename": "avatar.png",
  "content": "iVBORw0KGgoAAAANSUhEUgAA...",
  "type": "image/png",
  "memo": "memos/abc123"
}
```

说明：
- `content` 为 base64。
- 资源可绑定到某个 memo。

### 4.8 获取资源二进制

```http
GET /file/resources/xyz/avatar.png?thumbnail=true
Authorization: Bearer <access_token>
```

说明：
- `thumbnail=true` 时优先返回缩略图。
- 如果资源挂在非公开 memo 下，需要携带可用 token。

## 5. 接口目录

下面是外部 AI 最常见会用到的接口分组。完整字段定义以 `proto/gen/apidocs.swagger.yaml` 为准。

### 5.1 AuthService

- `POST /api/v1/auth/status`
- `POST /api/v1/auth/signin`
- `POST /api/v1/auth/signup`
- `POST /api/v1/auth/signout`

用途：
- 登录态检查
- 用户登录
- 用户注册
- 用户退出

### 5.2 WorkspaceService

- `GET /api/v1/workspace/profile`

用途：
- 获取工作区所有者、版本、模式和站点地址。

### 5.3 WorkspaceSettingService

- `GET /api/v1/workspace/{name=settings/*}`
- `PATCH /api/v1/workspace/{setting.name=settings/*}`

用途：
- 读取和更新工作区设置
- 通常只建议管理员或宿主账号调用

### 5.4 UserService

- `GET /api/v1/users`
- `GET /api/v1/{name=users/*}`
- `GET /api/v1/users:username`
- `GET /api/v1/{name=users/*}/stats`
- `POST /api/v1/users/-/stats`
- `GET /api/v1/{name=users/*}/access_tokens`
- `POST /api/v1/{name=users/*}/access_tokens`
- `DELETE /api/v1/{name=users/*}/access_tokens/{accessToken}`

用途：
- 用户查询
- 用户统计
- 用户访问令牌管理

### 5.5 MemoService

- `GET /api/v1/memos`
- `POST /api/v1/memos`
- `GET /api/v1/{name=memos/*}`
- `PATCH /api/v1/{memo.name=memos/*}`
- `DELETE /api/v1/{name=memos/*}`
- `PATCH /api/v1/{parent=memos/*}/tags:rename`
- `DELETE /api/v1/{parent=memos/*}/tags/{tag}`
- `PATCH /api/v1/{name=memos/*}/resources`
- `GET /api/v1/{name=memos/*}/resources`
- `PATCH /api/v1/{name=memos/*}/relations`
- `GET /api/v1/{name=memos/*}/relations`
- `POST /api/v1/{name=memos/*}/comments`
- `GET /api/v1/{name=memos/*}/comments`
- `GET /api/v1/{name=memos/*}/reactions`
- `POST /api/v1/{name=memos/*}/reactions`
- `DELETE /api/v1/reactions/{id}`

用途：
- memo CRUD
- 标签、资源、关系、评论、表情反应管理

### 5.6 ResourceService

- `POST /api/v1/resources`
- `GET /api/v1/resources`
- `GET /api/v1/{name=resources/*}`
- `GET /file/{name=resources/*}/{filename}`
- `PATCH /api/v1/{resource.name=resources/*}`
- `DELETE /api/v1/{name=resources/*}`

用途：
- 资源上传与下载
- 二进制文件访问

### 5.7 MarkdownService

- `POST /api/v1/markdown:parse`
- `POST /api/v1/markdown/node:restore`
- `POST /api/v1/markdown/node:stringify`
- `GET /api/v1/markdown/link:metadata`

用途：
- Markdown AST 解析
- AST 还原为 Markdown
- AST 转纯文本
- 链接元数据抓取

### 5.8 WebhookService

- `POST /api/v1/webhooks`
- `GET /api/v1/webhooks`
- `GET /api/v1/webhooks/{id}`
- `PATCH /api/v1/webhooks/{webhook.id}`
- `DELETE /api/v1/webhooks/{id}`

用途：
- 事件回调配置

### 5.9 其他常用服务

- `GET /api/v1/inboxes`
- `PATCH /api/v1/inboxes/{name=inboxes/*}`
- `DELETE /api/v1/inboxes/{name=inboxes/*}`
- `GET /api/v1/activities/{name=activities/*}`
- `GET /api/v1/identityProviders`
- `POST /api/v1/identityProviders`
- `GET /api/v1/identityProviders/{name=identityProviders/*}`
- `PATCH /api/v1/identityProviders/{identityProvider.name=identityProviders/*}`
- `DELETE /api/v1/identityProviders/{name=identityProviders/*}`
- `GET /api/v1/shortcuts`
- `POST /api/v1/shortcuts`
- `PATCH /api/v1/shortcuts/{shortcut.name=shortcuts/*}`
- `DELETE /api/v1/shortcuts/{name=shortcuts/*}`

## 6. 非 OpenAPI 的补充 HTTP 路由

这些路由在网关层直接注册，适合做系统集成时使用。

### 6.1 运行时内存限制

- `GET /api/v1/runtime/memory-limit`
- `PUT /api/v1/runtime/memory-limit`

说明：
- 仅宿主账号可用。
- `PUT` 请求体格式：

```json
{
  "enabled": true,
  "memory_limit_mb": 1024
}
```

### 6.2 Clash Verge 订阅

- `GET /api/v1/clash-verge/subscription?source=...&include=...`
- `GET /api/v1/clash-verge/subscriptions`
- `POST /api/v1/clash-verge/subscriptions`
- `PATCH /api/v1/clash-verge/subscriptions/{id}`
- `DELETE /api/v1/clash-verge/subscriptions/{id}`
- `GET /api/v1/clash-verge/subscriptions/{id}/config`

说明：
- 这组接口返回 YAML 或 JSON。
- 其中 `source` 与 `include` 是核心参数。
- 管理接口按当前用户隔离；`GET /api/v1/clash-verge/subscriptions/{id}/config` 不要求登录，持有规则 URL 即可读取。
- 自动更新由读取配置时触发；刷新失败且有旧缓存时返回旧内容，并附带 `X-Memos-Subscription-Stale: true`。

### 6.3 情绪报告

- `GET /api/v1/emotions/report`
- 要求登录，仅统计当前用户的正常、非评论 memo。
- 必填查询参数 `start_time`、`end_time` 使用 RFC3339；`granularity` 支持 `week`、`month`，默认 `week`。
- 可附加 `filter` 与 `old_filter`；响应包含 `total`、`averageScore`、`dominantEmotion`、`breakdown`、`series`。
- `breakdown[].ratio` 是百分数；周桶从请求开始日期起每 7 天划分，并非自然周。

## 7. 错误处理

### 7.1 gRPC-Gateway 接口

大多数 `/api/v1/*` 接口会返回标准错误结构，常见字段包括：

```json
{
  "code": 7,
  "message": "permission denied",
  "details": []
}
```

### 7.2 自定义 HTTP 接口

自定义路由通常返回：

```json
{
  "error": "unauthorized"
}
```

### 7.3 常见状态码

- `200` 成功
- `204` 成功但无响应体
- `400` 参数错误
- `401` 未登录或 token 无效
- `403` 权限不足
- `404` 资源不存在
- `500` 服务端错误

## 8. 给外部 AI 的直接调用建议

1. 先调用 `POST /api/v1/auth/status` 判断 token 是否有效。
2. 如果要长期运行，先创建一个专用 access token。
3. 读数据优先用 `GET /api/v1/memos`、`GET /api/v1/users:username`、`GET /api/v1/workspace/profile`。
4. 生成内容时优先用 `POST /api/v1/memos`。
5. 如果需要上传附件，先 `POST /api/v1/resources`，再把资源绑定到 memo。
6. 若只需要解析 Markdown，直接用 `POST /api/v1/markdown:parse`。

## 9. 机器可读规范

完整的 OpenAPI 规范文件位于：

- `proto/gen/apidocs.swagger.yaml`

如果你要把 Memos 接到外部 AI 框架、HTTP 工具平台或自动化平台，优先直接读取这个文件，再结合本文的鉴权与调用说明。
