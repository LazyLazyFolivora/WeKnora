# WeKnora 前端 UI 开发环境搭建 SOP

> 适用版本：v0.6.3  
> 目标：零 Docker、零本地后端，纯前端 UI 开发  
> 远程后端地址：`https://zkagent.cpu.edu.cn`

---

## 前置要求

| 工具 | 版本要求 | 验证命令 |
|---|---|---|
| Node.js | ≥ 18 | `node -v` |
| npm | ≥ 9 | `npm -v` |
| Git | ≥ 2.30 | `git --version` |

---

## 1. 克隆仓库并切换到基础分支

```bash
git clone https://github.com/LazyLazyFolivora/WeKnora.git
cd WeKnora
git checkout sync_v0.6.3
```

> **`sync_v0.6.3`** 是团队的 UI 开发集成分支。审核同事会将各自提交的分支测试后合并进这个分支，因此每次开发都应从此分支拉取最新代码。

---

## 2. 创建个人开发分支

```bash
git checkout -b sync_v0.6.3/UI_redesign/<你的名字缩写>
```

---

## 3. 配置远程后端代理

### 方式一：修改 vite.config.ts（推荐）

在 `frontend/vite.config.ts` 中，确认代理目标地址为远程后端：

```ts
DEV_PROXY_TARGET = 'https://zkagent.cpu.edu.cn'
```

### 方式二：通过 .env 文件

```bash
cd frontend
echo FRONTEND_BACKEND_URL=https://zkagent.cpu.edu.cn > .env
```

---

## 4. 安装依赖 & 启动

```bash
cd frontend
npm install
npm run dev
```

启动成功后访问 `http://localhost:5173`。

---

## 5. 登录方式

### 登录页分流（v0.6.3 起）

项目登录页已拆分为两个入口：

| 路径 | 页面 | 用途 |
|---|---|---|
| `/login` | 统一身份认证入口页 | **面向正式用户**，仅显示 OIDC/CAS 统一身份认证按钮 |
| `/loginpro` | 内部测试登录页 | **面向内部测试人员**，保留邮箱密码登录、注册、OIDC/CAS 全部功能 |

- 访问地址示例：`http://localhost:5173/loginpro`
- 文件对应关系：
  - `src/views/auth/Login.vue` → 仅统一身份认证入口
  - `src/views/auth/LoginPro.vue` → 完整登录功能（原 Login.vue 的副本）
- 路由定义在 `src/router/index.ts`，两个路由均不需要认证（`requiresAuth: false`）

### 使用邮箱密码登录（推荐，通过 /loginpro）

在本地 `http://localhost:5173/loginpro` 直接使用邮箱和密码登录即可。所有 API 请求通过 Vite 代理转发到 `zkagent.cpu.edu.cn`。

> **注意**：`/login` 页面已移除邮箱密码登录，仅保留统一身份认证。内部测试请使用 `/loginpro`。
> 不要通过统一身份认证（OIDC）登录，因为 OIDC 回调会跳转到生产环境 `zkagent.cpu.edu.cn`，导致本地修改不可见。

### ?debug 参数跳过自动登录

在 URL 后追加 `?debug` 或 `?skip` 参数可跳过 `autoSetup` 自动登录流程：

```
http://localhost:5173?debug
http://localhost:5173/platform/chat?skip
```

适用于需要手动控制登录状态的开发场景。

---

## 6. 用户权限与租户说明

| tenant_id | 角色 | 可见功能 |
|---|---|---|
| `10000` | 管理员租户（Admin） | 全部功能：用户信息、API信息、成员管理、消息管理、模型组、数据与扩展、平台等 |
| 其他 | 普通租户 | 仅：全部设置、登出 |

### 权限控制实现

- **auth store**：`src/stores/auth.ts` 中定义了 `isAdminTenant` 计算属性
  ```ts
  const isAdminTenant = computed(() => {
    const tid = effectiveTenantId.value
    return tid !== null && Number(tid) === 10000
  })
  ```

