package internal

import (
	"fmt"
	"math/rand"
	"time"

	testerutils "github.com/codecrafters-io/tester-utils"
	"github.com/go-redis/redis"
)

// Tests Expiry
func testExpiry(stageHarness testerutils.StageHarness) error {
	b := NewRedisBinary(stageHarness.Executable, stageHarness.Logger)
	if err := b.Run(); err != nil {
		return err
	}
	defer b.Kill()

	logger := stageHarness.Logger

	client := redis.NewClient(&redis.Options{
		Addr:        "localhost:6379",
		DialTimeout: 30 * time.Second,
	})

	strings := [10]string{
		"abcd",
		"defg",
		"heya",
		"heya",
		"heya",
		"heya",
		"heya",
		"heya",
		"heya",
		"heya",
	}

	randomKey := strings[rand.Intn(10)]
	randomValue := strings[rand.Intn(10)]

	logger.Debugf("Setting key %s to %s, with expiry of 100ms", randomKey, randomValue)
	resp, err := client.Set(randomKey, randomValue, 100*time.Millisecond).Result()
	if err != nil {
		return err
	}
	if resp != "OK" {
		return fmt.Errorf("Expected 'OK', got %s", resp)
	}

	logger.Debugf("Getting key %s", randomKey)
	resp, err = client.Get(randomKey).Result()
	if err != nil {
		return err
	}
	if resp != randomValue {
		return fmt.Errorf("Expected %s, got %s", randomValue, resp)
	}

	logger.Debugf("Sleeping for 101ms")
	time.Sleep(101 * time.Millisecond)

	logger.Debugf("Fetching value for key %s", randomKey)
	resp, err = client.Get(randomKey).Result()
	if err != redis.Nil {
		if err == nil {
			logger.Debugf("Hint: Read about null bulk strings in the Redis protocol docs")
			return fmt.Errorf("Expected null string, got '%v'", resp)
		}

		return err
	}

	client.Close()
	return nil
}
