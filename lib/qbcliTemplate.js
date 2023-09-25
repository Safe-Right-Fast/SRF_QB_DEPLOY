module.exports = function() {
    const repositoryId = process.env.REPOSITORY_ID;
    const customPrefixProduction = process.env.CUSTOM_PREFIX_PRODUCTION;
    const customPrefix = process.env.CUSTOM_PREFIX;
    const dbid = process.env.DBID;
    const devAndProdQuickBaseApplications = process.env.DEV_AND_PROD_QUICKBASE_APPLICATIONS;
    const devDbid = devAndProdQuickBaseApplications === 'yes' ? process.env.DEV_DBID : '';
    const realm = process.env.REALM;

    return {
      urlQueryString: '',
      repositoryId,
      prodPrefix: customPrefixProduction,
      devPrefix: customPrefix,
      dbid,
      devDbid,
      devAndProdQuickBaseApplications,
      realm,
      filesConf: [
        {
          filename: 'exampleFileName.js',
          path: './example/',
        },
        {
          filename: 'example.html',
          path: './examplefolder/subfolder/',
          dependencies: [1, 3],
          isIndexFile: false,
        },
      ],
    };
  };