- **用户菜单**：`src/components/UserMenu.vue` 根据 `isAdminTenant` 控制菜单项显示
- **设置页**：`src/views/settings/Settings.vue` 根据 `isAdminTenant` 隐藏受限设置项

---

## 7. 验证环境

浏览器打开 `http://localhost:5173`，确认：

- [ ] 页面正常显示登录页
- [ ] 使用邮箱密码登录成功
- [ ] 直接访问 `http://localhost:5173/platform/knowledge-bases` 可看到知识库列表页
- [ ] 非10000租户用户菜单仅显示"全部设置"和"登出"

---

## 技术原理

```
浏览器 → Vite Dev Server (localhost:5173)
              │
              ├── /api/*  ──►  https://zkagent.cpu.edu.cn/api/*
              ├── /files/* ──► https://zkagent.cpu.edu.cn/files/*
              └── 其他      →  本地前端代码（热更新）
```

---

## 设计约定

### 配色方案

| 用途 | 色值 |
|---|---|
| 主色（图标、按钮、链接） | `#667eea` — 蓝紫 |
| 辅色（渐变终点、hover 加深） | `#764ba2` — 紫 |
| 点缀色（光晕、背景装饰） | `#4A9BE8` — 医用蓝 |

### 输入框边框渐变

`src/components/Input-field.vue` 中的 `.rich-input-container`：

- **默认状态**：灰色普通边框 `1px solid var(--td-component-border)`
- **聚焦或有输入时**：蓝紫混合渐变边框
  ```css
  border: 1.5px solid transparent;
  background-image: 
    linear-gradient(var(--td-bg-color-container, #fff), var(--td-bg-color-container, #fff)), 
    linear-gradient(135deg, #667eea 0%, #7b68ee 20%, #764ba2 40%, #667eea 55%, #9b6bb5 70%, #764ba2 85%, #667eea 100%);
  background-origin: border-box;
  background-clip: padding-box, border-box;
  ```
- 触发条件：`:focus-within` 或 `:has(textarea:not(:placeholder-shown))`

### 输入框光晕

`src/views/creatChat/creatChat.vue` 中的 `.input-glow`：
- 中心蓝紫径向渐变 + 4 个低透明度蓝色光斑
- 480px × 480px 正圆，`blur(30px)`

### 圆角卡片规范

- 平台页面承载卡片：`border-radius: 16px`，`box-shadow: 0 2px 16px rgba(0,0,0,0.06)`
- 侧边栏：`border-radius: 12px`
- 内部小卡片：`border-radius: 8px`
- 页面底色：`var(--td-bg-color-page)`
- 卡片底色：`var(--td-bg-color-container)`

---

## 关键文件速查表

### 全局布局

| 文件 | 作用 |
|---|---|
| `src/App.vue` | 根组件 |
| `src/views/platform/index.vue` | 平台布局（sidebar + route outlet），圆角卡片容器在此定义 |
| `src/components/menu.vue` | 左侧导航栏，图标/文字/圆角样式 |
| `src/assets/theme/theme.css` | TDesign CSS 变量（品牌色、背景色、字体） |

### 页面

| 页面 | 文件 |
|---|---|
| 登录页（统一身份认证） | `src/views/auth/Login.vue` |
| 登录页（内部测试，邮箱密码） | `src/views/auth/LoginPro.vue` |
| 新建聊天 | `src/views/creatChat/creatChat.vue` |
| 聊天对话 | `src/views/chat/index.vue` |
| 知识库列表 | `src/views/knowledge/KnowledgeBaseList.vue` |
| 知识库详情 | `src/views/knowledge/KnowledgeBase.vue` |
| 智能体列表 | `src/views/agent/AgentList.vue` |
| 共享空间 | `src/views/organization/OrganizationList.vue` |
| 设置面板 | `src/views/settings/Settings.vue` |
| 通用设置 | `src/views/settings/GeneralSettings.vue` |
| 共享空间设置 | `src/views/organization/OrganizationSettingsModal.vue` |

