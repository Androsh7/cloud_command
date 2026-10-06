package ec2

import (
	"errors"
	"testing"

	"github.com/aws/smithy-go"
)

func TestIsNotFoundErrorMatchesNotFoundCodes(t *testing.T) {
	err := &smithy.GenericAPIError{Code: "InvalidInstanceID.NotFound", Message: "gone"}

	if !IsNotFoundError(err) {
		t.Fatal("expected a not found error")
	}
}

func TestIsNotFoundErrorIgnoresOtherErrors(t *testing.T) {
	apiError := &smithy.GenericAPIError{Code: "UnauthorizedOperation", Message: "denied"}

	if IsNotFoundError(apiError) || IsNotFoundError(errors.New("plain")) {
		t.Fatal("expected other errors to not match")
	}
}
