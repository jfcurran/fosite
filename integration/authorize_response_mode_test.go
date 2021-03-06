/*
 * Copyright © 2015-2018 Aeneas Rekkas <aeneas+oss@aeneas.io>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * @author		Aeneas Rekkas <aeneas+oss@aeneas.io>
 * @copyright 	2015-2018 Aeneas Rekkas <aeneas+oss@aeneas.io>
 * @license 	Apache-2.0
 *
 */

package integration_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pkg/errors"

	"github.com/ory/fosite/handler/openid"
	"github.com/ory/fosite/internal"
	"github.com/ory/fosite/token/jwt"

	"github.com/stretchr/testify/require"
	goauth "golang.org/x/oauth2"

	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
)

func TestAuthorizeResponseModes(t *testing.T) {
	session := &defaultSession{
		DefaultSession: &openid.DefaultSession{
			Claims: &jwt.IDTokenClaims{
				Subject: "peter",
			},
			Headers: &jwt.Headers{},
		},
	}
	f := compose.ComposeAllEnabled(new(compose.Config), fositeStore, []byte("some-secret-thats-random-some-secret-thats-random-"), internal.MustRSAKey())
	ts := mockServer(t, f, session)
	defer ts.Close()

	oauthClient := newOAuth2Client(ts)
	defaultClient := fositeStore.Clients["my-client"].(*fosite.DefaultClient)
	defaultClient.RedirectURIs[0] = ts.URL + "/callback"
	responseModeClient := &fosite.DefaultResponseModeClient{
		DefaultClient: defaultClient,
		ResponseModes: []fosite.ResponseModeType{},
	}
	fositeStore.Clients["response-mode-client"] = responseModeClient
	oauthClient.ClientID = "response-mode-client"

	var state string
	for k, c := range []struct {
		description  string
		setup        func()
		check        func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string)
		responseType string
		responseMode string
	}{
		{
			description:  "Should give err because implicit grant with response mode query",
			responseType: "id_token%20token",
			responseMode: "query",
			setup: func() {
				state = "12345678901234567890"
				oauthClient.Scopes = []string{"openid"}
				responseModeClient.ResponseModes = []fosite.ResponseModeType{fosite.ResponseModeQuery}
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.NotEmpty(t, err["Name"])
				assert.NotEmpty(t, err["Description"])
				assert.Equal(t, "Insecure response_mode 'query' for the response_type '[id_token token]'.", err["Hint"])
			},
		},
		{
			description:  "Should pass implicit grant with response mode form_post",
			responseType: "id_token%20token",
			responseMode: "form_post",
			setup: func() {
				state = "12345678901234567890"
				oauthClient.Scopes = []string{"openid"}
				responseModeClient.ResponseModes = []fosite.ResponseModeType{fosite.ResponseModeFormPost}
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.EqualValues(t, state, stateFromServer)
				assert.NotEmpty(t, token.TokenType)
				assert.NotEmpty(t, token.AccessToken)
				assert.NotEmpty(t, token.Expiry)
				assert.NotEmpty(t, iDToken)
			},
		},
		{
			description:  "Should fail because response mode form_post is not allowed by the client",
			responseType: "id_token%20token",
			responseMode: "form_post",
			setup: func() {
				state = "12345678901234567890"
				oauthClient.Scopes = []string{"openid"}
				responseModeClient.ResponseModes = []fosite.ResponseModeType{fosite.ResponseModeQuery}
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.NotEmpty(t, err["Name"])
				assert.NotEmpty(t, err["Description"])
				assert.Equal(t, "The client is not allowed to request response_mode \"form_post\".", err["Hint"])
			},
		},
		{
			description:  "Should pass Authorization code grant test with response mode fragment",
			responseType: "code",
			responseMode: "fragment",
			setup: func() {
				state = "12345678901234567890"
				responseModeClient.ResponseModes = []fosite.ResponseModeType{fosite.ResponseModeFragment}
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.EqualValues(t, state, stateFromServer)
				assert.NotEmpty(t, code)
			},
		},
		{
			description:  "Should pass Authorization code grant test with response mode form_post",
			responseType: "code",
			responseMode: "form_post",
			setup: func() {
				state = "12345678901234567890"
				responseModeClient.ResponseModes = []fosite.ResponseModeType{fosite.ResponseModeFormPost}
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.EqualValues(t, state, stateFromServer)
				assert.NotEmpty(t, code)
			},
		},
		{
			description:  "Should fail Hybrid grant test with query",
			responseType: "token%20code",
			responseMode: "query",
			setup: func() {
				state = "12345678901234567890"
				oauthClient.Scopes = []string{"openid"}
				responseModeClient.ResponseModes = []fosite.ResponseModeType{fosite.ResponseModeQuery}
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				//assert.EqualValues(t, state, stateFromServer)
				assert.NotEmpty(t, err["Name"])
				assert.NotEmpty(t, err["Description"])
				assert.Equal(t, "Insecure response_mode 'query' for the response_type '[token code]'.", err["Hint"])
			},
		},
		{
			description:  "Should pass Hybrid grant test with form_post",
			responseType: "token%20code",
			responseMode: "form_post",
			setup: func() {
				state = "12345678901234567890"
				oauthClient.Scopes = []string{"openid"}
				responseModeClient.ResponseModes = []fosite.ResponseModeType{fosite.ResponseModeFormPost}
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.EqualValues(t, state, stateFromServer)
				assert.NotEmpty(t, code)
				assert.NotEmpty(t, token.TokenType)
				assert.NotEmpty(t, token.AccessToken)
				assert.NotEmpty(t, token.Expiry)
			},
		},
	} {
		t.Run(fmt.Sprintf("case=%d/description=%s", k, c.description), func(t *testing.T) {
			c.setup()
			authURL := strings.Replace(oauthClient.AuthCodeURL(state, goauth.SetAuthURLParam("response_mode", c.responseMode), goauth.SetAuthURLParam("nonce", "111111111")), "response_type=code", "response_type="+c.responseType, -1)
			var callbackURL *url.URL
			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					callbackURL = req.URL
					return errors.New("Dont follow redirects")
				},
			}

			var code, state, iDToken string
			var token goauth.Token
			var errResp map[string]string

			resp, err := client.Get(authURL)
			if callbackURL != nil {
				if fosite.ResponseModeType(c.responseMode) == fosite.ResponseModeFragment {
					require.Error(t, err)
					//fragment
					fragment, err := url.ParseQuery(callbackURL.Fragment)
					require.NoError(t, err)
					code, state, iDToken, token, errResp = getParameters(t, fragment)
				} else if fosite.ResponseModeType(c.responseMode) == fosite.ResponseModeQuery {
					require.Error(t, err)
					//query
					query, err := url.ParseQuery(callbackURL.RawQuery)
					require.NoError(t, err)
					code, state, iDToken, token, errResp = getParameters(t, query)
				}
			}
			if fosite.ResponseModeType(c.responseMode) == fosite.ResponseModeFormPost && resp.Body != nil {
				//form_post
				code, state, iDToken, token, _, errResp, err = internal.ParseFormPostResponse(fositeStore.Clients["response-mode-client"].GetRedirectURIs()[0], resp.Body)
			}
			c.check(t, state, code, token, iDToken, errResp)
		})
	}
}

func getParameters(t *testing.T, param url.Values) (code, state, iDToken string, token goauth.Token, errResp map[string]string) {
	errResp = make(map[string]string)
	if param.Get("error") != "" {
		errResp["Name"] = param.Get("error")
		errResp["Description"] = param.Get("error_description")
		errResp["Hint"] = param.Get("error_hint")
	} else {
		code = param.Get("code")
		state = param.Get("state")
		iDToken = param.Get("id_token")
		token = goauth.Token{
			AccessToken:  param.Get("access_token"),
			TokenType:    param.Get("token_type"),
			RefreshToken: param.Get("refresh_token"),
		}
		if param.Get("expires_in") != "" {
			expires, err := strconv.Atoi(param.Get("expires_in"))
			require.NoError(t, err)
			token.Expiry = time.Now().UTC().Add(time.Duration(expires) * time.Second)
		}
	}
	return
}
