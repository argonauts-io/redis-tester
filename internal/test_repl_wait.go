package internal

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/codecrafters-io/redis-tester/internal/instrumented_resp_connection"
	"github.com/codecrafters-io/redis-tester/internal/redis_executable"
	resp_utils "github.com/codecrafters-io/redis-tester/internal/resp"
	resp_connection "github.com/codecrafters-io/redis-tester/internal/resp/connection"
	"github.com/codecrafters-io/redis-tester/internal/resp_assertions"
	"github.com/codecrafters-io/redis-tester/internal/test_cases"

	"github.com/codecrafters-io/tester-utils/logger"
	testerutils_random "github.com/codecrafters-io/tester-utils/random"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

type WaitTest struct {
	// WriteCommand is the command we'll issue to the master
	WriteCommand []string

	// WaitReplicaCount is the number of replicas we'll specify in the WAIT command
	WaitReplicaCount int

	// WaitTimeoutMilli is the timeout we'll specify in the WAIT command
	WaitTimeoutMilli int

	// ActualNumberOfAcks is the number of ACKs we'll send back to the master
	ActualNumberOfAcks int

	// ShouldVerifyTimeout is a flag to verify if the WAIT command returned after the timeout
	ShouldVerifyTimeout bool

	// Logger is the logger to use for this test
	Logger *logger.Logger
}

// In this stage, we:
//  1. Boot the user's code as a Redis master.
//  2. Spawn multiple replicas and have each perform a handshake with the master.
//  3. Connect to Master, and execute RunWaitTest
//  4. RunWaitTest :
//     4.1. Issue a write command to the master
//     4.2. Issue a WAIT command with WaitReplicaCount as the expected number of replicas
//     4.3. Read propagated command on replicas + respond to subset of GETACKs
//     4.4. Assert response of WAIT command is ActualNumberOfAcks
//     4.5. Assert that the WAIT command returned after the timeout
func testWait(stageHarness *test_case_harness.TestCaseHarness) error {
	deleteRDBfile()

	// Step 1: Boot the user's code as a Redis master.
	master := redis_executable.NewRedisExecutable(stageHarness)
	if err := master.Run([]string{
		"--port", "6379",
	}...); err != nil {
		return err
	}

	logger := stageHarness.Logger

	// Step 2: Spawn multiple replicas and have each perform a handshake
	replicaCount := testerutils_random.RandomInt(3, 5)
	logger.Infof("Proceeding to create %v replicas.", replicaCount)

	replicas, err := SpawnReplicas(replicaCount, stageHarness, logger, "localhost:6379")
	if err != nil {
		return err
	}
	for _, replica := range replicas {
		defer replica.Close()
	}

	// Step 3: Connect to master
	client, err := instrumented_resp_connection.NewFromAddr(stageHarness, "localhost:6379", "client")
	if err != nil {
		logFriendlyError(logger, err)
		return err
	}
	defer client.Close()

	waitCommandReplicaSubsetCount := 1
	acksSendByReplicaSubsetCount := 1

	if err = RunWaitTest(client, replicas, WaitTest{
		WriteCommand:        []string{"SET", "foo", "123"},
		WaitReplicaCount:    waitCommandReplicaSubsetCount,
		ActualNumberOfAcks:  acksSendByReplicaSubsetCount,
		WaitTimeoutMilli:    500,
		ShouldVerifyTimeout: false,
		Logger:              logger,
	}); err != nil {
		return err
	}

	logger.Successf("Passed first WAIT test.")

	waitCommandReplicaSubsetCount = testerutils_random.RandomInt(2, replicaCount) + 1
	acksSendByReplicaSubsetCount = waitCommandReplicaSubsetCount - 1
	if err = RunWaitTest(client, replicas, WaitTest{
		WriteCommand:        []string{"SET", "baz", "789"},
		WaitReplicaCount:    waitCommandReplicaSubsetCount,
		ActualNumberOfAcks:  acksSendByReplicaSubsetCount,
		WaitTimeoutMilli:    2000,
		ShouldVerifyTimeout: true,
		Logger:              logger,
	}); err != nil {
		return err
	}

	return nil
}

