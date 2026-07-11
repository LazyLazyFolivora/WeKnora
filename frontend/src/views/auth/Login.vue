dev<template>
  <div class="login-page">
    <div class="login-container">
    <!-- 左侧登录面板 -->
    <div class="login-panel">
      <!-- 登录表单区域 -->
      <div class="panel-content">
        <!-- Logo + 品牌 -->
        <div class="brand-section">
<img src="@/assets/img/login-page/中国药科大学-logo-2048px.png" alt="Logo" class="brand-logo" />
          <h1 class="brand-title"><span class="brand-title-text">中国药科大学</span> · <span class="brand-title-gradient">药到知来</span></h1>
          <p class="brand-subtitle">CPUBrain</p>
          <p class="brand-slogan">「药问，就知道」</p>
        </div>

        <!-- 登录模式 - 仅显示统一身份认证入口 -->
        <div class="form-card">
          <div class="form-header">
            <h2 class="form-title">{{ $t('auth.login') }}</h2>
            <p class="form-desc">{{ $t('auth.subtitle') }}</p>
          </div>

          <div class="form-body">
            <!-- 统一身份认证 / CAS 登录 -->
            <div class="oidc-section" v-if="oidcEnabled || casEnabled">
              <t-button v-if="oidcEnabled" theme="primary" size="large" block :loading="oidcLoading"
                class="oidc-btn oidc-btn--primary" @click="handleOIDCLogin">
                {{ oidcLoading ? $t('auth.redirectingToOIDC') : oidcLoginText }}
              </t-button>
              <t-button v-if="casEnabled" theme="primary" size="large" block :loading="casLoading"
                class="oidc-btn oidc-btn--primary" @click="handleCASLogin">
                {{ casLoading ? $t('auth.redirectingToCAS') : casLoginText }}
              </t-button>
            </div>

            <!-- 加载状态 -->
            <div v-if="!oidcEnabled && !casEnabled && authConfigLoading" class="auth-loading">
              <t-loading size="small" />
            </div>

            <!-- 无可用认证方式提示 -->
            <div v-if="!oidcEnabled && !casEnabled && !authConfigLoading" class="auth-unavailable">
              <p>{{ $t('auth.noAuthMethod') }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- 底部版权 -->
      <div class="panel-footer">
        <span>Copyright © CPUBrain, All Right Reserved</span>
      </div>
    </div>

    <!-- 右侧展示面板 -->
    <div class="showcase-panel">
      <!-- 背景图片轮播（无滤镜，暴露原图） -->
      <div class="bg-carousel">
        <div v-for="(slide, index) in backgroundSlides" :key="index" class="bg-slide"
          :class="{ active: currentBgSlide === index }" :style="{ backgroundImage: `url(${slide})` }"></div>
      </div>

      <!-- 功能展示卡片 - 水平排列在文字上方 -->
      <div class="feature-cards">
        <div class="feature-item">
          <div class="feature-header">
            <div class="feature-icon">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2"
                stroke-linecap="round" stroke-linejoin="round">
                <path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20" />
                <path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" />
                <line x1="8" y1="7" x2="16" y2="7" />
                <line x1="8" y1="11" x2="14" y2="11" />
              </svg>
            </div>
            <span class="feature-text">{{ $t('platform.multimodalParsing') }}</span>
          </div>
          <img src="/video/自建知识库2.gif" alt="自建知识库" class="feature-gif" />
        </div>

        <div class="feature-item">
          <div class="feature-header">
            <div class="feature-icon">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2"
                stroke-linecap="round" stroke-linejoin="round">
                <circle cx="11" cy="11" r="8" />
                <line x1="21" y1="21" x2="16.65" y2="16.65" />
              </svg>
            </div>
            <span class="feature-text">{{ $t('platform.hybridSearchEngine') }}</span>
          </div>
          <img src="/video/定向回答.gif" alt="定向回答" class="feature-gif" />
        </div>

        <div class="feature-item">
          <div class="feature-header">
            <div class="feature-icon">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2"
                stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="5" r="2.5" />
                <circle cx="5" cy="19" r="2.5" />
                <circle cx="19" cy="19" r="2.5" />
                <circle cx="19" cy="11" r="2" />
                <circle cx="5" cy="11" r="2" />
                <line x1="12" y1="7.5" x2="5" y2="9" />
                <line x1="12" y1="7.5" x2="19" y2="9" />
                <line x1="7" y1="11" x2="5" y2="16.5" />
                <line x1="17" y1="11" x2="19" y2="16.5" />
                <line x1="7.5" y1="19" x2="16.5" y2="19" />
              </svg>
            </div>
            <span class="feature-text">{{ $t('platform.ragQandA') }}</span>
          </div>
          <img src="/video/图谱推理.gif?v=2" alt="图谱推理" class="feature-gif" />
        </div>
      </div>

      <!-- 底部文字 -->
      <div class="showcase-footer">
        <h2 class="showcase-title">药问，就知道</h2>
        <p class="showcase-desc">{{ $t('platform.hybridSearchEngine') }} · {{ $t('platform.ragQandA') }}</p>
        <!-- 轮播指示器 -->
        <div class="carousel-indicators">
          <span v-for="(_, index) in backgroundSlides" :key="index" class="indicator"
            :class="{ active: currentBgSlide === index }"></span>
        </div>
      </div>
    </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, onMounted, onBeforeUnmount, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import {
  getOIDCAuthorizationURL,
  getOIDCConfig,
  getCASLoginURL,
  getCASConfig,
  autoSetup,
  userInfoFromApi,
} from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'

// Import background images
import bgImage1 from '@/assets/img/login-page/login-picture1.jpg'
import bgImage2 from '@/assets/img/login-page/login-picture2.jpg'
import bgImage3 from '@/assets/img/login-page/login-picture3.jpg'
import bgImage4 from '@/assets/img/login-page/login-picture4.jpg'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { t } = useI18n()

// Background carousel
const backgroundSlides = [bgImage1, bgImage2, bgImage3, bgImage4]
const currentBgSlide = ref(0)
const bgCarouselInterval = ref<number | null>(null)

const startBgCarousel = () => {
  bgCarouselInterval.value = window.setInterval(() => {
    currentBgSlide.value = (currentBgSlide.value + 1) % backgroundSlides.length
  }, 5000)
}

const stopBgCarousel = () => {
  if (bgCarouselInterval.value !== null) {
    clearInterval(bgCarouselInterval.value)
    bgCarouselInterval.value = null
  }
}

// State management
const oidcLoading = ref(false)
const casLoading = ref(false)
const oidcEnabled = ref(false)
const oidcProviderName = ref('')
const casEnabled = ref(false)
const casProviderName = ref('')
const authConfigLoading = ref(true)

const oidcLoginText = computed(() => {
  if (oidcProviderName.value) {
    return t('auth.oidcLoginWithProvider', { provider: oidcProviderName.value })
  }
  return t('auth.oidcLogin')
})
const casLoginText = computed(() => {
  if (casProviderName.value) {
    return t('auth.casLoginWithProvider', { provider: casProviderName.value })
  }
  return t('auth.casLogin')
})

onBeforeUnmount(() => {
  stopBgCarousel()
})

const persistLoginResponse = async (response: any) => {
  const activeTenant = response.active_tenant || response.tenant
  if (response.user && activeTenant && response.token) {
    const homeTenantIdRaw = response.user.tenant_id ?? activeTenant?.id ?? ''
    authStore.setUser(userInfoFromApi(response.user, homeTenantIdRaw))
    authStore.setToken(response.token)
    if (response.refresh_token) {
      authStore.setRefreshToken(response.refresh_token)
    }
    if (activeTenant) {
      authStore.setTenant({
        id: String(activeTenant.id) || '',
        name: activeTenant.name || '',
        owner_id: response.user.id || '',
        created_at: activeTenant.created_at || new Date().toISOString(),
        updated_at: activeTenant.updated_at || new Date().toISOString()
      })
    } else {
      authStore.setTenant(null)
    }
    if (Array.isArray(response.memberships)) {
      authStore.setMemberships(response.memberships)
    }
    // If the backend dropped us into a non-home tenant (honoured a
    // remembered "last active tenant" preference), set the override so
    // subsequent requests carry X-Tenant-ID and the UI stays consistent.
    // Otherwise clear any stale override left in localStorage by a
    // previous session for a different account.
    const activeIdNum = Number(activeTenant?.id)
    const homeIdNum = Number(homeTenantIdRaw)
    if (Number.isFinite(activeIdNum) && Number.isFinite(homeIdNum) && activeIdNum !== homeIdNum) {
      authStore.setSelectedTenant(activeIdNum, activeTenant?.name || null)
    } else {
      authStore.setSelectedTenant(null, null)
    }
  }

  // Pull runtime capabilities (including whether ordinary users may create
  // workspaces) before entering the main UI so create actions never flash
  // briefly when the deployment is invitation-only.
  await authStore.refreshFromAuthMe()
  await nextTick()
  router.replace(authStore.hasValidTenant ? '/platform/knowledge-bases' : '/onboarding/workspace')
}

const getBackendOIDCRedirectURI = () => `${window.location.origin}/api/v1/auth/oidc/callback`
const getBackendCASRedirectURI = () => `${window.location.origin}/api/v1/auth/cas/callback`

const loadOIDCConfig = async () => {
  try {
    const response = await getOIDCConfig()
    oidcEnabled.value = !!response.success && !!response.enabled
    oidcProviderName.value = response.provider_display_name || ''
  } catch {
    oidcEnabled.value = false
    oidcProviderName.value = ''
  }
}

const loadCASConfig = async () => {
  try {
    const response = await getCASConfig()
    casEnabled.value = !!response.success && !!response.enabled
    casProviderName.value = response.provider_display_name || ''
  } catch {
    casEnabled.value = false
    casProviderName.value = ''
  }
}

const handleOIDCLogin = async () => {
  try {
    oidcLoading.value = true
    const response = await getOIDCAuthorizationURL(getBackendOIDCRedirectURI())
    const authorizationURL = response.authorization_url

    if (!response.success || !authorizationURL) {
      MessagePlugin.error(response.message || t('auth.oidcLoginFailed'))
      return
    }

    window.location.href = authorizationURL
  } catch (error: any) {
    console.error('OIDC 登录跳转失败:', error)
    MessagePlugin.error(error.message || t('auth.oidcLoginFailed'))
  } finally {
    oidcLoading.value = false
  }
}

const handleCASLogin = async () => {
  try {
    casLoading.value = true
    const response = await getCASLoginURL(getBackendCASRedirectURI())
    const authorizationURL = response.authorization_url

    if (!response.success || !authorizationURL) {
      MessagePlugin.error(response.message || t('auth.casLoginFailed'))
      return
    }

    window.location.href = authorizationURL
  } catch (error: any) {
    console.error('CAS 登录跳转失败:', error)
    MessagePlugin.error(error.message || t('auth.casLoginFailed'))
  } finally {
    casLoading.value = false
  }
}

onMounted(async () => {
  startBgCarousel()

  if (authStore.isLoggedIn) {
    router.replace('/platform/knowledge-bases')
    return
  }

  const AUTO_SETUP_FAILED_KEY = 'weknora_auto_setup_failed'
  // URL 带 ?skip 或 ?debug 参数时跳过 autoSetup，方便开发调试登录页
  const skipAutoSetup = route.query.skip !== undefined || route.query.debug !== undefined
  if (!skipAutoSetup && localStorage.getItem(AUTO_SETUP_FAILED_KEY) !== 'true') {
    try {
      const response = await autoSetup()
      if (response.success) {
        authStore.setLiteMode(true)
        await persistLoginResponse(response)
        return
      } else {
        localStorage.setItem(AUTO_SETUP_FAILED_KEY, 'true')
      }
    } catch {
      localStorage.setItem(AUTO_SETUP_FAILED_KEY, 'true')
    }
  }

  await Promise.all([loadOIDCConfig(), loadCASConfig()])
  authConfigLoading.value = false
})
</script>

<style lang="less" scoped>
/* ========================================
   登录页面 - 左右分栏布局
   ======================================== */
.login-page {
  display: flex;
  width: 100%;
  min-height: 100vh;
  overflow: hidden;
  background: #eef1f6;
  align-items: center;
  justify-content: center;
  padding: 5vh 5%;
  box-sizing: border-box;
}

/* 圆角容器 - 两个独立的圆角卡片 */
.login-container {
  display: flex;
  gap: 20px;
  width: 100%;
  height: 90vh;
  align-items: stretch;
  justify-content: center;
}

/* ========================================
   左侧登录面板
   ======================================== */
.login-panel {
  width: 460px;
  min-width: 400px;
  display: flex;
  flex-direction: column;
  background: #ffffff;
  border-radius: 24px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  position: relative;
  z-index: 2;
  overflow: hidden;
  box-sizing: border-box;
}

/* 内容区域 */
.panel-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 20px 40px;
  overflow-y: auto;
}

