export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (url.pathname.startsWith('/api/')) {
      const target = new URL(url.pathname + url.search, 'https://llpoa-api.codevex.app');
      return fetch(target, {
        method: request.method,
        headers: request.headers,
        body: request.body,
      });
    }
    return env.ASSETS.fetch(request);
  }
};
