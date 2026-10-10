package cli

import (
	"errors"
	"testing"
)

func TestClassifyGraphQLOrderNotFound(t *testing.T) {
	err := errors.New(`walmart: GraphQL error: Error resolving field: Order "2000000000000001" cannot be found.`)
	got := classifyAPIErrorOnly(err)
	if ExitCode(got) != 3 {
		t.Fatalf("ExitCode=%d want 3; err=%v", ExitCode(got), got)
	}
}
