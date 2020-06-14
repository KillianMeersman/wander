package request

import (
	"fmt"

	"github.com/go-redis/redis/v7"
)

type RedisCache struct {
	client *redis.Client
	key    string
}

func NewRedisCache(host string, port int, password, key string, db int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", host, port),
		Password: password,
		DB:       db,
	})

	_, err := client.Ping().Result()
	if err != nil {
		return nil, err
	}

	return &RedisCache{
		client: client,
		key:    key,
	}, nil
}

func (r *RedisCache) AddRequest(req *Request) error {
	res := r.client.HSet(r.key, req.URL.String(), "t")
	return res.Err()
}

func (r *RedisCache) VisitedURL(req *Request) (bool, error) {
	res := r.client.HGet(r.key, req.URL.String())
	val, err := res.Result()
	if err != nil && err.Error() == "redis: nil" {
		err = nil
	}
	return val == "t", err
}

func (r *RedisCache) Clear() error {
	res := r.client.Del(r.key)
	return res.Err()
}
