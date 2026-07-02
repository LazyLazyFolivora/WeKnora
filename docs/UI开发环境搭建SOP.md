# WeKnora 前端 UI 开发环境搭建 SOP

> 适用版本：v0.6.2  
> 目标：零 Docker、零本地后端，直连远程测试服务器进行前端 UI 开发  
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
git clone <仓库地址> WeKnora
cd WeKnora
git checkout sync_v0.6.2（咱用这个新版本）
```

---

## 2. 创建个人开发分支

分支命名规范：`feat_v0.6.2/UI_redesign/<你的名字缩写>`

```bash
git checkout -b feat_v0.6.2/UI_redesign/yourname
```

---

## 3. 配置远程后端代理

在 `frontend/` 目录下创建 `.env` 文件：

```bash
cd frontend
echo FRONTEND_BACKEND_URL=https://zkagent.cpu.edu.cn > .env
```

`.env` 文件的内容：

```
FRONTEND_BACKEND_URL=https://zkagent.cpu.edu.cn
```

> **说明**：此变量会被 `vite.config.ts` 读取（已通过 `loadEnv` 加载），Vite 开发服务器会将所有 `/api` 和 `/files` 请求代理转发到远程后端。

---

## 4. 安装依赖

```bash
# 在 frontend/ 目录下
npm install
```

---

## 5. 启动开发服务器

```bash
npm run dev
```

启动成功后输出：

```
VITE v7.x.x  ready in xxx ms
➜  Local:   http://localhost:5173/
➜  Network: http://192.168.x.x:5173/
```

---

## 6. 验证环境

浏览器打开 `http://localhost:5173`，确认：

- [ ] 页面正常加载，显示 WeKnora 登录页 （确认是否和后端那个版本一致，比如可以学号登入）
- [ ] 使用远程环境账号可以正常登录
- [ ] 登录后各页面数据正常展示（应该要有新手导航教程）

> 若未显示登录页或页面空白，打开浏览器控制台检查 API 请求是否代理到 `zkagent.cpu.edu.cn`。

---

## 技术原理

```
浏览器 → Vite Dev Server (localhost:5173)
              │
              ├── /api/*  ──►  https://zkagent.cpu.edu.cn/api/*
              ├── /files/* ──► https://zkagent.cpu.edu.cn/files/*
              └── 其他      →  本地前端代码（热更新）
```

Vite 的 `server.proxy` 配置将 `/api` 和 `/files` 开头的请求转发到远程后端，其余请求由本地 Vite 处理，前端代码修改即时热更新。

---

## 关键文件说明

| 文件 | 作用 |
|---|---|
| `frontend/.env` | 设置 `FRONTEND_BACKEND_URL`，指定远程后端地址 |
| `frontend/vite.config.ts` | 代理配置 + `loadEnv` 加载 `.env` |
| `frontend/src/utils/api-base.ts` | 生产环境 API base URL（开发时返回空字符串，走代理） |

---

## 每日开发流程

```bash
# 1. 拉取最新代码
git checkout sync_v0.6.2
git pull origin sync_v0.6.2
git checkout feat_v0.6.2/UI_redesign/yourname
git merge sync_v0.6.2

# 2. 启动开发服务器
cd frontend
npm run dev

# 3. 开发完成后提交
git add .
git commit -m "feat: xxx"

# 4. 推送到远程
git push origin feat_v0.6.2/UI_redesign/yourname
```

---

## 常见问题

### Q: 启动后页面是旧版本/和远程不一样？

确保当前分支是基于 `sync_v0.6.2` 创建的，不是 `main`：

```bash
git branch --show-current   # 应显示 sync_v0.6.2 或基于它的分支
```

### Q: 代理报错 ECONNREFUSED？

检查 `.env` 文件是否存在于 `frontend/` 目录下，且内容正确：

```bash
cat frontend/.env
# 应输出: FRONTEND_BACKEND_URL=https://zkagent.cpu.edu.cn
```

### Q: API 返回 401？

正常现象，说明代理已连通。请用远程环境的账号登录即可。

### Q: 远程后端地址变了怎么办？

修改 `frontend/.env` 中的 `FRONTEND_BACKEND_URL`，然后重启 `npm run dev`。
