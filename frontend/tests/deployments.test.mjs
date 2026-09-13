import { test } from 'node:test';
import assert from 'node:assert/strict';
import { emptyFilters, filterDeployments, formatQuantity } from '../src/deployments.ts';

const rows = [
  { id: 'a/chat', name: 'Chat-Prod', tenantId: 't1', subscriptionId: 's1', region: 'eastus', accountId: 'a', model: 'gpt-4.1' },
  { id: 'b/chat', name: 'Chat-Prod', tenantId: 't2', subscriptionId: 's2', region: 'japaneast', accountId: 'b', model: 'gpt-4.1-mini' },
  { id: 'a/embed', name: 'embedding', tenantId: 't1', subscriptionId: 's1', region: 'eastus', accountId: 'a', model: 'text-embedding-3-large' },
];
test('同名デプロイを保持し、6属性それぞれで絞り込む', () => {
  assert.equal(filterDeployments(rows, emptyFilters).length, 3);
  for (const [key, value, expected] of [['tenantId', 't2', 1], ['subscriptionId', 's2', 1], ['region', 'japaneast', 1], ['accountId', 'b', 1], ['model', 'gpt-4.1-mini', 1], ['name', ' cHAt ', 2]]) {
    assert.equal(filterDeployments(rows, { ...emptyFilters, [key]: value }).length, expected, key);
  }
});
test('条件をANDで組み合わせ、該当なしと解除を区別する', () => {
  assert.deepEqual(filterDeployments(rows, { ...emptyFilters, tenantId: 't1', name: 'chat' }).map(row => row.id), ['a/chat']);
  assert.equal(filterDeployments(rows, { ...emptyFilters, tenantId: 't1', accountId: 'b' }).length, 0);
  assert.equal(filterDeployments(rows, emptyFilters).length, 3);
});
test('欠損を0にせず、明示された0と設定値を表示する', () => {
  assert.equal(formatQuantity(null), '不明');
  assert.equal(formatQuantity(0), '0');
  assert.equal(formatQuantity(120000), '120,000');
});
