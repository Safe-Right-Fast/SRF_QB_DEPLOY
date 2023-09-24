#!/usr/bin/env node

// Required npm packages
const clear = require('clear');
const Configstore = require('configstore');
const pkg = require('./package.json');
const CLI = require('clui');
const Spinner = CLI.Spinner;
const minimist = require('minimist');
const path = require('path');
const opn = require('opn');
const xmlparser = require('fast-xml-parser');
const editJsonFile = require('edit-json-file');
const stripBom = require('strip-bom');
const keytar = require('keytar');

// Custom scripts
const files = require('./lib/files');
const helpers = require('./lib/helpers');
const qb = require('./lib/qb');
const alert = require('./lib/alerts');
const qbcliTemplate = require('./lib/qbcliTemplate');
const userInput = require('./lib/userInput');
const userConfirmation = require('./lib/userInputConfirmation');
const modifyPrefixInput = require('./lib/userInputModifyPrefix');

// Initialize Configstore
const configurationFile = new Configstore(pkg.name);

// Load enums/commands
const ENUMS = require('./lib/enums');

/**
 * Runs the main logic for the CLI Script
 */
const run = async () => {
  // Configurations used by multiple CLI commands
  const qbCLIConfName = ENUMS.QB_CLI_FILE_NAME;
  const pathToQBCLIJSON = path.join(process.cwd(), qbCLIConfName);
  const qbCliJsonExists = files.fileFolderExists(pathToQBCLIJSON);
  let existingQbCliConfigs = null;

  if (qbCliJsonExists) {
    existingQbCliConfigs = files.readJSONFile(pathToQBCLIJSON);
  }

  const args = minimist(process.argv.slice(2));

  // If running the install command
  if (args._.includes(ENUMS.DEPLOYQB_INIT_CMD)) {
    // Clear the screen
    clear();
    let input = null;

    if (!qbCliJsonExists) {
      input = await userInput.getInput();
    } else {
      input = await userInput.getRevisedUserInput(existingQbCliConfigs);
    }

    const repositoryId = input.repositoryId;

    // Set user token and app token in secure storage
    const setUserToken = await setSecureToken(`${repositoryId}ut`, input.usertoken);
    const setAppToken = await setSecureToken(`${repositoryId}at`, input.apptoken);

    // Set development user and app tokens if required
    if (input.devAndProdQuickBaseApplications === 'yes') {
      const setDevUserToken = await setSecureToken(`${repositoryId}utd`, input.devUsertoken);
      const setDevAppToken = await setSecureToken(`${repositoryId}atd`, input.devApptoken);
    }

    if (!input.customPrefix) {
      input.customPrefix = 'D';
    }

    if (!input.customPrefixProduction) {
      input.customPrefixProduction = 'P';
    }

    if (!input.customPrefixFeature) {
      input.customPrefixFeature = 'F';
    }

    // Create qbcli template object
    const data = qbcliTemplate(input);

    // If qbcli.json already exists, grab the URL query string and filesconf
    if (qbCliJsonExists) {
      data.urlQueryString = existingQbCliConfigs.urlQueryString;
      data.filesConf = existingQbCliConfigs.filesConf;
    }

    // Save feature prefix outside project/repo/qbcli.json as this is specific to an individual coder
    saveFeaturePrefix(repositoryId, input.customPrefixFeature);

    // Create qbcli.json file
    try {
      files.saveJSONToFile(qbCLIConfName, data);
    } catch (error) {
      alert.error(error);
      return;
    }

    alert.success('A qbcli.json file has been created in the root of your project directory. Please update this file to include all files that you need to deploy to QB');
  }
  // If running the production, development, or feature deploy option
  else if (
    args._.includes(ENUMS.DEPLOY_DEV_CMD) ||
    args._.includes(ENUMS.DEPLOY_PROD_CMD) ||
    args._.includes(ENUMS.DEPLOY_FEAT_CMD)
  ) {
    // Set the necessary deployment type
    const deploymentType = getDeploymentType(args);

    // Ensure user is running the command from the root of their directory
    if (!qbCliJsonExists) {
      alert.error('This deployqb command can only be run from the root of your directory.');
      return;
    }

    // Get repo ID and files to push to prod
    const { repositoryId, filesConf } = existingQbCliConfigs;
    if (filesConf.length < 1) {
      alert.error('You must list files to deploy in your qbcli.json.');
      return;
    }

    // Get configs stored from qbcli install
    const configs = getConfiguration(repositoryId);
    if (!configs) {
      alert.error('Project may never have been initialized - please run deployqb init.');
      return;
    }

    // If this is a prod/dev deployment, double-check if the user wants to deploy
    if (deploymentType === 'prod' || deploymentType === 'dev') {
      const confirmation = await userConfirmation.getInput(deploymentType);
      if (confirmation.answer !== 'yes') {
        return;
      }
    }

    // Get prefix for files
    const prefix = helpers.prefixGenerator(
      {
        customPrefix: existingQbCliConfigs.devPrefix,
        customPrefixProduction: existingQbCliConfigs.prodPrefix,
        customPrefixFeature: configs.customPrefixFeature,
      },
      deploymentType,
      repositoryId
    );

    try {
      const arrayOfFileContents = await getAllFileContents(filesConf, prefix);
      if (!arrayOfFileContents || arrayOfFileContents.length < 1) {
        alert.error(
          'Please check your qbcli.json in the root of your project. Make sure you have mapped the correct path to all of the files you are trying to deploy. Also, check all filenames match what is in those directories, and that all files have content (this tool will not deploy blank files - add a comment in the file if you would like to deploy without code).'
        );
        return;
      }

      // Add the appropriate extension prefix to each file depending on whether it is dev/prod deployment
      let indexFileName = null;
      const formattedFiles = arrayOfFileContents.map(([fileName, fileContents, isIndexFile]) => {
        if (isIndexFile) {
          indexFileName = fileName;
        }
        return [`${prefix}${fileName}`, fileContents];
      });

      // Handle API calls to deploy files
      await handleAPICalls(deploymentType, formattedFiles, existingQbCliConfigs);
    } catch (err) {
      alert.error(
        'Please check your qbcli.json in the root of your project. Make sure you have mapped the correct path to all of the files you are trying to deploy. Also, check all filenames match what is in those directories and make sure those files have content (this tool will not deploy blank files - add a comment if you would like to deploy without code).'
      );
      return;
    }
  }
  // If running the launch command
  else if (
    args._.includes(ENUMS.LAUNCH_PROD_CMD) ||
    args._.includes(ENUMS.LAUNCH_FEAT_CMD) ||
    args._.includes(ENUMS.LAUNCH_DEV_CMD)
  ) {
    const { launchDbid, pageId, errorMessage } = getLaunchParameters(args, existingQbCliConfigs);

    if (!pageId) {
      alert.error(errorMessage);
      return;
    }

    // Get repo ID and files to push to prod
    const { repositoryId, urlQueryString } = existingQbCliConfigs;
    const configs = getConfiguration(repositoryId);
    if (!configs) {
      alert.error('Project may never have been initialized - please run deployqb init.');
      return;
    }

    // Add optional query string if present from qbcli.json
    const encodedQueryString = urlQueryString ? `&${encodeURI(urlQueryString)}` : '';

    // Launch the webpage
    opn(`https://${existingQbCliConfigs.realm}.quickbase.com/db/${launchDbid}?a=dbpage&pageID=${pageId}${encodedQueryString}`);
  }
  // If running the help command
  else if (args._.includes(ENUMS.DEPLOYQB_HELP)) {
    displayHelp();
  }
  // If running the edit prefix command
  else if (
    args._.includes(ENUMS.EDIT_DEV_PREFIX_CMD) ||
    args._.includes(ENUMS.EDIT_PROD_PREFIX_CMD) ||
    args._.includes(ENUMS.EDIT_FEAT_PREFIX_CMD)
  ) {
    editPrefix(args, qbCliJsonExists, existingQbCliConfigs, configurationFile);
  }
  // If running the generate links command
  else if (args._.includes(ENUMS.GENERATE_LINKS_CMD)) {
    generateLinks(qbCliJsonExists, existingQbCliConfigs, configurationFile);
  }
};

