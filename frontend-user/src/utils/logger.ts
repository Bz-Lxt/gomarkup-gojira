const isProd = import.meta.env.PROD

export const logger = {
  debug(...args: unknown[]) {
    if (!isProd) console.debug('[gojira]', ...args)
  },
  info(...args: unknown[]) {
    if (!isProd) console.info('[gojira]', ...args)
  },
  warn(...args: unknown[]) {
    console.warn('[gojira]', ...args)
  },
  error(...args: unknown[]) {
    console.error('[gojira]', ...args)
  },
}
