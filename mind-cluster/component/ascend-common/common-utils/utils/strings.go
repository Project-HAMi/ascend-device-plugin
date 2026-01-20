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

// Package utils provides the util func
package utils

import (
	"crypto/sha256"
	"fmt"
	"unicode"
)

const (
	maskLen = 2
)

// ReplacePrefix replace string with prefix
func ReplacePrefix(source, prefix string) string {
	if prefix == "" {
		prefix = "****"
	}
	if len(source) <= maskLen {
		return prefix
	}
	end := string([]rune(source)[maskLen:len(source)])
	return prefix + end
}

// MaskPrefix mask string prefix with ****
func MaskPrefix(source string) string {
	return ReplacePrefix(source, "")
}

// GetSha256Code return the sha256 hash bytes
func GetSha256Code(data []byte) []byte {
	hash256 := sha256.New()
	if _, err := hash256.Write(data); err != nil {
		fmt.Println(err)
		return nil
	}
	return hash256.Sum(nil)
}

// ReverseString reverse string
func ReverseString(s string) string {
	runes := []rune(s)
	for start, end := 0, len(runes)-1; start < end; start, end = start+1, end-1 {
		runes[start], runes[end] = runes[end], runes[start]
	}
	return string(runes)
}

// IsDigitString return string is all digit
func IsDigitString(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}
	return true
}