/**
 * Set the user token and app token securely using keytar.
 * @param {string} key - The key under which to store the token.
 * @param {string} token - The token to store.
 * @returns {Promise<void>}
 */
const setSecureToken = async (key, token) => {
  try {
    await keytar.setPassword(ENUMS.DEPLOYQB_NAME, key, token);
  } catch (error) {
    alert.error('Error setting token. If you are on Linux, you may need to install "libsecret".');
    alert.error(error);
  }
};

/**
 * Save the feature prefix outside project/repo/qbcli.json.
 * @param {string} repositoryId - The repository ID.
 * @param {string} customPrefixFeature - The feature prefix to save.
 */
const saveFeaturePrefix = (repositoryId, customPrefixFeature) => {
  configurationFile.set(repositoryId, {
    customPrefixFeature,
  });
};

/**
 * Get the deployment type from command arguments.
 * @param {object} args - The command arguments.
 * @returns {string} - The deployment type (prod, dev, or feat).
 */
const getDeploymentType = (args) => {
  if (args._.includes(ENUMS.DEPLOY_DEV_CMD)) {
    return 'dev';
  } else if (args._.includes(ENUMS.DEPLOY_PROD_CMD)) {
    return 'prod';
  } else if (args._.includes(ENUMS.DEPLOY_FEAT_CMD)) {
    return 'feat';
  }
};

