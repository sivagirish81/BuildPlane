package integration

import (
	"os"
	"testing"
)

func TestIntegrationRequiresLocalInfra(t *testing.T) {
	if os.Getenv("BUILDPLANE_RUN_INTEGRATION") == "" {
		t.Skip("set BUILDPLANE_RUN_INTEGRATION=1 after make infra-up to run container-backed integration tests")
	}
}
