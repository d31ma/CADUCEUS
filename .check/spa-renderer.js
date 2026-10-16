window.__tc_rerender ??= async () => {
  window.dispatchEvent(new CustomEvent('tachyon:rerender'))
}
