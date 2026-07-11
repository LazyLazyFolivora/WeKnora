# 登录页面自定义指南

本文档说明如何自定义登录页面的背景图、功能卡片和其他内容。

## 快速开始

### 1. 修改轮播背景图

**文件位置**：`frontend/src/views/auth/LoginRedesign.vue` (第86-91行)

```typescript
// 修改前：使用 Unsplash 占位图
const backgroundImages = [
  'https://images.unsplash.com/photo-1519389950473-47ba0277781c?w=1200&h=800&fit=crop',
  'https://images.unsplash.com/photo-1552664730-d307ca884978?w=1200&h=800&fit=crop',
  'https://images.unsplash.com/photo-1522202176988-696ce0213ce0?w=1200&h=800&fit=crop',
  'https://images.unsplash.com/photo-1517694712202-14dd9538aa97?w=1200&h=800&fit=crop'
]

// 修改后：使用本地或自定义 URL
const backgroundImages = [
  '/images/university-campus-1.jpg',
  '/images/university-campus-2.jpg',
  '/images/university-campus-3.jpg',
  '/images/university-campus-4.jpg'
]
```

### 2. 添加功能卡片内容

**文件位置**：`frontend/src/views/auth/LoginRedesign.vue` (第128-136行)

当前的功能卡片是空的占位符。要添加内容，需要：

#### 方案 A：添加背景图（推荐）

修改模板部分（第92-96行）：

```vue
<div class="feature-card" v-for="(feature, index) in featureCards" :key="index">
  <div class="feature-card-inner">
    <!-- 添加背景图 -->
    <img :src="feature.image" :alt="feature.title" class="feature-screenshot" />
    <div class="feature-label">{{ feature.title }}</div>
  </div>
</div>
```

更新样式：

```less
.feature-screenshot {
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: center;
}

.feature-label {
  position: absolute;
  bottom: 8px;
  left: 8px;
  right: 8px;
  background: rgba(0, 0, 0, 0.4);
  color: white;
  padding: 6px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 500;
  text-align: center;
}
```

更新数据：

```typescript
const featureCards = [
  { 
    id: 1, 
    title: '智能检索',
    image: '/images/feature-1.png'
  },
  { 
    id: 2, 
    title: '知识管理',
    image: '/images/feature-2.png'
  },
  { 
    id: 3, 
    title: 'AI助手',
    image: '/images/feature-3.png'
  },
  { 
    id: 4, 
    title: '药学工具',
    image: '/images/feature-4.png'
  }
]
```

#### 方案 B：添加描述文本

```vue
<div class="feature-card" v-for="(feature, index) in featureCards" :key="index">
  <div class="feature-card-inner">
    <div class="feature-placeholder">
      <div class="feature-icon">{{ feature.icon }}</div>
      <div class="feature-title">{{ feature.title }}</div>
    </div>
  </div>
</div>
```

```typescript
const featureCards = [
  { id: 1, title: '智能检索', icon: '🔍' },
  { id: 2, title: '知识管理', icon: '📚' },
  { id: 3, title: 'AI助手', icon: '🤖' },
  { id: 4, title: '药学工具', icon: '⚗️' }
]
```

## 图片资源要求

### 背景图规格
```
尺寸:        1200px × 800px (最小)
格式:        JPG / WebP (推荐 WebP 格式以节省带宽)
大小:        每张 < 300KB (最佳: 100-200KB)
优化:        压缩并优化，支持响应式加载
```

### 功能卡片规格
```
尺寸:        400px × 400px 或正方形
格式:        PNG / WebP
大小:        每张 < 100KB
背景:        透明或纯色，与卡片风格搭配
```

## 配置示例

### 完整的自定义配置

```typescript
// ==================== 轮播配置 ====================
const backgroundImages = [
  // 方案1：使用本地资源
  new URL('@/assets/img/campus-library.jpg', import.meta.url).href,
  new URL('@/assets/img/campus-garden.jpg', import.meta.url).href,
  new URL('@/assets/img/campus-building.jpg', import.meta.url).href,
  new URL('@/assets/img/campus-entrance.jpg', import.meta.url).href,
  
  // 方案2：使用外部 CDN
  // 'https://cdn.example.com/images/campus-1.jpg',
  // 'https://cdn.example.com/images/campus-2.jpg',
]

// ==================== 功能卡片配置 ====================
const featureCards = [
  { 
    id: 1, 
    title: '智能检索',
    description: '基于向量的混合搜索',
    icon: '🔍',
    image: new URL('@/assets/img/feature-search.png', import.meta.url).href
  },
  { 
    id: 2, 
    title: '知识管理',
    description: '多源知识库整合',
    icon: '📚',
    image: new URL('@/assets/img/feature-kb.png', import.meta.url).href
  },
  { 
    id: 3, 
    title: 'AI对话',
    description: '智能学术助手',
    icon: '🤖',
    image: new URL('@/assets/img/feature-ai.png', import.meta.url).href
  },
  { 
    id: 4, 
    title: '药学工具',
    description: '专业药学工具集',
    icon: '⚗️',
    image: new URL('@/assets/img/feature-tools.png', import.meta.url).href
  }
]
```

