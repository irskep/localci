import { createApp } from 'vue'
import { createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import { definePreset } from '@primeuix/themes'
import Aura from '@primeuix/themes/aura'
import Breadcrumb from 'primevue/breadcrumb'
import Button from 'primevue/button'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import MeterGroup from 'primevue/metergroup'
import Menu from 'primevue/menu'
import Menubar from 'primevue/menubar'
import Message from 'primevue/message'
import Panel from 'primevue/panel'
import ProgressSpinner from 'primevue/progressspinner'
import Tag from 'primevue/tag'
import Tooltip from 'primevue/tooltip'

import App from './App.vue'
import router from './router'
import 'primeicons/primeicons.css'
import './styles/fonts.css'
import './styles/app.css'

const app = createApp(App)
const LocalciTheme = definePreset(Aura, {
  semantic: {
    primary: {
      50: '{purple.50}',
      100: '{purple.100}',
      200: '{purple.200}',
      300: '{purple.300}',
      400: '{purple.400}',
      500: '{purple.500}',
      600: '{purple.600}',
      700: '{purple.700}',
      800: '{purple.800}',
      900: '{purple.900}',
      950: '{purple.950}',
    },
  },
})

app.use(PrimeVue, {
  theme: {
    preset: LocalciTheme,
    options: {
      darkModeSelector: 'system',
    },
  },
})
app.use(createPinia())
app.use(router)
app.component('PBreadcrumb', Breadcrumb)
app.component('PButton', Button)
app.component('PColumn', Column)
app.component('PDataTable', DataTable)
app.component('PMeterGroup', MeterGroup)
app.component('PMenu', Menu)
app.component('PMenubar', Menubar)
app.component('PMessage', Message)
app.component('PPanel', Panel)
app.component('PProgressSpinner', ProgressSpinner)
app.component('PTag', Tag)
app.directive('tooltip', Tooltip)

app.mount('#app')
