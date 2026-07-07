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
[temporal]
address = "host:port"
namespace = "default"
[worker]
maxConcurrentSessions = 1
taskQueue = "moma-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[preprocessing.bagCreate]
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
			configFile: "moma-enduro-worker.toml",
			toml:       testConfig,
			wantFound:  true,
			wantCfg: config.Configuration{
				Debug:     true,
				Verbosity: 2,
				Temporal: config.TemporalConfig{
					Address:   "host:port",
					Namespace: "default",
				},
				Worker: config.WorkerConfig{
					MaxConcurrentSessions: 1,
					TaskQueue:             "moma-enduro",
				},
				Preprocessing: config.PreprocessingConfig{
					WorkflowName: "preprocessing",
					SharedPath:   "/home/enduro/shared",
					BagCreate: bagcreate.Config{
						ChecksumAlgorithm: "md5",
					},
				},
			},
		},
		{
			name:       "Errors when configuration values are not valid",
			configFile: "moma-enduro-worker.toml",
			wantFound:  true,
			wantErr: `invalid configuration
Temporal.Address: missing required value
Worker.TaskQueue: missing required value
Preprocessing.SharedPath: missing required value
Preprocessing.WorkflowName: missing required value`,
		},
		{
			name:       "Errors when MaxConcurrentSessions is less than 1",
			configFile: "moma-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
maxConcurrentSessions = -1
taskQueue = "moma-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
`,
			wantFound: true,
			wantErr: `invalid configuration
Worker.MaxConcurrentSessions: -1 is less than the minimum value (1)`,
		},
		{
			name:       "Errors when bagcreate checksumAlgorithm is invalid",
			configFile: "moma-enduro-worker.toml",
			toml: `# Config
[temporal]
address = "host:port"
[worker]
taskQueue = "moma-enduro"
[preprocessing]
workflowName = "preprocessing"
sharedPath = "/home/enduro/shared"
[preprocessing.bagCreate]
checksumAlgorithm = "unknown"
`,
			wantFound: true,
			wantErr: `invalid configuration
Preprocessing.BagCreate: ChecksumAlgorithm: invalid value "unknown", must be one of (md5, sha1, sha256, sha512)`,
		},
		{
			name:       "Errors when TOML is invalid",
			configFile: "moma-enduro-worker.toml",
			toml:       "bad TOML",
			wantFound:  true,
			wantErr:    "failed to read configuration file: While parsing config: toml: expected character =",
		},
		{
			name:            "Errors when no config file is found in the default paths",
			wantFound:       false,
			wantErrContains: "Config File \"moma-enduro-worker\" Not Found in \"[",
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

			tmpDir := fs.NewDir(t, "moma-enduro-worker-test", fs.WithFile("moma-enduro-worker.toml", tc.toml))

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
				assert.ErrorContains(t, err, tc.wantErrContains)
				return
			}

			assert.NilError(t, err)
			assert.Equal(t, found, true)
			assert.Equal(t, configFileUsed, configFile)
			assert.DeepEqual(t, c, tc.wantCfg)
		})
	}
}