/* 品牌区域 */
.brand-section {
  text-align: center;
  margin-bottom: 32px;
}

.brand-logo {
  width: 80px;
  height: 80px;
  object-fit: contain;
  margin-bottom: 16px;
}

.brand-title {
  font-size: 22px;
  font-weight: 600;
  margin: 0 0 4px;
  font-family: var(--app-font-family);
  letter-spacing: 1px;
}

.brand-title-text {
  color: #1a1a2e;
}

.brand-title-gradient {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.brand-subtitle {
  font-size: 14px;
  color: #667eea;
  font-weight: 600;
  margin: 0 0 8px;
  font-family: var(--app-font-family);
  letter-spacing: 2px;
}

.brand-slogan {
  font-size: 13px;
  color: #909399;
  margin: 0;
  font-family: var(--app-font-family);
  font-style: italic;
}

/* 表单卡片 */
.form-card {
  width: 100%;
  max-width: 380px;
}

.form-header {
  text-align: center;
  margin-bottom: 20px;
}

.form-title {
  font-size: 22px;
  font-weight: 600;
  color: #1a1a2e;
  margin: 0 0 6px;
  font-family: var(--app-font-family);
}

.form-desc {
  font-size: 13px;
  color: #909399;
  margin: 0;
  font-family: var(--app-font-family);
  line-height: 1.5;
}

/* 表单内容 */
.form-body {
  display: flex;
  flex-direction: column;
  align-items: center;
}

/* 登录按钮 */
.submit-btn {
  height: 48px;
  border-radius: 10px;
  font-size: 16px;
  font-weight: 600;
  font-family: var(--app-font-family);
  margin-top: 8px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%) !important;
  border: none !important;
  color: #ffffff;
  transition: all 0.3s;

  &:hover {
    opacity: 0.9;
    transform: translateY(-1px);
    box-shadow: 0 6px 20px rgba(102, 126, 234, 0.35);
  }

  &:active {
    transform: translateY(0);
  }
}

