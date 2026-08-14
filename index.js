#!/usr/bin/env node

require('dotenv').config();

const Configstore = require('configstore');
const path = require('path');
const opn = require('opn');
const pkg = require('./package.json');
const files = require('./lib/files');
const helpers = require('./lib/helpers');
const qb = require('./lib/qb');
const alert = require('./lib/alerts');
const qbcliTemplate = require('./lib/qbcliTemplate');
const userConfirmation = require('./lib/userInputConfirmation');
const modifyPrefixInput = require('./lib/userInputModifyPrefix');
const ENUMS = require('./lib/enums');

const configurationFile = new Configstore(pkg.name);

const configPath = () => path.join(process.cwd(), ENUMS.QB_CLI_FILE_NAME);
const loadProject = () => files.fileFolderExists(configPath()) ? files.readJSONFile(configPath()) : null;

const featureConfig = (repositoryId) => configurationFile.get(repositoryId);

const requireProject = () => {
  const project = loadProject();
  if (!project) alert.error('This deployqb command can only be run from the root of your directory.');
  return project;
};

const init = () => {
  const existing = loadProject();
  const project = qbcliTemplate();
  const missing = ['repositoryId', 'dbid', 'realm'].filter((key) => !project[key]);
  if (missing.length) {
    alert.error(`Missing required environment configuration: ${missing.join(', ')}`);
    return;
  }

  if (existing) {
    project.urlQueryString = existing.urlQueryString || '';
    project.filesConf = existing.filesConf || [];
    project.launchProdPageId = existing.launchProdPageId;
    project.launchDevPageId = existing.launchDevPageId;
  }

  configurationFile.set(project.repositoryId, {
    ...(featureConfig(project.repositoryId) || {}),
    customPrefixFeature: process.env.CUSTOM_PREFIX_FEATURE || 'F',
  });
  files.saveJSONToFile(ENUMS.QB_CLI_FILE_NAME, project);
  alert.success('A qbcli.json file has been created. Update filesConf with the files to deploy.');
};

const deploymentTypeFor = (command) => ({ dev: 'dev', prod: 'prod', feat: 'feat' })[command];

const deploy = async (deploymentType) => {
  const project = requireProject();
  if (!project) return;
  if (!Array.isArray(project.filesConf) || project.filesConf.length === 0) {
    alert.error('You must list files to deploy in your qbcli.json.');
    return;
  }

  const personal = featureConfig(project.repositoryId);
  if (!personal) {
    alert.error('Project may never have been initialized - please run deployqb init.');
    return;
  }

  if (deploymentType === 'prod' || deploymentType === 'dev') {
    const confirmation = await userConfirmation.getInput(deploymentType);
    if (confirmation.answer.toLowerCase() !== 'yes') return;
  }

  const prefix = helpers.prefixGenerator({
    customPrefix: project.devPrefix,
    customPrefixProduction: project.prodPrefix,
    customPrefixFeature: personal.customPrefixFeature,
  }, deploymentType, project.repositoryId);

  let contents;
  try {
    contents = helpers.getAllFileContents(project.filesConf, files.getFileContents, prefix);
  } catch (error) {
    alert.error(error.message);
    return;
  }
  if (!contents) {
    alert.error('Every configured deployment file must exist and contain content.');
    return;
  }

  const separateDevApp = project.devAndProdQuickBaseApplications === 'yes';
  const useDevCredentials = deploymentType !== 'prod' && separateDevApp;
  const dbid = useDevCredentials ? project.devDbid : project.dbid;
  const usertoken = useDevCredentials
    ? process.env.QB_DEV_USERTOKEN || process.env.QB_USERTOKEN
    : process.env.QB_USERTOKEN;
  const apptoken = useDevCredentials
    ? process.env.QB_DEV_APP_TOKEN || process.env.QB_APP_TOKEN
    : process.env.QB_APP_TOKEN;

  if (!dbid || !usertoken) {
    alert.error('The selected environment is missing a DBID or user token.');
    return;
  }

  try {
    for (const [filename, body, isIndexFile] of contents) {
      const pageID = await qb.addUpdateDbPage(dbid, project.realm, usertoken, apptoken, [`${prefix}${filename}`, body]);
      if (isIndexFile) saveLaunchPageID(project, personal, deploymentType, pageID);
    }
    alert.success(`Files have been successfully deployed to the ${deploymentType} environment.`);
  } catch (error) {
    alert.error(error.message);
  }
};

