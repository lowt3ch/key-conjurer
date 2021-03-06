package keyconjurer

import (
	"keyconjurer-lambda/authenticators"
	"os"

	log "keyconjurer-lambda/logger"

	"github.com/aws/aws-sdk-go/service/sts"
)

// KeyConjurer is used to generate temporary AWS credentials
type KeyConjurer struct {
	AWSClient     *AWSClient
	Authenticator authenticators.Authenticator
	Logger        *log.Logger
}

// New creates an KeyConjurer service
func NewKeyConjurer(client, clientVersion string, auth authenticators.Authenticator, logger *log.Logger) *KeyConjurer {
	awsRegion := os.Getenv("AWSRegion")
	awsClient := NewAWSClient(awsRegion, logger)
	settings := NewSettings(awsClient, awsRegion)
	awsClient.SetKMSKeyID(settings.AwsKMSKeyID)

	return &KeyConjurer{
		AWSClient:     awsClient,
		Authenticator: auth,
		Logger:        logger,
	}
}

// GetUserData retrieves the users devices and apps from OneLogin. The apps
//  are filtered to only include the AWS related applications
func (a *KeyConjurer) GetUserData(user *User) (*UserData, error) {
	authAccounts, err := a.Authenticator.Authenticate(user.Username, user.Password)
	if err != nil {
		a.Logger.Error("KeyConjurer", "GetUserData", "Error authenticating", err.Error())
		return nil, err
	}

	userData := &UserData{
		Devices: make([]Device, 0),
		Apps:    authAccounts,
		Creds:   user.Password,
	}

	return userData, nil
}

// GetAwsCreds authenticates the user against OneLogin, sends a Duo push request
//  to the user, then retrieves AWS credentials
func (a *KeyConjurer) GetAwsCreds(user *User, appID string, keyTimeoutInHours int) (*sts.Credentials, error) {
	samlAssertion, err := a.Authenticator.Authorize(user.Username, user.Password, appID)
	if err != nil {
		a.Logger.Error("KeyConjurer", "GetAWSCreds", "Unable to parse SAML assertion", err.Error())
		return nil, err
	}

	roleArn, principalArn, err := a.AWSClient.SelectRoleFromSaml(samlAssertion)
	if err != nil {
		a.Logger.Error("KeyConjurer", "Unable to select role from SAML", err.Error())
		return nil, err
	}

	a.Logger.Info("KeyConjurer", "Assuming role")
	credentials, err := a.AWSClient.AssumeRole(roleArn, principalArn, samlAssertion, keyTimeoutInHours)
	if err != nil {
		a.Logger.Error("KeyConjurer", "GetAWSCreds", "Unable to assume role", err.Error())
		return nil, err
	}
	return credentials, nil
}
