package integration_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artefactual-sdps/enduro/pkg/childwf"
	"github.com/go-logr/logr"
	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp/cmpopts"
	cp "github.com/otiai10/copy"
	temporalsdk_client "go.temporal.io/sdk/client"
	temporalsdk_testsuite "go.temporal.io/sdk/testsuite"
	"gotest.tools/v3/assert"
	tfs "gotest.tools/v3/fs"

	"github.com/artefactual-sdps/moma-enduro-workflows/cmd/worker/workercmd"
	"github.com/artefactual-sdps/moma-enduro-workflows/internal/config"
	"github.com/artefactual-sdps/moma-enduro-workflows/internal/workflows"
)

const (
	dirMode  fs.FileMode = 0o700
	fileMode fs.FileMode = 0o600

	smallFileSHA512Manifest = `8cbdd4ed5452f7c066509c066d5ea87fc03f30b0c67153624a1bce4d6e14b6709b5e78caf723cdf419d0efad4db96ba1cad3196783c26a7743029459bdd148b0  data/small.txt
`
)

type temporalInstance struct {
	client temporalsdk_client.Client
	addr   string // Used when we're connected to a user-provided instance.
}

func setUpTemporal(ctx context.Context, t *testing.T) *temporalInstance {
	t.Helper()

	// Fallback to development server provided by the Temporal GO SDK.
	s, err := temporalsdk_testsuite.StartDevServer(ctx, temporalsdk_testsuite.DevServerOptions{
		LogLevel: "error",
	})

	assert.NilError(t, err, "Failed to start Temporal development server.")
	t.Cleanup(func() {
		s.Stop()
	})

	c := s.Client()
	t.Cleanup(func() {
		c.Close()
	})

	return &temporalInstance{
		client: c,
		addr:   s.FrontendHostPort(),
	}
}

func defaultConfig() config.Configuration {
	return config.Configuration{
		Verbosity: 2,
		Debug:     true,
		Worker: config.WorkerConfig{
			MaxConcurrentSessions: 1,
			TaskQueue:             "preprocessing",
		},
		Temporal: config.TemporalConfig{
			Namespace: "default",
		},
		Preprocessing: config.PreprocessingConfig{
			WorkflowName: "preprocessing",
		},
	}
}

func defaultLogger(t *testing.T) logr.Logger {
	t.Helper()

	return testr.NewWithOptions(t, testr.Options{
		LogTimestamp: false,
		Verbosity:    2,
	})
}

type testEnv struct {
	t   *testing.T
	cfg config.Configuration

	testDir *tfs.Dir
}

func newTestEnv(t *testing.T, cfg config.Configuration) *testEnv {
	t.Helper()

	env := &testEnv{t: t, cfg: cfg}
	env.createTestDir()

	return env
}

func (env *testEnv) createTestDir() {
	env.t.Helper()

	env.testDir = tfs.NewDir(env.t, "moma-enduro-test")
	env.cfg.Preprocessing.SharedPath = env.testDir.Path()
}

func (env *testEnv) startWorker(ctx context.Context) {
	env.t.Helper()

	m := workercmd.NewMain(defaultLogger(env.t), env.cfg)

	env.t.Cleanup(func() {
		if err := m.Close(); err != nil {
			env.t.Fatal(err)
		}
	})

	done := make(chan error)
	go func() {
		done <- m.Run(ctx)
	}()

	err, ok := <-done
	if ok && err != nil {
		env.t.Fatal(err)
	}
}

func (env *testEnv) copyTestTransfer(name string) {
	env.t.Helper()

	src := filepath.Join("testdata", name)
	dest := env.testDir.Join(name)

	if err := cp.Copy(src, dest); err != nil {
		env.t.Fatalf("Error copying %s to %s", src, dest)
	}

	root, err := os.OpenRoot(dest)
	if err != nil {
		env.t.Fatalf("Error opening %s: %v", dest, err)
	}
	defer root.Close()

	// Explicitly set file modes.
	if err := fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		mode := fileMode
		if d.IsDir() {
			mode = dirMode
		}

		if err := root.Chmod(path, mode); err != nil {
			return err
		}

		return nil
	}); err != nil {
		env.t.Fatalf("Error setting file modes in %s: %v", dest, err)
	}
}

func TestIntegration(t *testing.T) {
	truthy := []string{"1", "t", "true"}
	v := strings.ToLower(os.Getenv("MOMA_ENDURO_INTEGRATION_TEST"))
	if !slices.Contains(truthy, v) {
		t.Skipf(
			"Set MOMA_ENDURO_INTEGRATION_TEST={%s} to run this test.",
			strings.Join(truthy, ","),
		)
	}

	ctx := context.Background()
	temporalServer := setUpTemporal(ctx, t)

	t.Run("Remove .DS_Store files and bag SIP", func(t *testing.T) {
		testTransfer := "small_with_ds_store"

		env := newTestEnv(t, defaultConfig())
		env.cfg.Temporal.Address = temporalServer.addr
		env.copyTestTransfer(testTransfer)
		env.startWorker(ctx)

		run, err := temporalServer.client.ExecuteWorkflow(
			ctx,
			temporalsdk_client.StartWorkflowOptions{
				TaskQueue:                env.cfg.Worker.TaskQueue,
				WorkflowExecutionTimeout: 30 * time.Second,
			},
			workflows.NewPreprocessingWorkflow(env.cfg.Preprocessing).Execute,
			&childwf.PreprocessingParams{
				RelativePath: testTransfer,
			},
		)
		assert.NilError(t, err, "Workflow could not be started.")

		var result childwf.PreprocessingResult
		err = run.Get(ctx, &result)
		assert.NilError(t, err)

		assert.DeepEqual(
			t,
			result,
			childwf.PreprocessingResult{
				Outcome:      childwf.OutcomeSuccess,
				RelativePath: testTransfer,
				Tasks: []*childwf.Task{
					{
						Name:    "Remove unwanted files",
						Message: "Unwanted files removed: 1",
						Outcome: childwf.TaskOutcomeSuccess,
					},
					{
						Name:    "Bag SIP",
						Message: "SIP has been bagged",
						Outcome: childwf.TaskOutcomeSuccess,
					},
				},
			},
			cmpopts.IgnoreFields(childwf.Task{}, "StartedAt", "CompletedAt"),
		)
		assert.Assert(t, tfs.Equal(
			env.testDir.Path(),
			tfs.Expected(t,
				tfs.WithDir(testTransfer, tfs.WithMode(dirMode),
					tfs.WithFile("bag-info.txt", "", tfs.MatchAnyFileContent, tfs.WithMode(fileMode)),
					tfs.WithFile("bagit.txt", "", tfs.MatchAnyFileContent, tfs.WithMode(fileMode)),
					tfs.WithFile("manifest-sha512.txt", smallFileSHA512Manifest, tfs.WithMode(fileMode)),
					tfs.WithFile("tagmanifest-sha512.txt", "", tfs.MatchAnyFileContent, tfs.WithMode(fileMode)),
					tfs.WithDir("data", tfs.WithMode(dirMode),
						tfs.WithFile(
							"small.txt", "I am a small file.\n", tfs.WithMode(fileMode),
						),
					),
				),
			),
		))
	})
}
