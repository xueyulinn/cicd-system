package cli

import (
	"errors"
	"testing"
)

func TestHandleUnknownCommandError_NonUnknownAndUnknown(t *testing.T) {
	handleUnknownCommandError(errors.New("some other error"))
	handleUnknownCommandError(errors.New(`unknown command "veriff" for "cicd"`))
}
