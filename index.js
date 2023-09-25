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
const { forEach } = require('lodash');
require('dotenv').config();

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

    const repositoryId = process.env.REPOSITORY_ID;


    if (!process.env.CUSTOM_PREFIX) {
      process.env.CUSTOM_PREFIX = 'D';
    }

    if (!process.env.CUSTOM_PREFIX_PROD) {
      process.env.CUSTOM_PREFIX_PROD = 'P';
    }

    if (!process.env.CUSTOM_PREFIX_FEATURE) {
      process.env.CUSTOM_PREFIX_FEATURE = 'F';
    }

    // Create qbcli template object
    const data = qbcliTemplate();


    // If qbcli.json already exists, grab the URL query string and filesconf
    if (qbCliJsonExists) {
      data.urlQueryString = existingQbCliConfigs.urlQueryString;
      data.filesConf = existingQbCliConfigs.filesConf;
    }

    // Save feature prefix outside project/repo/qbcli.json as this is specific to an individual coder
    saveFeaturePrefix(repositoryId, process.env.CUSTOM_PREFIX_FEATURE);

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
      for(i = 0; i < formattedFiles.length; i++) {
        await handleAPICalls(deploymentType, formattedFiles[i], existingQbCliConfigs);
      };

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
      'After running "deployqb init", you can update your prefixes by opening the qbcli.json file in the root of your project and manually updating the "customPrefix" and "customPrefixProduction" properties.'
    );
  } else if (args._.includes(ENUMS.EDIT_FEAT_PREFIX_CMD)) {
    if (qbCliJsonExists) {
      const repoId = existingQbCliConfigs.repositoryId;
      const configs = getConfiguration(repoId);
      if (configs) {
        const prefix = await modifyPrefixInput.getInput();
        configs.customPrefixFeature = prefix.customPrefixFeature;
        configurationFile.set(repoId, configs);
        alert.success('Feature prefix has been updated.');
      } else {
        alert.error('Project may never have been initialized - please run deployqb init.');
      }
    } else {
      alert.error('This deployqb command can only be run from the root of your directory.');
    }
  }
};

/**
 * Generate links based on the command.
 * @param {boolean} qbCliJsonExists - Whether the qbcli.json file exists.
 * @param {object} existingQbCliConfigs - Existing QB CLI configurations.
 * @param {object} configurationFile - The Configstore object.
 */
const generateLinks = async () => {
  console.log(        process.env.LAUNCH_DEV_PAGE_ID,
    process.env.REALM,
    process.env.CUSTOM_PREFIX,
    process.env.CUSTOM_PREFIX_PRODUCTION,
    process.env.DEV_AND_PROD_QUICKBASE_APPLICATIONS )
  const repoId = process.env.REPOSITORY_ID;
  const qbCliJsonExists = files.fileFolderExists(path.join(process.cwd(), ENUMS.QB_CLI_FILE_NAME));
  if (qbCliJsonExists) {
    const configs = getConfiguration(repoId);
    if (configs) {
      const devLink = generateLink(
        process.env.LAUNCH_DEV_PAGE_ID,
        process.env.REALM,
        process.env.CUSTOM_PREFIX,
        process.env.CUSTOM_PREFIX_PRODUCTION,
        process.env.DEV_AND_PROD_QUICKBASE_APPLICATIONS
      );
      const prodLink = generateLink(
        process.env.LAUNCH_PROD_PAGE_ID,
        process.env.QB_REALM,
        process.env.CUSTOM_PREFIX,
        process.env.CUSTOM_PREFIX_PRODUCTION
      );
      const featLink = generateLink(
        process.env.LAUNCH_FEAT_PAGE_ID,
        process.env.QB_REALM,
        process.env.CUSTOM_PREFIX,
        process.env.CUSTOM_PREFIX_PRODUCTION,
        false,
        process.env.CUSTOM_PREFIX_FEATURE
      );
      console.log('\nDevelopment Link:');
      console.log(devLink);
      console.log('\nProduction Link:');
      console.log(prodLink);
      console.log('\nFeature Link:');
      console.log(featLink);
    } else {
      alert.error('Project may never have been initialized - please run deployqb init.');
    }
  } else {
    alert.error('This deployqb command can only be run from the root of your directory.');
  }
};


/**
 * Generate a Quick Base application link.
 * @param {string} pageId - The page ID.
 * @param {string} realm - The Quick Base realm.
 * @param {string} customPrefix - The custom prefix.
 * @param {string} customPrefixProduction - The custom prefix for production.
 * @param {boolean} devAndProdQuickBaseApplications - Whether there are separate dev and prod Quick Base applications.
 * @param {string} customPrefixFeature - The custom prefix for feature environment.
 * @returns {string} - The Quick Base application link.
 */
const generateLink = (pageId, realm, customPrefix, customPrefixProduction, devAndProdQuickBaseApplications = false, customPrefixFeature) => {
  const prefix = devAndProdQuickBaseApplications ? customPrefix : customPrefixProduction;
  return `${realm}/db/main?a=dbpage&pageID=${pageId}&namespace=${prefix}${customPrefixFeature ? '-' + customPrefixFeature : ''}`;
};

/**
 * Get the contents of all the files to be deployed.
 * @param {array} filesConf - The list of files to deploy.
 * @param {string} prefix - The file prefix.
 * @returns {Promise<array>} - An array of file contents.
 */
const getAllFileContents = async (filesConf, prefix) => {
  const status = new Spinner('Loading files...');
  status.start();

  const arrayOfFileContents = [];
  for (let i = 0; i < filesConf.length; i++) {
    const fileConf = filesConf[i];
    const fileName = fileConf.filename;

    if (files.fileFolderExists(fileName)) {
      // Read the file contents
      const fileContents = await files.getFileContents(fileName);

      // Determine if it's an index file
      const isIndexFile = fileConf.isIndexFile === 'yes';

      arrayOfFileContents.push([fileName, fileContents, isIndexFile]);
    }
  }

  status.stop();

  return arrayOfFileContents;
};

/**
 * Handle the API calls to deploy the files.
 * @param {string} deploymentType - The deployment type (prod, dev, or feat).
 * @param {array} formattedFiles - An array of formatted files to deploy.
 * @param {object} existingQbCliConfigs - Existing QB CLI configurations.
 * @returns {Promise<void>} - A promise that resolves when the deployment is complete.
 */
const handleAPICalls = async (deploymentType, formattedFiles, existingQbCliConfigs) => {
  const {
    repositoryId,
    customPrefixProduction,
    customPrefixFeature,
    devAndProdQuickBaseApplications,
  } = existingQbCliConfigs;

  // Retrieve user token and app token from environment variables
  const usertoken = process.env.USERTOKEN;
  const apptoken = process.env.APP_TOKEN;


  const prefix = devAndProdQuickBaseApplications ? customPrefixProduction : customPrefixFeature;
  const status = new Spinner('Deploying files...');
  status.start();

  try {
    await qb.addUpdateDbPage(
      existingQbCliConfigs.dbid,
      existingQbCliConfigs.realm,
      usertoken,
      apptoken,
      formattedFiles,
    );

    status.stop();
    alert.success(`Files have been successfully deployed to the ${deploymentType} environment.`);
  } catch (error) {
    status.stop();
    alert.error(error.message);
  }
};

// Run the main script logic
run();
