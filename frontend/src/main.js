import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import 'element-plus/theme-chalk/dark/css-vars.css'
import 'element-plus/es/components/message/style/css'
import 'element-plus/es/components/message-box/style/css'
import './styles/theme.css'
import './styles/main.css'

// 默认深色主题；浅色偏好提前应用避免闪烁
if (localStorage.getItem('theme') === 'light') {
  document.documentElement.classList.remove('dark')
}

createApp(App).use(router).mount('#app')
