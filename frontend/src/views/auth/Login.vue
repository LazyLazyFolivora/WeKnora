<template>
  <div class="login-layout">

    <!-- ── Background ── -->
    <div class="bg">
      <!-- top-left: CPU logo + 药枢 brand bar -->
      <div class="bg-header">
        <div class="bg-header-left">
          <img :src="bgImg1" class="bg-logo-cpu" alt="中国药科大学" />
          <div class="bg-divider"></div>
          <img :src="bgImg2" class="bg-logo-drug" alt="药枢" />
          <span class="bg-brand-text">智能药学知识库</span>
        </div>
        <span class="bg-version">CPU·Brain v1.0</span>
      </div>

      <!-- bottom: campus skyline -->
      <div class="bg-skyline">
        <img :src="bgImg0" class="bg-skyline-img" alt="" aria-hidden="true" />
      </div>
    </div>

    <!-- ── Login card ── -->
    <div class="card-wrap">
      <!-- :class flips layout for register mode (right=blue, left=form) -->
      <div class="login-card" :class="{ 'is-register': isRegisterMode }">

        <!-- Blue panel: left on login, right on register -->
        <div class="card-blue">
          <template v-if="!isRegisterMode">
            <h2 class="welcome-title">欢迎使用药枢！</h2>
            <p class="panel-sub-text">还没有账户？</p>
            <button class="panel-btn" @click="switchToRegister">注 册</button>
          </template>
          <template v-else>
            <h2 class="welcome-title">已有账户？</h2>
            <p class="panel-sub-text">直接登录即可</p>
            <button class="panel-btn" @click="switchToLogin">登 录</button>
          </template>
        </div>

        <!-- Form panel: right on login, left on register -->
        <div class="card-form">

          <!-- Login form -->
          <template v-if="!isRegisterMode">
            <h2 class="form-title">登 录</h2>
            <t-form ref="formRef" :data="formData" :rules="formRules" @submit="handleLogin" layout="vertical">
              <t-form-item label="邮箱" name="email">
                <t-input v-model="formData.email" placeholder="输入邮箱" type="email" size="large" :disabled="loading" />
              </t-form-item>
              <t-form-item label="密码" name="password">
                <t-input v-model="formData.password" placeholder="输入密码" type="password" size="large" :disabled="loading" @keydown.enter="handleLogin" />
              </t-form-item>
              <div class="forgot-row">
                <a href="#" class="forgot-link">忘记密码？</a>
              </div>
              <t-button type="submit" theme="primary" size="large" block :loading="loading" class="submit-btn">
                {{ loading ? $t('auth.loggingIn') : '登 录' }}
              </t-button>
            </t-form>
            <template v-if="oidcEnabled">
              <div class="oidc-divider"><span>{{ $t('auth.orContinueWith') }}</span></div>
              <t-button theme="default" size="large" block :loading="oidcLoading" :disabled="loading" @click="handleOIDCLogin">
                {{ oidcLoading ? $t('auth.redirectingToOIDC') : oidcLoginText }}
              </t-button>
            </template>
          </template>

          <!-- Register form -->
          <template v-else>
            <h2 class="form-title">注 册</h2>
            <t-form ref="registerFormRef" :data="registerData" :rules="registerRules" @submit="handleRegister" layout="vertical">
              <t-form-item label="用户名" name="username">
                <t-input v-model="registerData.username" :placeholder="$t('auth.usernamePlaceholder')" size="large" :disabled="loading" />
              </t-form-item>
              <t-form-item label="邮箱" name="email">
                <t-input v-model="registerData.email" placeholder="输入邮箱" type="email" size="large" :disabled="loading" />
              </t-form-item>
              <t-form-item label="密码" name="password">
                <t-input v-model="registerData.password" placeholder="输入密码" type="password" size="large" :disabled="loading" />
              </t-form-item>
              <t-form-item label="确认密码" name="confirmPassword">
                <t-input v-model="registerData.confirmPassword" :placeholder="$t('auth.confirmPasswordPlaceholder')" type="password" size="large" :disabled="loading" @keydown.enter="handleRegister" />
              </t-form-item>
              <t-button type="submit" theme="primary" size="large" block :loading="loading" class="submit-btn">
                {{ loading ? $t('auth.registering') : $t('auth.register') }}
              </t-button>
            </t-form>
          </template>

        </div>
      </div>
    </div>



  </div>
