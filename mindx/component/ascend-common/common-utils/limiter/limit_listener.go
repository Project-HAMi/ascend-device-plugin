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

// Package limiter implement a token bucket limit listener, refer to "golang.org/x/net/netutil" and
// change the acquire method, if acquire failed, return false immediately
package limiter

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"ascend-common/common-utils/cache"
	"ascend-common/common-utils/hwlog"
)

const (
	maxConnection   = 1024
	maxIPConnection = 512

	largeMaxConnection = 16384
)

func commonLimitListener(l net.Listener, totalConnLimit, IPConnLimit, cacheSize int) (net.Listener, error) {
	if IPConnLimit < 0 || IPConnLimit > maxIPConnection {
		return nil, errors.New("the parameter IPConnLimit is illegal")
	}
	bucket := make(chan struct{}, totalConnLimit)
	ll := &localLimitListener{
		Listener:    l,
		buckets:     bucket,
		ipConnLimit: int64(IPConnLimit),
	}
	if cacheSize > 0 {
		ll.ipCache = cache.New(cacheSize)
	}
	return ll, nil
}

// LimitListener returns a Listener that accepts at most n connections at the same time
func LimitListener(l net.Listener, totalConnLimit, IPConnLimit, cacheSize int) (net.Listener, error) {
	if totalConnLimit < 0 || totalConnLimit > maxConnection {
		return nil, errors.New("the parameter totalConnLimit is illegal")
	}
	return commonLimitListener(l, totalConnLimit, IPConnLimit, cacheSize)
}

type localLimitListener struct {
	net.Listener
	buckets     chan struct{}
	closeOnce   sync.Once
	ipCache     *cache.ConcurrencyLRUCache
	ipConnLimit int64
}

// acquire acquires the limiting semaphore. Returns true if successfully
// accquired, false if the listener is closed or  reach the max limit
func (l *localLimitListener) acquire() bool {
	select {
	case l.buckets <- struct{}{}:
		return true
	default:
		return false
	}
}
func (l *localLimitListener) release() { <-l.buckets }

// Accept implement  net.Listener interface
func (l *localLimitListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	// ip connection limit
	ip, cacheKey := getIpAndKey(c)
	if ip != "" && l.ipCache != nil {
		if counts, err := l.ipCache.INCR(cacheKey, -1); err == nil && counts > l.ipConnLimit {
			hwlog.RunLog.Warn("ip connections reach max limit, connection will to force closed")
			return closeImmediately(c, l.ipCache), nil
		}
	}
	//  total tcp connection limit
	if l.acquire() {
		return &limitListenerConn{Conn: c, release: l.release, ipCache: l.ipCache}, nil
	}
	hwlog.RunLog.Warn("limit forbidden, connection will to force closed")
	return closeImmediately(c, l.ipCache), nil

}

func getIpAndKey(c net.Conn) (string, string) {
	ipWithPort := c.RemoteAddr().String()
	if ipWithPort != "" {
		s := strings.Split(ipWithPort, ":")
		return s[0], fmt.Sprintf("key-conn-%s", s[0])
	}
	return "", ""
}

func closeImmediately(c net.Conn, lruCache *cache.ConcurrencyLRUCache) net.Conn {
	// once the connection reach the max limit, force close the connection
	tcpConn, ok := c.(*net.TCPConn)
	if ok {
		if err := tcpConn.SetLinger(0); err != nil {
			hwlog.RunLog.Warnf("Error when setting linger: %s", err)
		}
	}

	err := c.Close()
	if err != nil {
		hwlog.RunLog.Warn(err)
	}
	return &limitListenerConn{Conn: c, release: func() {}, ipCache: lruCache}
}

// Close implement  net.Listener interface
func (l *localLimitListener) Close() error {
	err := l.Listener.Close()
	l.closeOnce.Do(func() { close(l.buckets) })
	return err
}

type limitListenerConn struct {
	net.Conn
	releaseOnce sync.Once
	release     func()
	ipCache     *cache.ConcurrencyLRUCache
}

// Close override  net.Conn interface
func (l *limitListenerConn) Close() error {
	err := l.Conn.Close()
	if err != nil {
		hwlog.RunLog.Debugf("close grpc connect failed: %v", err)
		return fmt.Errorf("close grpc connect failed: %v", err)
	}
	l.releaseOnce.Do(l.release)
	ip, cacheKey := getIpAndKey(l.Conn)
	if ip != "" && l.ipCache != nil {
		d, err := l.ipCache.DECR(cacheKey, time.Hour)
		if err != nil {
			hwlog.RunLog.Error(err)
		}
		hwlog.RunLog.Debugf("decrement ip connections %d", d)
	}
	return err
}
