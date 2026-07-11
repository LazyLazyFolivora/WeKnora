# 登录页面设计规范

## 色彩系统

### 主色系
```
Primary Blue:     #667eea (102, 126, 234)
Primary Purple:   #764ba2 (118, 75, 162)
Gradient:         linear-gradient(135deg, #667eea → #764ba2)
```

### 背景色
```
Dark Base:        #0f172a (15, 23, 42)
Dark Secondary:   #1a1f3a (26, 31, 58)
Dark Tertiary:    #2d1b4e (45, 27, 78)
```

### 文本色
```
Text Primary:     #ffffff (255, 255, 255)
Text Secondary:   rgba(255, 255, 255, 0.8)
Text Tertiary:    rgba(255, 255, 255, 0.6)
Text Muted:       rgba(255, 255, 255, 0.4)
```

### 边框/线条
```
Border Strong:    rgba(255, 255, 255, 0.15)
Border Medium:    rgba(255, 255, 255, 0.1)
Border Weak:      rgba(255, 255, 255, 0.08)
```

## 布局尺寸

### 页面容器
```
总宽度:          100% (全屏)
总高度:          100vh (全屏)
左侧轮播区:      50% (1024px+) / 40% (768-1024px) / 35% (<768px)
右侧表单区:      50% / 60% / 65%
```

### 内间距（Padding）
```
大容器:          40px
卡片:            28px
表单行:          18px 间距
标签:            8px margin
```

### 圆角（Border Radius）
```
卡片:            16px
按钮:            8px
输入框:          8px
功能卡片:        16px
```

### 字体大小
```
品牌名 (Desktop):    32px / 24px / 20px / 16px
表单标题:           24px / 20px / 18px
表单标签:           13px / 13px / 12px
文本内容:           14px / 13px / 12px
```

## 组件设计

### 输入框
```css
Background:     rgba(255, 255, 255, 0.05)
Border:         1px solid rgba(255, 255, 255, 0.1)
Hover Border:   rgba(102, 126, 234, 0.4)
Focus Border:   #667eea
Focus Shadow:   0 0 0 3px rgba(102, 126, 234, 0.1)
Border Radius:  8px
```

### 按钮

#### 主按钮 (Submit)
```css
Background:     linear-gradient(135deg, #667eea → #764ba2)
Padding:        h: 40px
Font Weight:    600
Hover:          
  - Box Shadow: 0 8px 24px rgba(102, 126, 234, 0.4)
  - Transform:  translateY(-2px)
Active:
  - Transform:  translateY(0)
```

#### 次按钮 (Register)
```css
Background:     transparent
Border:         1px solid #667eea
Color:          #667eea
Hover Background: rgba(102, 126, 234, 0.1)
```

### 轮播卡片
```css
Background:     rgba(255, 255, 255, 0.08)
Backdrop Filter: blur(10px)
Border:         1px solid rgba(255, 255, 255, 0.15)
Border Radius:  16px
Aspect Ratio:   1:1
Hover:
  - Background: rgba(255, 255, 255, 0.12)
  - Border:     rgba(255, 255, 255, 0.25)
  - Transform:  translateY(-4px)
  - Duration:   300ms ease
```

### 语言选择菜单
```css
Background:     rgba(25, 35, 70, 0.95)
Backdrop Filter: blur(10px)
Border:         1px solid rgba(102, 126, 234, 0.2)
Border Radius:  8px
Min Width:      140px
Item Padding:   10px 14px
Hover Item:
  - Background: rgba(102, 126, 234, 0.1)
  - Color:      rgba(255, 255, 255, 0.9)
Active Item:
  - Background: rgba(102, 126, 234, 0.2)
  - Color:      #667eea
```

## 动画 & 过渡

### 轮播
```
Duration:       5000ms (自动播放间隔)
Transition:     1000ms (slide过渡)
Effect:         Fade with crossFade
```

### UI 交互
```
标准过渡:       0.3s ease
快速反馈:       0.2s ease
文本过渡:       0.3s ease
```

## 响应式断点

### Desktop
```
宽度:           >1024px
布局:           flex row 1:1
轮播:           显示所有功能卡片 (grid 4列)
字体:           最大尺寸
```

### Tablet
```
宽度:           768px - 1024px
布局:           flex row 40:60
轮播:           功能卡片 (grid 2列)
字体:           中等尺寸
```

### Mobile
```
宽度:           <768px
布局:           flex column (上下堆叠)
轮播:           隐藏功能卡片
字体:           最小尺寸
特殊:           轮播高度: 200-300px
```

### Small Mobile
```
宽度:           <480px
布局:           全宽堆叠
功能卡片:       完全隐藏
品牌文字:       更小尺寸
表单:           最小化 padding
```

## 空间系统

### Spacing Scale
```
4px   - xs
8px   - sm
12px  - md
16px  - lg
20px  - xl
24px  - 2xl
28px  - 3xl
32px  - 4xl
40px  - 5xl
60px  - 6xl
```

### Gap 值
```
Form Items:     18px (行间距)
Cards:          16px (卡片间距)
Elements:       8px-12px (元素间距)
Sections:       28px-40px (大区块间距)
```

## 阴影系统

```
Subtle:         0 0 8px rgba(255, 255, 255, 0.15)
Light:          0 4px 16px rgba(0, 0, 0, 0.2)
Medium:         0 8px 24px rgba(102, 126, 234, 0.4)
Heavy:          0 10px 40px rgba(0, 0, 0, 0.4)
Dark Mode XL:   0 20px 60px rgba(0, 0, 0, 0.3)
```

## 特殊效果

### 玻璃态 (Glassmorphism)
```css
Background:     rgba(X, X, X, 0.08) / rgba(X, X, X, 0.12)
Backdrop Filter: blur(10px-20px)
Border:         1px solid rgba(255, 255, 255, 0.1-0.25)
```

### 渐变文本
```css
Background:     linear-gradient(135deg, #ffffff 0%, #e0e7ff 100%)
-webkit-background-clip: text
-webkit-text-fill-color: transparent
background-clip: text
```

### 品牌渐变
```css
Background:     linear-gradient(135deg, #667eea 0%, #764ba2 100%)
-webkit-background-clip: text
-webkit-text-fill-color: transparent
background-clip: text
```

## 可访问性

### 颜色对比度
- 文本 vs 背景：最小 4.5:1（WCAG AA）
- 交互元素：清晰的焦点状态
- 错误状态：结合颜色和图标/文本

### 交互元素
```
最小点击区域: 44x44px
焦点可见性:   0 0 0 3px 高亮环
Tab 顺序:     逻辑正确
```

## 国际化支持

### 文本长度规划
```
中文:           12-32字节 (最紧凑)
英文:           100-200字节 (最冗长)
俄文:           80-150字节
韩文:           12-40字符
```

### 字体堆栈
```
Primary:        -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto
Fallback:       'Helvetica Neue', Arial, sans-serif
CJK Support:    内置系统字体自动选择
```

---

## 实现检查清单

- [x] 色彩系统完整
- [x] 布局响应式
- [x] 动画流畅
- [x] 组件设计一致
- [x] 国际化支持
- [x] 可访问性考虑
- [ ] 生产环境图片资源
- [ ] 功能卡片内容

