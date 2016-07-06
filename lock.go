package redlock

import (
	"fmt"
	"math"
	"time"

	"github.com/garyburd/redigo/redis"
)

func prefixedKey(key string, prefix string) string {
	if prefix == "" {
		return key
	}
	return fmt.Sprintf("%s:%s", prefix, key)
}

// Lock tries to acquire a lock on the given connection for the key via SETNX as discussed at http://redis.io/commands/setnx, returning true if successful.
// Timeout is given in milliseconds.
func (rl *Redlock) Lock(key string, timeout time.Duration) (acquired bool, err error) {
	conn, err := rl.conn()
	if err != nil {
		return false, err
	}
	lockKey := prefixedKey(rl.conf.KeyPrefix, key)
	acqValue, err := redis.Int(conn.Do("SETNX", lockKey, time.Now().Add(timeout).UnixNano()))
	if err != nil {
		return
	}

	if acqValue == 1 {
		acquired = true
	} else {
		var expires int64
		expires, err = redis.Int64(conn.Do("GET", lockKey))
		expireTime := time.Unix(0, expires)
		if err != nil {
			acquired = false
		} else if expireTime.Before(time.Now()) { // try to reset the time
			newExpires := time.Now().Add(timeout).UnixNano()
			var newTime int64
			newTime, err = redis.Int64(conn.Do("GETSET", lockKey, newExpires))
			if err != nil {
				return
			} else if newTime == expires { // we set it
				acquired = true
			}
		}
	}

	if acquired {
		expire := int(math.Ceil(timeout.Seconds()))
		_, err = redis.Int(conn.Do("EXPIRE", lockKey, expire))
	}
	return
}

// WaitLock locks the given key for timeout duration, retrying the lock until it succeeds
// waiting retryInterval duration between retries.
func (rl *Redlock) WaitLock(key string, timeout time.Duration, retryInterval time.Duration) (acquired bool, err error) {
	acquired, err = rl.Lock(key, timeout)
	for !acquired {
		if err != nil {
			return false, err
		}
		time.Sleep(retryInterval)
		acquired, err = rl.Lock(key, timeout)
	}
	return
}

// Unlock deletes the given lock key, releasing the lock.
func (rl *Redlock) Unlock(key string) (err error) {
	conn, err := rl.conn()
	if err != nil {
		return
	}
	_, err = conn.Do("DEL", prefixedKey(rl.conf.KeyPrefix, key))
	return err
}

// Extend the time on a given lock - only to be called by lock's current holder or delegate
func (rl *Redlock) Renew(key string, timeout time.Duration) (renewed bool, err error) {
	conn, err := rl.conn()
	if err != nil {
		return false, err
	}

	lockKey := prefixedKey(rl.conf.KeyPrefix, key)
	newExpires := time.Now().Add(timeout).UnixNano()
	_, err = redis.Int64(conn.Do("GETSET", lockKey, newExpires))
	if err != nil {
		return
	} else {
		renewed = true
	}

	if renewed {
		expire := int(math.Ceil(timeout.Seconds()))
		_, err = redis.Int(conn.Do("EXPIRE", lockKey, expire))
	}
	return
}
