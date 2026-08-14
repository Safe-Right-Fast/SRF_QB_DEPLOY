package quickbase

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	HTTP    *http.Client
	BaseURL string
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

func realmURL(realm string) (string, error) {
	base := NormalizeRealm(realm)
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Port() != "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" || !strings.HasSuffix(strings.ToLower(parsed.Hostname()), ".quickbase.com") {
		return "", fmt.Errorf("realm must be a Quickbase realm or https://*.quickbase.com URL")
	}
	return base, nil
}

func defaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (client Client) AddReplaceDBPage(dbid, realm, userToken, appToken, pageName string, pageBody []byte) (int, error) {
	baseURL := client.BaseURL
	if baseURL == "" {
		var err error
		baseURL, err = realmURL(realm)
		if err != nil {
			return 0, err
		}
	}
	payload, err := xml.Marshal(addRequest{
		PageName: pageName, PageType: 1, PageBody: cdata{string(pageBody)}, UserToken: userToken, AppToken: appToken,
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(baseURL, "/")+"/db/"+url.PathEscape(dbid), strings.NewReader(string(payload)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("X_QUICKBASE_RETURN_HTTP_ERROR", "true")
	req.Header.Set("QUICKBASE-ACTION", "API_AddReplaceDBPage")
	req.Header.Set("Content-Type", "application/xml")

	httpClient := client.HTTP
	if httpClient == nil {
		httpClient = defaultHTTPClient()
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