/* 统一身份认证 区域 */
.oidc-section {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.oidc-btn--primary {
  height: 48px;
  border-radius: 10px;
  font-size: 16px;
  font-weight: 600;
  font-family: var(--app-font-family);
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%) !important;
  border: none !important;
  color: #ffffff;
  transition: all 0.3s;

  &:hover {
    opacity: 0.9;
    transform: translateY(-1px);
    box-shadow: 0 6px 20px rgba(102, 126, 234, 0.35);
  }

  &:active {
    transform: translateY(0);
  }
}

/* 加载状态 */
.auth-loading {
  display: flex;
  justify-content: center;
  padding: 24px 0;
}

/* 无可用认证方式提示 */
.auth-unavailable {
  text-align: center;
  padding: 24px 0;
  color: #909399;
  font-size: 14px;
  font-family: var(--app-font-family);
}

/* 底部版权 */
.panel-footer {
  padding: 16px 28px;
  text-align: center;
  font-size: 12px;
  color: #c0c4cc;
  font-family: var(--app-font-family);
}

/* ========================================
   右侧展示面板
   ======================================== */
.showcase-panel {
  flex: 1;
  position: relative;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  align-items: center;
  padding-bottom: 48px;
  background: #f0f2f5;
  border-radius: 24px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
  box-sizing: border-box;
}

