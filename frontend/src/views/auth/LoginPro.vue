<template>
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

        <!-- 登录模式 -->
        <div class="form-card" v-if="!isRegisterMode">
          <div class="form-header">
            <h2 class="form-title">{{ $t('auth.login') }}</h2>
            <p class="form-desc">{{ $t('auth.subtitle') }}</p>
          </div>

          <!-- Tab 切换：Sign In / Sign Up -->
          <div class="mode-tabs" v-if="registrationEnabled">
            <button class="mode-tab active" @click="toggleMode">{{ $t('auth.login') }}</button>
            <button class="mode-tab" @click="toggleMode">{{ $t('auth.createAccount') }}</button>
          </div>

          <div class="form-body">
            <t-form ref="formRef" :data="formData" :rules="formRules" @submit="handleLogin" layout="vertical">
              <t-form-item :label="$t('auth.email')" name="email">
                <t-input v-model="formData.email" :placeholder="$t('auth.emailPlaceholder')" type="text"
                  autocomplete="email" size="large" :disabled="loading" />
              </t-form-item>

              <t-form-item :label="$t('auth.password')" name="password">
                <t-input v-model="formData.password" :placeholder="$t('auth.passwordPlaceholder')" type="password"
                  autocomplete="current-password" size="large" :disabled="loading" @enter="handleLogin" />
              </t-form-item>

              <t-button type="submit" theme="primary" size="large" block :loading="loading" class="submit-btn">
                {{ loading ? $t('auth.loggingIn') : $t('auth.login') }}
              </t-button>
            </t-form>

            <!-- 统一身份认证 / CAS 登录（只显示有效的那个） -->
            <div class="oidc-section" v-if="oidcEnabled || casEnabled">
              <div class="oidc-divider">
                <span>{{ $t('auth.orContinueWith') }}</span>
              </div>
              <t-button v-if="oidcEnabled" theme="default" size="large" block :loading="oidcLoading" :disabled="loading"
                class="oidc-btn" @click="handleOIDCLogin">
                {{ oidcLoading ? $t('auth.redirectingToOIDC') : oidcLoginText }}
              </t-button>
              <t-button v-if="casEnabled" theme="default" size="large" block :loading="casLoading" :disabled="loading"
                class="oidc-btn" @click="handleCASLogin">
                {{ casLoading ? $t('auth.redirectingToCAS') : casLoginText }}
              </t-button>
            </div>

          </div>
        </div>

        <!-- 注册表单 -->
        <div class="form-card" v-if="isRegisterMode && (registrationEnabled || inviteLookup)">
          <!-- 邀请横幅 -->
          <div v-if="inviteLookup" class="invite-banner">
            <t-icon name="link" class="invite-banner__icon" />
            <div class="invite-banner__text">
              <div class="invite-banner__title">
                {{ $t('inviteRegister.bannerTitle', { tenant: inviteLookup.tenant_name || '' }) }}
              </div>
              <div class="invite-banner__hint">
                {{ $t('inviteRegister.bannerHint') }}
              </div>
            </div>
          </div>
          <div v-else-if="inviteLookupError" class="invite-banner invite-banner--error">
            {{ inviteLookupError }}
          </div>

          <div class="form-header">
            <h2 class="form-title">{{ $t('auth.createAccount') }}</h2>
            <p class="form-desc">{{ $t('auth.registerSubtitle') }}</p>
          </div>

          <!-- Tab 切换 -->
          <div class="mode-tabs">
            <button class="mode-tab" @click="toggleMode">{{ $t('auth.login') }}</button>
            <button class="mode-tab active" @click="toggleMode">{{ $t('auth.createAccount') }}</button>
          </div>

          <div class="form-body">
            <t-form ref="registerFormRef" :data="registerData" :rules="registerRules" @submit="handleRegister"
              layout="vertical">
              <t-form-item :label="$t('auth.username')" name="username">
                <t-input v-model="registerData.username" :placeholder="$t('auth.usernamePlaceholder')" size="large"
                  :disabled="loading" />
              </t-form-item>

              <t-form-item :label="$t('auth.email')" name="email">
                <t-input v-model="registerData.email" :placeholder="$t('auth.emailPlaceholder')" type="text"
                  autocomplete="email" size="large" :disabled="loading" />
              </t-form-item>

              <t-form-item :label="$t('auth.password')" name="password">
                <t-input v-model="registerData.password" :placeholder="$t('auth.passwordPlaceholder')" type="password"
                  autocomplete="new-password" size="large" :disabled="loading" />
              </t-form-item>

              <t-form-item :label="$t('auth.confirmPassword')" name="confirmPassword">
                <t-input v-model="registerData.confirmPassword" :placeholder="$t('auth.confirmPasswordPlaceholder')"
                  type="password" autocomplete="new-password" size="large" :disabled="loading" @enter="handleRegister" />
              </t-form-item>

              <t-button type="submit" theme="primary" size="large" block :loading="loading" class="submit-btn">
                {{ loading ? $t('auth.registering') : $t('auth.register') }}
              </t-button>
            </t-form>
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
          <div class="feature-image-placeholder"></div>
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
          <div class="feature-image-placeholder"></div>
        </div>

        <div class="feature-item">
          <div class="feature-header">
            <div class="feature-icon">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="#ffffff" stroke-width="2"
                stroke-linecap="round" stroke-linejoin="round">
                <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
              </svg>
            </div>
            <span class="feature-text">{{ $t('platform.ragQandA') }}</span>
          </div>
          <div class="feature-image-placeholder"></div>
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
import { ref, reactive, nextTick, onMounted, onBeforeUnmount, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { useRoleLabel } from '@/composables/useRoleLabel'
import { notifyLoginSuccess } from '@/utils/loginNotify'
import {
  login,
  register,
  getOIDCAuthorizationURL,
  getOIDCConfig,
  getCASLoginURL,
  getCASConfig,
  autoSetup,
  getAuthConfig,
  userInfoFromApi,
  getInvitationByToken,
  registerByInvite,
  type InviteLookup,
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
const { t, tm } = useI18n()
const { formatRole, roleIcon } = useRoleLabel()

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

// Form references
const formRef = ref()
const registerFormRef = ref()

// State management
const loading = ref(false)
const oidcLoading = ref(false)
const casLoading = ref(false)
const isRegisterMode = ref(false)
const oidcEnabled = ref(false)
const oidcProviderName = ref('')
const casEnabled = ref(false)
const casProviderName = ref('')
// registrationEnabled defaults to true so that on first paint the Register
// link is visible; the actual mode is fetched from /auth/config in onMounted.
// In invite_only mode the link/card are hidden.
const registrationEnabled = ref(true)

// invite-link state
const inviteToken = ref('')
const inviteLookup = ref<InviteLookup | null>(null)
const inviteLookupError = ref('')
const inviteLookupLoading = ref(false)

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
// Login form data
const formData = reactive<{ [key: string]: any }>({
  email: '',
  password: '',
})

// Register form data
const registerData = reactive<{ [key: string]: any }>({
  username: '',
  email: '',
  password: '',
  confirmPassword: ''
})

// Login form validation rules
const formRules = computed(() => ({
  email: [
    { required: true, message: t('auth.emailRequired'), type: 'error' },
    { email: true, message: t('auth.emailInvalid'), type: 'error' }
  ],
  password: [
    { required: true, message: t('auth.passwordRequired'), type: 'error' },
    { min: 8, message: t('auth.passwordMinLength'), type: 'error' },
    { max: 32, message: t('auth.passwordMaxLength'), type: 'error' },
    { pattern: /[a-zA-Z]/, message: t('auth.passwordMustContainLetter'), type: 'error' },
    { pattern: /\d/, message: t('auth.passwordMustContainNumber'), type: 'error' }
  ]
}))

// Register form validation rules
const registerRules = computed(() => ({
  username: [
    { required: true, message: t('auth.usernameRequired'), type: 'error' },
    { min: 2, message: t('auth.usernameMinLength'), type: 'error' },
    { max: 20, message: t('auth.usernameMaxLength'), type: 'error' },
    {
      pattern: /^[a-zA-Z0-9_\u4e00-\u9fa5]+$/,
      message: t('auth.usernameInvalid'),
      type: 'error'
    }
  ],
  email: [
    { required: true, message: t('auth.emailRequired'), type: 'error' },
    { email: true, message: t('auth.emailInvalid'), type: 'error' }
  ],
  password: [
    { required: true, message: t('auth.passwordRequired'), type: 'error' },
    { min: 8, message: t('auth.passwordMinLength'), type: 'error' },
    { max: 32, message: t('auth.passwordMaxLength'), type: 'error' },
    { pattern: /[a-zA-Z]/, message: t('auth.passwordMustContainLetter'), type: 'error' },
    { pattern: /\d/, message: t('auth.passwordMustContainNumber'), type: 'error' }
  ],
  confirmPassword: [
    { required: true, message: t('auth.confirmPasswordRequired'), type: 'error' },
    {
      validator: (val: string) => val === registerData.password,
      message: t('auth.passwordMismatch'),
      type: 'error'
    }
  ]
}))

// Toggle login/register mode
const toggleMode = () => {
  isRegisterMode.value = !isRegisterMode.value
  Object.keys(registerData).forEach(key => {
    (registerData as any)[key] = ''
  })
}

onBeforeUnmount(() => {
  stopBgCarousel()
})

const persistLoginResponse = async (response: any) => {
  const activeTenant = response.active_tenant || response.tenant
  if (response.user && activeTenant && response.token) {
    const homeTenantIdRaw = response.user.tenant_id ?? activeTenant.id
    authStore.setUser(userInfoFromApi(response.user, homeTenantIdRaw))
    authStore.setToken(response.token)
    if (response.refresh_token) {
      authStore.setRefreshToken(response.refresh_token)
    }
    authStore.setTenant({
      id: String(activeTenant.id) || '',
      name: activeTenant.name || '',
      api_key: activeTenant.api_key || '',
      owner_id: response.user.id || '',
      created_at: activeTenant.created_at || new Date().toISOString(),
      updated_at: activeTenant.updated_at || new Date().toISOString()
    })
    if (Array.isArray(response.memberships)) {
      authStore.setMemberships(response.memberships)
    }
    const activeIdNum = Number(activeTenant.id)
    const homeIdNum = Number(homeTenantIdRaw)
    if (Number.isFinite(activeIdNum) && Number.isFinite(homeIdNum) && activeIdNum !== homeIdNum) {
      authStore.setSelectedTenant(activeIdNum, activeTenant.name || null)
    } else {
      authStore.setSelectedTenant(null, null)
    }
  }
  await nextTick()
  router.replace('/platform/knowledge-bases')
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

// loadAuthConfig fetches /auth/config and caches whether self-service
// registration is allowed. Failures fall back to "enabled" so a transient
// network glitch doesn't lock new users out of an open deployment.
const loadAuthConfig = async () => {
  try {
    const response = await getAuthConfig()
    registrationEnabled.value = response.registration_mode !== 'invite_only'
  } catch {
    registrationEnabled.value = true
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

// Handle login
const handleLogin = async () => {
  try {
    const valid = await formRef.value?.validate()
    if (valid !== true) return

    loading.value = true

    const response = await login({
      email: formData.email,
      password: formData.password,
    })

    if (response.success) {
      await persistLoginResponse(response)
      notifyLoginSuccess(response, t, tm, formatRole, roleIcon)
    } else {
      MessagePlugin.error(response.message || t('auth.loginError'))
    }
  } catch (error: any) {
    console.error('登录错误:', error)
    MessagePlugin.error(error.message || t('auth.loginErrorRetry'))
  } finally {
    loading.value = false
  }
}

const handleRegister = async () => {
  try {
    const valid = await registerFormRef.value?.validate()
    if (valid !== true) return

    loading.value = true

    if (inviteToken.value) {
      const response = await registerByInvite({
        token: inviteToken.value,
        username: registerData.username,
        email: registerData.email,
        password: registerData.password,
      })
      if (!response.success) {
        MessagePlugin.error(response.message || t('auth.registerFailed'))
        return
      }
      MessagePlugin.success(t('auth.registerSuccess'))
      await persistLoginResponse(response)
      return
    }

    const response = await register({
      username: registerData.username,
      email: registerData.email,
      password: registerData.password
    })

    if (response.success) {
      MessagePlugin.success(t('auth.registerSuccess'))
      isRegisterMode.value = false
      formData.email = registerData.email

      Object.keys(registerData).forEach(key => {
        (registerData as any)[key] = ''
      })
    } else {
      MessagePlugin.error(response.message || t('auth.registerFailed'))
    }
  } catch (error: any) {
    console.error('注册错误:', error)
    MessagePlugin.error(error.message || t('auth.registerError'))
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  startBgCarousel()
  const tokenFromQuery = String(route.query.token || '').trim()
  if (tokenFromQuery) {
    inviteToken.value = tokenFromQuery
    inviteLookupLoading.value = true
    try {
      const resp = await getInvitationByToken(tokenFromQuery)
      if (resp.success && resp.data) {
        inviteLookup.value = resp.data
        registrationEnabled.value = true
        isRegisterMode.value = true
      } else {
        inviteLookupError.value = resp.message || t('inviteRegister.invalidBody')
      }
    } catch {
      inviteLookupError.value = t('inviteRegister.invalidBody')
    } finally {
      inviteLookupLoading.value = false
    }
    loadOIDCConfig()
    loadCASConfig()
    return
  }

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

  loadOIDCConfig()
  loadCASConfig()
  loadAuthConfig()
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

/* 模式切换 Tab */
.mode-tabs {
  display: flex;
  background: #f5f7fa;
  border-radius: 10px;
  padding: 4px;
  margin-bottom: 24px;
}

.mode-tab {
  flex: 1;
  padding: 10px 0;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #909399;
  font-size: 14px;
  font-weight: 500;
  font-family: var(--app-font-family);
  cursor: pointer;
  transition: all 0.25s;

  &:hover {
    color: #606266;
  }

  &.active {
    background: #ffffff;
    color: #667eea;
    box-shadow: 0 2px 8px rgba(102, 126, 234, 0.15);
    font-weight: 600;
  }
}

/* 表单内容 */
.form-body {
  :deep(.t-form-item__label) {
    font-size: 14px;
    color: #303133;
    font-weight: 500;
    margin-bottom: 8px;
    font-family: var(--app-font-family);
  }

  :deep(.t-input) {
    border: 1.5px solid #e0e4ea;
    border-radius: 10px;
    background: #fafbfc;
    transition: all 0.2s;

    &:hover {
      border-color: #c0c5ce;
    }

    &:focus-within {
      border-color: #667eea;
      box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.1);
      background: #ffffff;
    }

    .t-input__inner {
      font-size: 15px;
      font-family: var(--app-font-family);
      color: #303133;

      &::placeholder {
        color: #c0c4cc;
      }
    }
  }

  :deep(.t-form-item) {
    margin-bottom: 18px;
  }
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
  margin-top: 4px;
}

.oidc-divider {
  position: relative;
  margin: 16px 0 12px;
  text-align: center;
  color: #c0c4cc;
  font-size: 12px;

  span {
    position: relative;
    z-index: 1;
    padding: 0 12px;
    background: #ffffff;
  }

  &::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    top: 50%;
    border-top: 1px solid #e8eaed;
  }
}

.oidc-btn {
  height: 46px;
  border-radius: 10px;
  font-size: 15px;
  font-weight: 500;
  border: 1.5px solid #e0e4ea;
  color: #606266;
  background: #fafbfc;

  &:hover {
    background: #f0f2f5;
    border-color: #c0c5ce;
  }
}

/* 邀请横幅 */
.invite-banner {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  margin-bottom: 16px;
  border-radius: 10px;
  background: #f0f5ff;
  border: 1px solid #d4e4ff;
}

.invite-banner__icon {
  margin-top: 2px;
  font-size: 18px;
  color: #667eea;
}

.invite-banner__title {
  font-size: 14px;
  font-weight: 600;
  color: #1a1a2e;
}

.invite-banner__hint {
  font-size: 12px;
  color: #909399;
}

.invite-banner--error {
  background: #fef0f0;
  border-color: #fde2e2;
  color: #f56c6c;
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

/* 浮动功能卡片 */
.floating-cards {
  position: relative;
  z-index: 10;
  width: 80%;
  max-width: 520px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-bottom: 32px;
}

.feature-card {
  background: rgba(255, 255, 255, 0.25);
  border: 1px solid rgba(255, 255, 255, 0.35);
  border-radius: 16px;
  padding: 20px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.08);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  transition: transform 0.3s, box-shadow 0.3s;

  &:hover {
    transform: translateY(-4px);
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.15);
    background: rgba(255, 255, 255, 0.35);
  }
}

.card-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 12px;
  background: rgba(255, 255, 255, 0.3);
  border: 1px solid rgba(255, 255, 255, 0.2);
}

.card-label {
  font-size: 14px;
  font-weight: 600;
  color: #ffffff;
  font-family: var(--app-font-family);
  margin-bottom: 12px;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.15);
}

.card-placeholder {
  height: 80px;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.15);
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
  padding: 16px;
  background: rgba(255, 255, 255, 0.2);
  border: 1px solid rgba(255, 255, 255, 0.3);
  border-radius: 12px;
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  transition: all 0.3s;
  width: 200px;
}

.feature-header {
  display: flex;
  align-items: center;
  gap: 10px;
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

  .floating-cards {
    width: 85%;
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

  .mode-tabs {
    background: #1e1e3a;
  }

  .mode-tab {
    color: #606370;

    &:hover {
      color: #a0a3b0;
    }

    &.active {
      background: #252545;
      color: #8b9cf7;
    }
  }

  .form-body {
    :deep(.t-form-item__label) {
      color: #c0c3d0;
    }

    :deep(.t-input) {
      border-color: #2a2a4a;
      background: #1a1a35;

      &:hover {
        border-color: #353560;
      }

      &:focus-within {
        border-color: #667eea;
        box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.15);
        background: #1e1e3a;
      }

      .t-input__inner {
        color: #e8eaf0;

        &::placeholder {
          color: #4a4a6a;
        }
      }
    }
  }

  .oidc-divider span {
    background: #141428;
  }

  .oidc-btn {
    border-color: #2a2a4a;
    background: #1a1a35;
    color: #a0a3b0;

    &:hover {
      background: #252545;
      border-color: #353560;
    }
  }

  .panel-footer {
    color: #3a3a5a;
  }

  .invite-banner {
    background: rgba(102, 126, 234, 0.1);
    border-color: rgba(102, 126, 234, 0.2);
  }

  .invite-banner__title {
    color: #e8eaf0;
  }

  .invite-banner__hint {
    color: #606370;
  }

  .invite-banner--error {
    background: rgba(245, 108, 108, 0.1);
    border-color: rgba(245, 108, 108, 0.2);
  }
}

@media (prefers-reduced-motion: reduce) {
  .bg-slide {
    transition: none !important;
  }
}
</style>