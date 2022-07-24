# AWS Security Connectors [![Build Status](https://github.com/bookingcom/aws-security-connectors/workflows/build/badge.svg)](https://github.com/bookingcom/aws-security-connectors/actions/workflows/ci-build.yml) [![Run Status](https://github.com/bookingcom/aws-security-connectors/workflows/run/badge.svg)](https://github.com/bookingcom/aws-security-connectors/actions/workflows/ci-run.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/bookingcom/aws-security-connectors)](https://goreportcard.com/report/github.com/bookingcom/aws-security-connectors) [![Image Size](https://img.shields.io/docker/image-size/paskal/aws-security-connectors)](https://hub.docker.com/r/paskal/aws-security-connectors)

## Overview

Supported actions:

- [Palo Alto Networks Prisma Cloud](https://www.paloaltonetworks.com/cloud-security): add new account or
 update existing one with new information
- AWS Security Hub: connect member account to master, both member and master must have service already enabled
- AWS GuardDuty: connect member account to master, both member and master must have service already enabled
- AWS Detective: connect member account to master, both member and master must have service already enabled

## How to run

```console
git clone https://github.com/bookingcom/aws-security-connectors.git
cd aws-security-connectors
# build a docker image with the application
docker-compose build aws-security-connectors
docker-compose run aws-security-connectors --help
# or build on your machine
go build -o bin/aws-security-connectors main.go
./bin/aws-security-connectors --help   
```

### Parameters

| Command line          | Environment          | Default          | Description                           |
| --------------------- | -------------------- | ---------------- | ------------------------------------- |
| --aws.account_id      | AWS_ACCOUNT_ID       |                  | ID of AWS account to add, *required*  |
| --aws.account_email   | AWS_ACCOUNT_EMAIL    |                  | Member account email for invitation sending |
| --aws.role_name       | AWS_ROLE_NAME        |                  | Name of member account AWS role to assume for invitation accepting |
| --aws.region_exceptions | AWS_REGION_EXCEPTIONS | `ap-east-1,me-south-1` | Regions to skip              |
| --aws.detective       | AWS_DETECTIVE        |                  | Connect Detective                     |
| --aws.guardduty       | AWS_GUARDDUTY        |                  | Connect GuardDuty                     |
| --aws.security_hub    | AWS_SECURITY_HUB     |                  | Connect Security Hub                  |
| --prisma.account_name | PRISMA_ACCOUNT_NAME  | aws_account_id   | Name for AWS connection               |
| --prisma.external_id  | PRISMA_EXTERNAL_ID   |                  | An UUID that is used to enable the trust relationship in the role's trust policy |
| --prisma.role_name    | PRISMA_ROLE_NAME     |                  | Name of AWS role, created for Prisma  |
| --prisma.api_url      | PRISMA_API_URL       | `https://api.eu.prismacloud.io` | Prisma API URL         |
| --prisma.api_key      | PRISMA_API_KEY       |                  | Prisma API key                        |
| --prisma.api_password | PRISMA_API_PASSWORD  |                  | Prisma API password                   |
| --dbg                 | DEBUG                |                  | debug mode                            |

## Instructions

### Palo Alto Prisma Cloud

Before proceeding, you need to do
[initial AWS setup](https://docs.paloaltonetworks.com/prisma/prisma-cloud/prisma-cloud-admin/connect-your-cloud-platform-to-prisma-cloud/onboard-your-aws-account/add-aws-cloud-account-to-prisma-cloud.html)
(by Terraform, for example) as this program only connects specified account to Prisma using Prisma API.

Then you need to generate Prisma Cloud API [Access Key](https://app.eu.prismacloud.io/settings/access_keys)
with System Admin permissions and write down Access Key ID and Secret Key: they should be passed as Key and Password to the program.

The last step is to run the program itself with the right environment variables:

```sh
AWS_ACCOUNT_ID=112233445566 \
PRISMA_API_KEY=00aaa000aa000a00aaaa000a0a00aaa00000 \
PRISMA_API_PASSWORD=aaa+0aaaaaaaaaaaaaaaa00a0aa= \
PRISMA_EXTERNAL_ID=0000aaa000a0000a0a00000000a0000a \
PRISMA_ROLE_NAME=PrismaReadOnlyRole \
PRISMA_ACCOUNT_NAME="AWS child account 1" \
./bin/aws-security-connectors
```

### AWS Detective \ Security Hub \ GuardDuty

Before starting, you should have:

- appropriate role credentials on host which is running the script
    which are usable in [standard AWS way](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html).
    Required permissions:
    ``` yaml
    # for Detective
    - "detective:GetMembers",
    - "detective:ListMembers",
    - "detective:CreateMembers",
    - "detective:ListGraphs"
    # for Security Hub
    - "securityhub:GetMembers",
    - "securityhub:ListMembers",
    - "securityhub:CreateMembers",
    - "securityhub:InviteMembers",
    # for GuardDuty
    - "guardduty:GetMembers"
    - "guardduty:ListMembers"
    - "guardduty:CreateMembers"
    - "guardduty:InviteMembers"
    - "guardduty:ListDetectors"
    ```
- role in member account which your currently used role can assume (`SecurityInviter` in example below)
    with sufficient permissions:
    ```yaml
    # for Detective
    - "detective:AcceptInvitation"
    - "detective:ListInvitations"
    # for Security Hub
    - "securityhub:AcceptInvitation"
    - "securityhub:ListInvitations"
    # for GuardDuty
    - "guardduty:AcceptInvitation"
    - "guardduty:ListInvitations"
    - "guardduty:ListDetectors"
    ```
- for any service, service enabled in both master and member account
- for GuardDuty, detector enabled both in master and member account
- for Detective, graph created in master account

If pre-requisites are present, run following command in order to get
member created in master account, invitation sent from master and accepted
in member account:

```sh
# enable any set of following services, from one to all:
#AWS_DETECTIVE=true \
#AWS_SECURITY_HUB=true \
AWS_GUARDDUTY=true \
AWS_ACCOUNT_ID=112233445566 \
AWS_ROLE_NAME="SecurityInviter" \
AWS_ACCOUNT_EMAIL="test@example.org" \
./bin/aws-security-connectors
```

## Acknowledgment

This software was originally developed at [Booking.com](http://www.booking.com).
With approval from [Booking.com](http://www.booking.com), this software was released
as Open Source, for which the authors would like to express their gratitude.