/* 背景图片轮播 */
.bg-carousel {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
}

.bg-slide {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background-size: cover;
  background-position: center;
  opacity: 0;
  transition: opacity 1.2s ease-in-out;

  &.active {
    opacity: 1;
  }
}

/* 底部展示文字 */
.showcase-footer {
  position: relative;
  z-index: 10;
  text-align: center;
}

.showcase-title {
  font-size: 32px;
  font-weight: 700;
  color: #1a1a2e;
  margin: 0 0 10px;
  font-family: var(--app-font-family);
  letter-spacing: 3px;
}

.showcase-desc {
  font-size: 14px;
  color: #606266;
  margin: 0 0 20px;
  font-family: var(--app-font-family);
}

/* 轮播指示器 */
.carousel-indicators {
  display: flex;
  justify-content: center;
  gap: 8px;
}

.indicator {
  width: 32px;
  height: 4px;
  border-radius: 2px;
  background: #d0d3d8;
  transition: all 0.3s;

  &.active {
    background: #667eea;
    width: 48px;
  }
}

/* 功能展示卡片 - 水平排列 */
.feature-cards {
  display: flex;
  justify-content: center;
  gap: 24px;
  padding: 0 20px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}

.feature-item {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  background: rgba(255, 255, 255, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.3);
  border-radius: 12px;
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  transition: all 0.3s;
  width: 260px;
}

