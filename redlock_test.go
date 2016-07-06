package redlock

import (
	"fmt"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func GetTestConfig(t *testing.T) *RedisConfig {
	viper.AutomaticEnv()
	host, ok := viper.Get("redlock_test_host").(string)
	if !ok {
		t.Fatalf("REDLOCK_TEST_HOST envvar not set. Please ensure the env var REDLOCK_TEST_HOST is set, and points to an accessible redis host:port.")
	}
	return &RedisConfig{
		Address:        host,
		Database:       0,
		ConnectTimeout: time.Second * time.Duration(5),
	}
}

func channelLock(key string, timeout time.Duration, id int, c chan bool, t *testing.T) {
	testRl := New(GetTestConfig(t))
	defer testRl.Close()
	lock, err := testRl.Lock(key, timeout)
	if err != nil {
		fmt.Println(err)
	}
	c <- lock
}

func TestLock(t *testing.T) {
	var (
		lock        bool
		renewed     bool
		err         error
		testTimeout = time.Duration(50) * time.Millisecond
		testKey     = "test"
	)

	rl := New(GetTestConfig(t))
	defer rl.Close()

	start := time.Now()
	lock, err = rl.Lock(testKey, testTimeout)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if !lock {
		t.Fatalf("Failed to acquire lock.")
	}

	renewed, err = rl.Renew(testKey, testTimeout)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if !renewed {
		t.Fatalf("Failed to renew lock.")
	}

	if time.Since(start) > testTimeout {
		t.Fatalf("Aborting tests due to slow redis test instance")
	}
	lock, err = rl.Lock(testKey, testTimeout)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if lock {
		t.Fatalf("Acquired lock already held.")
	}
	err = rl.Unlock(testKey)
	if err != nil {
		t.Fatalf(err.Error())
	}
	lock, err = rl.Lock(testKey, testTimeout)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if !lock {
		t.Fatalf("Lock couldn't be acquired after being released.")
	}
	for time.Since(start) < testTimeout {
		time.Sleep(testTimeout - time.Since(start))
	}
	// set up a contention using goroutines
	c := make(chan bool)
	contenders := 2
	for i := 0; i < contenders; i++ {
		go channelLock(testKey, testTimeout, i, c, t)
	}
	lockedCount := 0
	for i := 0; i < contenders; i++ {
		locked := <-c
		if locked {
			lockedCount += 1
		}
	}
	if lockedCount == 0 {
		t.Fatalf("Nobody acquired expired lock.")
	}
	if lockedCount > 1 {
		t.Fatalf("Multiple contenders (%d) acquired expired lock.", lockedCount)
	}

}
