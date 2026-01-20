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

// Package hwlog provides the capability of processing Huawei log rules.
package hwlog

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
)

// printHelper helper function for log printing
func printHelper(lg *log.Logger, msg string, maxLogLength int, ctx ...context.Context) {
	str := getCallerInfo(ctx...)
	trimMsg := strings.Replace(msg, "\r", " ", -1)
	trimMsg = strings.Replace(trimMsg, "\n", " ", -1)
	runeArr := []rune(trimMsg)
	if length := len(runeArr); length > maxLogLength {
		trimMsg = string(runeArr[:maxLogLength])
	}
	lg.Println(str + trimMsg)
}

// getCallerInfo gets the caller's information
func getCallerInfo(ctx ...context.Context) string {
	var deep = stackDeep
	var userID interface{}
	var traceID interface{}
	for _, c := range ctx {
		if c == nil {
			deep++
			continue
		}
		userID = c.Value(UserID)
		traceID = c.Value(ReqID)
		if val := c.Value(extraDeepKey); val != nil {
			currentVal, _ := val.(int) // security type assertions, invalid values are automatically zeroed
			deep += currentVal
		}
	}
	var funcName string
	pc, codePath, codeLine, ok := runtime.Caller(deep)
	if ok {
		funcName = runtime.FuncForPC(pc).Name()
	}
	p := strings.Split(codePath, "/")
	l := len(p)
	if l == pathLen {
		funcName = p[l-1]
	} else if l > pathLen {
		funcName = fmt.Sprintf("%s/%s", p[l-pathLen], p[l-1])
	}
	callerPath := fmt.Sprintf("%s:%d", funcName, codeLine)
	goroutineID := getGoroutineID()
	str := fmt.Sprintf("%-8s%s    ", goroutineID, callerPath)
	if userID != nil || traceID != nil {
		str = fmt.Sprintf("%s{%#v}-{%#v} ", str, userID, traceID)
	}
	return str
}

// getCallerGoroutineID gets the goroutineID
func getGoroutineID() string {
	b := make([]byte, bitsize, bitsize)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	return string(b)
}

// DeepIncrease increases the stack depth by 1
func DeepIncrease(ctx context.Context) context.Context {
	if ctx == nil {
		return context.WithValue(context.Background(), extraDeepKey, 1)
	}

	var currentVal int
	if val := ctx.Value(extraDeepKey); val != nil {
		currentVal, _ = val.(int) // security type assertions, invalid values are automatically zeroed
	}

	return context.WithValue(ctx, extraDeepKey, currentVal+1)
}