</template>

<script setup lang="ts">
import { ref, reactive, nextTick, onMounted, onBeforeUnmount, computed } from 'vue'
import { useRouter } from 'vue-router'
import { MessagePlugin } from 'tdesign-vue-next'
import { login, register, getOIDCAuthorizationURL, getOIDCConfig, autoSetup } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useI18n } from 'vue-i18n'
import bgImg0 from '@/assets/img/login-bg-img0.png'
import bgImg1 from '@/assets/img/login-bg-img1.png'
import bgImg2 from '@/assets/img/login-bg-img2.png'

const router = useRouter()
const authStore = useAuthStore()
const { t, locale } = useI18n()

const formRef = ref()
const registerFormRef = ref()
const loading = ref(false)
const oidcLoading = ref(false)
const isRegisterMode = ref(false)
const showLangMenu = ref(false)
const oidcEnabled = ref(false)
const oidcProviderName = ref('')

const languageOptions = [
  { value: 'zh-CN', label: '简体中文', shortLabel: '中文', flag: '🇨🇳' },
  { value: 'en-US', label: 'English',  shortLabel: 'EN',   flag: '🇺🇸' },
  { value: 'ru-RU', label: 'Русский',  shortLabel: 'RU',   flag: '🇷🇺' },
  { value: 'ko-KR', label: '한국어',   shortLabel: '한국어', flag: '🇰🇷' },
]

const currentLanguage   = computed(() => locale.value)
const currentLangOption = computed(() => languageOptions.find(l => l.value === currentLanguage.value))
const oidcLoginText     = computed(() =>
  oidcProviderName.value
    ? t('auth.oidcLoginWithProvider', { provider: oidcProviderName.value })
    : t('auth.oidcLogin')
)

const formData     = reactive<Record<string, any>>({ email: '', password: '' })
const registerData = reactive<Record<string, any>>({ username: '', email: '', password: '', confirmPassword: '' })

const formRules = computed(() => ({
  email:    [{ required: true, message: t('auth.emailRequired'), type: 'error' }, { email: true, message: t('auth.emailInvalid'), type: 'error' }],
  password: [
    { required: true, message: t('auth.passwordRequired'), type: 'error' },
    { min: 8,  message: t('auth.passwordMinLength'), type: 'error' },
    { max: 32, message: t('auth.passwordMaxLength'), type: 'error' },
    { pattern: /[a-zA-Z]/, message: t('auth.passwordMustContainLetter'), type: 'error' },
    { pattern: /\d/,       message: t('auth.passwordMustContainNumber'), type: 'error' },
  ],
}))

const registerRules = computed(() => ({
  username: [
    { required: true, message: t('auth.usernameRequired'), type: 'error' },
    { min: 2, max: 20, message: t('auth.usernameMinLength'), type: 'error' },
    { pattern: /^[a-zA-Z0-9_\u4e00-\u9fa5]+$/, message: t('auth.usernameInvalid'), type: 'error' },
  ],
  email:           [{ required: true, message: t('auth.emailRequired'), type: 'error' }, { email: true, message: t('auth.emailInvalid'), type: 'error' }],
  password:        formRules.value.password,
  confirmPassword: [
    { required: true, message: t('auth.confirmPasswordRequired'), type: 'error' },
    { validator: (v: string) => v === registerData.password, message: t('auth.passwordMismatch'), type: 'error' },
  ],
}))

const switchToRegister = () => { isRegisterMode.value = true }
const switchToLogin    = () => { isRegisterMode.value = false }
const toggleLangMenu   = () => { showLangMenu.value = !showLangMenu.value }

const selectLang = (lang: string) => {
  locale.value = lang
  localStorage.setItem('locale', lang)
  showLangMenu.value = false
  MessagePlugin.success(t('language.languageSaved'))
}

const handleClickOutside = (e: MouseEvent) => {
  if (!(e.target as HTMLElement).closest('.lang-switch')) showLangMenu.value = false
}
onMounted(()      => document.addEventListener('click', handleClickOutside))
onBeforeUnmount(() => document.removeEventListener('click', handleClickOutside))

