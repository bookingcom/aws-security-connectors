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
	"github.com/aws/aws-sdk-go/service/securityhub"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHubInviter_AddMember(t *testing.T) {
	// mock requests
	var (
		invitationID    = "mock_invitation"
		memberAccID     = "112233445566"
		masterAccID     = "665544332211"
		testEmail       = "email@example.com"
		badGMReq        = shGetMembersReq{err: fmt.Errorf("mock err")}
		emptyGMReq      = shGetMembersReq{output: &securityhub.GetMembersOutput{}}
		associatedGMReq = shGetMembersReq{output: &securityhub.GetMembersOutput{
			Members: []*securityhub.Member{{MemberStatus: aws.String("Associated")}}}}
		invitedGMReq = shGetMembersReq{output: &securityhub.GetMembersOutput{
			Members: []*securityhub.Member{{MemberStatus: aws.String("Invited")}}}}
		badCMReq   = shCreateMembersReq{err: fmt.Errorf("mock err")}
		badIMReq   = shInviteMembersReq{err: fmt.Errorf("mock err")}
		badLIReq   = shListInvitationsReq{err: fmt.Errorf("mock err")}
		emptyLIReq = shListInvitationsReq{output: &securityhub.ListInvitationsOutput{}}
		goodLIReq  = shListInvitationsReq{output: &securityhub.ListInvitationsOutput{
			Invitations: []*securityhub.Invitation{{AccountId: &masterAccID, InvitationId: &invitationID}}}}
		badAIReq = shAcceptInvitationReq{err: fmt.Errorf("mock err")}
	)

	var testAPIRequestsDataset = []struct {
		description string
		error       string
		gmReq       shGetMembersReq
		cmReq       shCreateMembersReq
		imReq       shInviteMembersReq
		liReq       shListInvitationsReq
		aiReq       shAcceptInvitationReq
	}{
		{description: "problem checking existing members",
			gmReq: badGMReq,
			error: "error retrieving information about existing member account: error getting existing members: mock err"},
		{description: "member already associated", gmReq: associatedGMReq},
		{description: "problem creating member account",
			gmReq: emptyGMReq,
			cmReq: badCMReq,
			error: "error setting up master account: error creating member account: mock err"},
		{description: "problem inviting member account",
			gmReq: emptyGMReq,
			imReq: badIMReq,
			error: "error setting up master account: error sending invitation: mock err"},
		{description: "problem listing invitations",
			gmReq: invitedGMReq,
			liReq: badLIReq,
			error: "error accepting invitation in member account: error retrieving list of invitations: mock err"},
		{description: "invitation not found",
			gmReq: invitedGMReq,
			liReq: emptyLIReq,
			error: "error accepting invitation in member account: can't find invitation from master account"},
		{description: "problem accepting invitation",
			gmReq: invitedGMReq,
			liReq: goodLIReq,
			aiReq: badAIReq,
			error: "error accepting invitation in member account: error accepting invitation: mock err"},
		{description: "correctly send and accept invitation",
			gmReq: invitedGMReq,
			liReq: goodLIReq},
	}

	masterSess, memberSess := NewMasterMemberSess("us-west-2", "", "")
	for i, x := range testAPIRequestsDataset {
		t.Run(x.description, func(t *testing.T) {
			master := &mockSHMasterClient{
				t:           t,
				email:       &testEmail,
				memberAccID: &memberAccID,
				gmReq:       x.gmReq,
				cmReq:       x.cmReq,
				imReq:       x.imReq,
			}
			member := &mockSHMemberClient{
				t:               t,
				masterAccountID: &masterAccID,
				invitationID:    &invitationID,
				liReq:           x.liReq,
				aiReq:           x.aiReq,
			}
			s := NewSecurityHubInviter(masterSess, memberSess)
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

type mockSHMasterClient struct {
	t           *testing.T
	email       *string
	memberAccID *string
	gmReq       shGetMembersReq
	cmReq       shCreateMembersReq
	imReq       shInviteMembersReq
}

type shGetMembersReq struct {
	output *securityhub.GetMembersOutput
	err    error
}
type shCreateMembersReq struct {
	err error
}
type shInviteMembersReq struct {
	err error
}

func (s mockSHMasterClient) GetMembers(input *securityhub.GetMembersInput) (*securityhub.GetMembersOutput, error) {
	assert.Equal(s.t, &securityhub.GetMembersInput{AccountIds: []*string{s.memberAccID}}, input)
	return s.gmReq.output, s.gmReq.err
}

func (s mockSHMasterClient) CreateMembers(input *securityhub.CreateMembersInput) (*securityhub.CreateMembersOutput, error) {
	assert.Equal(s.t, &securityhub.CreateMembersInput{
		AccountDetails: []*securityhub.AccountDetails{{
			AccountId: s.memberAccID,
			Email:     s.email,
		}},
	}, input)
	return nil, s.cmReq.err
}

func (s mockSHMasterClient) InviteMembers(input *securityhub.InviteMembersInput) (*securityhub.InviteMembersOutput, error) {
	assert.Equal(s.t, &securityhub.InviteMembersInput{AccountIds: []*string{s.memberAccID}}, input)
	return nil, s.imReq.err
}

type mockSHMemberClient struct {
	t               *testing.T
	masterAccountID *string
	invitationID    *string
	liReq           shListInvitationsReq
	aiReq           shAcceptInvitationReq
}

type shListInvitationsReq struct {
	output *securityhub.ListInvitationsOutput
	err    error
}
type shAcceptInvitationReq struct {
	err error
}

func (s mockSHMemberClient) ListInvitations(input *securityhub.ListInvitationsInput) (*securityhub.ListInvitationsOutput, error) {
	assert.Nil(s.t, input)
	return s.liReq.output, s.liReq.err
}

func (s mockSHMemberClient) AcceptInvitation(input *securityhub.AcceptInvitationInput) (*securityhub.AcceptInvitationOutput, error) {
	assert.Equal(s.t, &securityhub.AcceptInvitationInput{InvitationId: s.invitationID, MasterId: s.masterAccountID}, input)
	return nil, s.aiReq.err
}
