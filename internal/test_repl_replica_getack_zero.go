package internal

import (
	"fmt"
	"net"

	"github.com/codecrafters-io/redis-tester/internal/instrumented_resp_connection"
	"github.com/codecrafters-io/redis-tester/internal/redis_executable"
	"github.com/codecrafters-io/redis-tester/internal/test_cases"

	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testReplGetaAckZero(stageHarness *test_case_harness.TestCaseHarness) error {
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

	replicationTestCase := test_cases.ReplicationTestCase{}

	return replicationTestCase.RunGetAck(master, logger, 0)
}
