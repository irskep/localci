const page = document.body?.dataset?.page ?? "unknown";

document.documentElement.dataset.localciUi = "ready";
document.body?.setAttribute("data-js", "ready");

console.info(`[localci ui] loaded ${page}`);
