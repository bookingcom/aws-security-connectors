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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/detective"
	"github.com/stretchr/testify/assert"
)

func TestDetectiveInviter_AddMember(t *testing.T) {
	// mock requests
	var (
		graphARN        = "mock_graph"
		memberAccID     = "112233445566"
		masterAccID     = "665544332211"
		testEmail       = "email@example.com"
		badGMReq        = dGetMembersReq{err: fmt.Errorf("mock err")}
		emptyGMReq      = dGetMembersReq{output: &detective.GetMembersOutput{}}
		associatedGMReq = dGetMembersReq{output: &detective.GetMembersOutput{
			MemberDetails: []*detective.MemberDetail{{Status: aws.String("Enabled")}}}}
		invitedGMReq = dGetMembersReq{output: &detective.GetMembersOutput{
			MemberDetails: []*detective.MemberDetail{{Status: aws.String("Invited")}}}}
		badCMReq   = dCreateMembersReq{err: fmt.Errorf("mock err")}
		badLIReq   = dListInvitationsReq{err: fmt.Errorf("mock err")}
		emptyLIReq = dListInvitationsReq{output: &detective.ListInvitationsOutput{}}
		goodLIReq  = dListInvitationsReq{output: &detective.ListInvitationsOutput{
			Invitations: []*detective.MemberDetail{{AccountId: &masterAccID, GraphArn: &graphARN}}}}
		badAIReq  = dAcceptInvitationReq{err: fmt.Errorf("mock err")}
		badDReq   = dGraphReq{err: fmt.Errorf("mock err")}
		emptyDReq = dGraphReq{output: &detective.ListGraphsOutput{}}
		goodDReq  = dGraphReq{output: &detective.ListGraphsOutput{GraphList: []*detective.Graph{{Arn: &graphARN}}}}
	)

	var testAPIRequestsDataset = []struct {
		description string
		error       string
		gmReq       dGetMembersReq
		cmReq       dCreateMembersReq
		liReq       dListInvitationsReq
		aiReq       dAcceptInvitationReq
		dReq        dGraphReq
	}{
		{description: "problem checking existing members",
			dReq:  goodDReq,
			gmReq: badGMReq,
			error: "error retrieving information about existing member account: error getting existing members: mock err"},
		{description: "error checking graph during check of existing account",
			gmReq: associatedGMReq,
			dReq:  badDReq,
			error: "can't get graphARN of master account: error listing graphs: mock err"},
		{description: "empty graph during check of existing account",
			gmReq: associatedGMReq,
			dReq:  emptyDReq,
			error: "can't get graphARN of master account: 0 graphs found instead of one"},
		{description: "member already enabled", gmReq: associatedGMReq, dReq: goodDReq},
		{description: "problem creating member account",
			dReq:  goodDReq,
			gmReq: emptyGMReq,
			cmReq: badCMReq,
			error: "error setting up master account: error creating member account: mock err"},
		{description: "problem listing invitations",
			dReq:  goodDReq,
			gmReq: invitedGMReq,
			liReq: badLIReq,
			error: "error accepting invitation in member account: error retrieving list of invitations: mock err"},
		{description: "invitation not found",
			dReq:  goodDReq,
			gmReq: invitedGMReq,
			liReq: emptyLIReq,
			error: "error accepting invitation in member account: can't find invitation from master account"},
		{description: "problem accepting invitation",
			dReq:  goodDReq,
			gmReq: invitedGMReq,
			liReq: goodLIReq,
			aiReq: badAIReq,
			error: "error accepting invitation in member account: error accepting invitation: mock err"},
		{description: "correctly send and accept invitation",
			dReq:  goodDReq,
			gmReq: invitedGMReq,
			liReq: goodLIReq},
	}

	masterSess, memberSess := NewMasterMemberSess("us-west-2", "", "")
	for i, x := range testAPIRequestsDataset {
		t.Run(x.description, func(t *testing.T) {
			master := &mockDMasterClient{
				t:           t,
				email:       &testEmail,
				memberAccID: &memberAccID,
				graphArn:    &graphARN,
				gmReq:       x.gmReq,
				cmReq:       x.cmReq,
				dReq:        x.dReq,
			}
			member := &mockDMemberClient{
				t:               t,
				masterAccountID: &masterAccID,
				graphArn:        &graphARN,
				liReq:           x.liReq,
				aiReq:           x.aiReq,
			}
			s := NewDetectiveInviter(masterSess, memberSess)
			s.masterSvc = master
			s.memberSvc = member
			err := s.AddMember(memberAccID, testEmail, masterAccID)

			if x.error != "" {
				assert.EqualError(t, err, x.error, "Test case %d error check failed", i)
			} else {
				assert.NoError(t, err, "Test case %d error check failed", i)
			}
		})
	}
}

type mockDMasterClient struct {
	t           *testing.T
	email       *string
	memberAccID *string
	graphArn    *string
	gmReq       dGetMembersReq
	cmReq       dCreateMembersReq
	dReq        dGraphReq
}

type dGetMembersReq struct {
	output *detective.GetMembersOutput
	err    error
}
type dCreateMembersReq struct {
	err error
}

type dGraphReq struct {
	output *detective.ListGraphsOutput
	err    error
}

func (s mockDMasterClient) ListGraphs(input *detective.ListGraphsInput) (*detective.ListGraphsOutput, error) {
	assert.Nil(s.t, input)
	return s.dReq.output, s.dReq.err
}

func (s mockDMasterClient) GetMembers(input *detective.GetMembersInput) (*detective.GetMembersOutput, error) {
	assert.Equal(s.t, &detective.GetMembersInput{AccountIds: []*string{s.memberAccID}, GraphArn: s.graphArn}, input)
	return s.gmReq.output, s.gmReq.err
}

func (s mockDMasterClient) CreateMembers(input *detective.CreateMembersInput) (*detective.CreateMembersOutput, error) {
	assert.Equal(s.t, &detective.CreateMembersInput{
		GraphArn: s.graphArn,
		Accounts: []*detective.Account{{
			AccountId:    s.memberAccID,
			EmailAddress: s.email,
		}},
	}, input)
	return nil, s.cmReq.err
}

type mockDMemberClient struct {
	t               *testing.T
	masterAccountID *string
	graphArn        *string
	liReq           dListInvitationsReq
	aiReq           dAcceptInvitationReq
}

type dListInvitationsReq struct {
	output *detective.ListInvitationsOutput
	err    error
}
type dAcceptInvitationReq struct {
	err error
}

func (s mockDMemberClient) ListInvitations(input *detective.ListInvitationsInput) (*detective.ListInvitationsOutput, error) {
	assert.Nil(s.t, input)
	return s.liReq.output, s.liReq.err
}

func (s mockDMemberClient) AcceptInvitation(input *detective.AcceptInvitationInput) (*detective.AcceptInvitationOutput, error) {
	assert.Equal(s.t, &detective.AcceptInvitationInput{GraphArn: s.graphArn}, input)
	return nil, s.aiReq.err
}
