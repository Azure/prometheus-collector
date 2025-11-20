package utils

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// TODO: Merge with GetQueryAccessToken()??
func GetDefaultQueryAccessToken(scope string) (string, error) {

	if len(strings.TrimSpace(scope)) == 0 {
		return "", fmt.Errorf("scope is empty")
	}

	cred, err := CreateDefaultAzureCredential(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create identity credential: %s", err.Error())
	}

	fmt.Printf("Requesting access token for scope: %s\n", scope)
	opts := policy.TokenRequestOptions{
		Scopes: []string{scope},
	}

	accessToken, err := cred.GetToken(context.Background(), opts)
	if err != nil {
		return "", fmt.Errorf("failed to get accesstoken: %s", err.Error())
	}

	return accessToken.Token, nil
}
