// Copyright 2020 Booking.com
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package connectors

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRequest struct {
	method string
	url    string
	answer string
	err    error
}

func TestPrisma_AddAWSAccount(t *testing.T) {
	// mock requests
	var (
		getAccListErr      = mockRequest{url: "/cloud", method: "GET", err: fmt.Errorf("mock error")}
		getAccListBadJSON  = mockRequest{url: "/cloud", method: "GET", answer: "not_json"}
		getAccListEmpty    = mockRequest{url: "/cloud", method: "GET", answer: `[]`}
		getAccListGood     = mockRequest{url: "/cloud", method: "GET", answer: `[{"accountId":"011223344556"}]`}
		getAccInfoErr      = mockRequest{url: "/cloud/aws/011223344556", method: "GET", err: fmt.Errorf("mock error")}
		getAccInfoBadJSON  = mockRequest{url: "/cloud/aws/011223344556", method: "GET", answer: "not_json"}
		getAccInfoGoodDiff = mockRequest{url: "/cloud/aws/011223344556", method: "GET",
			answer: `{"accountId":"011223344556"}`}
		getAccInfoGoodEqual = mockRequest{url: "/cloud/aws/011223344556", method: "GET",
			answer: `{"accountId":"011223344556","enabled":true,"externalId":"test_external_id",
"RoleArn":"arn:aws:iam::011223344556:role/test_role_name"}`}
		getAccUpdateErr  = mockRequest{url: "/cloud/aws/011223344556", method: "PUT", err: fmt.Errorf("mock error")}
		getAccUpdateGood = mockRequest{url: "/cloud/aws/011223344556", method: "PUT"}
		getAccCreateErr  = mockRequest{url: "/cloud/aws/", method: "POST", err: fmt.Errorf("mock error")}
		getAccCreateGood = mockRequest{url: "/cloud/aws/", method: "POST"}
	)

	var testAPIRequestsDataset = []struct {
		description string
		error       string
		requests    []mockRequest
	}{
		{description: "problem checking existing account existence",
			requests: []mockRequest{getAccListErr},
			error:    "error checking for existing account: error retrieving list of accounts: mock error"},
		{description: "json problem checking existing account",
			requests: []mockRequest{getAccListBadJSON},
			error: "error checking for existing account: error unmarshalling accounts information: " +
				"invalid character 'o' in literal null (expecting 'u')"},
		{description: "problem checking existing account details",
			requests: []mockRequest{getAccListGood, getAccInfoErr},
			error:    "error updating existing account: error retrieving existing account details: mock error"},
		{description: "json problem checking existing account details",
			requests: []mockRequest{getAccListGood, getAccInfoBadJSON},
			error: "error updating existing account: error unmarshalling account details: " +
				"invalid character 'o' in literal null (expecting 'u')"},
		{description: "existing account equal to desired",
			requests: []mockRequest{getAccListGood, getAccInfoGoodEqual}},
		{description: "problem updating existing account",
			requests: []mockRequest{getAccListGood, getAccInfoGoodDiff, getAccUpdateErr},
			error:    "error updating existing account: error sending API request: mock error"},
		{description: "existing account updated",
			requests: []mockRequest{getAccListGood, getAccInfoGoodDiff, getAccUpdateGood}},
		{description: "problem creating new account",
			requests: []mockRequest{getAccListEmpty, getAccCreateErr},
			error:    "error creating new account: error sending API request: mock error"},
		{description: "problem creating new account",
			requests: []mockRequest{getAccListEmpty, getAccCreateGood}},
	}

	for i, x := range testAPIRequestsDataset {
		t.Run(x.description, func(t *testing.T) {
			m := &mockClient{t: t, requests: x.requests}
			p := NewPrisma("", "", "")
			p.api = m
			err := p.AddAWSAccount("011223344556", "", "test_external_id", "test_role_name")

			if x.error != "" {
				assert.EqualError(t, err, x.error, "Test case %d error check failed", i)
			} else {
				assert.NoError(t, err, "Test case %d error check failed", i)
			}
			assert.True(t, m.requestsDepleted())
		})
	}
}

type mockClient struct {
	t          *testing.T
	currentReq int
	requests   []mockRequest
}

func (m *mockClient) Call(method, url string, _ io.Reader) ([]byte, error) {
	require.False(m.t, m.currentReq >= len(m.requests), "we're out of mocked requests")
	i := m.currentReq
	m.currentReq++
	assert.Equal(m.t, m.requests[i].url, url)
	assert.Equal(m.t, m.requests[i].method, method)
	return []byte(m.requests[i].answer), m.requests[i].err
}

func (m *mockClient) requestsDepleted() bool {
	return m.currentReq == len(m.requests)
}
