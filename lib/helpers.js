module.exports = {
  getAllFileContents(filesConf, getFileContents, prefix) {
    let missing = false;
    const contents = filesConf.map((file, index) => {
      let body;
      try {
        body = getFileContents(require('path').join(file.path || '', file.filename));
      } catch (error) {
        throw new Error(`Cannot read ${file.path || ''}${file.filename}: ${error.message}`);
      }
      if (!body.length) missing = true;
      body = body.replace(/^\uFEFF/, '');

      for (const dependencyIndex of file.dependencies || []) {
        const dependency = filesConf[dependencyIndex];
        if (!dependency) throw new Error(`${file.filename} has invalid dependency index ${dependencyIndex}`);
        body = body.replaceAll(`pagename=${dependency.filename}`, `pagename=${prefix}${dependency.filename}`);
      }

      return [file.filename, body, file.isIndexFile === true || file.isIndexFile === 'yes'];
    });
    return missing ? false : contents;
  },

  prefixGenerator(config, deploymentType, repositoryId) {
    const defaults = { dev: 'D', prod: 'P', feat: 'F' };
    const configured = {
      dev: config.customPrefix,
      prod: config.customPrefixProduction,
      feat: config.customPrefixFeature,
    }[deploymentType];
    if (!defaults[deploymentType]) throw new Error(`Unknown deployment type: ${deploymentType}`);
    return `${configured || defaults[deploymentType]}_${repositoryId}_`;
  },
};
