const assert = require('node:assert/strict');
const test = require('node:test');
const { checkIdempotency } = require('./check-idempotency.cjs');

const discovery = {
  operationId: 'llm_fetch_upstream_models',
  'x-idempotency': { mode: 'read_only', reason: 'Reads the provider model catalog without storing credentials or changing state.' },
};

test('reviewed discovery POST requires a valid read-only declaration', () => {
  assert.equal(checkIdempotency('post', '/v1/llm/upstream-models', discovery, []), null);
  assert.match(checkIdempotency('post', '/v1/llm/upstream-models', { operationId: discovery.operationId }, []), /idempotency missing/);
  for (const declaration of [null, true, [], {}, { mode: 'read_only' }, { mode: 'read_only', reason: '' }, { mode: 'read_only', reason: 'read', extra: true }]) {
    assert.match(checkIdempotency('post', '/v1/llm/upstream-models', { ...discovery, 'x-idempotency': declaration }, []), /exemption invalid/);
  }
});

test('read-only metadata cannot exempt a different operation, path or method', () => {
  assert.match(checkIdempotency('post', '/v1/llm/providers', discovery, []), /exemption invalid/);
  assert.match(checkIdempotency('post', '/v1/llm/upstream-models', { ...discovery, operationId: 'llm_create_provider' }, []), /exemption invalid/);
  for (const method of ['put', 'patch', 'delete']) {
    assert.match(checkIdempotency(method, '/v1/llm/upstream-models', discovery, []), /exemption invalid/);
  }
});

test('ordinary writes still require a mandatory replay header', () => {
  const mutation = { operationId: 'llm_create_provider' };
  for (const method of ['post', 'put', 'patch', 'delete']) {
    assert.match(checkIdempotency(method, '/v1/llm/providers', mutation, []), /idempotency missing/);
    assert.match(checkIdempotency(method, '/v1/llm/providers', mutation, [{ in: 'header', name: 'Idempotency-Key', required: false }]), /idempotency missing/);
    assert.equal(checkIdempotency(method, '/v1/llm/providers', mutation, [{ in: 'header', name: 'Idempotency-Key', required: true }]), null);
  }
});