### 组件

| 组件 | 文件 | 说明 |
|---|---|---|
| 输入框 | `src/components/Input-field.vue` | 聊天输入框，含渐变边框、知识库选择、模型选择 |
| 用户菜单 | `src/components/UserMenu.vue` | 右上角用户菜单，含权限控制 |
| 智能体选择器 | `src/components/AgentSelector.vue` | 输入框内智能体下拉 |
| 提及选择器 | `src/components/MentionSelector.vue` | @ 知识库/文件选择器 |
| 知识库选择器 | `src/components/KnowledgeBaseSelector.vue` | 知识库下拉选择 |

### 状态管理

| Store | 文件 | 说明 |
|---|---|---|
| auth | `src/stores/auth.ts` | 认证状态、tenant_id、isAdminTenant |
| settings | `src/stores/settings.ts` | 智能体/知识库/模型选中状态 |
| ui | `src/stores/ui.ts` | UI 状态（模态框、侧边栏等） |
| menu | `src/stores/menu.ts` | 菜单/会话列表 |
| chatResources | `src/stores/chatResources.ts` | 聊天资源缓存（模型、知识库、智能体） |

### 资源

| 目录 / 文件 | 用途 |
|---|---|
| `src/assets/img/login-page/` | 登录页背景图、校徽 |
| `src/assets/img/药小知.png` | 药小知形象图 |
| `src/assets/img/*.svg` | 菜单图标（`-green` 后缀为选中态变体，实际是蓝紫色） |
| `src/i18n/locales/zh-CN.ts` | 中文文案 |
| `src/i18n/locales/en-US.ts` | 英文文案 |

### 配置

| 文件 | 作用 |
|---|---|
| `frontend/.env` | `FRONTEND_BACKEND_URL` 远程后端地址 |
| `frontend/vite.config.ts` | 代理配置（`DEV_PROXY_TARGET`） |
| `frontend/src/utils/api-base.ts` | API base URL |
| `frontend/src/utils/request.ts` | Axios 实例、拦截器、401 处理 |
| `frontend/src/router/index.ts` | 路由配置、导航守卫 |
| `frontend/src/stores/auth.ts` | 认证状态管理 |

---

## 每日开发流程

```bash
# 1. 拉取最新代码
git checkout sync_v0.6.3
git fetch origin sync_v0.6.3
git pull origin sync_v0.6.3
git checkout sync_v0.6.3/UI_redesign/<你的名字缩写>
git merge sync_v0.6.3

# 2. 启动开发服务器
cd frontend
npm run dev

# 3. 在浏览器中用邮箱密码登录 http://localhost:5173

# 4. 开发...

# 5. 提交
git add <修改的文件>
git commit -m "feat: 简要描述改动内容"

# 6. 推送
git push origin sync_v0.6.3/UI_redesign/<你的名字缩写>
```

---

## Git 提交规范

### 分支命名

```
sync_v0.6.3/UI_redesign/<名字缩写>
```

### Commit Message 格式

```
feat: 简要描述改动内容

- 改动点1
- 改动点2
- 改动点3
```

**示例：**

```
feat: UI定制 - 隐藏多语言、权限菜单、输入框渐变、示例问题移除

- 隐藏登录页语言切换按钮及GeneralSettings语言选择器
- auth store添加isAdminTenant计算属性(tenant_id=10000为管理员)
- 用户菜单：非10000租户只显示全部设置+登出按钮
- 输入框边框：默认灰色，点击或有输入时显示蓝紫混合渐变
- 新对话页面移除药学示例问题区块
```

### 提交前检查

```bash
# 查看将要提交的文件
git status

# 只提交前端相关修改
git add frontend/src/ frontend/vite.config.ts

# 避免提交无关文件（如根目录的文档草稿）
```

