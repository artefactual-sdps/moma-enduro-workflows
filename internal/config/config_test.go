package config_test

import (
	"testing"

	"github.com/artefactual-sdps/temporal-activities/bagcreate"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/moma-enduro-workflows/internal/config"
)

const testConfig = `# Config
debug = true
verbosity = 2
sharedPath = "/home/enduro/shared"
[temporal]
address = "host:port"
namespace = "default"
taskQueue = "preprocessing"
workflowName = "preprocessing"
[worker]
maxConcurrentSessions = 1
[bagit]
checksumAlgorithm = "md5"
`

func TestConfig(t *testing.T) {
	t.Parallel()

	type test struct {
		name            string
		configFile      string
		toml            string
		wantFound       bool
		wantCfg         config.Configuration
		wantErr         string
		wantErrContains string
	}

	for _, tc := range []test{
		{
			name:       "Loads configuration from a TOML file",
			configFile: "moma-enduro.toml",
			toml:       testConfig,
			wantFound:  true,
			wantCfg: config.Configuration{
				Debug:      true,
				Verbosity:  2,
				SharedPath: "/home/enduro/shared",
				Temporal: config.Temporal{
					Address:      "host:port",
					Namespace:    "default",
					TaskQueue:    "preprocessing",
					WorkflowName: "preprocessing",
				},
				Worker: config.WorkerConfig{
					MaxConcurrentSessions: 1,
				},
				Bagit: bagcreate.Config{
					ChecksumAlgorithm: "md5",
				},
			},
		},
		{
			name:       "Errors when configuration values are not valid",
			configFile: "moma-enduro.toml",
			wantFound:  true,
			wantErr: `invalid configuration
SharedPath: missing required value
Temporal.TaskQueue: missing required value
Temporal.WorkflowName: missing required value`,
		},
		{
			name:       "Errors when MaxConcurrentSessions is less than 1",
			configFile: "moma-enduro.toml",
			toml: `# Config
sharedPath = "/home/enduro/shared"
[temporal]
taskQueue = "preprocessing"
workflowName = "preprocessing"
[worker]
maxConcurrentSessions = -1
`,
			wantFound: true,
			wantErr: `invalid configuration
Worker.MaxConcurrentSessions: -1 is less than the minimum value (1)`,
		},
		{
			name:       "Errors when bagit checksumAlgorithm is invalid",
			configFile: "moma-enduro.toml",
			toml: `# Config
sharedPath = "/home/enduro/shared"
[temporal]
taskQueue = "preprocessing"
workflowName = "preprocessing"
[bagit]
checksumAlgorithm = "unknown"
`,
			wantFound: true,
			wantErr: `invalid configuration
Bagit.ChecksumAlgorithm: invalid value "unknown", must be one of (md5, sha1, sha256, sha512)`,
		},
		{
			name:       "Errors when TOML is invalid",
			configFile: "moma-enduro.toml",
			toml:       "bad TOML",
			wantFound:  true,
			wantErr:    "failed to read configuration file: While parsing config: toml: expected character =",
		},
		{
			name:            "Errors when no config file is found in the default paths",
			wantFound:       false,
			wantErrContains: "Config File \"moma-enduro\" Not Found in \"[",
		},
		{
			name:            "Errors when the given configFile is not found",
			configFile:      "missing.toml",
			wantFound:       false,
			wantErrContains: "configuration file not found: ",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := fs.NewDir(t, "moma-enduro-test", fs.WithFile("moma-enduro.toml", tc.toml))

			configFile := ""
			if tc.configFile != "" {
				configFile = tmpDir.Join(tc.configFile)
			}

			var c config.Configuration
			found, configFileUsed, err := config.Read(&c, configFile)
			if tc.wantErr != "" {
				assert.Equal(t, found, tc.wantFound)
				assert.Error(t, err, tc.wantErr)
				return
			}
			if tc.wantErrContains != "" {
				assert.Equal(t, found, tc.wantFound)
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, found, true)
			assert.Equal(t, configFileUsed, configFile)
			assert.DeepEqual(t, c, tc.wantCfg)
		})
	}
}
