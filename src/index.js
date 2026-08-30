import { Container } from "@cloudflare/containers";

export class LLPOA extends Container {
  defaultPort = 8080;
  sleepAfter = "10m";

  constructor(ctx, env, options) {
    super(ctx, env, options);
    this.envVars = {
      OPENROUTER_API_KEY: env.OPENROUTER_API_KEY,
      SENTRY_DSN: env.SENTRY_DSN,
      SENTRY_OTLP_HOST: env.SENTRY_OTLP_HOST,
      SENTRY_ORG_ID: env.SENTRY_ORG_ID,
      SENTRY_PUBLIC_KEY: env.SENTRY_PUBLIC_KEY,
      LEGISCAN_API_KEY: env.LEGISCAN_API_KEY,
      LEGISCAN_STATE: env.LEGISCAN_STATE,
      LEGISCAN_SESSION_ID: env.LEGISCAN_SESSION_ID,
    };
  }
}

export default {
  async fetch(request, env) {
    const container = env.LLPOA.getByName("main");
    return container.fetch(request);
  },
};
