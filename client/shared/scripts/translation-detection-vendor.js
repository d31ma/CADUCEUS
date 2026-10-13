// This source shim keeps TypeScript and Bun tests on the exact locked package.
// Tachyon emits the full detector and its language models into the browser
// bundle so detection never depends on a hosted service.
export { francAll } from 'franc'
