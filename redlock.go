/*
Package redlock implements a simple mutex on a single redis instance, using a connection pool for
performance and efficiency.

Note that this design does not guarantee correctness. It is lightweight and appropriate for use cases where two
clients holding the same lock simultaneously is not a critical issue. It is possible, in cases of processes running
overlong, or the redis node's failure, for more than one process to hold a given lock simultaneously. This should
happen only very rarely, but it can happen.
*/
package redlock

import (
	"errors"
	"time"

	"github.com/garyburd/redigo/redis"
)

type RedisConfig struct {
	Address        string // address:port
	Database       int    // database number to connect to
	KeyPrefix      string // optional prefix applied to keys
	ConnectTimeout time.Duration
}

type Redlock struct {
	p    *redis.Pool
	conf *RedisConfig
}

var ConfigNotSetError string = "No redis config set."

// New returns a redlock instance. It is the consumer's responsibility to close the Redlock's
// connection pool via Close() if possible.
func New(conf *RedisConfig) (rl *Redlock) {
	return &Redlock{
		conf: conf,
	}
}

func (rl *Redlock) pool() (pool *redis.Pool, err error) {
	if rl.conf == nil {
		err = errors.New(ConfigNotSetError)
		return
	}

	pool = &redis.Pool{
		MaxIdle: 3,
		Dial: func() (redis.Conn, error) {
			dbOption := redis.DialDatabase(rl.conf.Database)
			timeout := redis.DialConnectTimeout(rl.conf.ConnectTimeout)
			c, err := redis.Dial("tcp", rl.conf.Address, dbOption, timeout)
			if err != nil {
				return nil, err
			}
			return c, err
		},
	}
	return
}

func (rl *Redlock) conn() (connection redis.Conn, err error) {
	if rl.p == nil {
		rl.p, err = rl.pool()
		if err != nil {
			return nil, err
		}
	}
	return rl.p.Get(), err
}

// Close the Redlock's redis pool.
func (rl *Redlock) Close() (err error) {
	if rl.p != nil {
		return (*rl.p).Close()
	}
	return
}