/**
 * Get the configuration object from Configstore.
 * @param {string} repositoryId - The repository ID.
 * @returns {object|null} - The configuration object or null if not found.
 */
const getConfiguration = (repositoryId) => {
  return configurationFile.get(repositoryId);
};

/**
 * Get the launch parameters based on the command.
 * @param {object} args - The command arguments.
 * @param {object} existingQbCliConfigs - Existing QB CLI configurations.
 * @returns {object} - Launch parameters.
 */
const getLaunchParameters = (args, existingQbCliConfigs) => {
  let launchDbid = null;
  let pageId = null;
  let errorMessage = null;

  // Ensure user is running the command from the root of their directory
  if (!qbCliJsonExists) {
    errorMessage = 'This deployqb command can only be run from the root of your directory.';
  } else {
    // Get repo ID
    const { repositoryId, urlQueryString } = existingQbCliConfigs;

    // Set correct pageID for prod/dev/feat
    if (args._.includes(ENUMS.LAUNCH_PROD_CMD)) {
      launchDbid = existingQbCliConfigs.dbid;
      pageId = existingQbCliConfigs.launchProdPageId;
      errorMessage =
        'You must first deploy the production files to the Quick Base application before you can use this command. Try running "deployqb prod" first. If you have done that, then you need to set an "isIndexFile" in your qbcli.json to use this command (see npm docs).';
    } else if (args._.includes(ENUMS.LAUNCH_DEV_CMD)) {
      if (existingQbCliConfigs.devAndProdQuickBaseApplications === 'yes') {
        launchDbid = existingQbCliConfigs.devDbid;
      } else {
        launchDbid = existingQbCliConfigs.dbid;
      }
      pageId = existingQbCliConfigs.launchDevPageId;
      errorMessage =
        'You must first deploy the development files to the Quick Base application before you can use this command. Try running "deployqb dev" first. If you have done that, then you need to set an "isIndexFile" in your qbcli.json to use this command (see npm docs).';
    } else if (args._.includes(ENUMS.LAUNCH_FEAT_CMD)) {
      if (existingQbCliConfigs.devAndProdQuickBaseApplications === 'yes') {
        launchDbid = existingQbCliConfigs.devDbid;
      } else {
        launchDbid = existingQbCliConfigs.dbid;
      }
      const configs = configurationFile.get(repositoryId);
      pageId = configs.launchFeatPageId;
      errorMessage =
        'You must first deploy the feature files to the Quick Base application before you can use this command. Try running "deployqb feat" first. If you have done that, then you need to set an "isIndexFile" in your qbcli.json to use this command (see npm docs).';
    }
  }

  return { launchDbid, pageId, errorMessage };
};

