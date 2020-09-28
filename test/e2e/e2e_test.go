package e2e

import "testing"

func TestE2E(t *testing.T) {
	t.Run("joinTest", testJoin)
	t.Run("netTest", testNet)
	t.Run("testDeployApp", testDeployApp)
}
