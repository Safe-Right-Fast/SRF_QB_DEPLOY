const test = require('node:test');
const assert = require('node:assert/strict');
const { buildRequestXML, normalizeRealm, parseResponse } = require('../lib/qb');

test('Quickbase XML escapes metadata and preserves CDATA terminators in page content', () => {
  const xml = buildRequestXML('a&b', 'before]]>after', 'u<t', 'a&b');
  assert.match(xml, /<pagename>a&amp;b<\/pagename>/);
  assert.match(xml, /<pagebody><!\[CDATA\[before\]\]\]\]><!\[CDATA\[>after\]\]><\/pagebody>/);
  assert.match(xml, /<usertoken>u&lt;t<\/usertoken>/);
  assert.match(xml, /<apptoken>a&amp;b<\/apptoken>/);
});

test('Quickbase realms accept a short realm, hostname, or URL', () => {
  assert.equal(normalizeRealm('acme'), 'https://acme.quickbase.com');
  assert.equal(normalizeRealm('acme.quickbase.com'), 'https://acme.quickbase.com');
  assert.equal(normalizeRealm('https://acme.quickbase.com/'), 'https://acme.quickbase.com');
});

test('Quickbase response returns page ID and reports API errors', () => {
  assert.equal(parseResponse('<qdbapi><errcode>0</errcode><errtext>No error</errtext><pageID>42</pageID></qdbapi>'), 42);
  assert.throws(() => parseResponse('<qdbapi><errcode>5</errcode><errtext>Denied</errtext></qdbapi>'), /Denied/);
});
