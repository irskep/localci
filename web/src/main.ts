import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import Aura from '@primeuix/themes/aura'
import Button from 'primevue/button'
import Card from 'primevue/card'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Message from 'primevue/message'
import ProgressSpinner from 'primevue/progressspinner'
import Tag from 'primevue/tag'
import Toolbar from 'primevue/toolbar'

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
app.component('PButton', Button)
app.component('PCard', Card)
app.component('PColumn', Column)
app.component('PDataTable', DataTable)
app.component('PMessage', Message)
app.component('PProgressSpinner', ProgressSpinner)
app.component('PTag', Tag)
app.component('PToolbar', Toolbar)

app.mount('#app')