const saveLaunchPageID = (project, personal, deploymentType, pageID) => {
  if (deploymentType === 'feat') {
    configurationFile.set(project.repositoryId, { ...personal, launchFeatPageId: pageID });
    return;
  }
  project[deploymentType === 'prod' ? 'launchProdPageId' : 'launchDevPageId'] = pageID;
  files.saveJSONToFile(ENUMS.QB_CLI_FILE_NAME, project);
};

const launchParameters = (command, project, personal) => {
  const development = command !== ENUMS.LAUNCH_PROD_CMD;
  return {
    dbid: development && project.devAndProdQuickBaseApplications === 'yes' ? project.devDbid : project.dbid,
    pageID: command === ENUMS.LAUNCH_PROD_CMD
      ? project.launchProdPageId
      : command === ENUMS.LAUNCH_DEV_CMD ? project.launchDevPageId : personal && personal.launchFeatPageId,
  };
};

const launch = (command) => {
  const project = requireProject();
  if (!project) return;
  const params = launchParameters(command, project, featureConfig(project.repositoryId));
  if (!params.pageID) {
    alert.error('Deploy this environment with an isIndexFile entry before launching it.');
    return;
  }
  const query = project.urlQueryString ? `&${project.urlQueryString}` : '';
  opn(`${qb.normalizeRealm(project.realm)}/db/${params.dbid}?a=dbpage&pageID=${params.pageID}${query}`);
};

const editFeaturePrefix = async () => {
  const project = requireProject();
  if (!project) return;
  const personal = featureConfig(project.repositoryId);
  if (!personal) {
    alert.error('Project may never have been initialized - please run deployqb init.');
    return;
  }
  const { newPrefix } = await modifyPrefixInput.getInput();
  configurationFile.set(project.repositoryId, { ...personal, customPrefixFeature: newPrefix });
  alert.success('Feature prefix has been updated.');
};

const generateLinks = () => {
  const project = requireProject();
  if (!project) return;
  const personal = featureConfig(project.repositoryId) || {};
  for (const environment of ['dev', 'prod', 'feat']) {
    const prefix = helpers.prefixGenerator({
      customPrefix: project.devPrefix,
      customPrefixProduction: project.prodPrefix,
      customPrefixFeature: personal.customPrefixFeature,
    }, environment, project.repositoryId);
    console.log(`\n${environment}:`);
    for (const file of project.filesConf || []) console.log(`?a=dbpage&pagename=${prefix}${file.filename}`);
  }
};

const displayHelp = () => {
  console.log(`deployqb commands
init         Create qbcli.json from environment configuration
feat         Deploy to the feature environment
dev          Deploy to the development environment
prod         Deploy to the production environment
lfeat        Open the feature environment
ldev         Open the development environment
lprod        Open the production environment
efeatprefix  Edit the per-user feature prefix
genlinks     Display dbpage links for configured files
help         Display this help`);
};

const run = async (command = process.argv[2]) => {
  if (command === ENUMS.DEPLOYQB_INIT_CMD) return init();
  if (deploymentTypeFor(command)) return deploy(deploymentTypeFor(command));
  if ([ENUMS.LAUNCH_PROD_CMD, ENUMS.LAUNCH_DEV_CMD, ENUMS.LAUNCH_FEAT_CMD].includes(command)) return launch(command);
  if (command === ENUMS.EDIT_FEAT_PREFIX_CMD) return editFeaturePrefix();
  if ([ENUMS.EDIT_DEV_PREFIX_CMD, ENUMS.EDIT_PROD_PREFIX_CMD].includes(command)) {
    return alert.warning('Edit devPrefix or prodPrefix in qbcli.json.');
  }
  if (command === ENUMS.GENERATE_LINKS_CMD) return generateLinks();
  displayHelp();
};

if (require.main === module) run();

module.exports = { launchParameters, run };