.feature-header {
  display: flex;
  align-items: center;
  gap: 10px;
}

.feature-icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  background: rgba(102, 126, 234, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.feature-text {
  font-size: 14px;
  font-weight: 500;
  color: #ffffff;
  font-family: var(--app-font-family);
  white-space: nowrap;
}

.feature-image-placeholder {
  width: 100%;
  height: 60px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.15);
  border: 1px dashed rgba(255, 255, 255, 0.3);
}

.feature-gif {
  width: 100%;
  height: 200px;
  object-fit: cover;
  border-radius: 8px;
  aspect-ratio: 16 / 9;
}

/* ========================================
   响应式设计
   ======================================== */
@media (max-width: 1024px) {
  .login-page {
    padding: 4vh 6vw;
  }

  .login-container {
    height: 92vh;
  }

  .login-panel {
    width: 400px;
    min-width: 360px;
  }

  .panel-content {
    padding: 20px 32px;
  }
}

@media (max-width: 768px) {
  .login-page {
    padding: 0;
  }

  .login-container {
    height: 100vh;
    gap: 0;
  }

  .login-panel {
    width: 100%;
    min-width: auto;
    flex: 1;
    order: 1;
    border-radius: 0;
  }

  .showcase-panel {
    display: none;
  }

  .panel-content {
    padding: 20px 10vw;
  }

  .brand-title {
    font-size: 20px;
  }
}

@media (max-width: 480px) {
  .panel-content {
    padding: 16px 6vw;
  }

  .brand-title {
    font-size: 18px;
  }

  .brand-logo {
    width: 60px;
    height: 60px;
  }

  .form-card {
    max-width: 100%;
  }
}

/* ========================================
   深色模式
   ======================================== */
html[theme-mode="dark"] {
  .login-page {
    background: #0a0a1a;
  }

  .login-panel {
    background: #141428;
  }

  .brand-title-text {
    color: #e8eaf0;
  }

  .brand-subtitle {
    color: #8b9cf7;
  }

  .brand-slogan {
    color: #606370;
  }

  .form-title {
    color: #e8eaf0;
  }

  .form-desc {
    color: #606370;
  }

  .panel-footer {
    color: #3a3a5a;
  }
}

@media (prefers-reduced-motion: reduce) {
  .bg-slide {
    transition: none !important;
  }
}
</style>