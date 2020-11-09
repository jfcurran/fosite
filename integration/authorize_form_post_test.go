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
	"strings"
	"testing"

	"github.com/ory/fosite/handler/openid"
	"github.com/ory/fosite/internal"
	"github.com/ory/fosite/token/jwt"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	goauth "golang.org/x/oauth2"

	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
)

func TestAuthorizeFormPostResponseMode(t *testing.T) {
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
		ResponseModes: []fosite.ResponseModeType{fosite.ResponseModeFormPost},
	}
	fositeStore.Clients["response-mode-client"] = responseModeClient
	oauthClient.ClientID = "response-mode-client"

	var state string
	for k, c := range []struct {
		description  string
		setup        func()
		check        func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string)
		responseType string
	}{
		{
			description:  "implicit grant #1 test with form_post",
			responseType: "id_token%20token",
			setup: func() {
				state = "12345678901234567890"
				oauthClient.Scopes = []string{"openid"}
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
			description:  "implicit grant #2 test with form_post",
			responseType: "id_token",
			setup: func() {
				state = "12345678901234567890"
				oauthClient.Scopes = []string{"openid"}
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.EqualValues(t, state, stateFromServer)
				assert.NotEmpty(t, iDToken)
			},
		},
		{
			description:  "Authorization code grant test with form_post",
			responseType: "code",
			setup: func() {
				state = "12345678901234567890"
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.EqualValues(t, state, stateFromServer)
				assert.NotEmpty(t, code)
			},
		},
		{
			description:  "Hybrid #1 grant test with form_post",
			responseType: "token%20code",
			setup: func() {
				state = "12345678901234567890"
				oauthClient.Scopes = []string{"openid"}
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.EqualValues(t, state, stateFromServer)
				assert.NotEmpty(t, code)
				assert.NotEmpty(t, token.TokenType)
				assert.NotEmpty(t, token.AccessToken)
				assert.NotEmpty(t, token.Expiry)
			},
		},
		{
			description:  "Hybrid #2 grant test with form_post",
			responseType: "token%20id_token%20code",
			setup: func() {
				state = "12345678901234567890"
				oauthClient.Scopes = []string{"openid"}
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.EqualValues(t, state, stateFromServer)
				assert.NotEmpty(t, code)
				assert.NotEmpty(t, iDToken)
				assert.NotEmpty(t, token.TokenType)
				assert.NotEmpty(t, token.AccessToken)
				assert.NotEmpty(t, token.Expiry)
			},
		},
		{
			description:  "Hybrid #3 grant test with form_post",
			responseType: "id_token%20code",
			setup: func() {
				state = "12345678901234567890"
				oauthClient.Scopes = []string{"openid"}
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.EqualValues(t, state, stateFromServer)
				assert.NotEmpty(t, code)
				assert.NotEmpty(t, iDToken)
			},
		},
		{
			description:  "error message test for form_post response",
			responseType: "foo",
			setup: func() {
				state = "12345678901234567890"
			},
			check: func(t *testing.T, stateFromServer string, code string, token goauth.Token, iDToken string, err map[string]string) {
				assert.EqualValues(t, state, stateFromServer)
				assert.NotEmpty(t, err["ErrorField"])
				assert.NotEmpty(t, err["DescriptionField"])
			},
		},
	} {
		t.Run(fmt.Sprintf("case=%d/description=%s", k, c.description), func(t *testing.T) {
			c.setup()
			authURL := strings.Replace(oauthClient.AuthCodeURL(state, goauth.SetAuthURLParam("response_mode", "form_post"), goauth.SetAuthURLParam("nonce", "111111111")), "response_type=code", "response_type="+c.responseType, -1)
			client := &http.Client{
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return errors.New("Dont follow redirects")
				},
			}
			resp, err := client.Get(authURL)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			code, state, token, iDToken, _, errResp, err := internal.ParseFormPostResponse(fositeStore.Clients["response-mode-client"].GetRedirectURIs()[0], resp.Body)
			require.NoError(t, err)
			c.check(t, state, code, iDToken, token, errResp)
		})
	}
}