const persistLoginResponse = async (res: any) => {
  if (res.user && res.tenant && res.token) {
    authStore.setUser({
      id: res.user.id || '', username: res.user.username || '', email: res.user.email || '',
      avatar: res.user.avatar, tenant_id: String(res.tenant.id) || '',
      can_access_all_tenants: res.user.can_access_all_tenants || false,
      created_at: res.user.created_at || new Date().toISOString(),
      updated_at: res.user.updated_at || new Date().toISOString(),
    })
    authStore.setToken(res.token)
    if (res.refresh_token) authStore.setRefreshToken(res.refresh_token)
    authStore.setTenant({
      id: String(res.tenant.id) || '', name: res.tenant.name || '',
      api_key: res.tenant.api_key || '', owner_id: res.user.id || '',
      created_at: res.tenant.created_at || new Date().toISOString(),
      updated_at: res.tenant.updated_at || new Date().toISOString(),
    })
  }
  await nextTick()
  router.replace('/platform/knowledge-bases')
}

const loadOIDCConfig = async () => {
  try {
    const res = await getOIDCConfig()
    oidcEnabled.value     = !!res.success && !!res.enabled
    oidcProviderName.value = res.provider_display_name || ''
  } catch { oidcEnabled.value = false }
}

const handleOIDCLogin = async () => {
  try {
    oidcLoading.value = true
    const res = await getOIDCAuthorizationURL(`${window.location.origin}/api/v1/auth/oidc/callback`)
    if (!res.success || !res.authorization_url) { MessagePlugin.error(res.message || t('auth.oidcLoginFailed')); return }
    window.location.href = res.authorization_url
  } catch (e: any) { MessagePlugin.error(e.message || t('auth.oidcLoginFailed'))
  } finally { oidcLoading.value = false }
}

const handleLogin = async () => {
  try {
    if (await formRef.value?.validate() !== true) return
    loading.value = true
    const res = await login({ email: formData.email, password: formData.password })
    if (res.success) { MessagePlugin.success(t('auth.loginSuccess')); await persistLoginResponse(res) }
    else MessagePlugin.error(res.message || t('auth.loginError'))
  } catch (e: any) { MessagePlugin.error(e.message || t('auth.loginErrorRetry'))
  } finally { loading.value = false }
}

const handleRegister = async () => {
  try {
    if (await registerFormRef.value?.validate() !== true) return
    loading.value = true
    const res = await register({ username: registerData.username, email: registerData.email, password: registerData.password })
    if (res.success) {
      MessagePlugin.success(t('auth.registerSuccess'))
      isRegisterMode.value = false
      formData.email = registerData.email
      Object.keys(registerData).forEach(k => { registerData[k] = '' })
    } else MessagePlugin.error(res.message || t('auth.registerFailed'))
  } catch (e: any) { MessagePlugin.error(e.message || t('auth.registerError'))
  } finally { loading.value = false }
}

onMounted(async () => {
  if (authStore.isLoggedIn) { router.replace('/platform/knowledge-bases'); return }
  const KEY = 'weknora_auto_setup_failed'
  if (localStorage.getItem(KEY) !== 'true') {
    try {
      const res = await autoSetup()
      if (res.success) { authStore.setLiteMode(true); await persistLoginResponse(res); return }
      else localStorage.setItem(KEY, 'true')
    } catch { localStorage.setItem(KEY, 'true') }
  }
  loadOIDCConfig()
})
</script>

