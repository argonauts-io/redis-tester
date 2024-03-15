package test_cases

import (
	"bytes"
	"fmt"

	resp_client "github.com/codecrafters-io/redis-tester/internal/resp/connection"
	"github.com/codecrafters-io/redis-tester/internal/resp_assertions"
	logger "github.com/codecrafters-io/tester-utils/logger"
	rdb_parser "github.com/hdt3213/rdb/parser"
)

// SendReplicationHandshakeTestCase is a test case where we connect to a master
// as a replica and perform either all or a subset of the replication handshake.
//
// RunAll will run all the steps in the replication handshake. Alternatively, you
// can run each step individually.
type SendReplicationHandshakeTestCase struct{}

func (t SendReplicationHandshakeTestCase) RunAll(client *resp_client.RespConnection, logger *logger.Logger) error {
	if err := t.RunPingStep(client, logger); err != nil {
		return err
	}

	if err := t.RunReplconfStep(client, logger); err != nil {
		return err
	}

	if err := t.RunPsyncStep(client, logger); err != nil {
		return err
	}

	if err := t.RunReceiveRDBStep(client, logger); err != nil {
		return err
	}

	return nil
}

func (t SendReplicationHandshakeTestCase) RunPingStep(client *resp_client.RespConnection, logger *logger.Logger) error {
	commandTest := SendCommandAndReceiveValueTestCase{
		Command:   "PING",
		Args:      []string{},
		Assertion: resp_assertions.NewStringAssertion("PONG"),
	}

	return commandTest.Run(client, logger)
}

func (t SendReplicationHandshakeTestCase) RunReplconfStep(client *resp_client.RespConnection, logger *logger.Logger) error {
	commandTest := SendCommandAndReceiveValueTestCase{
		Command:   "REPLCONF",
		Args:      []string{"listening-port", "6380"},
		Assertion: resp_assertions.NewStringAssertion("OK"),
	}

	if err := commandTest.Run(client, logger); err != nil {
		return err
	}

	commandTest = SendCommandAndReceiveValueTestCase{
		Command:   "REPLCONF",
		Args:      []string{"capa", "psync2"},
		Assertion: resp_assertions.NewStringAssertion("OK"),
	}

	return commandTest.Run(client, logger)
}

func (t SendReplicationHandshakeTestCase) RunPsyncStep(client *resp_client.RespConnection, logger *logger.Logger) error {
	commandTest := SendCommandAndReceiveValueTestCase{
		Command:                   "PSYNC",
		Args:                      []string{"?", "-1"},
		Assertion:                 resp_assertions.NewRegexStringAssertion("FULLRESYNC \\w+ 0"),
		ShouldSkipUnreadDataCheck: true, // We're expecting the RDB file to be sent next
	}

	return commandTest.Run(client, logger)
}

func (t SendReplicationHandshakeTestCase) RunReceiveRDBStep(client *resp_client.RespConnection, logger *logger.Logger) error {
	logger.Debugln("Reading RDB file...")

	rdbFileBytes, err := client.ReadFullResyncRDBFile()
	if err != nil {
		return err
	}

	// We don't care about the contents of the RDB file, we just want to make sure the file was valid
	processRedisObject := func(_ rdb_parser.RedisObject) bool {
		return true
	}

	decoder := rdb_parser.NewDecoder(bytes.NewReader(rdbFileBytes))
	if err = decoder.Parse(processRedisObject); err != nil {
		return fmt.Errorf("Invalid RDB file: %v", err)
	}

	client.ReadIntoBuffer() // Let's make sure there's no extra data

	if client.UnreadBuffer.Len() > 0 {
		return fmt.Errorf("Found extra data: %q", string(client.LastValueBytes)+client.UnreadBuffer.String())
	}

	logger.Successf("Received RDB file.")
	return nil
}
