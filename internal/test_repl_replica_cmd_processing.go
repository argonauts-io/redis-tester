package internal

import (
	"fmt"
	"github.com/codecrafters-io/redis-tester/internal/instrumented_resp_connection"
	"github.com/codecrafters-io/redis-tester/internal/redis_executable"
	"github.com/codecrafters-io/redis-tester/internal/resp_assertions"
	"github.com/codecrafters-io/redis-tester/internal/test_cases"
	"net"

	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testReplCmdProcessing(stageHarness *test_case_harness.TestCaseHarness) error {
	deleteRDBfile()

	logger := stageHarness.Logger

	listener, err := net.Listen("tcp", ":6379")
	if err != nil {
		logFriendlyBindError(logger, err)
		return fmt.Errorf("Error starting TCP server: %v", err)
	}
	defer listener.Close()

	logger.Infof("Master is running on port 6379")

	replica := redis_executable.NewRedisExecutable(stageHarness)
	if err := replica.Run([]string{
		"--port", "6380",
		"--replicaof", "localhost", "6379",
	}...); err != nil {
		return err
	}

	conn, err := listener.Accept()
	if err != nil {
		fmt.Println("Error accepting: ", err.Error())
		return err
	}
	defer conn.Close()

	master, err := instrumented_resp_connection.NewFromConn(stageHarness, conn, "master")
	if err != nil {
		logFriendlyError(logger, err)
		return err
	}

	receiveReplicationHandshakeTestCase := test_cases.ReceiveReplicationHandshakeTestCase{}

	if err := receiveReplicationHandshakeTestCase.RunAll(master, logger); err != nil {
		return err
	}

	replicaClient, err := instrumented_resp_connection.NewFromAddr(stageHarness, "localhost:6380", "replica")
	if err != nil {
		logFriendlyError(logger, err)
		return err
	}
	defer replicaClient.Close()

	kvMap := map[int][]string{
		1: {"foo", "123"},
		2: {"bar", "456"},
		3: {"baz", "789"},
	}

	for i := 1; i <= len(kvMap); i++ { // We need order of commands preserved
		key, value := kvMap[i][0], kvMap[i][1]
		// We are propagating commands to Replica as Master, don't expect any response back.
		if err := master.SendCommand("SET", []string{key, value}...); err != nil {
			return err
		}
	}

	for i := 1; i <= len(kvMap); i++ {
		key, value := kvMap[i][0], kvMap[i][1]
		logger.Infof("Getting key %s", key)
		getCommandTestCase := test_cases.SendCommandAndReceiveValueWithRetriesTestCase{
			Command:   "GET",
			Args:      []string{key},
			Assertion: resp_assertions.NewStringAssertion(value),
			Retries:   5,
		}

		if err := getCommandTestCase.Run(replicaClient, logger); err != nil {
			return err
		}
	}

	return nil
}