<style lang="less" scoped>
/* ─────────────────────────────────────────
   Root layout: full viewport, deep blue bg
───────────────────────────────────────── */
.login-layout {
  position: relative;
  width: 100%;
  min-height: 100vh;
  background: #063190;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

/* ─────────────────────────────────────────
   Background layer (absolute, fills parent)
───────────────────────────────────────── */
.bg {
  position: absolute;
  inset: 0;
  pointer-events: none;
  z-index: 0;
}

/* Top header bar with logos */
.bg-header {
  position: absolute;
  top: 0; left: 0; right: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: clamp(12px, 2.5vw, 24px) clamp(16px, 4vw, 48px);
}

.bg-header-left {
  display: flex;
  align-items: center;
  gap: clamp(8px, 1.5vw, 18px);
}

/* CPU university logo — scales with viewport */
.bg-logo-cpu {
  height: clamp(40px, 6vw, 80px);
  width: auto;
  object-fit: contain;
}

.bg-divider {
  width: 2px;
  height: clamp(32px, 5vw, 60px);
  background: rgba(3, 196, 251, 0.9);
  flex-shrink: 0;
}

/* Drug logo (pill icon) */
.bg-logo-drug {
  height: clamp(28px, 4vw, 56px);
  width: auto;
  object-fit: contain;
}

.bg-brand-text {
  color: rgba(255, 255, 255, 0.92);
  font-size: clamp(12px, 1.4vw, 20px);
  font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
  letter-spacing: 1px;
  white-space: nowrap;
}

.bg-version {
  color: rgba(255, 255, 255, 0.65);
  font-size: clamp(10px, 1vw, 14px);
  font-family: sans-serif;
  white-space: nowrap;
}

/* Campus skyline pinned to bottom, full width */
.bg-skyline {
  position: absolute;
  bottom: 0; left: 0; right: 0;
  line-height: 0;
}

.bg-skyline-img {
  width: 100%;
  height: auto;
  /* The skyline image is white-on-transparent; show only bottom ~40% of viewport */
  max-height: 38vh;
  object-fit: cover;
  object-position: center bottom;
  display: block;
}

/* ─────────────────────────────────────────
   Login card — centered over background
───────────────────────────────────────── */
.card-wrap {
  position: relative;
  z-index: 10;
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: clamp(80px, 12vh, 140px) clamp(16px, 4vw, 40px) clamp(60px, 10vh, 120px);
  box-sizing: border-box;
}

.login-card {
  display: flex;
  width: 100%;
  max-width: 820px;
  min-height: 360px;
  border-radius: 20px;
  overflow: hidden;
  box-shadow:
    0 0 0 4px rgba(3, 196, 251, 0.45),
    0 12px 48px rgba(0, 0, 0, 0.5);

  /* Register mode: blue panel moves to the right */
  &.is-register {
    flex-direction: row-reverse;

    .card-blue {
      border-radius: 0 20px 20px 0;
      /* blobs mirror to the right side */
      &::before { left: auto; right: -25%; }
      &::after  { left: auto; right: -32%; }
    }
    .card-form {
      border-radius: 20px 0 0 20px;
    }
  }
}

/* ── Blue panel (left on login, right on register) ── */
.card-blue {
  flex: 0 0 42%;
  background: linear-gradient(155deg, #6a83bc 0%, #395aa7 45%, #2a4490 100%);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 18px;
  padding: 40px 28px;
  box-sizing: border-box;
  position: relative;
  overflow: hidden;
  border-radius: 20px 0 0 20px;

  &::before {
    content: '';
    position: absolute;
    top: 5%; left: -25%;
    width: 90%; height: 90%;
    border-radius: 108px;
    background: rgba(106, 131, 188, 0.28);
  }
  &::after {
    content: '';
    position: absolute;
    top: 5%; left: -32%;
    width: 84%; height: 90%;
    border-radius: 108px;
    background: rgba(57, 90, 167, 0.22);
  }
}

.welcome-title {
  position: relative;
  z-index: 1;
  color: #fff;
  font-size: clamp(16px, 2vw, 22px);
  font-weight: 700;
  font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
  margin: 0;
  text-align: center;
  letter-spacing: 1px;
}

.panel-sub-text {
  position: relative;
  z-index: 1;
  color: rgba(255,255,255,0.78);
  font-size: 14px;
  font-family: "PingFang SC", sans-serif;
  margin: 0;
}

.panel-btn {
  position: relative;
  z-index: 1;
  padding: 9px 40px;
  border: 1.5px solid rgba(255,255,255,0.85);
  border-radius: 24px;
  background: transparent;
  color: #fff;
  font-size: 15px;
  font-weight: 600;
  font-family: "PingFang SC", sans-serif;
  letter-spacing: 3px;
  cursor: pointer;
  transition: background 0.2s;
  &:hover { background: rgba(255,255,255,0.14); }
}

/* ── Form panel (right on login, left on register) ── */
.card-form {
  flex: 1;
  background: #e6eaf5;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: clamp(24px, 4vw, 44px) clamp(20px, 4vw, 44px);
  box-sizing: border-box;
  overflow-y: auto;
  border-radius: 0 20px 20px 0;
}

.form-title {
  font-size: clamp(20px, 2.2vw, 26px);
  font-weight: 700;
  color: #1a1a2e;
  font-family: "PingFang SC", "Microsoft YaHei", sans-serif;
  letter-spacing: 6px;
  text-align: center;
  margin: 0 0 20px;
}

/* Override TDesign input to match ef.svg white inputs */
:deep(.t-form-item) { margin-bottom: 14px; }
:deep(.t-form-item__label) {
  font-size: 14px;
  font-weight: 500;
  color: #333;
  font-family: "PingFang SC", sans-serif;
}
:deep(.t-input) {
  background: #fff !important;
  border: 1px solid #c8cde0 !important;
  border-radius: 8px !important;
  &:hover, &:focus-within {
    border-color: #395aa7 !important;
    box-shadow: 0 0 0 2px rgba(57,90,167,0.1) !important;
  }
}

.forgot-row {
  text-align: right;
  margin: -4px 0 8px;
}
.forgot-link {
  font-size: 13px;
  color: #5a6a9a;
  text-decoration: none;
  font-family: "PingFang SC", sans-serif;
  &:hover { color: #395aa7; text-decoration: underline; }
}

/* Login / Register button — matches ef.svg #395AA7 */
.submit-btn {
  margin-top: 6px;
  height: 44px;
  border-radius: 8px !important;
  font-size: 16px !important;
  font-weight: 600 !important;
  letter-spacing: 4px;
  background: #395aa7 !important;
  border-color: #395aa7 !important;
  &:hover { background: #2a4490 !important; border-color: #2a4490 !important; }
}

.oidc-divider {
  position: relative;
  margin: 14px 0 8px;
  text-align: center;
  color: #8a94b8;
  font-size: 12px;
  span { position: relative; z-index: 1; padding: 0 10px; background: #e6eaf5; }
  &::before { content: ''; position: absolute; left: 0; right: 0; top: 50%; border-top: 1px solid #c8cde0; }
}



/* ─────────────────────────────────────────
   Language switch (fixed bottom-right)
───────────────────────────────────────── */
.lang-wrap {
  position: fixed;
  bottom: 20px;
  right: 24px;
  z-index: 100;
}
.lang-switch { position: relative; }
.lang-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 7px 13px;
  border-radius: 20px;
  background: rgba(255,255,255,0.16);
  border: 1px solid rgba(255,255,255,0.3);
  color: #fff;
  font-size: 13px;
  cursor: pointer;
  &:hover { background: rgba(255,255,255,0.26); }
}
.lang-dropdown {
  position: absolute;
  bottom: calc(100% + 6px);
  right: 0;
  min-width: 148px;
  background: #fff;
  border: 1px solid #e0e4f0;
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0,0,0,0.14);
  overflow: hidden;
  z-index: 1000;
}
.lang-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 13px;
  cursor: pointer;
  font-size: 13px;
  color: #333;
  .check { margin-left: auto; color: #395aa7; font-weight: 700; }
  &:hover { background: #f0f3ff; }
  &.active { background: #e8ecff; color: #395aa7; }
}

/* ─────────────────────────────────────────
   Responsive breakpoints
───────────────────────────────────────── */

/* Tablet: stack card vertically */
@media (max-width: 680px) {
  .login-card {
    flex-direction: column !important; /* override row-reverse too */
    max-width: 420px;
  }
  .card-blue {
    flex: none;
    padding: 28px 24px;
    border-radius: 20px 20px 0 0 !important;
  }
  .card-form {
    padding: 28px 24px;
    border-radius: 0 0 20px 20px !important;
  }
  .bg-skyline-img {
    max-height: 22vh;
  }
}

/* Mobile: tighter spacing */
@media (max-width: 420px) {
  .card-wrap {
    padding: 70px 12px 50px;
  }
  .bg-header { padding: 10px 14px; }
  .bg-brand-text { display: none; }
  .form-title { letter-spacing: 3px; }
}
</style>
