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

// Package connectors implements connectors to different security services,
// like AWS GuardDuty, AWS Security Hub, or Palo Alto Prisma Cloud.
package connectors

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/pkg/errors"
)

// return valid AWS role ARN for provided accountID and role name
func buildRoleARN(accountID, roleName string) string {
	return fmt.Sprintf("arn:aws:iam::%s:role/%s", accountID, roleName)
}

// GetAccountID returns AWS account ID using provided session, without error handling because in case of problem
// with credentials we'll see it on the first use
func GetAccountID(session client.ConfigProvider) (string, error) {
	arn, err := sts.New(session).GetCallerIdentity(nil)
	if err != nil {
		return "", errors.Wrap(err, "problem retrieving account id")
	}
	return *arn.Account, nil
}

// NewMasterMemberSess returns AWS session.Session object for specified region for master account and
// provided role in member account
func NewMasterMemberSess(region, memberAccountID, memberRole string) (*session.Session, *session.Session) {
	masterSess := session.Must(session.NewSession(
		&aws.Config{
			Region: aws.String(region),
		}))

	assumeArn := buildRoleARN(memberAccountID, memberRole)
	stsCreds := stscreds.NewCredentials(masterSess, assumeArn)
	memberSess := session.Must(session.NewSession(
		&aws.Config{
			Credentials: stsCreds,
			Region:      aws.String(region),
		}))
	return masterSess, memberSess
}
