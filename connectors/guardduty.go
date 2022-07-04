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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/guardduty"
)

// GuardDutyInviter is a per-region structure which contains all information
// for adding new member account to GuardDuty master.
type GuardDutyInviter struct {
	masterSvc GuardDutyMasterClient
	memberSvc GuardDutyMemberClient
}

// GuardDutyListDetectors is interface for ListDetectors function which is used both in master and member.
type GuardDutyListDetectors interface {
	ListDetectors(*guardduty.ListDetectorsInput) (*guardduty.ListDetectorsOutput, error)
}

// GuardDutyMasterClient is a subset of aws-sdk-go/service/guardduty which is used for sending
// invitations from GuardDuty master.
type GuardDutyMasterClient interface {
	GuardDutyListDetectors
	GetMembers(*guardduty.GetMembersInput) (*guardduty.GetMembersOutput, error)
	CreateMembers(*guardduty.CreateMembersInput) (*guardduty.CreateMembersOutput, error)
	InviteMembers(*guardduty.InviteMembersInput) (*guardduty.InviteMembersOutput, error)
}

// GuardDutyMemberClient is a subset of aws-sdk-go/service/guardduty which is used for accepting
// invitations on GuardDuty member.
type GuardDutyMemberClient interface {
	GuardDutyListDetectors
	ListInvitations(*guardduty.ListInvitationsInput) (*guardduty.ListInvitationsOutput, error)
	AcceptAdministratorInvitation(*guardduty.AcceptAdministratorInvitationInput) (*guardduty.AcceptAdministratorInvitationOutput, error)
}

// NewGuardDutyInviter creates new instance of GuardDutyInviter which is capable of inviting
// specified member account to master account GuardDuty
func NewGuardDutyInviter(masterSess, memberSess client.ConfigProvider) *GuardDutyInviter {
	return &GuardDutyInviter{
		masterSvc: guardduty.New(masterSess),
		memberSvc: guardduty.New(memberSess),
	}
}

// AddMember adds new member account to master, sends invite to it,
// and then accepts invite from the member account.
// In case the member is already in place and connected (enabled), nothing is done.
// https://docs.aws.amazon.com/guardduty/latest/ug/guardduty_accounts.html
func (g GuardDutyInviter) AddMember(accountID, accountEmail, masterAccountID string) error {
	detectorID, err := getDetectorID(g.masterSvc)
	if err != nil {
		return fmt.Errorf("can't get detectorID of master account: %w", err)
	}

	connected, err := ifGuardDutyMemberAlreadyEnabled(g.masterSvc, detectorID, &accountID)
	if err != nil {
		return fmt.Errorf("error retrieving information about existing member account: %w", err)
	}
	if connected {
		return nil
	}

	err = setUpGuardDutyMaster(g.masterSvc, detectorID, &accountID, &accountEmail)
	if err != nil {
		return fmt.Errorf("error setting up master account: %w", err)
	}

	err = acceptGuardDutyMemberInvitation(g.memberSvc, &masterAccountID)
	if err != nil {
		return fmt.Errorf("error accepting invitation in member account: %w", err)
	}

	return nil
}

// ifGuardDutyMemberAlreadyEnabled checks if member account is already present
// in master and is in Enabled state.
func ifGuardDutyMemberAlreadyEnabled(g GuardDutyMasterClient, detectorID, memberAccountID *string) (bool, error) {
	members, err := g.GetMembers(&guardduty.GetMembersInput{
		DetectorId: detectorID,
		AccountIds: []*string{memberAccountID},
	})
	if err != nil {
		return false, fmt.Errorf("error getting existing members: %w", err)
	}

	// Search conditions looking for particular account and we expect to get either zero results
	// (account is not yet connected) or one result (account is connected with either Invited or Enabled status).
	// Situation with more than single member in the results is impossible but yet be handled correctly by this code.
	if len(members.Members) == 1 {
		if *members.Members[0].RelationshipStatus == "Enabled" {
			return true, nil
		}
	}

	// The check didn't fail but didn't found that member account is in Enabled state, returning no error.
	return false, nil
}

// setUpGuardDutyMaster creates new member account and sends invite to it.
func setUpGuardDutyMaster(g GuardDutyMasterClient, detectorID, memberAccountID, email *string) error {
	_, err := g.CreateMembers(&guardduty.CreateMembersInput{
		DetectorId: detectorID,
		AccountDetails: []*guardduty.AccountDetail{{
			AccountId: memberAccountID,
			Email:     email,
		}},
	})
	if err != nil {
		return fmt.Errorf("error creating member account: %w", err)
	}

	_, err = g.InviteMembers(&guardduty.InviteMembersInput{
		DetectorId:               detectorID,
		AccountIds:               []*string{memberAccountID},
		DisableEmailNotification: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("error sending invitation: %w", err)
	}

	return nil
}

// acceptGuardDutyMemberInvitation looks for invitation from specified master account and accepts it
func acceptGuardDutyMemberInvitation(g GuardDutyMemberClient, masterAccountID *string) error {
	invitations, err := g.ListInvitations(nil)
	if err != nil {
		return fmt.Errorf("error retrieving list of invitations: %w", err)
	}
	var invitationID *string
	for _, inv := range invitations.Invitations {
		if *inv.AccountId == *masterAccountID {
			invitationID = inv.InvitationId
			break
		}
	}
	if invitationID == nil {
		return fmt.Errorf("can't find invitation from master account")
	}

	detector, err := getDetectorID(g)
	if err != nil {
		return fmt.Errorf("can't get detectorID to accept invitation: %w", err)
	}

	_, err = g.AcceptAdministratorInvitation(
		&guardduty.AcceptAdministratorInvitationInput{
			DetectorId:      detector,
			InvitationId:    invitationID,
			AdministratorId: masterAccountID,
		})
	if err != nil {
		return fmt.Errorf("error accepting invitation: %w", err)
	}

	return nil
}

// getDetectorID looks for a single detector and returns its ID, or error otherwise
func getDetectorID(g GuardDutyListDetectors) (*string, error) {
	detectors, err := g.ListDetectors(nil)
	if err != nil {
		return nil, fmt.Errorf("error listing detectors: %w", err)
	}
	if len(detectors.DetectorIds) != 1 {
		return nil, fmt.Errorf(
			"%d detectors found instead of one",
			len(detectors.DetectorIds),
		)
	}
	return detectors.DetectorIds[0], nil
}
