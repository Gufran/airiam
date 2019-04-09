package airiam

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lexruntimeservice"
)

func getAwsRegion() (string, error) {
	sess, err := session.NewSession()
	if err != nil {
		return "", err
	}

	meta := ec2metadata.New(sess)
	return meta.Region()
}

func getLexClient(iamRole, region string) (*lexruntimeservice.LexRuntimeService, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})

	if err != nil {
		return nil, err
	}

	var creds *credentials.Credentials

	if iamRole != "" {
		creds = stscreds.NewCredentials(sess, iamRole)
	}

	return lexruntimeservice.New(sess, &aws.Config{
		Credentials: creds,
	}), nil
}
