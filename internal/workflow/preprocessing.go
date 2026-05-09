package workflow

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/artefactual-sdps/enduro/pkg/childwf"
	"github.com/artefactual-sdps/temporal-activities/bagcreate"
	"github.com/artefactual-sdps/temporal-activities/removefiles"
	"go.artefactual.dev/tools/temporal"
	temporalsdk_temporal "go.temporal.io/sdk/temporal"
	temporalsdk_workflow "go.temporal.io/sdk/workflow"
)

type PreprocessingWorkflow struct {
	sharedPath string
}

func NewPreprocessingWorkflow(sharedPath string) *PreprocessingWorkflow {
	return &PreprocessingWorkflow{sharedPath: sharedPath}
}

func (w *PreprocessingWorkflow) Execute(
	ctx temporalsdk_workflow.Context,
	params *childwf.PreprocessingParams,
) (*childwf.PreprocessingResult, error) {
	result := &childwf.PreprocessingResult{}
	logger := temporalsdk_workflow.GetLogger(ctx)
	logger.Debug("PreprocessingWorkflow workflow running!", "params", params)

	if params == nil || params.RelativePath == "" {
		return nil, temporal.NewNonRetryableError(fmt.Errorf("error calling workflow with unexpected inputs"))
	}
	result.RelativePath = params.RelativePath

	localPath := filepath.Join(w.sharedPath, filepath.Clean(params.RelativePath))

	// Remove unwanted files.
	task := result.NewTask(temporalsdk_workflow.Now(ctx), "Remove unwanted files")
	var removeFilesResult removefiles.Result
	err := temporalsdk_workflow.ExecuteActivity(
		withLocalActOpts(ctx),
		removefiles.Name,
		&removefiles.Params{
			Path:        localPath,
			RemoveNames: []string{".DS_Store"},
		},
	).Get(ctx, &removeFilesResult)
	if err != nil {
		logger.Error("System error", "message", err.Error())
		result.SystemError(temporalsdk_workflow.Now(ctx), task, "removing unwanted files has failed")
		return result, nil
	}
	task.Succeed(
		temporalsdk_workflow.Now(ctx),
		"Unwanted files removed: %d",
		removeFilesResult.Count,
	)

	// Bag the SIP for Enduro processing.
	task = result.NewTask(temporalsdk_workflow.Now(ctx), "Bag SIP")
	var createBag bagcreate.Result
	err = temporalsdk_workflow.ExecuteActivity(
		withLocalActOpts(ctx),
		bagcreate.Name,
		&bagcreate.Params{
			SourcePath: filepath.Join(w.sharedPath, params.RelativePath),
		},
	).Get(ctx, &createBag)
	if err != nil {
		logger.Error("System error", "message", err.Error())
		result.SystemError(temporalsdk_workflow.Now(ctx), task, "bagging has failed")
		return result, nil
	}
	task.Succeed(temporalsdk_workflow.Now(ctx), "SIP has been bagged")

	return result, nil
}

func withLocalActOpts(ctx temporalsdk_workflow.Context) temporalsdk_workflow.Context {
	return temporalsdk_workflow.WithActivityOptions(
		ctx,
		temporalsdk_workflow.ActivityOptions{
			ScheduleToCloseTimeout: 5 * time.Minute,
			RetryPolicy: &temporalsdk_temporal.RetryPolicy{
				MaximumAttempts: 1,
			},
		},
	)
}
