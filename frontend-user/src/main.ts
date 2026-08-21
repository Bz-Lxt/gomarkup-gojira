import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import { ApiError } from './api/types'
import { setApiErrorHandler, setUnauthorizedHandler } from './api/http'
import { useAuthStore } from './stores/auth'
import { useToastStore } from './stores/toast'
import './styles/tailwind.css'
import './styles/tokens.css'

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(router)

const toast = useToastStore(pinia)
const auth = useAuthStore(pinia)

setUnauthorizedHandler(() => {
  auth.logout()
  if (router.currentRoute.value.path !== '/login') {
    void router.push({ path: '/login', query: { redirect: router.currentRoute.value.fullPath } })
  }
})

setApiErrorHandler((err: ApiError) => {
  const rid = err.requestId ? ` · ${err.requestId}` : ''
  toast.error(`${err.message}${rid}`)
})

void auth.hydrate()

app.mount('#app')
