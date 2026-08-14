package quickbase

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	HTTP *http.Client
}

type cdata struct {
	Value string `xml:",cdata"`
}

type addRequest struct {
	XMLName   xml.Name `xml:"qdbapi"`
	PageName  string   `xml:"pagename"`
	PageType  int      `xml:"pagetype"`
	PageBody  cdata    `xml:"pagebody"`
	UserToken string   `xml:"usertoken"`
	AppToken  string   `xml:"apptoken,omitempty"`
}

type addResponse struct {
	ErrorCode int    `xml:"errcode"`
	ErrorText string `xml:"errtext"`
	PageID    int    `xml:"pageID"`
}

func NormalizeRealm(realm string) string {
	realm = strings.TrimSuffix(realm, "/")
	if strings.HasPrefix(realm, "http://") || strings.HasPrefix(realm, "https://") {
		return realm
	}
	if !strings.Contains(realm, ".") {
		realm += ".quickbase.com"
	}
	return "https://" + realm
}

func (client Client) AddReplaceDBPage(dbid, realm, userToken, appToken, pageName string, pageBody []byte) (int, error) {
	payload, err := xml.Marshal(addRequest{
		PageName: pageName, PageType: 1, PageBody: cdata{string(pageBody)}, UserToken: userToken, AppToken: appToken,
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, NormalizeRealm(realm)+"/db/"+dbid, strings.NewReader(string(payload)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("X_QUICKBASE_RETURN_HTTP_ERROR", "true")
	req.Header.Set("QUICKBASE-ACTION", "API_AddReplaceDBPage")
	req.Header.Set("Content-Type", "application/xml")

	httpClient := client.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	response, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return 0, fmt.Errorf("Quickbase returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	var result addResponse
	if err := xml.NewDecoder(response.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode Quickbase response: %w", err)
	}
	if result.ErrorCode != 0 {
		return 0, fmt.Errorf("Quickbase error %d: %s", result.ErrorCode, result.ErrorText)
	}
	return result.PageID, nil
}
