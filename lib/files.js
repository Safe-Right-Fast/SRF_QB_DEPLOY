const fs = require('fs');

module.exports = {
  fileFolderExists: fs.existsSync,
  saveJSONToFile(filename, data) {
    fs.writeFileSync(filename, JSON.stringify(data, null, 2) + '\n');
  },
  readJSONFile(filename) {
    return JSON.parse(fs.readFileSync(filename, 'utf8'));
  },
  getFileContents(filename) {
    return fs.readFileSync(filename, 'utf8');
  },
};
