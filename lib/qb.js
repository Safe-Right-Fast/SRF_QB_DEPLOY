const axios = require('axios');

module.exports = {


    addUpdateDbPage: ( dbid, realm, usertoken, apptoken=null, formattedFiles )=>{
        const pagebody = formattedFiles[1]
        const pagename = formattedFiles[0]

        const url = `${realm}/db/${dbid}`;
        var apptokenString = '';
        if( apptoken ) {
            apptokenString = `<apptoken>${apptoken}</apptoken>`;
        }
        var data = `
            <qdbapi>
                <pagename>${pagename}</pagename>
                <pagetype>1</pagetype>
                <pagebody><![CDATA[${pagebody}]]></pagebody>
                <usertoken>${usertoken}</usertoken>
                ${apptokenString}
            </qdbapi>
        `;
        // Send a POST request
        return axios({
            method: 'post',
            url: url,
            headers: {
                "X_QUICKBASE_RETURN_HTTP_ERROR": "true",
                'QUICKBASE-ACTION': 'API_AddReplaceDBPage',
                'Content-Type': 'application/xml'
            },
            data
        });
    }


}