func consumeReplicationStreamAndSendAcks(replicas []*resp_connection.RespConnection, logger *logger.Logger, acksSendByReplicaSubsetCount int, command []string) error {
	var err error
	for j := 0; j < len(replicas); j++ {
		replica := replicas[j]
		logger.Infof("Testing Replica : %v", j+1)
		receiveCommandTestCase := &test_cases.ReceiveValueTestCase{
			Assertion:                 resp_assertions.NewCommandAssertion(command[0], command[1:]...),
			ShouldSkipUnreadDataCheck: true,
		}

		err := receiveCommandTestCase.Run(replica, logger)

		if err != nil {
			// Redis sends a SELECT command, but we don't expect it from users.
			// If the first command is a SELECT command, we'll re-run the test case to test the next command instead
			if resp_utils.IsSelectCommand(receiveCommandTestCase.ActualValue) {
				err := receiveCommandTestCase.Run(replica, logger)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}

		receiveGetackCommandTestCase := &test_cases.ReceiveValueTestCase{
			Assertion:                 resp_assertions.NewCommandAssertion("REPLCONF", "GETACK", "*"),
			ShouldSkipUnreadDataCheck: false,
		}
		if err = receiveGetackCommandTestCase.Run(replica, logger); err != nil {
			return err
		}

		if j < acksSendByReplicaSubsetCount {
			// Remove GETACK command bytes from offset before sending ACK.
			if err := replica.SendCommand("REPLCONF", []string{"ACK", strconv.Itoa(replica.ReceivedBytes - len(replica.LastValueBytes))}...); err != nil {
				return err
			}
		}
	}
	return err
}

func RunWaitTest(client *resp_connection.RespConnection, replicas []*resp_connection.RespConnection, waitTest WaitTest) (err error) {
	// Step 1: Issue a write command
	setCommandTestCase := test_cases.SendCommandAndReceiveValueTestCase{
		Command:   waitTest.WriteCommand[0],
		Args:      waitTest.WriteCommand[1:],
		Assertion: resp_assertions.NewStringAssertion("OK"),
	}
	if err := setCommandTestCase.Run(client, waitTest.Logger); err != nil {
		return err
	}

	// Step 2: Issue a WAIT command with a subset as the expected number of replicas
	startTimeMilli := time.Now().UnixMilli()
	if err := client.SendCommand("WAIT", []string{strconv.Itoa(waitTest.WaitReplicaCount), strconv.Itoa(waitTest.WaitTimeoutMilli)}...); err != nil {
		return err
	}

	// Step 3: Read propagated command on replicas + respond to subset of GETACKs
	// We then assert that across all the replicas we receive the SET commands in order
	err = consumeReplicationStreamAndSendAcks(replicas, waitTest.Logger, waitTest.ActualNumberOfAcks, waitTest.WriteCommand)
	if err != nil {
		return err
	}

	// Step 4: Assert response of WAIT command is replicaAcksCount
	value, err := client.ReadValueWithTimeout(4 * time.Second)
	if err != nil {
		return err
	}

	if err := resp_assertions.NewIntegerAssertion(waitTest.ActualNumberOfAcks).Run(value); err != nil {
		return err
	}

	endTimeMilli := time.Now().UnixMilli()

	// Step 5: If shouldVerifyTimeout is true : Assert that the WAIT command returned after the timeout
	if waitTest.ShouldVerifyTimeout {
		threshold := 500 // ms
		elapsedTimeMilli := endTimeMilli - startTimeMilli
		waitTest.Logger.Infof(fmt.Sprintf("WAIT command returned after %v ms", elapsedTimeMilli))
		if math.Abs(float64(elapsedTimeMilli-int64(waitTest.WaitTimeoutMilli))) > float64(threshold) {
			return fmt.Errorf("Expected WAIT to return exactly after %v ms timeout elapsed.", 1000)
		}
	}

	return nil
}
