const test = require('node:test');
const assert = require('node:assert/strict');
const { launchParameters } = require('../index');
const ENUMS = require('../lib/enums');

const project = {
  dbid: 'prod-db',
  devDbid: 'dev-db',
  devAndProdQuickBaseApplications: 'yes',
  launchProdPageId: 10,
  launchDevPageId: 20,
};

test('launch parameters select the correct database and page', () => {
  assert.deepEqual(launchParameters(ENUMS.LAUNCH_PROD_CMD, project, {}), { dbid: 'prod-db', pageID: 10 });
  assert.deepEqual(launchParameters(ENUMS.LAUNCH_DEV_CMD, project, {}), { dbid: 'dev-db', pageID: 20 });
  assert.deepEqual(launchParameters(ENUMS.LAUNCH_FEAT_CMD, project, { launchFeatPageId: 30 }), { dbid: 'dev-db', pageID: 30 });
});
