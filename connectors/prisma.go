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
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/paskal/go-prisma"
	log "github.com/sirupsen/logrus"
)

// Prisma contain credentials for API access
type Prisma struct {
	api apiCaller
}

type apiCaller interface {
	Call(method, url string, body io.Reader) ([]byte, error)
}

type prismaCloudAccount struct {
	AccountID string `json:"accountId"`
}

type awsAccountInfo struct {
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	ExternalID string `json:"externalId"`
	RoleArn    string `json:"roleArn"`
	AccountID  string `json:"accountId"`
}

// NewPrisma returns new Prisma client
func NewPrisma(username, password, apiURL string) *Prisma {
	log.Infof("Creating Prisma connection using API key %s", username)
	p := Prisma{}
	p.api = prisma.NewClient(username, password, apiURL)
	return &p
}

// AddAWSAccount adds an AWS account to Prisma, or updates existing one
// with provided AWS credentials in case it's necessary
func (p Prisma) AddAWSAccount(accountID, name, externalID, roleName string) error {
	exists, err := p.ifAWSAccountExists(accountID)
	if err != nil {
		return fmt.Errorf("error checking for existing account: %w", err)
	}

	newAcc := awsAccountInfo{
		Name:       name,
		Enabled:    true,
		ExternalID: externalID,
		RoleArn:    buildRoleARN(accountID, roleName),
		AccountID:  accountID,
	}

	if exists {
		log.Print("Account already exists in Prisma")
		if err := p.updateExistingAWSAccount(newAcc); err != nil {
			return fmt.Errorf("error updating existing account: %w", err)
		}
		return nil
	}

	err = p.createNewAWSAccount(newAcc)
	if err != nil {
		return fmt.Errorf("error creating new account: %w", err)
	}

	return nil
}

// ifAWSAccountExists returns if AWS account is already exist in Prisma,
// false in other case
func (p Prisma) ifAWSAccountExists(accountID string) (bool, error) {
	// https://api.docs.prismacloud.io/reference#get-cloud-accounts
	rawAccounts, err := p.api.Call("GET", "/cloud", nil)
	if err != nil {
		return false, fmt.Errorf("error retrieving list of accounts: %w", err)
	}

	var accounts []prismaCloudAccount
	if err := json.Unmarshal(rawAccounts, &accounts); err != nil {
		return false, fmt.Errorf("error unmarshalling accounts information: %w", err)
	}

	for _, acc := range accounts {
		if acc.AccountID == accountID {
			return true, nil
		}
	}

	return false, nil
}

// updateExistingAWSAccount checks provided account against given one and updates it if necessary.
// Empty name is ignored.
func (p Prisma) updateExistingAWSAccount(acc awsAccountInfo) error {
	// https://api.docs.prismacloud.io/reference#get-cloud-account
	rawAccountInfo, err := p.api.Call("GET", "/cloud/aws/"+acc.AccountID, nil)
	if err != nil {
		return fmt.Errorf("error retrieving existing account details: %w", err)
	}

	var oldAcc awsAccountInfo
	if err := json.Unmarshal(rawAccountInfo, &oldAcc); err != nil {
		return fmt.Errorf("error unmarshalling account details: %w", err)
	}

	// Names are unique and should not be empty.
	// In case we don't have new account name provided by user, take old one instead of updating it.
	if acc.Name == "" {
		acc.Name = oldAcc.Name
	}

	if oldAcc != acc {
		log.Debugf("Existing Prisma account details: %+v", oldAcc)
		log.Debugf("Desired Prisma account details: %+v", acc)

		b, err := json.Marshal(acc)
		if err != nil {
			return fmt.Errorf("error marshaling account info: %w", err)
		}

		// https://api.docs.prismacloud.io/reference#update-cloud-account
		_, err = p.api.Call("PUT", "/cloud/aws/"+acc.AccountID, bytes.NewBuffer(b))
		if err != nil {
			return fmt.Errorf("error sending API request: %w", err)
		}

		log.Info("Prisma account information updated")
		return nil
	}

	log.Info("Prisma account already up to date, doing nothing")
	return nil
}

// createNewAWSAccount creates new cloud account in Prisma.
// Empty name replaced with accountID.
func (p Prisma) createNewAWSAccount(acc awsAccountInfo) error {
	log.Debugf("New Prisma account details %+v", acc)

	if acc.Name == "" {
		acc.Name = acc.AccountID
	}

	b, err := json.Marshal(acc)
	if err != nil {
		return fmt.Errorf("error marshaling account info: %w", err)
	}

	// https://api.docs.prismacloud.io/reference#add-cloud-account
	_, err = p.api.Call("POST", "/cloud/aws/", bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("error sending API request: %w", err)
	}

	log.Info("Prisma account created")
	return nil
}
