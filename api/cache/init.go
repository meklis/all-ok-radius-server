package cache

import (
	"github.com/meklis/all-ok-radius-server/prom"
	"github.com/meklis/all-ok-radius-server/radius/events"
	"github.com/meklis/go-cache"
	"time"
)

type CacheApi struct {
	responses *cache.Cache
}

func Init(expireTimeout time.Duration) *CacheApi {
	c := new(CacheApi)
	c.responses = cache.New(expireTimeout, 10*time.Minute)
	go func() {
		for {
			prom.SetCacheSize(c.responses.ItemCount())
			time.Sleep(time.Second * 3)
		}
	}()
	return c
}

func (c *CacheApi) Get(hash string) (events.Response, bool) {
	if resp, exist := c.responses.Get(hash); exist {
		return resp.(events.Response), true
	} else {
		return events.Response{}, false
	}
}

func (c *CacheApi) Set(hash string, resp events.Response) *CacheApi {
	c.responses.SetDefault(hash, resp)
	return c
}
