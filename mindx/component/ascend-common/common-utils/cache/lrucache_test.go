/* Copyright(C) 2023. Huawei Technologies Co.,Ltd. All rights reserved.
   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

// Package cache implement a memory-based LRU local cache
package cache

import (
	"container/list"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/smartystreets/goconvey/convey"
)

const (
	cacheTime      = 500
	goRoutineCount = 10
)

func TestSet(t *testing.T) {
	cache := New(1)
	convey.Convey("test lru cacheTime", t, func() {
		cache.Set("testkey1", "1", cacheTime*time.Millisecond)
		v, err := cache.Get("testkey1")
		convey.So(err, convey.ShouldEqual, nil)
		convey.So(v, convey.ShouldEqual, "1")
		time.Sleep(cacheTime * time.Millisecond)
		v, err = cache.Get("testkey1")
		convey.So(v, convey.ShouldEqual, nil)
		convey.So(err, convey.ShouldNotEqual, nil)
	})
	convey.Convey("test set twice", t, func() {
		cache.Set("testkey1", "1", time.Minute)
		v, err := cache.Get("testkey1")
		convey.So(err, convey.ShouldEqual, nil)
		convey.So(v, convey.ShouldEqual, "1")
		cache.Set("testkey1", "2", time.Minute)
		v, err = cache.Get("testkey1")
		convey.So(err, convey.ShouldEqual, nil)
		convey.So(v, convey.ShouldEqual, "2")
	})
	convey.Convey("SET failed", t, func() {
		c := &lruCache{}
		err := c.setValue("test", "1", time.Minute)
		convey.So(err.Error(), convey.ShouldEqual, "not initializes")
		_, err = c.getValue("test")
		convey.So(err.Error(), convey.ShouldEqual, "not initializes")
	})
	convey.Convey("SET not expired", t, func() {
		cache.Set("testkey2", "1", time.Second)
		err := cache.Set("testkey2", "1", time.Duration(negInt64One))
		convey.So(err, convey.ShouldEqual, nil)
		v, err := cache.Get("testkey2")
		convey.So(err, convey.ShouldEqual, nil)
		convey.So(v, convey.ShouldEqual, "1")
	})
	convey.Convey("SET parameter error", t, func() {
		err := cache.Set("testkey2", "1", -time.Second)
		convey.So(err.Error(), convey.ShouldEqual, "parameter error")
	})
}

func TestDelete(t *testing.T) {
	cache := New(1)
	convey.Convey("test lru delete", t, func() {
		cache.Set("testkey1", "1", time.Minute)
		v, err := cache.Get("testkey1")
		convey.So(err, convey.ShouldEqual, nil)
		convey.So(v, convey.ShouldEqual, "1")
		cache.Delete("testkey1")
		v, err = cache.Get("testkey1")
		convey.So(v, convey.ShouldEqual, nil)
		convey.So(err, convey.ShouldNotEqual, nil)
	})
	convey.Convey("Delete no thing", t, func() {
		c := &lruCache{}
		c.delValue("test")
	})
}

func TestSetIfNX(t *testing.T) {
	cache := New(1)
	convey.Convey("SetIfNX set parameter error", t, func() {
		r := cache.SetIfNX("testkey1", "1", -time.Millisecond)
		convey.So(r, convey.ShouldEqual, false)
	})
	convey.Convey("SetIfNX set success", t, func() {
		r := cache.SetIfNX("testkey1", "1", cacheTime*time.Millisecond)
		convey.So(r, convey.ShouldEqual, true)
	})
	convey.Convey("SetIfNX set success failed", t, func() {
		r := cache.SetIfNX("testkey1", "1", cacheTime*time.Millisecond)
		convey.So(r, convey.ShouldEqual, false)
	})
	time.Sleep(cacheTime * time.Millisecond)
	convey.Convey("SetIfNX set success", t, func() {
		r := cache.SetIfNX("testkey1", "1", time.Second)
		convey.So(r, convey.ShouldEqual, true)
	})
	convey.Convey("SetIfNX expireTime -1", t, func() {
		r := cache.SetIfNX("testkey", "1", time.Duration(negInt64One))
		convey.So(r, convey.ShouldEqual, true)
		r = cache.SetIfNX("testkey", "1", time.Duration(negInt64One))
		convey.So(r, convey.ShouldEqual, false)
	})

}

func TestSetIfNXConcurrencyTest(t *testing.T) {
	cache := New(1)
	convey.Convey("SetIfNX concurrency test", t, func() {
		var count = 0
		count = testSetIfNX(cache, count)
		convey.So(count, convey.ShouldEqual, 1)
	})
}

func testSetIfNX(cache *ConcurrencyLRUCache, count int) int {
	l := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(goRoutineCount)
	for i := 0; i < goRoutineCount; i++ {
		go func() {
			r := cache.SetIfNX("testkey2", "1", time.Second)
			if r {
				l.Lock()
				count++
				l.Unlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()
	return count
}

func TestINCRConcurrencyTest(t *testing.T) {
	cache := New(1)
	convey.Convey("INCR concurrency test", t, func() {
		max := testIncr(cache)
		convey.So(max, convey.ShouldEqual, goRoutineCount)
	})
}

func testIncr(cache *ConcurrencyLRUCache) int64 {
	var max = int64Zero
	l := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(goRoutineCount)
	for i := 0; i < goRoutineCount; i++ {
		go func() {
			r, err := cache.INCR("testkey1", time.Second)
			if err != nil {
				return
			}
			l.Lock()
			if r > max {
				max = r
			}
			l.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()
	return max
}

func TestDECRConcurrencyTest(t *testing.T) {
	cache := New(1)
	cache.Set("testkey1", int64(goRoutineCount), time.Minute)
	convey.Convey("INCR concurrency test", t, func() {
		min := testDecr(cache)
		convey.So(min, convey.ShouldEqual, 0)
	})
}

func testDecr(cache *ConcurrencyLRUCache) int64 {
	var min = int64(math.MaxInt)
	l := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(goRoutineCount)
	for i := 0; i < goRoutineCount; i++ {
		go func() {
			r, err := cache.DECR("testkey1", time.Second)
			if err != nil {
				return
			}
			l.Lock()
			if r < min {
				min = r
			}
			l.Unlock()
			wg.Done()
		}()
	}
	wg.Wait()
	return min
}

func TestINCR(t *testing.T) {
	cache := New(1)
	convey.Convey("not initializes", t, func() {
		c := &lruCache{}
		_, err := c.increment("test", time.Minute)
		convey.So(err, convey.ShouldEqual, notInitErr)
	})
	convey.Convey("parameter error", t, func() {
		_, err := cache.INCR("testkey", -time.Minute)
		convey.So(err, convey.ShouldEqual, paraErr)
	})
	convey.Convey("INCR success", t, func() {
		r, err := cache.INCR("testkey", time.Minute)
		convey.So(r, convey.ShouldEqual, 1)
		convey.So(err, convey.ShouldEqual, nil)
		r, err = cache.INCR("testkey", time.Minute)
		convey.So(r, convey.ShouldEqual, intTwo)
	})

	convey.Convey("INCR success when exits", t, func() {
		cache.Set("testkey1", int64Zero, cacheTime*time.Millisecond)
		r, err := cache.INCR("testkey1", cacheTime*time.Millisecond)
		convey.So(r, convey.ShouldEqual, 1)
		convey.So(err, convey.ShouldEqual, nil)
		time.Sleep(cacheTime * time.Millisecond)
		r, err = cache.INCR("testkey1", time.Minute)
		convey.So(r, convey.ShouldEqual, 1)
	})
}

func TestDECR(t *testing.T) {
	cache := New(1)
	convey.Convey("not initializes", t, func() {
		c := &lruCache{}
		_, err := c.decrement("test", time.Minute)
		convey.So(err, convey.ShouldEqual, notInitErr)
	})
	convey.Convey("parameter error", t, func() {
		_, err := cache.DECR("testkey1", -time.Minute)
		convey.So(err, convey.ShouldEqual, paraErr)
	})
	convey.Convey("SetIfNX set success", t, func() {
		r, err := cache.DECR("testkey1", time.Minute)
		convey.So(r, convey.ShouldEqual, negInt64One)
		convey.So(err, convey.ShouldEqual, nil)
		cache.Set("testkey1", int64One, time.Minute)
		r, err = cache.DECR("testkey1", time.Minute)
		convey.So(r, convey.ShouldEqual, 0)
		convey.So(err, convey.ShouldEqual, nil)
	})
	convey.Convey("Decr success when exits", t, func() {
		cache.Set("testkey2", int64One, cacheTime*time.Millisecond)
		r, err := cache.DECR("testkey2", cacheTime*time.Millisecond)
		convey.So(r, convey.ShouldEqual, 0)
		convey.So(err, convey.ShouldEqual, nil)
		time.Sleep(cacheTime * time.Millisecond)
		r, err = cache.DECR("testkey2", time.Minute)
		convey.So(err, convey.ShouldEqual, nil)
		convey.So(r, convey.ShouldEqual, negInt64One)
	})
}

func TestLRU(t *testing.T) {
	convey.Convey("not initializes", t, func() {
		c := &lruCache{
			maxSize:   intTwo,
			elemIndex: make(map[string]*list.Element, segmentCount),
			List:      list.New(),
			mu:        sync.Mutex{},
		}
		c.setValue("test", "1", time.Minute)
		c.setValue("test1", "1", time.Minute)
		c.setValue("test2", "1", time.Minute)
		_, err := c.getValue("test")
		convey.So(err.Error(), convey.ShouldEqual, "no value found")
	})
}

func BenchmarkSetIfNx(b *testing.B) {
	cache := New(1)
	for n := 0; n < b.N; n++ {
		cache.SetIfNX(fmt.Sprintf("key%d", n), "xx", time.Second)
	}
}

func BenchmarkINCR(b *testing.B) {
	cache := New(1)
	for n := 0; n < b.N; n++ {
		cache.INCR("sdds", time.Second)
	}
}
