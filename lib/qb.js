const axios = require('axios');
const { XMLParser } = require('fast-xml-parser');

const escapeXML = (value) => String(value ?? '')
  .replaceAll('&', '&amp;')
  .replaceAll('<', '&lt;')
  .replaceAll('>', '&gt;')
  .replaceAll('"', '&quot;')
  .replaceAll("'", '&apos;');

const normalizeRealm = (realm) => {
  const value = String(realm || '').replace(/\/$/, '');
  if (/^https?:\/\//i.test(value)) return value;
  return `https://${value.includes('.') ? value : `${value}.quickbase.com`}`;
};

const buildRequestXML = (pagename, pagebody, usertoken, apptoken) => `
<qdbapi>
  <pagename>${escapeXML(pagename)}</pagename>
  <pagetype>1</pagetype>
  <pagebody><![CDATA[${String(pagebody).replaceAll(']]>', ']]]]><![CDATA[>')}]]></pagebody>
  <usertoken>${escapeXML(usertoken)}</usertoken>
  ${apptoken ? `<apptoken>${escapeXML(apptoken)}</apptoken>` : ''}
</qdbapi>`;

const parseResponse = (data) => {
  const parsed = typeof data === 'string' ? new XMLParser().parse(data) : data;
  const result = parsed.qdbapi || parsed;
  if (String(result.errcode) !== '0') throw new Error(result.errtext || `Quickbase error ${result.errcode}`);
  return result.pageID;
};

const addUpdateDbPage = async (dbid, realm, usertoken, apptoken, [pagename, pagebody]) => {
  const response = await axios({
    method: 'post',
    url: `${normalizeRealm(realm)}/db/${dbid}`,
    headers: {
      X_QUICKBASE_RETURN_HTTP_ERROR: 'true',
      'QUICKBASE-ACTION': 'API_AddReplaceDBPage',
      'Content-Type': 'application/xml',
    },
    data: buildRequestXML(pagename, pagebody, usertoken, apptoken),
  });
  return parseResponse(response.data);
};

module.exports = { addUpdateDbPage, buildRequestXML, normalizeRealm, parseResponse };
