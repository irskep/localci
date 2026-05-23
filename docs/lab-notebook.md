# Lab Notebook

## 2026-05-22: Run table vertical padding

Problem: run table body cells still appear to have no added vertical padding even after adding a `.run-table` selector.

Observations:
- DevTools shows the relevant element is a `td` with `data-pc-section="bodycell"`.
- Previous selector changed from `.run-table.p-datatable .p-datatable-tbody > tr > td` to `.run-table [data-pc-section='bodycell']`, but user reports no visible change.
- Live DOM on `/` confirms `.run-table` is present and `.run-table [data-pc-section='bodycell']` matches.
- Computed padding is still `6px` top/bottom.
- Matched rules show PrimeVue applies `.p-datatable.p-datatable-sm .p-datatable-tbody > tr > td { padding: var(--p-datatable-body-cell-sm-padding); }`, which beats the lower-specificity local rule.

Hypotheses:
- PrimeVue's generated CSS wins by specificity.
- The served bundle is stale or the browser session is cached.

Next checks:
- Patch the selector to beat PrimeVue's small-table body-cell selector without `!important`.
- Rebuild/restart and verify computed padding changes from `6px`.

Result:
- Patched selector to `.run-table.p-datatable.p-datatable-sm .p-datatable-tbody > tr > [data-pc-section='bodycell']`.
- Rebuilt and restarted daemon.
- Live browser verification on `/` shows computed `paddingTop: 20px` and `paddingBottom: 20px` for `.run-table [data-pc-section=bodycell]`.
