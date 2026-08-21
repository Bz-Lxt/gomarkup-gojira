/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}

declare module 'vuedraggable' {
  import type { DefineComponent } from 'vue'
  const draggable: DefineComponent
  export default draggable
}
