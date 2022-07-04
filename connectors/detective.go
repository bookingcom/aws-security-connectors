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
	"github.com/aws/aws-sdk-go/service/detective"
)

// DetectiveInviter is a per-region structure which contains all information
// for adding new member account to Detective master.
type DetectiveInviter struct {
	masterSvc DetectiveMasterClient
	memberSvc DetectiveMemberClient
}

// DetectiveMasterClient is a subset of aws-sdk-go/service/detective which is used for sending
// invitations from Detective master.
type DetectiveMasterClient interface {
	GetMembers(*detective.GetMembersInput) (*detective.GetMembersOutput, error)
	CreateMembers(*detective.CreateMembersInput) (*detective.CreateMembersOutput, error)
	ListGraphs(*detective.ListGraphsInput) (*detective.ListGraphsOutput, error)
}

// DetectiveMemberClient is a subset of aws-sdk-go/service/detective which is used for accepting
// invitations on Detective member.
type DetectiveMemberClient interface {
	ListInvitations(*detective.ListInvitationsInput) (*detective.ListInvitationsOutput, error)
	AcceptInvitation(*detective.AcceptInvitationInput) (*detective.AcceptInvitationOutput, error)
}

// NewDetectiveInviter creates new instance of DetectiveInviter which is capable of inviting
// specified member account to master account Detective
func NewDetectiveInviter(masterSess, memberSess client.ConfigProvider) *DetectiveInviter {
	return &DetectiveInviter{
		masterSvc: detective.New(masterSess),
		memberSvc: detective.New(memberSess),
	}
}

// AddMember adds new member account to master, sends invite to it,
// and then accepts invite from the member account.
// In case the member is already in place and connected (enabled), nothing is done.
// https://docs.aws.amazon.com/detective/latest/userguide/detective-accounts.html
func (d DetectiveInviter) AddMember(accountID, accountEmail, masterAccountID string) error {
	graphARN, err := getGraphARN(d.masterSvc)
	if err != nil {
		return fmt.Errorf("can't get graphARN of master account: %w", err)
	}

	connected, err := ifDetectiveMemberAlreadyEnabled(d.masterSvc, graphARN, &accountID)
	if err != nil {
		return fmt.Errorf("error retrieving information about existing member account: %w", err)
	}
	if connected {
		return nil
	}

	err = setUpDetectiveMaster(d.masterSvc, graphARN, &accountID, &accountEmail)
	if err != nil {
		return fmt.Errorf("error setting up master account: %w", err)
	}

	err = acceptDetectiveMemberInvitation(d.memberSvc, &masterAccountID)
	if err != nil {
		return fmt.Errorf("error accepting invitation in member account: %w", err)
	}

	return nil
}

// ifDetectiveMemberAlreadyEnabled checks if member account is already present
// in master and is in Associated state.
func ifDetectiveMemberAlreadyEnabled(d DetectiveMasterClient, graphARN, memberAccountID *string) (bool, error) {
	members, err := d.GetMembers(&detective.GetMembersInput{
		AccountIds: []*string{memberAccountID},
		GraphArn:   graphARN,
	})
	if err != nil {
		return false, fmt.Errorf("error getting existing members: %w", err)
	}

	// Search conditions looking for particular account and we expect to get either zero results
	// (account is not yet connected) or one result (account is connected with either Invited or Enabled status).
	// Situation with more than single member in the results is impossible but yet be handled correctly by this code.
	if len(members.MemberDetails) == 1 {
		if *members.MemberDetails[0].Status == "Enabled" {
			return true, nil
		}
	}

	// The check didn't fail but didn't found that member account is in Enabled state, returning no error.
	return false, nil
}

// setUpDetectiveMaster creates new member account and sends invite to it.
func setUpDetectiveMaster(d DetectiveMasterClient, graphARN, memberAccountID, email *string) error {
	_, err := d.CreateMembers(&detective.CreateMembersInput{
		Accounts: []*detective.Account{{
			AccountId:    memberAccountID,
			EmailAddress: email,
		}},
		GraphArn: graphARN,
	})
	if err != nil {
		return fmt.Errorf("error creating member account: %w", err)
	}

	return nil
}

// acceptDetectiveMemberInvitation looks for invitation from specified master account and accepts it
func acceptDetectiveMemberInvitation(d DetectiveMemberClient, masterAccountID *string) error {
	invitations, err := d.ListInvitations(nil)
	if err != nil {
		return fmt.Errorf("error retrieving list of invitations: %w", err)
	}
	var graphArn *string
	for _, inv := range invitations.Invitations {
		if *inv.AccountId == *masterAccountID {
			graphArn = inv.GraphArn
			break
		}
	}
	if graphArn == nil {
		return fmt.Errorf("can't find invitation from master account")
	}

	_, err = d.AcceptInvitation(&detective.AcceptInvitationInput{
		GraphArn: graphArn,
	})
	if err != nil {
		return fmt.Errorf("error accepting invitation: %w", err)
	}

	return nil
}

// getGraphARN looks for a single graph and returns its ARN, or error otherwise
func getGraphARN(d DetectiveMasterClient) (*string, error) {
	graphs, err := d.ListGraphs(nil)
	if err != nil {
		return nil, fmt.Errorf("error listing graphs: %w", err)
	}
	if len(graphs.GraphList) != 1 {
		return nil, fmt.Errorf(
			"%d graphs found instead of one",
			len(graphs.GraphList),
		)
	}
	return graphs.GraphList[0].Arn, nil
}
