const test = require('node:test');
const assert = require('node:assert/strict');
const helpers = require('../lib/helpers');

test('deployment files honor paths, dependencies, BOM, and boolean index flags', () => {
  const seen = [];
  const files = [
    { filename: 'site.css', path: 'build/css' },
    { filename: 'index.html', path: 'build', dependencies: [0], isIndexFile: true },
  ];
  const contents = helpers.getAllFileContents(files, (filename) => {
    seen.push(filename);
    return filename.endsWith('index.html') ? '\uFEFF?pagename=site.css' : 'body {}';
  }, 'D_repo_');

  assert.deepEqual(seen, ['build/css/site.css', 'build/index.html']);
  assert.deepEqual(contents, [
    ['site.css', 'body {}', false],
    ['index.html', '?pagename=D_repo_site.css', true],
  ]);
});

test('invalid dependency index is rejected', () => {
  assert.throws(
    () => helpers.getAllFileContents([{ filename: 'index.html', dependencies: [2] }], () => 'body', 'D_repo_'),
    /invalid dependency index 2/,
  );
});

test('prefixes use configured values and environment defaults', () => {
  assert.equal(helpers.prefixGenerator({ customPrefix: 'DEV' }, 'dev', 'repo'), 'DEV_repo_');
  assert.equal(helpers.prefixGenerator({}, 'prod', 'repo'), 'P_repo_');
  assert.equal(helpers.prefixGenerator({}, 'feat', 'repo'), 'F_repo_');
});
