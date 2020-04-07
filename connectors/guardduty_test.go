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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/guardduty"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestGuardDutyInviter_AddMember(t *testing.T) {
	// mock requests
	var (
		invitationID    = "mock_invitation"
		detectorID      = "mock_detector"
		memberAccID     = "112233445566"
		masterAccID     = "665544332211"
		testEmail       = "email@example.com"
		badGMReq        = gdGetMembersReq{err: errors.New("mock err")}
		emptyGMReq      = gdGetMembersReq{output: &guardduty.GetMembersOutput{}}
		associatedGMReq = gdGetMembersReq{output: &guardduty.GetMembersOutput{
			Members: []*guardduty.Member{{RelationshipStatus: aws.String("Enabled")}}}}
		invitedGMReq = gdGetMembersReq{output: &guardduty.GetMembersOutput{
			Members: []*guardduty.Member{{RelationshipStatus: aws.String("Invited")}}}}
		badCMReq   = gdCreateMembersReq{err: errors.New("mock err")}
		badIMReq   = gdInviteMembersReq{err: errors.New("mock err")}
		badLIReq   = gdListInvitationsReq{err: errors.New("mock err")}
		emptyLIReq = gdListInvitationsReq{output: &guardduty.ListInvitationsOutput{}}
		goodLIReq  = gdListInvitationsReq{output: &guardduty.ListInvitationsOutput{
			Invitations: []*guardduty.Invitation{{AccountId: &masterAccID, InvitationId: &invitationID}}}}
		badAIReq  = gdAcceptInvitationReq{err: errors.New("mock err")}
		badDReq   = gdDetectorReq{err: errors.New("mock err")}
		emptyDReq = gdDetectorReq{output: &guardduty.ListDetectorsOutput{}}
		goodDReq  = gdDetectorReq{output: &guardduty.ListDetectorsOutput{DetectorIds: []*string{&detectorID}}}
	)

	var testAPIRequestsDataset = []struct {
		description string
		error       string
		gmReq       gdGetMembersReq
		cmReq       gdCreateMembersReq
		imReq       gdInviteMembersReq
		liReq       gdListInvitationsReq
		aiReq       gdAcceptInvitationReq
		dReqMember  gdDetectorReq
		dReqMaster  gdDetectorReq
	}{
		{description: "problem checking existing members",
			dReqMaster: goodDReq,
			gmReq:      badGMReq,
			error:      "error retrieving information about existing member account: error getting existing members: mock err"},
		{description: "error checking detector during check of existing account",
			gmReq:      associatedGMReq,
			dReqMaster: badDReq,
			error:      "can't get detectorID of master account: error listing detectors: mock err"},
		{description: "empty detector during check of existing account",
			gmReq:      associatedGMReq,
			dReqMaster: emptyDReq,
			error:      "can't get detectorID of master account: 0 detectors found instead of one"},
		{description: "member already enabled", gmReq: associatedGMReq, dReqMaster: goodDReq},
		{description: "problem creating member account",
			dReqMaster: goodDReq,
			gmReq:      emptyGMReq,
			cmReq:      badCMReq,
			error:      "error setting up master account: error creating member account: mock err"},
		{description: "problem inviting member account",
			dReqMaster: goodDReq,
			gmReq:      emptyGMReq,
			imReq:      badIMReq,
			error:      "error setting up master account: error sending invitation: mock err"},
		{description: "problem listing invitations",
			dReqMaster: goodDReq,
			gmReq:      invitedGMReq,
			liReq:      badLIReq,
			error:      "error accepting invitation in member account: error retrieving list of invitations: mock err"},
		{description: "invitation not found",
			dReqMaster: goodDReq,
			gmReq:      invitedGMReq,
			liReq:      emptyLIReq,
			error:      "error accepting invitation in member account: can't find invitation from master account"},
		{description: "error checking detector during accepting invitation",
			dReqMaster: goodDReq,
			dReqMember: badDReq,
			gmReq:      invitedGMReq,
			liReq:      goodLIReq,
			error: "error accepting invitation in member account: can't get detectorID to accept invitation: " +
				"error listing detectors: mock err"},
		{description: "empty detector during accepting invitation",
			dReqMaster: goodDReq,
			dReqMember: emptyDReq,
			gmReq:      invitedGMReq,
			liReq:      goodLIReq,
			error: "error accepting invitation in member account: can't get detectorID to accept invitation: " +
				"0 detectors found instead of one"},
		{description: "problem accepting invitation",
			dReqMaster: goodDReq,
			dReqMember: goodDReq,
			gmReq:      invitedGMReq,
			liReq:      goodLIReq,
			aiReq:      badAIReq,
			error:      "error accepting invitation in member account: error accepting invitation: mock err"},
		{description: "correctly send and accept invitation",
			dReqMaster: goodDReq,
			dReqMember: goodDReq,
			gmReq:      invitedGMReq,
			liReq:      goodLIReq},
	}

	masterSess, memberSess := NewMasterMemberSess("us-west-2", "", "")
	for i, x := range testAPIRequestsDataset {
		i := i
		x := x
		t.Run(x.description, func(t *testing.T) {
			master := &mockGDMasterClient{
				email:       &testEmail,
				memberAccID: &memberAccID,
				detectorID:  &detectorID,
				gmReq:       x.gmReq,
				cmReq:       x.cmReq,
				imReq:       x.imReq,
			}
			master.t = t               // promoted field
			master.dReq = x.dReqMaster // promoted field
			member := &mockGDMemberClient{
				masterAccountID: &masterAccID,
				invitationID:    &invitationID,
				detectorID:      &detectorID,
				liReq:           x.liReq,
				aiReq:           x.aiReq,
			}
			member.t = t               // promoted field
			member.dReq = x.dReqMember // promoted field
			s := NewGuardDutyInviter(masterSess, memberSess)
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

type mockGDDetectorClient struct {
	t    *testing.T
	dReq gdDetectorReq
}

type gdDetectorReq struct {
	output *guardduty.ListDetectorsOutput
	err    error
}

func (s mockGDDetectorClient) ListDetectors(input *guardduty.ListDetectorsInput) (*guardduty.ListDetectorsOutput, error) {
	assert.Nil(s.t, input)
	return s.dReq.output, s.dReq.err
}

type mockGDMasterClient struct {
	mockGDDetectorClient
	email       *string
	memberAccID *string
	detectorID  *string
	gmReq       gdGetMembersReq
	cmReq       gdCreateMembersReq
	imReq       gdInviteMembersReq
}

type gdGetMembersReq struct {
	output *guardduty.GetMembersOutput
	err    error
}
type gdCreateMembersReq struct {
	err error
}
type gdInviteMembersReq struct {
	err error
}

func (s mockGDMasterClient) GetMembers(input *guardduty.GetMembersInput) (*guardduty.GetMembersOutput, error) {
	assert.Equal(s.t, &guardduty.GetMembersInput{AccountIds: []*string{s.memberAccID}, DetectorId: s.detectorID}, input)
	return s.gmReq.output, s.gmReq.err
}

func (s mockGDMasterClient) CreateMembers(input *guardduty.CreateMembersInput) (*guardduty.CreateMembersOutput, error) {
	assert.Equal(s.t, &guardduty.CreateMembersInput{
		DetectorId: s.detectorID,
		AccountDetails: []*guardduty.AccountDetail{{
			AccountId: s.memberAccID,
			Email:     s.email,
		}},
	}, input)
	return nil, s.cmReq.err
}

func (s mockGDMasterClient) InviteMembers(input *guardduty.InviteMembersInput) (*guardduty.InviteMembersOutput, error) {
	assert.Equal(s.t, &guardduty.InviteMembersInput{AccountIds: []*string{s.memberAccID}, DetectorId: s.detectorID, DisableEmailNotification: aws.Bool(true)}, input)
	return nil, s.imReq.err
}

type mockGDMemberClient struct {
	mockGDDetectorClient
	masterAccountID *string
	invitationID    *string
	detectorID      *string
	liReq           gdListInvitationsReq
	aiReq           gdAcceptInvitationReq
}

type gdListInvitationsReq struct {
	output *guardduty.ListInvitationsOutput
	err    error
}
type gdAcceptInvitationReq struct {
	err error
}

func (s mockGDMemberClient) ListInvitations(input *guardduty.ListInvitationsInput) (*guardduty.ListInvitationsOutput, error) {
	assert.Nil(s.t, input)
	return s.liReq.output, s.liReq.err
}

func (s mockGDMemberClient) AcceptInvitation(input *guardduty.AcceptInvitationInput) (*guardduty.AcceptInvitationOutput, error) {
	assert.Equal(s.t, &guardduty.AcceptInvitationInput{InvitationId: s.invitationID, MasterId: s.masterAccountID, DetectorId: s.detectorID}, input)
	return nil, s.aiReq.err
}
