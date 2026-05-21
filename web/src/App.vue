<template>
  <div class="app-shell">
    <header class="topbar">
      <div class="topbar-inner">
        <RouterLink class="brand" to="/">
          <span class="brand-mark">lc</span>
          <span>localci</span>
        </RouterLink>
        <nav class="nav-actions" aria-label="Primary">
          <RouterLink to="/">
            <PButton label="Home" text size="small" icon="pi pi-home" />
          </RouterLink>
          <RouterLink to="/repo">
            <PButton label="Repo" text size="small" icon="pi pi-folder" />
          </RouterLink>
          <RouterLink to="/queue">
            <PButton label="Queue" text size="small" icon="pi pi-list-check" />
          </RouterLink>
        </nav>
      </div>
    </header>
    <RouterView />
  </div>
</template>
