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
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

const (
	segmentCount               = 16
	int64One     int64         = 1
	int64Zero    int64         = 0
	negInt64One  int64         = -1
	intTwo                     = 2
	hashInit     uint32        = 2166136261
	prime32      uint32        = 16777619
	twentyYears  time.Duration = 20 * 365 * 24 * time.Hour
)

var (
	notInitErr = errors.New("not initializes")
	paraErr    = errors.New("parameter error")
)

type cacheEle struct {
	key        string
	data       interface{}
	expireTime int64
}

type lruCache struct {
	maxSize   int
	elemIndex map[string]*list.Element
	*list.List
	mu sync.Mutex
}

// ConcurrencyLRUCache is a memory-based LRU local cache, default total 16 segment to improve concurrent performance
// LRU is not  real least recently used for the total cache,but just for each buket
// we just need a proper method to clear cache
type ConcurrencyLRUCache struct {
	segment    int
	cacheBuket [segmentCount]*lruCache
}

// Set create or update an element using key
//      key:    The identity of an element
//      value:  new value of the element
//      expireTime:    expire time, positive int64 or -1 which means never overdue
func (cl *ConcurrencyLRUCache) Set(key string, value interface{}, expireTime time.Duration) error {
	if cl == nil || cl.cacheBuket[0] == nil {
		return notInitErr
	}
	if expireTime < time.Duration(negInt64One) || expireTime > twentyYears {
		return paraErr
	}
	cacheIndex := cl.index(key)
	if cacheIndex < 0 || cacheIndex >= segmentCount {
		return errors.New("index out of valid value")
	}
	return cl.cacheBuket[cacheIndex].setValue(key, value, expireTime)
}

// Get get the value of a cached element by key. If key do not exist, this function will return nil and an error msg
//      key:    The identity of an element
//      return:
//          value:  the cached value, nil if key do not exist
//          err:    error info, nil if value is not nil
func (cl *ConcurrencyLRUCache) Get(key string) (interface{}, error) {
	if cl == nil || cl.cacheBuket[0] == nil {
		return nil, notInitErr
	}
	cacheIndex := cl.index(key)
	if cacheIndex < 0 || cacheIndex >= segmentCount {
		return nil, errors.New("index out of valid value")
	}
	return cl.cacheBuket[cacheIndex].getValue(key)
}

// Delete delete the value  by key, no error returned
func (cl *ConcurrencyLRUCache) Delete(key string) {
	if cl == nil || cl.cacheBuket[0] == nil {
		return
	}
	cacheIndex := cl.index(key)
	if cacheIndex < 0 || cacheIndex >= segmentCount {
		return
	}
	cl.cacheBuket[cacheIndex].delValue(key)
}

// SetIfNX if the key not exist or expired, will set the new value to cache and return true ,otherwise return false
func (cl *ConcurrencyLRUCache) SetIfNX(key string, value interface{}, expireTime time.Duration) bool {
	if cl == nil || cl.cacheBuket[0] == nil {
		return false
	}
	if expireTime < time.Duration(negInt64One) || expireTime > twentyYears {
		return false
	}
	cacheIndex := cl.index(key)
	if cacheIndex < 0 || cacheIndex >= segmentCount {
		return false
	}
	return cl.cacheBuket[cacheIndex].setIfNotExist(key, value, expireTime)
}

// INCR add one to  the value(must int64) of the key , if the key not exist, initialize with 0 and then add one
func (cl *ConcurrencyLRUCache) INCR(key string, expireTime time.Duration) (int64, error) {
	if err := validate(cl, expireTime); err != nil {
		return 0, err
	}
	cacheIndex := cl.index(key)
	if cacheIndex < 0 || cacheIndex >= segmentCount {
		return 0, errors.New("index out of valid value")
	}
	return cl.cacheBuket[cacheIndex].increment(key, expireTime)
}

// DECR minus one to the value(must int64) of the key,if the key not exist, initialize with 0 and then minus one
func (cl *ConcurrencyLRUCache) DECR(key string, expireTime time.Duration) (int64, error) {
	if err := validate(cl, expireTime); err != nil {
		return 0, err
	}
	cacheIndex := cl.index(key)
	if cacheIndex < 0 || cacheIndex >= segmentCount {
		return 0, errors.New("index out of valid value")
	}
	return cl.cacheBuket[cacheIndex].decrement(key, expireTime)
}

func validate(cl *ConcurrencyLRUCache, expireTime time.Duration) error {
	if cl == nil || cl.cacheBuket[0] == nil {
		return paraErr
	}
	if expireTime <= 0 && expireTime != time.Duration(negInt64One) {
		return paraErr
	}
	return nil
}

// index calculate the key hashcode and index the right buket
func (cl *ConcurrencyLRUCache) index(key string) int {
	var hash = hashInit
	for i := 0; i < len(key); i++ {
		hash *= prime32
		hash ^= uint32(key[i])
	}
	return int(hash & (uint32(cl.segment) - 1))
}

// New create an instance of ConcurrencyLRUCache
// maxEntries  the cache size, will to convert to (n/16+n%16>0?1:0)*16
func New(maxEntries int) *ConcurrencyLRUCache {
	if maxEntries <= 0 {
		return nil
	}
	size := maxEntries / segmentCount
	remain := maxEntries % segmentCount
	if remain > 0 {
		size += 1
	}
	var cache [segmentCount]*lruCache
	for i := 0; i < segmentCount; i++ {
		cache[i] = &lruCache{
			maxSize:   size,
			elemIndex: make(map[string]*list.Element, segmentCount),
			List:      list.New(),
			mu:        sync.Mutex{},
		}
	}
	return &ConcurrencyLRUCache{
		segment:    segmentCount,
		cacheBuket: cache,
	}
}

