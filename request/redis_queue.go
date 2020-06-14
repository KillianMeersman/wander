package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v7"
)

type RedisQueue struct {
	client    *redis.Client
	key       string
	isDone    bool
	waitGroup *sync.WaitGroup
}

func NewRedisQueue(host string, port int, password, key string, db int) (*RedisQueue, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       db,
	})

	_, err := client.Ping().Result()
	if err != nil {
		return nil, err
	}

	return &RedisQueue{
		client:    client,
		key:       key,
		isDone:    false,
		waitGroup: &sync.WaitGroup{},
	}, nil
}

func (r *RedisQueue) Enqueue(req *Request, priority int) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	res := r.client.ZAdd(r.key, &redis.Z{
		Score:  float64(priority),
		Member: data,
	})
	return res.Err()
}

func (r *RedisQueue) Dequeue() <-chan QueueResult {
	outlet := make(chan QueueResult)
	go func() {
		r.waitGroup.Add(1)

		var zWithKey *redis.ZWithKey
		var err error
		for zWithKey == nil && !r.isDone {
			zKeyCommand := r.client.BZPopMax(5*time.Second, r.key)
			zWithKey, err = zKeyCommand.Result()
		}

		if !r.isDone {
			if err != nil {
				outlet <- QueueResult{
					Error: err,
				}
				return
			}

			data, ok := zWithKey.Member.(string)
			if !ok {
				outlet <- QueueResult{
					Error: errors.New("Cannot convert Redis item to bytes"),
				}
				return
			}

			var req Request
			err := json.Unmarshal([]byte(data), &req)
			if err != nil {
				outlet <- QueueResult{
					Error: errors.New("Cannot convert Redis item to bytes"),
				}
				return
			}

			outlet <- QueueResult{
				Request: &req,
			}
		}

		r.waitGroup.Done()
	}()
	return outlet
}

func (r *RedisQueue) Close() error {
	r.isDone = true
	r.waitGroup.Wait()
	return nil
}

func (r *RedisQueue) Clear() {
	r.client.Del(r.key)
}

func (r *RedisQueue) Count() (int, error) {
	res := r.client.ZCount(r.key, "0", "999999999999")
	return int(res.Val()), res.Err()
}
