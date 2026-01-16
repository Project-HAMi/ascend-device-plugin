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

// Package limiter implement a token bucket limiter
package limiter

import (
	"errors"
	"net"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/smartystreets/goconvey/convey"
)

const (
	len2 = 2
)

func TestLimitListenerAccept(t *testing.T) {
	convey.Convey("test Accept function", t, func() {

		limitLor, err := LimitListener(&mockLicener{}, len2, len2, DefaultCacheSize)
		if err != nil {
			return
		}
		l, ok := limitLor.(*localLimitListener)
		if !ok {
			return
		}
		mock2 := gomonkey.ApplyFunc(getIpAndKey, func(net.Conn) (string, string) {
			return "127.0.0.1", "key-127.0.0.1"
		})
		defer mock2.Reset()
		convey.Convey("acquire token success", func() {
			_, err = l.Accept()
			convey.So(err, convey.ShouldEqual, nil)
		})

		convey.Convey("accept failed", func() {
			mock := gomonkey.ApplyMethodFunc(l.Listener, "Accept", func() (net.Conn, error) {
				return nil, errors.New("mock error")
			})
			defer mock.Reset()
			con, err := l.Accept()
			convey.So(err, convey.ShouldNotEqual, nil)
			convey.So(con, convey.ShouldEqual, nil)
		})

		convey.Convey("acquire token failed", func() {
			mock := gomonkey.ApplyPrivateMethod(l, "acquire", func(*localLimitListener) bool {
				return false
			})
			defer mock.Reset()
			con, err := l.Accept()
			convey.So(err, convey.ShouldEqual, nil)
			conm, ok := con.(*limitListenerConn)
			if !ok {
				return
			}
			convey.So(conm.release, convey.ShouldNotEqual, nil)
		})

	})
}

type mockLicener struct {
}

func (l *mockLicener) Accept() (net.Conn, error) {
	return &net.TCPConn{}, nil
}

func (l *mockLicener) Addr() net.Addr {
	return &net.IPAddr{
		IP:   []byte("127.0.0.1"),
		Zone: "",
	}
}

func (l *mockLicener) Close() error {
	return nil
}

func TestGetIpAndKey(t *testing.T) {
	convey.Convey("test getIp function", t, func() {
		c := net.TCPConn{}
		mock := gomonkey.ApplyMethodFunc(&c, "RemoteAddr", func() net.Addr {
			return &net.IPAddr{
				IP:   []byte("127.0.0.1"),
				Zone: "",
			}
		})
		defer mock.Reset()
		ip, _ := getIpAndKey(&c)
		convey.So(ip, convey.ShouldNotEqual, "")
	})
}

func TestLimitListener(t *testing.T) {
	convey.Convey("test new listener function success", t, func() {
		l, err := LimitListener(&mockLicener{}, maxConnection, maxIPConnection, DefaultDataLimit)
		convey.So(l, convey.ShouldNotEqual, nil)
		convey.So(err, convey.ShouldEqual, nil)
	})
	convey.Convey("test new listener function", t, func() {
		_, err := LimitListener(&mockLicener{}, maxConnection+1, maxIPConnection, DefaultDataLimit)
		convey.So(err, convey.ShouldNotEqual, nil)
	})
	convey.Convey("test new listener function", t, func() {
		_, err := LimitListener(&mockLicener{}, maxConnection, maxIPConnection+1, DefaultDataLimit)
		convey.So(err, convey.ShouldNotEqual, nil)
	})
}