/**
 * Display the help commands.
 */
const displayHelp = () => {
  alert.success('deployqb commands');
  console.log('init:        Initializes this project.');
  console.log('feat:        Deploys your files to the feature environment.');
  console.log('dev:         Deploys your files to the development environment.');
  console.log('prod:        Deploys your files to the production environment.');
  console.log('lfeat:       Open your feature environment in Quick Base with your default browser.');
  console.log('ldev:        Open your development environment in Quick Base with your default browser.');
  console.log('lprod:       Open your production environment in Quick Base with your default browser.');
  console.log('efeatprefix: Feature prefix is stored outside qbcli.json - this allows you to edit the Feature environment prefix.');
  console.log('genlinks:    Displays a list of possible links for each file in your project.\n');
};

/**
 * Edit the prefix based on the command.
 * @param {object} args - The command arguments.
 * @param {boolean} qbCliJsonExists - Whether the qbcli.json file exists.
 * @param {object} existingQbCliConfigs - Existing QB CLI configurations.
 * @param {object} configurationFile - The Configstore object.
 */
const editPrefix = async (args, qbCliJsonExists, existingQbCliConfigs, configurationFile) => {
  if (args._.includes(ENUMS.EDIT_DEV_PREFIX_CMD) || args._.includes(ENUMS.EDIT_PROD_PREFIX_CMD)) {
    alert.warning(
      'After running "deployqb init", you can update your dev and prod prefix in the qbcli.json file in the root of your project. Only efeatprefix is still supported, as this prefix is saved outside the qbcli.json file.'
    );
    return;
  }

  // Make sure user is running this from the root of their directory
  if (!qbCliJsonExists) {
    alert.error('This deployqb command can only be run from the root of your directory.');
    return;
  }

  // Get repo ID and configs
  const { repositoryId } = existingQbCliConfigs;
  const configs = getConfiguration(repositoryId);
  if (!configs) {
    alert.error('Project may never have been initialized - please run deployqb init.');
    return;
  }

  // Get the correct name for the prefix
  let prefixReference = null;
  if (args._.includes(ENUMS.EDIT_FEAT_PREFIX_CMD)) {
    prefixReference = 'customPrefixFeature';
  }

  // Set the feature prefix
  alert.warning('Your current developer prefix is: ' + configs[prefixReference]);
  const input = await modifyPrefixInput.getInput();
  configs[prefixReference] = input.newPrefix;
  configurationFile.set(repositoryId, configs);
  alert.success('Your development prefix has been updated successfully.');
};

/**
 * Generate and display links for files in the project.
 * @param {boolean} qbCliJsonExists - Whether the qbcli.json file exists.
 * @param {object} existingQbCliConfigs - Existing QB CLI configurations.
 * @param {object} configurationFile - The Configstore object.
 */
const generateLinks = (qbCliJsonExists, existingQbCliConfigs, configurationFile) => {
  // Make sure user is running this from the root of their directory
  if (!qbCliJsonExists) {
    alert.error('This deployqb command can only be run from the root of your directory.');
    return;
  }

  // Get repo ID, files, and configs
  const { repositoryId, filesConf, dbid } = existingQbCliConfigs;
  const configs = getConfiguration(repositoryId);
  if (!configs) {
    alert.error('Project may never have been initialized - please run deployqb init.');
    return;
  }

  if (filesConf && filesConf.length > 0) {
    alert.warning('\nPOSSIBLE DEPENDENCY LINKS BASED ON YOUR QBCLI.json CONFIGS:');
    filesConf.forEach((file) => {
      alert.soft('__________________________________________');
      const { filename } = file;
      console.log(filename + ':\n');
      console.log(`\t?a=dbpage&pagename=${filename}`);
      alert.soft('__________________________________________');
    });
  }
};
console.log("hello world")
run();
