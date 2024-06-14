package internal

import (
	"fmt"

	"github.com/codecrafters-io/redis-tester/internal/redis_executable"
	resp_value "github.com/codecrafters-io/redis-tester/internal/resp/value"
	"github.com/codecrafters-io/redis-tester/internal/resp_assertions"

	"github.com/codecrafters-io/redis-tester/internal/test_cases"
	"github.com/codecrafters-io/tester-utils/random"
	"github.com/codecrafters-io/tester-utils/test_case_harness"
)

func testTxMultiTx(stageHarness *test_case_harness.TestCaseHarness) error {
	b := redis_executable.NewRedisExecutable(stageHarness)
	if err := b.Run(); err != nil {
		return err
	}

	logger := stageHarness.Logger

	clients, err := SpawnClients(3, "localhost:6379", stageHarness, logger)
	if err != nil {
		return err
	}
	for _, client := range clients {
		defer client.Close()
	}

	uniqueKeys := random.RandomWords(2)
	key1, key2 := uniqueKeys[0], uniqueKeys[1]
	randomIntegerValue := random.RandomInt(1, 100)

	for i, client := range clients {
		multiCommandTestCase := test_cases.MultiCommandTestCase{
			Commands: [][]string{
				{"SET", key2, fmt.Sprint(randomIntegerValue)},
				{"INCR", key1},
			},
			Assertions: []resp_assertions.RESPAssertion{
				resp_assertions.NewStringAssertion("OK"),
				resp_assertions.NewIntegerAssertion(i + 1),
			},
		}

		if err := multiCommandTestCase.RunAll(client, logger); err != nil {
			return err
		}
	}

	for _, client := range clients {
		transactionTestCase := test_cases.TransactionTestCase{
			CommandQueue: [][]string{
				{"INCR", key1},
				{"INCR", key2},
			},
		}
		if err := transactionTestCase.RunWithoutExec(client, logger); err != nil {
			return err
		}
	}

	for i, client := range clients {
		transactionTestCase := test_cases.TransactionTestCase{
			// Before a single transaction is queued,
			// We run 3x INCR key1, and set key2 to randomIntegerValue
			// Inside each transaction, we run 1x INCR key1, key2
			// So it increases by 1 for each transaction
			// `i` here is 0-indexed, so we add 1 to the expected value
			ResultArray: []resp_value.Value{resp_value.NewIntegerValue(3 + (1 + i)), resp_value.NewIntegerValue(randomIntegerValue + (1 + i))},
		}
		if err := transactionTestCase.RunExec(client, logger); err != nil {
			return err
		}
	}

	return nil
}
