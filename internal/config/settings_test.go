package config

import (
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetViperForTest(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestSetSettingValueAcceptsSupportedTypes(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  string
		assert func(*testing.T)
	}{
		{
			name:  "boolean",
			key:   "log_enabled",
			value: "true",
			assert: func(t *testing.T) {
				assert.True(t, viper.GetBool(LOG_ENABLED_KEY))
			},
		},
		{
			name:  "duration",
			key:   "self_update_timeout",
			value: "2s",
			assert: func(t *testing.T) {
				assert.Equal(t, 2*time.Second, viper.GetDuration(SELF_UPDATE_TIMEOUT_KEY))
			},
		},
		{
			name:  "string",
			key:   "self_update_base_url",
			value: "https://updates.example.test",
			assert: func(t *testing.T) {
				assert.Equal(t, "https://updates.example.test", viper.GetString(SELF_UPDATE_BASE_URL_KEY))
			},
		},
		{
			name:  "integer",
			key:   "metric_statsd_port",
			value: "9125",
			assert: func(t *testing.T) {
				assert.Equal(t, 9125, viper.GetInt(METRIC_STATSD_PORT_KEY))
			},
		},
		{
			name:  "log level",
			key:   "log_level",
			value: "DEBUG",
			assert: func(t *testing.T) {
				assert.Equal(t, "debug", viper.GetString(LOG_LEVEL_KEY))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetViperForTest(t)

			require.NoError(t, SetSettingValue(test.key, test.value))
			test.assert(t)
		})
	}
}

func TestSetSettingValueRejectsInvalidValuesWithoutMutation(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		initial  any
		invalid  string
		readBack func() any
	}{
		{
			name:    "boolean",
			key:     LOG_ENABLED_KEY,
			initial: true,
			invalid: "yes",
			readBack: func() any {
				return viper.GetBool(LOG_ENABLED_KEY)
			},
		},
		{
			name:    "integer",
			key:     METRIC_STATSD_PORT_KEY,
			initial: 8125,
			invalid: "not-a-port",
			readBack: func() any {
				return viper.GetInt(METRIC_STATSD_PORT_KEY)
			},
		},
		{
			name:    "log level",
			key:     LOG_LEVEL_KEY,
			initial: "info",
			invalid: "verbose",
			readBack: func() any {
				return viper.GetString(LOG_LEVEL_KEY)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resetViperForTest(t)
			viper.Set(test.key, test.initial)

			err := SetSettingValue(test.key, test.invalid)

			require.Error(t, err)
			assert.Equal(t, test.initial, test.readBack())
		})
	}
}

func TestSetSettingValueRejectsUnsupportedKey(t *testing.T) {
	resetViperForTest(t)

	err := SetSettingValue("not_a_setting", "value")

	require.EqualError(t, err, "unsupported config not_a_setting")
}

func TestRemoteLifecycle(t *testing.T) {
	resetViperForTest(t)

	require.NoError(t, AddRemote("engineering", "/repos/engineering", "https://engineering.example.test", "daily"))
	require.NoError(t, AddRemote("fallback", "/repos/fallback", "https://fallback.example.test", "invalid-policy"))

	remotes, err := Remotes()
	require.NoError(t, err)
	require.Len(t, remotes, 2)
	assert.Equal(t, ExtraRemote{
		Name:          "engineering",
		RemoteBaseUrl: "https://engineering.example.test",
		RepositoryDir: "/repos/engineering",
		SyncPolicy:    "daily",
	}, remotes[0])
	assert.Equal(t, "always", remotes[1].SyncPolicy, "AddRemote currently normalizes an invalid policy to always")

	require.EqualError(t,
		AddRemote("engineering", "/repos/duplicate-name", "https://other.example.test", "always"),
		"remote already exists",
	)
	require.EqualError(t,
		AddRemote("other-name", "/repos/duplicate-url", "https://engineering.example.test", "always"),
		"remote already exists",
	)

	require.NoError(t, UpdateRemote("engineering", "weekly"))
	require.Error(t, UpdateRemote("engineering", "sometimes"))
	require.EqualError(t, UpdateRemote("missing", "daily"), "remote 'missing' not found")

	remotes, err = Remotes()
	require.NoError(t, err)
	require.Len(t, remotes, 2)
	assert.Equal(t, "weekly", remotes[0].SyncPolicy)

	require.NoError(t, RemoveRemote("engineering"))
	remotes, err = Remotes()
	require.NoError(t, err)
	require.Len(t, remotes, 1)
	assert.Equal(t, "fallback", remotes[0].Name)
}

func TestValidSyncPolicies(t *testing.T) {
	expected := []string{"never", "always", "hourly", "daily", "weekly", "monthly"}
	assert.Equal(t, expected, ValidSyncPolicies())

	for _, policy := range expected {
		assert.True(t, IsValidSyncPolicy(policy), policy)
	}
	assert.False(t, IsValidSyncPolicy(""))
	assert.False(t, IsValidSyncPolicy("sometimes"))
}
