# CPUBrain 前端 UI 开发环境搭建 SOP

> 适用版本：v0.6.3  
> 目标：零 Docker、零本地后端，纯前端 UI 开发（无需登录即可查看所有内部页面）  
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
git checkout feat_v0.6.3
```

> **`feat_v0.6.3`** 是团队的 UI 开发集成分支。审核同事会将各自提交的分支测试后合并进这个分支，因此每次开发都应从此分支拉取最新代码。

---

## 2. 创建个人开发分支

```bash
git checkout -b feat_v0.6.3/UI_redesign/<你的名字缩写>
```

---

## 3. 配置远程后端代理

在 `frontend/` 目录下创建 `.env` 文件：

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

## 5. Dev 模式：无需登录即可开发内部 UI

基础分支已内置 dev 模式认证绕过。启动 `npm run dev` 后，所有 `/platform/*` 页面**无需登录直接可访问**。

### 原理

| 文件 | 改动 | skip-worktree |
|---|---|---|
| `src/router/index.ts` | `import.meta.env.DEV` 时自动注入假 token + user | ✅ |
| `src/utils/request.ts` | dev 模式禁止 401 跳转登录页（打断死循环） | ✅ |
| `vite.config.ts` | `open: false` 禁止弹外部浏览器 | ✅ |

这三个文件已通过 `git update-index --skip-worktree` 保护，**本地改动不会被 git 追踪、不会误提交、不影响服务器构建**。

### 恢复 / 取消保护

```bash
# 查看受保护文件
git ls-files -v | grep "^S"

# 取消保护（如果需要提交这些文件）
git update-index --no-skip-worktree frontend/src/router/index.ts
```

---

## 6. 验证环境

浏览器打开 `http://localhost:5173`，确认：

- [ ] 页面正常显示登录页
- [ ] 直接访问 `http://localhost:5173/platform/knowledge-bases` 可看到知识库列表页
- [ ] API 请求在控制台返回 401（正常——假 token 无法通过远程认证）

> **提示**：VS Code 内置浏览器（Ctrl+Shift+P → Simple Browser）对 Vite HMR 支持不稳定，建议使用外部浏览器。

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
| 按钮渐变 | `linear-gradient(135deg, #667eea 0%, #764ba2 100%)` |
| 文字选中渐变 | `linear-gradient(135deg, #667eea 0%, #764ba2 100%)` + `-webkit-background-clip: text` |

### 圆角卡片规范

- 平台页面承载卡片：`border-radius: 16px`，`box-shadow: 0 2px 16px rgba(0,0,0,0.06)`
- 侧边栏：`border-radius: 12px`
- 内部小卡片：`border-radius: 8px`
- 页面底色：`var(--td-bg-color-page)`
- 卡片底色：`var(--td-bg-color-container)`

### 输入框光晕

`src/views/creatChat/creatChat.vue` 中的 `.input-glow`：
- 中心蓝紫径向渐变 + 4 个低透明度蓝色光斑
- 480px × 480px 正圆，`blur(30px)`

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
| 登录页 | `src/views/auth/Login.vue` |
| 新建聊天 | `src/views/creatChat/creatChat.vue` |
| 聊天对话 | `src/views/chat/index.vue` |
| 知识库列表 | `src/views/knowledge/KnowledgeBaseList.vue` |
| 知识库详情 | `src/views/knowledge/KnowledgeBase.vue` |
| 智能体列表 | `src/views/agent/AgentList.vue` |
| 共享空间 | `src/views/organization/OrganizationList.vue` |
| 设置面板 | `src/views/settings/Settings.vue` |
| 共享空间设置 | `src/views/organization/OrganizationSettingsModal.vue` |

### 资源

| 目录 / 文件 | 用途 |
|---|---|
| `src/assets/img/login-page/` | 登录页背景图、校徽 |
| `src/assets/img/藥小知.png` | 药小知形象图 |
| `src/assets/img/*.svg` | 菜单图标（`-green` 后缀为选中态变体，实际是蓝紫色） |
| `src/i18n/locales/zh-CN.ts` | 中文文案 |
| `src/i18n/locales/en-US.ts` | 英文文案 |

### 配置

| 文件 | 作用 |
|---|---|
| `frontend/.env` | `FRONTEND_BACKEND_URL` 远程后端地址 |
| `frontend/vite.config.ts` | 代理配置 |
| `frontend/src/utils/api-base.ts` | API base URL |
| `frontend/src/utils/request.ts` | Axios 实例、拦截器、401 处理 |
| `frontend/src/router/index.ts` | 路由配置、导航守卫 |
| `frontend/src/stores/auth.ts` | 认证状态管理 |

---

## 每日开发流程

```bash
# 1. 拉取最新代码
git checkout feat_v0.6.3
git fetch origin feat_v0.6.3
git pull origin feat_v0.6.3
git checkout feat_v0.6.3/UI_redesign/<你的名字缩写>
git merge feat_v0.6.3

# 2. 启动开发服务器
cd frontend
npm run dev

# 3. 开发...

# 4. 提交（注意跳过 skip-worktree 文件）
git add .
git commit -m "feat(ui): xxx"

# 5. 推送
git push origin feat_v0.6.3/UI_redesign/yourname
```

---

## 常见问题

### Q: 启动后页面不断在 login 和 platform 之间跳转？

检查 `src/utils/request.ts` 中 `redirectToLogin()` 是否有 `import.meta.env.DEV` 的 return 保护。如丢失，参照上述「Dev 模式」章节重新添加。

### Q: 页面白屏、控制台报错 `withDefaults`？

正常的 Vue 编译器 warning，不影响功能。

### Q: 改文件太多、怕不小心提交了不该提交的？

```bash
git status                                       # 检查
git ls-files -v | grep "^S"                      # 列出 skip-worktree 文件
```

### Q: 如何直接查看某个内部页面？

浏览器地址栏直接输入：
- `http://localhost:5173/platform/knowledge-bases` — 知识库
- `http://localhost:5173/platform/agents` — 智能体
- `http://localhost:5173/platform/organizations` — 共享空间
- `http://localhost:5173/platform/chat` — 聊天

均无需登录。

### Q: 远程后端地址变了怎么办？

修改 `frontend/.env` 中的 `FRONTEND_BACKEND_URL`，重启 `npm run dev`。

### Q: 想让同事看到我的改动？

```bash
git add <文件>
git commit -m "..."
git push origin feat_v0.6.3/UI_redesign/yourname
# 然后在 GitHub 上创建 PR 到 feat_v0.6.3
```
