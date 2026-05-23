import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import Aura from '@primeuix/themes/aura'
import Breadcrumb from 'primevue/breadcrumb'
import Button from 'primevue/button'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Message from 'primevue/message'
import Panel from 'primevue/panel'
import ProgressSpinner from 'primevue/progressspinner'
import Tag from 'primevue/tag'

import App from './App.vue'
import router from './router'
import 'primeicons/primeicons.css'
import './styles/app.css'

const app = createApp(App)

app.use(PrimeVue, {
  theme: {
    preset: Aura,
    options: {
      darkModeSelector: false,
    },
  },
})
app.use(createPinia())
app.use(router)
app.component('PBreadcrumb', Breadcrumb)
app.component('PButton', Button)
app.component('PColumn', Column)
app.component('PDataTable', DataTable)
app.component('PMessage', Message)
app.component('PPanel', Panel)
app.component('PProgressSpinner', ProgressSpinner)
app.component('PTag', Tag)

app.mount('#app')
