package redis_executable

import (
	"strings"

	executable "github.com/codecrafters-io/tester-utils/executable"
	logger "github.com/codecrafters-io/tester-utils/logger"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

type RedisExecutable struct {
	executable *executable.Executable
	logger     *logger.Logger
	args       []string
}

func NewRedisExecutable(stageHarness *test_case_harness.TestCaseHarness) *RedisExecutable {
	b := &RedisExecutable{
		executable: stageHarness.NewExecutable(),
		logger:     stageHarness.Logger,
	}

	stageHarness.RegisterTeardownFunc(func() { b.Kill() })

	return b
}

func (b *RedisExecutable) Run(args []string) error {
	b.args = args
	if b.args == nil || len(b.args) == 0 {
		b.logger.Infof("$ ./spawn_redis_server.sh")
	} else {
		b.logger.Infof("$ ./spawn_redis_server.sh %s", strings.Join(b.args, " "))
	}

	if err := b.executable.Start(b.args...); err != nil {
		return err
	}

	return nil
}

func (b *RedisExecutable) HasExited() bool {
	return b.executable.HasExited()
}

func (b *RedisExecutable) Kill() error {
	b.logger.Debugf("Terminating program")
	if err := b.executable.Kill(); err != nil {
		b.logger.Debugf("Error terminating program: '%v'", err)
		return err
	}

	b.logger.Debugf("Program terminated successfully")
	return nil // When does this happen?
}