## 品牌定制

### 修改品牌文本

**文件位置**：模板部分第101-105行

```vue
<!-- 修改品牌名称 -->
<h1 class="brand-name">中国药科大学 · 药到知来</h1>

<!-- 修改 Slogan -->
<p class="brand-slogan">药问，就知道</p>

<!-- 修改英文名 -->
<p class="brand-en">CPUBrains</p>
```

### 修改颜色主题

修改样式变量（`.less` 文件顶部）：

```less
// 修改梯度色
--gradient-primary: linear-gradient(135deg, #667eea 0%, #764ba2 100%);

// 修改主色
--color-blue: #667eea;
--color-purple: #764ba2;

// 或使用 CSS 变量覆盖
:root {
  --gradient-primary: linear-gradient(135deg, #00d4ff 0%, #0099ff 100%);
  --color-blue: #00d4ff;
  --color-purple: #0099ff;
}
```

## 轮播配置

### 修改自动播放时间

**文件位置**：模板部分 `.bg-carousel` Swiper 属性

```vue
<!-- 修改前：5秒切换 -->
<swiper :autoplay="{ delay: 5000, disableOnInteraction: false }">

<!-- 修改后：3秒切换 -->
<swiper :autoplay="{ delay: 3000, disableOnInteraction: false }">
```

### 修改过渡效果

```vue
<!-- 当前使用 Fade 效果 -->
:effect="'fade'"
:fade-effect="{ crossFade: true }"

<!-- 改为 Slide 效果 -->
:effect="'slide'"
:speed="1000"
```

## i18n 多语言支持

### 添加新语言

1. 在 `featureCards` 中添加 i18n 翻译：

```typescript
// 方案1：直接使用 i18n
const featureCards = computed(() => [
  { 
    id: 1, 
    title: t('feature.search'),
    icon: '🔍'
  },
  // ...
])

// 方案2：创建多语言数据对象
const featureCardsI18n = {
  'zh-CN': [
    { id: 1, title: '智能检索', icon: '🔍' },
    { id: 2, title: '知识管理', icon: '📚' },
  ],
  'en-US': [
    { id: 1, title: 'Smart Search', icon: '🔍' },
    { id: 2, title: 'Knowledge Management', icon: '📚' },
  ]
}

const featureCards = computed(() => featureCardsI18n[currentLanguage.value])
```

## 性能优化建议

### 1. 图片优化

```bash
# 使用 WebP 格式（更小的文件大小）
convert image.jpg -quality 80 image.webp

# 使用 TinyPNG/TinyJPG 压缩
# 或使用在线工具压缩

# 推荐工具
- ImageOptim (macOS)
- ImageMagick (CLI)
- Sharp (Node.js)
```

### 2. 响应式图片

```html
<!-- 推荐使用 srcset 提供多分辨率 -->
<img 
  src="image-500w.jpg"
  srcset="image-300w.jpg 300w, image-500w.jpg 500w, image-800w.jpg 800w"
  alt="Background"
/>
```

### 3. 延迟加载

```vue
<!-- 不需要立即显示的图片 -->
<img 
  :src="feature.image" 
  loading="lazy"
  alt="Feature"
/>
```

## 常见问题

### Q: 如何添加视频背景？

A: 修改轮播以支持视频：

```vue
<swiper-slide v-for="(item, index) in slides" :key="index">
  <video v-if="item.type === 'video'" autoplay muted loop>
    <source :src="item.src" type="video/mp4" />
  </video>
  <div v-else class="carousel-bg" :style="{ backgroundImage: `url('${item.src}')` }"></div>
</swiper-slide>
```

### Q: 如何改变卡片布局（如 3 列而非 4 列）？

A: 修改样式：

```less
.feature-cards {
  grid-template-columns: repeat(3, 1fr);  // 改为 3 列
  // 或
  grid-template-columns: repeat(auto-fit, minmax(100px, 1fr));  // 自适应
}
```

### Q: 如何禁用自动播放？

A: 修改轮播配置：

```vue
<swiper 
  :autoplay="false"  <!-- 禁用自动播放 -->
>
```

## 测试清单

- [ ] 所有图片正确加载
- [ ] 轮播在桌面、平板、手机上工作正常
- [ ] 颜色对比度满足 WCAG 标准
- [ ] 文本在所有分辨率上可读
- [ ] 表单提交正常
- [ ] 语言切换正常
- [ ] 页面加载时间 < 3 秒

## 部署检查

```bash
# 构建前端
cd frontend
npm run build

# 检查输出大小
du -sh dist/

# 本地预览
npm run preview

# 上传到服务器
# 将 dist 文件夹上传到 CDN 或 Web 服务器
```

---

**最后更新**：2026-07-02  
**维护者**：前端团队
