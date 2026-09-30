package vault

import (
	"fmt"
	"testing"

	"github.com/jdevera/command-launcher/internal/context"
	"github.com/stretchr/testify/require"
)

var testContext context.LauncherContext

func init() {
	testContext = context.InitContext("testvault", "1.0.0", "1")
}

func isolateVault(t *testing.T) {
	t.Helper()

	home := t.TempDir()
	t.Setenv(testContext.AppHomeEnvVar(), home)
	t.Setenv(testContext.VaultSecretEnvVar(), "very_secret")

	// CreateVault checks the legacy user-home path before initializing a new
	// vault. Keep that migration lookup inside the test directory too.
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

func TestVault_Init(t *testing.T) {
	isolateVault(t)

	_, err := CreateVault("unit-test")
	require.NoError(t, err)
}

func TestVault_WriteRead(t *testing.T) {
	isolateVault(t)

	fv, err := CreateVault("unit-test")
	require.NoError(t, err)

	err = fv.Write("mykey", "myvalue")
	require.NoError(t, err)

	val, err := fv.Read("mykey")
	require.NoError(t, err)
	require.Equal(t, "myvalue", val)
}

func TestVault_MultiWriteRead(t *testing.T) {
	isolateVault(t)

	fv, err := CreateVault("unit-test")
	require.NoError(t, err)

	for i := 0; i < 1000; i++ {
		err = fv.Write(fmt.Sprintf("mykey-%d", i), fmt.Sprintf("myvalue-%d", i))
		require.NoError(t, err)
	}

	for i := 0; i < 1000; i++ {
		val, err := fv.Read(fmt.Sprintf("mykey-%d", i))
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("myvalue-%d", i), val)
	}
}
