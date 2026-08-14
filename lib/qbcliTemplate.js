module.exports = () => ({
  urlQueryString: '',
  repositoryId: process.env.REPOSITORY_ID,
  prodPrefix: process.env.CUSTOM_PREFIX_PRODUCTION || 'P',
  devPrefix: process.env.CUSTOM_PREFIX || 'D',
  dbid: process.env.QB_DBID,
  devDbid: process.env.QB_DEV_DBID || '',
  devAndProdQuickBaseApplications: process.env.DEV_AND_PROD_QUICKBASE_APPLICATIONS || 'no',
  realm: process.env.QB_REALM,
  filesConf: [],
});
