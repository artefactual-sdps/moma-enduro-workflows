package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/artefactual-sdps/temporal-activities/bagcreate"
	"github.com/spf13/viper"
)

type ConfigurationValidator interface {
	Validate() error
}

type Configuration struct {
	// Debug toggles human readable logs or JSON logs (default).
	Debug bool

	// Verbosity sets the verbosity level of log messages, with 0 (default)
	// logging only critical messages and each higher number increasing the
	// number of messages logged.
	Verbosity int

	// Temporal configures the Temporal client.
	Temporal TemporalConfig

	// Worker configures the Temporal worker.
	Worker WorkerConfig

	// Preprocessing configures the preprocessing workflow.
	Preprocessing PreprocessingConfig
}

type TemporalConfig struct {
	// Address is the Temporal server host and port (required).
	Address string

	// Namespace is the Temporal namespace the preprocessing worker should run
	// in (default: "default").
	Namespace string
}

type WorkerConfig struct {
	// MaxConcurrentSessions limits the number of workflow sessions the
	// custom worker can handle simultaneously (default: 1).
	MaxConcurrentSessions int

	// TaskQueue is the Temporal task queue from which the preprocessing worker
	// will pull tasks (required).
	TaskQueue string
}

type PreprocessingConfig struct {
	// WorkflowName is the name of the preprocessing Temporal workflow
	// (required).
	WorkflowName string

	// SharedPath is a directory path that both the custom worker and the Enduro
	// workers can access. This is required for preprocessing workers to share
	// the SIP.
	SharedPath string

	// BagCreate configures the bagcreate activity used in the preprocessing
	// workflow.
	BagCreate bagcreate.Config
}

func (c Configuration) Validate() error {
	return errors.Join(
		c.Temporal.Validate(),
		c.Worker.Validate(),
		c.Preprocessing.Validate(),
	)
}

func (c TemporalConfig) Validate() error {
	var errs error

	if c.Address == "" {
		errs = errors.Join(errs, errRequired("Temporal.Address"))
	}
	if c.Namespace == "" {
		errs = errors.Join(errs, errRequired("Temporal.Namespace"))
	}

	return errs
}

func (c WorkerConfig) Validate() error {
	var errs error

	if c.TaskQueue == "" {
		errs = errors.Join(errs, errRequired("Worker.TaskQueue"))
	}

	// Verify that MaxConcurrentSessions is >= 1.
	if c.MaxConcurrentSessions < 1 {
		errs = errors.Join(errs, fmt.Errorf(
			"Worker.MaxConcurrentSessions: %d is less than the minimum value (1)",
			c.MaxConcurrentSessions,
		))
	}

	return errs
}

func (c PreprocessingConfig) Validate() error {
	var errs error

	if c.SharedPath == "" {
		errs = errors.Join(errs, errRequired("Preprocessing.SharedPath"))
	}
	if c.WorkflowName == "" {
		errs = errors.Join(errs, errRequired("Preprocessing.WorkflowName"))
	}

	if err := c.BagCreate.Validate(); err != nil {
		errs = errors.Join(errs, fmt.Errorf("Preprocessing.BagCreate: %v", err))
	}

	return errs
}

func Read(config *Configuration, configFile string) (found bool, configFileUsed string, err error) {
	v := viper.New()

	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.config/")
	v.AddConfigPath("/etc")
	v.SetConfigName("moma-enduro-worker")
	v.SetEnvPrefix("MOMA_ENDURO_WORKER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Defaults.
	v.SetDefault("Temporal.Namespace", "default")
	v.SetDefault("Worker.MaxConcurrentSessions", 1)
	v.SetDefault("Preprocessing.BagCreate.ChecksumAlgorithm", "sha512")

	if configFile != "" {
		// Viper will not return a viper.ConfigFileNotFoundError error when
		// SetConfigFile() is passed a path to a file that doesn't exist, so we
		// need to check ourselves.
		if _, err := os.Stat(configFile); errors.Is(err, os.ErrNotExist) {
			return false, "", fmt.Errorf("configuration file not found: %s", configFile)
		}

		v.SetConfigFile(configFile)
	}

	if err = v.ReadInConfig(); err != nil {
		switch err.(type) {
		case viper.ConfigFileNotFoundError:
			return false, "", err
		default:
			return true, "", fmt.Errorf("failed to read configuration file: %w", err)
		}
	}

	err = v.Unmarshal(config)
	if err != nil {
		return true, "", fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if err := config.Validate(); err != nil {
		return true, "", errors.Join(errors.New("invalid configuration"), err)
	}

	return true, v.ConfigFileUsed(), nil
}

func errRequired(name string) error {
	return fmt.Errorf("%s: missing required value", name)
}
