// Only explicitly reviewed read-only POST operations may omit a replay key.
// Keep the method, path and operation ID bound together so adding metadata to a
// mutation cannot silently disable its existing idempotency requirement.
const readOnlyPosts = new Map([
  ['llm_fetch_upstream_models', '/v1/llm/upstream-models'],
]);

function checkIdempotency(method, route, operation, parameters) {
  const id = operation.operationId;
  const declaration = operation['x-idempotency'];
  if (declaration !== undefined) {
    const valid = declaration !== null && typeof declaration === 'object' &&
      !Array.isArray(declaration) &&
      Object.keys(declaration).every(key => ['mode', 'reason'].includes(key)) &&
      declaration.mode === 'read_only' &&
      typeof declaration.reason === 'string' && declaration.reason.trim().length > 0 &&
      method === 'post' && readOnlyPosts.get(id) === route;
    return valid ? null : 'idempotency exemption invalid ' + id;
  }
  if (!['get', 'head', 'options'].includes(method) && !route.startsWith('/v1/auth/') &&
      !parameters.some(p => p.in === 'header' && p.name === 'Idempotency-Key' && p.required === true)) {
    return 'idempotency missing ' + id;
  }
  return null;
}

module.exports = { checkIdempotency };
