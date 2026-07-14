import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const schemaUrl = new URL('../schemas/dm-turn.schema.json', import.meta.url);

function assertStrictObjects(schema, path = 'root') {
  if (!schema || typeof schema !== 'object') return;

  if (schema.type === 'object') {
    const propertyNames = Object.keys(schema.properties || {}).sort();
    const requiredNames = [...(schema.required || [])].sort();
    assert.deepEqual(requiredNames, propertyNames, `${path}.required must include every property`);
  }

  for (const [key, value] of Object.entries(schema)) {
    if (Array.isArray(value)) {
      value.forEach((entry, index) => assertStrictObjects(entry, `${path}.${key}[${index}]`));
    } else if (value && typeof value === 'object') {
      assertStrictObjects(value, `${path}.${key}`);
    }
  }
}

test('DM output schema makes every object property required for Structured Outputs', async () => {
  const schema = JSON.parse(await readFile(schemaUrl, 'utf8'));
  assertStrictObjects(schema);
});
