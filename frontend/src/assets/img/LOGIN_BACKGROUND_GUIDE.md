# 登录页面背景图片指南

## 概述
登录页面支持自定义背景图片（学校照片、校园风景等）。背景图片应该是横屏格式，建议分辨率为 **1200x800px** 或更高。

## 如何添加背景图片

### 第1步：准备图片文件
1. 准备4张学校的横屏照片（或任意数量）
2. 建议图片规格：
   - 宽高比：4:3 或 16:9
   - 最小分辨率：1200x800px
   - 推荐分辨率：1600x900px 或 1920x1080px
3. 文件格式：JPG、PNG 或 WebP（推荐 JPG 以减小文件大小）

### 第2步：放置图片文件
将准备好的图片文件放在本目录 (`frontend/src/assets/img/`) 中，例如：
- `school-photo-1.jpg`
- `school-photo-2.jpg`
- `school-photo-3.jpg`
- `school-photo-4.jpg`

### 第3步：配置背景图片URL
编辑 `frontend/src/views/auth/Login.vue` 文件：

1. 找到以下代码片段（大约在第385行）：
```javascript
const backgroundImages = [
  // 暂时为空，等待用户添加学校照片
  // 'url(/img/school-photo-1.jpg)',
  // 'url(/img/school-photo-2.jpg)',
  // 'url(/img/school-photo-3.jpg)',
  // 'url(/img/school-photo-4.jpg)',
]
```

2. 取消注释并修改为你的图片文件名：
```javascript
const backgroundImages = [
  'url(/img/school-photo-1.jpg)',
  'url(/img/school-photo-2.jpg)',
  'url(/img/school-photo-3.jpg)',
  'url(/img/school-photo-4.jpg)',
]
```

3. 保存文件，页面会自动刷新（HMR）

### 第4步：验证效果
1. 访问登录页面：`http://localhost:5173/login`
2. 检查背景图片是否正确显示
3. 左侧登录面板应显示磨砂透明效果（glassmorphism）
4. 可以刷新页面查看不同的背景图片

## 设计特点

### 玻璃态形态（Glassmorphism）
- **背景模糊**：登录面板使用 `backdrop-filter: blur(25px)` 实现毛玻璃效果
- **透明度**：面板背景为 `rgba(255, 255, 255, 0.12)` 提供半透明外观
- **边框**：浅色半透明边框 `rgba(255, 255, 255, 0.28)` 增强层次感

### 品牌色调叠加
- 登录页面在背景图片上方覆盖半透明的蓝紫色渐变
- 这样可以保证品牌色调一致，同时让背景图片可见

### 响应式设计
- 背景图片会自动适配所有屏幕尺寸
- 使用 `background-size: cover` 和 `background-position: center` 确保图片填满屏幕

## 技术细节

### CSS变量
登录页面使用 CSS 变量 `--login-bg-image` 来动态设置背景图片：
```css
--login-bg-image: url(/img/school-photo-1.jpg)
```

### JavaScript 实现
在 `Login.vue` 的脚本中：
- `backgroundImages` 数组存储所有背景图片URL
- `setBackgroundImage()` 函数在页面加载时随机选择一张背景图片
- 每次页面加载都会循环选择不同的背景

## 故障排除

### 背景图片不显示
1. 检查图片文件是否正确放在 `frontend/src/assets/img/` 目录
2. 检查 `Login.vue` 中的URL是否正确（注意大小写）
3. 打开浏览器开发者工具，检查Network标签是否有404错误
4. 清空浏览器缓存并刷新页面

### 图片显示但背景色不对
- 检查 `--login-bg-image` CSS 变量是否正确应用
- 确认品牌色的半透明覆盖层是否正确渲染

### 性能问题
- 确保图片文件大小合理（推荐 200KB-500KB）
- 考虑使用 WebP 格式以减小文件大小
- 可以使用在线工具压缩图片

## 示例配置

### 完整示例（4张图片）
```javascript
const backgroundImages = [
  'url(/img/school-campus-1.jpg)',
  'url(/img/school-campus-2.jpg)',
  'url(/img/school-library-1.jpg)',
  'url(/img/school-outdoor-1.jpg)',
]
```

### 单张图片配置
```javascript
const backgroundImages = [
  'url(/img/school-main-building.jpg)',
]
```

## 深色模式支持
登录页面已支持深色模式。深色模式下：
- 背景渐变色调整为深蓝紫色
- 玻璃态效果的透明度和模糊程度保持一致

## 相关文件
- **登录页面**：`frontend/src/views/auth/Login.vue`
- **背景图片目录**：`frontend/src/assets/img/`
- **样式部分**：Login.vue 中的 `<style lang="less" scoped>` 部分

## 后续优化建议
1. 可以添加背景图片加载动画
2. 可以实现定时轮换背景图片
3. 可以添加背景图片的人工智能优化（如自适应明度调整）
