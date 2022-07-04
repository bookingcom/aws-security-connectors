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

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/securityhub"
)

// SecurityHubInviter is a per-region structure which contains all information
// for adding new member account to Security Hub master.
type SecurityHubInviter struct {
	masterSvc SecurityHubMasterClient
	memberSvc SecurityHubMemberClient
}

// SecurityHubMasterClient is a subset of aws-sdk-go/service/securityhub which is used for sending
// invitations from Security Hub master.
type SecurityHubMasterClient interface {
	GetMembers(*securityhub.GetMembersInput) (*securityhub.GetMembersOutput, error)
	CreateMembers(*securityhub.CreateMembersInput) (*securityhub.CreateMembersOutput, error)
	InviteMembers(*securityhub.InviteMembersInput) (*securityhub.InviteMembersOutput, error)
}

// SecurityHubMemberClient is a subset of aws-sdk-go/service/securityhub which is used for accepting
// invitations on Security Hub member.
type SecurityHubMemberClient interface {
	ListInvitations(*securityhub.ListInvitationsInput) (*securityhub.ListInvitationsOutput, error)
	AcceptInvitation(*securityhub.AcceptInvitationInput) (*securityhub.AcceptInvitationOutput, error)
}

// NewSecurityHubInviter creates new instance of SecurityHubInviter which is capable of inviting
// specified member account to master account SecurityHub
func NewSecurityHubInviter(masterSess, memberSess client.ConfigProvider) *SecurityHubInviter {
	return &SecurityHubInviter{
		masterSvc: securityhub.New(masterSess),
		memberSvc: securityhub.New(memberSess),
	}
}

// AddMember adds new member account to master, sends invite to it,
// and then accepts invite from the member account.
// In case the member is already in place and connected (enabled), nothing is done.
// https://docs.aws.amazon.com/securityhub/latest/userguide/securityhub-accounts.html
func (s SecurityHubInviter) AddMember(accountID, accountEmail, masterAccountID string) error {
	connected, err := ifSecurityHubMemberAlreadyAssociated(s.masterSvc, &accountID)
	if err != nil {
		return fmt.Errorf("error retrieving information about existing member account: %w", err)
	}
	if connected {
		return nil
	}

	err = setUpSecurityHubMaster(s.masterSvc, &accountID, &accountEmail)
	if err != nil {
		return fmt.Errorf("error setting up master account: %w", err)
	}

	err = acceptSecurityHubMemberInvitation(s.memberSvc, &masterAccountID)
	if err != nil {
		return fmt.Errorf("error accepting invitation in member account: %w", err)
	}

	return nil
}

// ifSecurityHubMemberAlreadyAssociated checks if member account is already present
// in master and is in Associated state.
func ifSecurityHubMemberAlreadyAssociated(s SecurityHubMasterClient, memberAccountID *string) (bool, error) {
	members, err := s.GetMembers(&securityhub.GetMembersInput{
		AccountIds: []*string{memberAccountID},
	})
	if err != nil {
		return false, fmt.Errorf("error getting existing members: %w", err)
	}

	// Search conditions looking for particular account and we expect to get either zero results
	// (account is not yet connected) or one result (account is connected with either Invited or Associated status).
	// Situation with more than single member in the results is impossible but yet be handled correctly by this code.
	if len(members.Members) == 1 {
		if *members.Members[0].MemberStatus == "Associated" {
			return true, nil
		}
	}

	// The check didn't fail but didn't found that member account is in Associated state, returning no error.
	return false, nil
}

// setUpSecurityHubMaster creates new member account and sends invite to it.
func setUpSecurityHubMaster(s SecurityHubMasterClient, memberAccountID, email *string) error {
	_, err := s.CreateMembers(&securityhub.CreateMembersInput{
		AccountDetails: []*securityhub.AccountDetails{{
			AccountId: memberAccountID,
			Email:     email,
		}},
	})
	if err != nil {
		return fmt.Errorf("error creating member account: %w", err)
	}

	_, err = s.InviteMembers(
		&securityhub.InviteMembersInput{
			AccountIds: []*string{memberAccountID},
		})
	if err != nil {
		return fmt.Errorf("error sending invitation: %w", err)
	}

	return nil
}

// acceptSecurityHubMemberInvitation looks for invitation from specified master account and accepts it
func acceptSecurityHubMemberInvitation(s SecurityHubMemberClient, masterAccountID *string) error {
	invitations, err := s.ListInvitations(nil)
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

	_, err = s.AcceptInvitation(&securityhub.AcceptInvitationInput{
		InvitationId: invitationID,
		MasterId:     masterAccountID,
	})
	if err != nil {
		return fmt.Errorf("error accepting invitation: %w", err)
	}

	return nil
}