func (c *lruCache) setValue(key string, value interface{}, expireTime time.Duration) error {
	if c == nil || c.elemIndex == nil {
		return errors.New("not initializes")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.elemIndex[key]
	if !ok {
		// if the cache not exist
		c.setInner(key, value, expireTime)
		return nil
	}
	ele, ok := v.Value.(*cacheEle)
	if !ok {
		c.safeDeleteByKey(key, v)
		return errors.New("cacheElement convert failed")
	}
	c.MoveToFront(v)
	pkgElement(ele, value, expireTime)
	return nil
}

func pkgElement(ele *cacheEle, value interface{}, expireTime time.Duration) {
	ele.data = value
	if expireTime == time.Duration(negInt64One) {
		ele.expireTime = negInt64One
		return
	}
	ele.expireTime = time.Now().UnixNano() + int64(expireTime)
}

func (c *lruCache) getValue(key string) (interface{}, error) {
	if c == nil || c.elemIndex == nil {
		return nil, errors.New("not initializes")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.elemIndex[key]
	if !ok {
		return nil, errors.New("no value found")
	}
	c.MoveToFront(v)
	ele, ok := v.Value.(*cacheEle)
	if !ok {
		c.safeDeleteByKey(key, v)
		return nil, errors.New("cacheElement convert failed")
	}
	if ele.expireTime != negInt64One && time.Now().UnixNano() > ele.expireTime {
		// if  cache expired
		c.safeDeleteByKey(key, v)
		return nil, errors.New("the key was expired")
	}
	return ele.data, nil
}

// Delete delete an element
func (c *lruCache) delValue(key string) {
	if c == nil || c.elemIndex == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.elemIndex[key]; ok {
		c.safeDeleteByKey(key, v)
	}
}

func (c *lruCache) setIfNotExist(key string, value interface{}, expireTime time.Duration) bool {
	if c == nil || c.elemIndex == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.elemIndex[key]
	if !ok {
		// if the cache not exist
		c.setInner(key, value, expireTime)
		return true
	}
	ele, ok := v.Value.(*cacheEle)
	if !ok {
		c.safeDeleteByKey(key, v)
		return false
	}
	c.MoveToFront(v)
	if ele.expireTime == negInt64One || time.Now().UnixNano() < ele.expireTime {
		return false
	}
	// if  cache expired
	pkgElement(ele, value, expireTime)
	return true
}

func (c *lruCache) increment(key string, expireTime time.Duration) (int64, error) {
	if c == nil || c.elemIndex == nil {
		return 0, notInitErr
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.elemIndex[key]
	if !ok {
		c.setInner(key, int64One, expireTime)
		return int64One, nil
	}
	ele, ok := v.Value.(*cacheEle)
	if !ok {
		c.safeDeleteByKey(key, v)
		c.setInner(key, int64One, expireTime)
		return int64One, nil
	}
	c.MoveToFront(v)
	if ele.expireTime == negInt64One || time.Now().UnixNano() < ele.expireTime {
		newValue, ok := ele.data.(int64)
		if !ok || newValue == math.MaxInt64 {
			return 0, fmt.Errorf("the cache value is not valid, ok:%v", ok)
		}
		newValue++
		pkgElement(ele, newValue, expireTime)
		return newValue, nil
	}
	// if  cache expired
	pkgElement(ele, int64One, expireTime)
	return int64One, nil
}

func (c *lruCache) decrement(key string, expireTime time.Duration) (int64, error) {
	if c == nil || c.elemIndex == nil {
		return 0, notInitErr
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.elemIndex[key]
	if !ok {
		// if the cache not exist
		c.setInner(key, negInt64One, expireTime)
		return negInt64One, nil
	}
	ele, ok := v.Value.(*cacheEle)
	if !ok {
		c.safeDeleteByKey(key, v)
		c.setInner(key, negInt64One, expireTime)
		return negInt64One, nil
	}
	c.MoveToFront(v)
	if ele.expireTime == negInt64One || time.Now().UnixNano() < ele.expireTime {
		newValue, ok := ele.data.(int64)
		if !ok || newValue == math.MinInt64 {
			return 0, fmt.Errorf("the cache value is not valid, ok:%v", ok)
		}
		newValue--
		pkgElement(ele, newValue, expireTime)
		return newValue, nil
	}
	// if  cache expired
	pkgElement(ele, negInt64One, expireTime)
	return negInt64One, nil
}

func (c *lruCache) setInner(key string, value interface{}, expireTime time.Duration) {
	if c == nil {
		return
	}
	if c.Len()+1 > c.maxSize {
		c.safeRemoveOldest()
	}
	newElem := &cacheEle{
		key:        key,
		data:       value,
		expireTime: negInt64One,
	}
	if expireTime != time.Duration(negInt64One) {
		newElem.expireTime = time.Now().UnixNano() + int64(expireTime)
	}
	e := c.PushFront(newElem)
	c.elemIndex[key] = e
}

func (c *lruCache) safeDeleteByKey(key string, v *list.Element) {
	if c == nil {
		return
	}
	c.List.Remove(v)
	delete(c.elemIndex, key)
}

func (c *lruCache) safeRemoveOldest() {
	if c == nil {
		return
	}
	v := c.List.Back()
	if v == nil {
		return
	}
	c.List.Remove(v)
	ele, ok := v.Value.(*cacheEle)
	if !ok {
		return
	}
	delete(c.elemIndex, ele.key)
}
