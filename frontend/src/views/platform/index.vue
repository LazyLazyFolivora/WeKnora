<template>
    <div class="main" ref="dropzone">
        <!-- 渐变背景层 -->
        <div class="app-bg" aria-hidden="true">
        </div>

        <Menu></Menu>
        <div class="main-content">
            <RouterView v-if="isRouterAlive" />
        </div>
        <!-- door.png：横跨整个页面底部 -->
        <img :src="appBgDoor" class="app-bg-door" aria-hidden="true" alt="" />
        <div class="upload-mask" v-show="ismask">
            <input type="file" style="display: none" ref="uploadInput" accept=".pdf,.docx,.doc,.pptx,.ppt,.txt,.md,.jpg,.jpeg,.png,.csv,.xls,.xlsx" />
            <UploadMask></UploadMask>
        </div>
        <!-- 全局设置模态框，供所有 platform 子路由使用 -->
        <Settings />
    </div>
</template>
<script setup lang="ts">
import Menu from '@/components/menu.vue'
import { ref, onMounted, onUnmounted, nextTick, provide } from 'vue';
import { useRoute } from 'vue-router'
import useKnowledgeBase from '@/hooks/useKnowledgeBase'
import UploadMask from '@/components/upload-mask.vue'
import Settings from '@/views/settings/Settings.vue'
import { getKnowledgeBaseById } from '@/api/knowledge-base/index'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import appBgDoor from '@/assets/img/door.png'

let { requestMethod } = useKnowledgeBase()
const route = useRoute();
let ismask = ref(false)
let uploadInput = ref();
const { t } = useI18n();

const isRouterAlive = ref(true)
const reloadApp = () => {
    isRouterAlive.value = false
    nextTick(() => {
        isRouterAlive.value = true
    })
}
provide('app:reload', reloadApp)

const handleGlobalKeyDown = (e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'r') {
        e.preventDefault()
        reloadApp()
    }
}

// 用于跟踪拖拽进入/离开的计数器，解决子元素触发 dragleave 的问题
let dragCounter = 0;

// 获取当前知识库ID
const getCurrentKbId = (): string | null => {
    return (route.params as any)?.kbId as string || null
}

// 检查知识库初始化状态
const checkKnowledgeBaseInitialization = async (): Promise<boolean> => {
    const currentKbId = getCurrentKbId();
    
    if (!currentKbId) {
        MessagePlugin.error(t('knowledgeBase.missingId'));
        return false;
    }
    
    try {
        const kbResponse = await getKnowledgeBaseById(currentKbId);
        const kb = kbResponse.data;
        
        if (!kb.embedding_model_id || !kb.summary_model_id) {
            MessagePlugin.warning(t('knowledgeBase.notInitialized'));
            return false;
        }
        return true;
    } catch (error) {
        MessagePlugin.error(t('knowledgeBase.getInfoFailed'));
        return false;
    }
}


// 全局拖拽事件处理
const handleGlobalDragEnter = (event: DragEvent) => {
    event.preventDefault();
    dragCounter++;
    if (event.dataTransfer) {
        event.dataTransfer.effectAllowed = 'all';
    }
    ismask.value = true;
}

const handleGlobalDragOver = (event: DragEvent) => {
    event.preventDefault();
    if (event.dataTransfer) {
        event.dataTransfer.dropEffect = 'copy';
    }
}

const handleGlobalDragLeave = (event: DragEvent) => {
    event.preventDefault();
    dragCounter--;
    if (dragCounter === 0) {
        ismask.value = false;
    }
}

const handleGlobalDrop = async (event: DragEvent) => {
    event.preventDefault();
    dragCounter = 0;
    ismask.value = false;
    
    const DataTransferFiles = event.dataTransfer?.files ? Array.from(event.dataTransfer.files) : [];
    const DataTransferItemList = event.dataTransfer?.items ? Array.from(event.dataTransfer.items) : [];
    
    const isInitialized = await checkKnowledgeBaseInitialization();
    if (!isInitialized) {
        return;
    }
    
    if (DataTransferFiles.length > 0) {
        DataTransferFiles.forEach(file => requestMethod(file, uploadInput));
    } else if (DataTransferItemList.length > 0) {
        DataTransferItemList.forEach(dataTransferItem => {
            const fileEntry = dataTransferItem.webkitGetAsEntry() as FileSystemFileEntry | null;
            if (fileEntry) {
                fileEntry.file((file: File) => requestMethod(file, uploadInput));
            }
        });
    } else {
        MessagePlugin.warning(t('knowledgeBase.dragFileNotText'));
    }
}

// 组件挂载时添加全局事件监听器
onMounted(() => {
    document.addEventListener('dragenter', handleGlobalDragEnter, true);
    document.addEventListener('dragover', handleGlobalDragOver, true);
    document.addEventListener('dragleave', handleGlobalDragLeave, true);
    document.addEventListener('drop', handleGlobalDrop, true);
    window.addEventListener('keydown', handleGlobalKeyDown);
    // @ts-ignore
    if (window.runtime?.EventsOn) {
        // @ts-ignore
        window.runtime.EventsOn('app:reload', () => {
            reloadApp()
        })
    }
});

// 组件卸载时移除全局事件监听器
onUnmounted(() => {
    document.removeEventListener('dragenter', handleGlobalDragEnter, true);
    document.removeEventListener('dragover', handleGlobalDragOver, true);
    document.removeEventListener('dragleave', handleGlobalDragLeave, true);
    document.removeEventListener('drop', handleGlobalDrop, true);
    window.removeEventListener('keydown', handleGlobalKeyDown);
    // @ts-ignore
    if (window.runtime?.EventsOff) {
        // @ts-ignore
        window.runtime.EventsOff('app:reload')
    }
    dragCounter = 0;
});
</script>
<style lang="less">
.main {
    display: flex;
    width: 100%;
    height: 100%;
    min-width: 600px;
    position: relative;
    background: #063190;
    overflow: hidden;
}

/* 渐变背景层 */
.app-bg {
    position: absolute;
    inset: 0;
    pointer-events: none;
    z-index: 0;
    background:
        radial-gradient(circle 1158px at 66% 93%, #20469B 0%, transparent 100%),
        radial-gradient(circle 664px at 41% 100%, #395AA7 0%, transparent 100%),
        radial-gradient(circle 480px at 31% 107%, #526FB1 0%, transparent 100%),
        #063190;
}

/* 白色内容面板 */
.main-content {
    position: relative;
    z-index: auto;
    flex: 1;
    min-width: 0;
    margin: 16px 16px 0 0;
    background: #E6EAF5;
    border-radius: 16px 16px 0 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
}

/* door.png：内容面板内底部，z-index:0 在 RouterView 下面 */
.app-bg-door {
    position: absolute;
    bottom: 0;
    left: 0;
    width: 100%;
    height: auto;
    display: block;
    pointer-events: none;
    z-index: 3;
    opacity: 0.5;
    filter: hue-rotate(-71deg) saturate(0.4) brightness(0.7) sepia(0.3);
}

/* RouterView 在 door 上面 */
.main-content > *:not(.app-bg-door) {
    flex: 1;
    min-height: 0;
    overflow: auto;
    position: relative;
    z-index: 1;
}

.upload-mask {
    background-color: rgba(255, 255, 255, 0.8);
    position: fixed;
    width: 100%;
    height: 100%;
    z-index: 999;
    display: flex;
    justify-content: center;
    align-items: center;
}

img {
    -webkit-user-drag: none;
    -khtml-user-drag: none;
    -moz-user-drag: none;
    -o-user-drag: none;
    user-drag: none;
}
</style>