<template>
  <ol class="setup-empty">
    <li>
      <div class="setup-step-title">Install LocalCI with Mise</div>
      <pre><code>[tools]
"github:irskep/localci" = "latest"</code></pre>
      <pre><code>mise install</code></pre>
    </li>

    <li>
      <div class="setup-step-title">Define checks</div>
      <div class="setup-command-note">
        LocalCI discovers Mise tasks whose names start with <code>localci:</code>.
        <a href="https://steveasleep.com/localci/defining-tasks/" target="_blank" rel="noreferrer"
          >Defining tasks</a
        >
        covers LocalCI examples. Mise also documents
        <a href="https://mise.en.dev/tasks/toml-tasks.html" target="_blank" rel="noreferrer"
          >TOML tasks</a
        >
        and
        <a href="https://mise.en.dev/tasks/file-tasks.html" target="_blank" rel="noreferrer"
          >file tasks</a
        >.
      </div>
      <pre><code>mkdir -p mise-tasks/localci</code></pre>
      <div class="code-block">
        <div class="code-title">mise-tasks/localci/test</div>
        <pre><code>go test ./...</code></pre>
      </div>
      <pre><code>chmod +x mise-tasks/localci/test</code></pre>
    </li>

    <li>
      <div class="setup-step-title">Start the LocalCI daemon</div>
      <pre><code>localci start</code></pre>
    </li>

    <li>
      <div class="setup-step-title">Install the hook in this repo</div>
      <pre><code>localci install-hooks</code></pre>
    </li>

    <li>
      <div class="setup-step-title">Check results after a commit</div>
      <div class="setup-command-pair">
        <div>
          <pre><code># Wait in the terminal until the latest run finishes.
localci wait</code></pre>
        </div>
        <div>
          <pre><code># Open the local web UI for run details and artifacts.
localci web</code></pre>
        </div>
      </div>
    </li>
  </ol>
</template>

<style scoped>
.setup-empty {
  display: grid;
  gap: var(--app-space-5);
  width: min(920px, 100%);
  min-width: 0;
  margin: 0;
  padding-left: var(--app-space-5);
}

.setup-empty li {
  padding-left: var(--app-space-2);
}

.setup-step-title {
  margin-bottom: var(--app-space-3);
  font-weight: 600;
}

pre {
  margin: var(--app-space-3) 0 var(--app-space-5);
  padding: var(--app-space-4);
  border: 1px solid var(--p-content-border-color);
  border-radius: var(--p-content-border-radius);
  background: var(--p-content-background);
  overflow: auto;
}

.code-block {
  display: grid;
  margin-top: var(--app-space-3);
  margin-bottom: var(--app-space-5);
}

.code-block pre {
  margin-top: 0;
  margin-bottom: 0;
  border-top-left-radius: 0;
}

.code-title {
  justify-self: start;
  padding: var(--app-space-2) var(--app-space-3);
  border: 1px solid var(--p-content-border-color);
  border-bottom: 0;
  border-radius: var(--p-content-border-radius) var(--p-content-border-radius) 0 0;
  background: var(--p-content-background);
  color: var(--p-text-muted-color);
  font-family: var(--app-font-family-mono);
  font-size: var(--app-log-font-size);
}

.setup-command-pair {
  display: grid;
  gap: var(--app-space-3);
}

.setup-command-note {
  color: var(--p-text-muted-color);
}

code {
  font-size: var(--app-log-font-size);
}
</style>
