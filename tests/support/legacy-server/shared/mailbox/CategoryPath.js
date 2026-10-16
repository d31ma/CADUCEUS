export class CategoryPath {
  static #MAX_CATEGORY_PATH_LENGTH = 60

  static #CATEGORY_PATH = /^[a-z0-9](?:[a-z0-9._/-]{0,58}[a-z0-9])?$/

  /** Normalize a bounded provider folder or label path. @param {unknown} value */
  static normalizeCategoryPath(value) {
    if (typeof value !== 'string') return ''
    const path = value.trim().toLowerCase()
    if (!path || path.length > CategoryPath.#MAX_CATEGORY_PATH_LENGTH || !CategoryPath.#CATEGORY_PATH.test(path)) return ''
    if (path.includes('//') || path.split('/').some(part => part === '.' || part === '..')) return ''
    return path
  }

  /** Add hierarchy metadata without exposing caller-provided markup. @param {string} path @param {string} kind @param {string | undefined} [role] */
  static presentCategory(path, kind, role) {
    const parts = path.split('/')
    const leaf = parts.at(-1) || path
    return {
      path,
      kind,
      displayName: leaf.charAt(0).toUpperCase() + leaf.slice(1),
      ...(parts.length > 1 ? { parentPath: parts.slice(0, -1).join('/') } : {}),
      depth: parts.length - 1,
      ...(role ? { role } : {}),
    }
  }
}