---

## 已完成的 UI 定制项

以下是已完成并提交到分支的定制修改，供参考和后续维护：

| 定制项 | 修改文件 | 说明 |
|---|---|---|
| 隐藏多语言 | `Login.vue`, `GeneralSettings.vue` | 登录页移除语言切换按钮，设置页移除语言选择器 |
| 权限菜单 | `auth.ts`, `UserMenu.vue`, `Settings.vue` | tenant_id≠10000 只显示全部设置+登出 |
| 输入框渐变 | `Input-field.vue` | 默认灰色边框，聚焦/有输入时蓝紫混合渐变 |
| 示例问题移除 | `creatChat.vue` | 移除 pharmacy-examples 区块（HTML/JS/CSS） |
| 推荐问题移除 | `chat/index.vue` | 移除对话页下方推荐问题卡片 |
| 用户头像移除 | `chat/index.vue` | 对话页只保留智能体头像 |
| OIDC按钮修复 | `Login.vue` | 防止重复渲染OIDC登录按钮 |
| 代理配置 | `vite.config.ts` | DEV_PROXY_TARGET 指向 zkagent.cpu.edu.cn |
| 登录页分流 | `Login.vue`, `LoginPro.vue`, `router/index.ts`, `i18n/*.ts` | `/login` 仅保留统一身份认证入口；`/loginpro` 保留完整邮箱密码登录功能供内部测试使用 |

---

## 常见问题

### Q: 启动后页面不断在 login 和 platform 之间跳转？

检查 `src/utils/request.ts` 中 `redirectToLogin()` 是否有 `import.meta.env.DEV` 的 return 保护。如丢失，参照 `request.ts` 源码重新添加 `import.meta.env.DEV` 时的 return。

### Q: 页面白屏、控制台报错 `withDefaults`？

正常的 Vue 编译器 warning，不影响功能。

### Q: 用户统一身份认证登录后跳到了生产环境？

统一身份认证（OIDC）的回调地址指向 `zkagent.cpu.edu.cn`，登录后会跳转到生产环境。应在本地 `http://localhost:5173` 使用**邮箱密码**登录，而非统一身份认证。

### Q: 如何跳过 autoSetup 自动登录？

在 URL 追加 `?debug` 或 `?skip` 参数：
```
http://localhost:5173?debug
```

### Q: 改了 auth store 但页面没更新？

Vite HMR 对 store 文件的热更新有时不完全。重启 `npm run dev` 即可。

### Q: 远程后端地址变了怎么办？

修改 `frontend/vite.config.ts` 中的 `DEV_PROXY_TARGET`，或修改 `frontend/.env` 中的 `FRONTEND_BACKEND_URL`，重启 `npm run dev`。

### Q: 想让同事看到我的改动？

```bash
git add <文件>
git commit -m "..."
git push origin sync_v0.6.3/UI_redesign/yourname
# 然后在 GitHub 上创建 PR 到 sync_v0.6.3
```

### Q: 如何查看某个内部页面？

浏览器地址栏直接输入：
- `http://localhost:5173/login` — 登录页（仅统一身份认证）
- `http://localhost:5173/loginpro` — 登录页（内部测试，邮箱密码登录）
- `http://localhost:5173/platform/knowledge-bases` — 知识库
- `http://localhost:5173/platform/agents` — 智能体
- `http://localhost:5173/platform/organizations` — 共享空间
- `http://localhost:5173/platform/chat` — 聊天
- `http://localhost:5173/platform/settings` — 设置

### Q: /login 和 /loginpro 有什么区别？

`/login`（`Login.vue`）仅保留了 OIDC/CAS 统一身份认证入口，面向正式用户。`/loginpro`（`LoginPro.vue`）保留了完整的邮箱密码登录、注册、OIDC/CAS 全部功能，供内部测试人员使用。两个页面共享相同的样式和布局，仅表单内容不同。
