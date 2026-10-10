import routes from './routes.json' with { type: 'json' }

/**
 * The application's routes by controller name, so suites name a route the way
 * the server does without loading server code. Each entry has `name`, `path`
 * (the @RequestRoute path, with {placeholders}) and `verbs`.
 *
 * @type {Record<string, { name: string, path: string, verbs: string[] }>}
 */
export const Routes = Object.fromEntries(
  Object.entries(routes).map(([name, route]) => [name, { name, ...route }]),
)
