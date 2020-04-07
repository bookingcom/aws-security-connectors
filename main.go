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

package main

import (
	"os"

	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/hashicorp/go-multierror"
	"github.com/jessevdk/go-flags"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/bookingcom/aws-security-connectors/connectors"
)

//nolint:staticcheck
type opts struct {
	Prisma struct {
		AccountName string `long:"account_name" env:"ACCOUNT_NAME" description:"Name for AWS connection"`
		ExternalID  string `long:"external_id" env:"EXTERNAL_ID" description:"An UUID that is used to enable the trust relationship in the role's trust policy"`
		RoleName    string `long:"role_name" env:"ROLE_NAME" description:"Name of AWS role, created for Prisma"`
		APIUrl      string `long:"api_url" env:"API_URL" default:"https://api.eu.prismacloud.io" description:"Prisma API URL"`
		APIKey      string `long:"api_key" env:"API_KEY" description:"Prisma API key"`
		APIPassword string `long:"api_password" env:"API_PASSWORD" description:"Prisma API password"`
	} `group:"Prisma parameters" namespace:"prisma" env-namespace:"PRISMA"`
	AWS struct {
		AccountID        string   `long:"account_id" env:"ACCOUNT_ID" required:"true" description:"ID of AWS account to add"`
		Email            string   `long:"account_email" env:"ACCOUNT_EMAIL" description:"Member account email for invitation sending"`
		RoleName         string   `long:"role_name" env:"ROLE_NAME" description:"Name of member account AWS role to assume for invitation accepting"`
		RegionExceptions []string `long:"region_exceptions" env:"REGION_EXCEPTIONS" default:"ap-east-1" default:"me-south-1" description:"Regions to skip" env-delim:","`
		Detective        bool     `long:"detective" env:"DETECTIVE" description:"Connect Detective"`
		GuardDuty        bool     `long:"guardduty" env:"GUARDDUTY" description:"Connect GuardDuty"`
		SecurityHub      bool     `long:"security_hub" env:"SECURITY_HUB" description:"Connect Security Hub"`
	} `group:"AWS security services parameters" namespace:"aws" env-namespace:"AWS"`
	Dbg bool `long:"dbg" env:"DEBUG" description:"debug mode"`
}

func main() {
	var opts = opts{}
	if _, err := flags.Parse(&opts); err != nil {
		os.Exit(1)
	}

	if opts.Dbg {
		log.SetLevel(log.DebugLevel)
		log.SetReportCaller(true)
	}

	log.Infof("Starting account %s adding to cloud security tools", opts.AWS.AccountID)

	var result error

	if opts.Prisma.APIKey != "" && opts.Prisma.APIPassword != "" {
		p := connectors.NewPrisma(opts.Prisma.APIKey, opts.Prisma.APIPassword, opts.Prisma.APIUrl)
		if err := p.AddAWSAccount(
			opts.AWS.AccountID,
			opts.Prisma.AccountName,
			opts.Prisma.ExternalID,
			opts.Prisma.RoleName,
		); err != nil {
			result = multierror.Append(result,
				errors.Wrap(err, "problem adding account to Prisma"))
		}
	}

	if opts.AWS.GuardDuty || opts.AWS.SecurityHub || opts.AWS.Detective {
		var masterAccountID string
		var memberSess client.ConfigProvider
		var masterSess client.ConfigProvider

		for region := range endpoints.AwsPartition().Regions() {
			if contains(opts.AWS.RegionExceptions, region) {
				continue
			}

			masterSess, memberSess = connectors.NewMasterMemberSess(region, opts.AWS.AccountID, opts.AWS.RoleName)

			// retrieve master account ID once
			if masterAccountID == "" {
				var err error
				if masterAccountID, err = connectors.GetAccountID(masterSess); err != nil {
					result = multierror.Append(result,
						errors.Wrapf(err, "problem retrieving master account ID, aborting AWS services adding"))
					break
				}
			}

			if opts.AWS.GuardDuty {
				g := connectors.NewGuardDutyInviter(masterSess, memberSess)
				if err := g.AddMember(opts.AWS.AccountID, opts.AWS.Email, masterAccountID); err != nil {
					result = multierror.Append(result,
						errors.Wrapf(err, "problem adding member account to AWS GuardDuty in %s", region))
				}
			}

			if opts.AWS.SecurityHub {
				s := connectors.NewSecurityHubInviter(masterSess, memberSess)
				if err := s.AddMember(opts.AWS.AccountID, opts.AWS.Email, masterAccountID); err != nil {
					result = multierror.Append(result,
						errors.Wrapf(err, "problem adding member account to AWS Security Hub in %s", region))
				}
			}

			if opts.AWS.Detective {
				d := connectors.NewDetectiveInviter(masterSess, memberSess)
				if err := d.AddMember(opts.AWS.AccountID, opts.AWS.Email, masterAccountID); err != nil {
					result = multierror.Append(result,
						errors.Wrapf(err, "problem adding member account to AWS Detective in %s", region))
				}
			}
		}
	}

	if result != nil {
		log.Errorf("Problem(s) with adding member account to security tools:\n%s", result)
		os.Exit(3)
	}
	log.Info("Done without errors")
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
