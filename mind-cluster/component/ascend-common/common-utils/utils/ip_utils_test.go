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

// Package utils offer the some utils for certificate handling
package utils

import (
	"net/http"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

const (
	localhost     = "127.0.0.1"
	localhostLoop = "0.0.0.0"
)

func TestClientIP(t *testing.T) {
	convey.Convey("test ClientIP func", t, func() {
		convey.Convey("get IP from X-Forwarded-For", func() {
			ip := ClientIP(mockRequest(map[string][]string{"X-Forwarded-For": {localhost, localhostLoop}}))
			convey.So(ip, convey.ShouldEqual, localhost)
		})
		convey.Convey("get IP from X-Real-Ip", func() {
			ip := ClientIP(mockRequest(map[string][]string{"X-Forwarded-For": {},
				"X-Real-Ip": {localhost}}))
			convey.So(ip, convey.ShouldEqual, localhost)
		})
		convey.Convey("get IP from RemoteAddr", func() {
			ip := ClientIP(mockRequest(map[string][]string{"X-Forwarded-For": {},
				"X-Real-Ip": {}}))
			convey.So(ip, convey.ShouldEqual, localhost)
		})
		convey.Convey("get IP from RemoteAddr failed", func() {
			ip := ClientIP(&http.Request{RemoteAddr: localhost})
			convey.So(ip, convey.ShouldEqual, "")
		})
		convey.Convey("get IP failed", func() {
			ip := ClientIP(&http.Request{})
			convey.So(ip, convey.ShouldEqual, "")
		})
	})
}

func mockRequest(header map[string][]string) *http.Request {
	return &http.Request{
		Method:        "GET",
		URL:           nil,
		Proto:         "HTTP",
		ProtoMajor:    0,
		ProtoMinor:    0,
		Header:        header,
		ContentLength: 0,
		Close:         false,
		Host:          "www.test.com",
		RemoteAddr:    "127.0.0.1:8080",
	}
}

func TestCheckDomain(t *testing.T) {
	convey.Convey("CheckDomain function test suite", t, func() {
		testDomainFormatValidation()
		testLocalUsageConstraints()
		testParameterCombinations()
	})
}

// Test domain format validation
func testDomainFormatValidation() {
	convey.Convey("Validate domain format rules", func() {
		convey.Convey("Valid domain should pass validation", func() {
			err := CheckDomain("example.com", false)
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("Domain with special characters should be rejected", func() {
			err := CheckDomain("example@com", false)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "domain does not match allowed regex")
		})

		convey.Convey("Domain starting with hyphen should be rejected", func() {
			err := CheckDomain("-example.com", false)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})
}

// Test local usage constraints
func testLocalUsageConstraints() {
	convey.Convey("Validate constraints for local usage (forLocalUsage=true)", func() {
		convey.Convey("All-digit domain should be rejected", func() {
			err := CheckDomain("123456", true)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "domain can not be all digits")
		})

		convey.Convey("Domain containing 'localhost' should be rejected", func() {
			err := CheckDomain("my-localhost.com", true)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(err.Error(), convey.ShouldContainSubstring, "domain can not contain localhost")
		})

		convey.Convey("Valid local domain should pass validation", func() {
			err := CheckDomain("local-app.example", true)
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

// Test parameter combinations
func testParameterCombinations() {
	convey.Convey("Validate parameter combinations", func() {
		convey.Convey("All-digit restriction ignored when forLocalUsage=false", func() {
			err := CheckDomain("123456", false)
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("DNS check skipped when forLocalUsage=false", func() {
			err := CheckDomain("unresolvable.test", false)
			convey.So(err, convey.ShouldBeNil)
		})
	})
}

func TestIsHostValid(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
		errMsg  string
	}{
		{
			name: "invalid IP format but domain", ip: "not.an.ip",
			wantErr: false,
		},
		{
			name: "valid IPv4", ip: "192.168.1.1", wantErr: false,
		},
		{
			name: "valid IPv6", ip: "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			wantErr: false,
		},
		{
			name: "unspecified IPv4", ip: "0.0.0.0",
			wantErr: true, errMsg: "is all zeros ip",
		},
		{
			name: "unspecified IPv6", ip: "::",
			wantErr: true, errMsg: "is all zeros ip",
		},
		{
			name: "IPv6 multicast", ip: "ff02::1",
			wantErr: true, errMsg: "is multicast ip",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsHostValid(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsIPValid() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && err.Error() != tt.errMsg {
				t.Errorf("IsIPValid() error = %v, wantErrMsg %v",
					err.Error(), tt.errMsg)
			}
		})
	}
}
