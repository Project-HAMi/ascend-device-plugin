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

// Package utils this file for password handler
package utils

import (
	"bytes"
	"errors"
	"regexp"
)

const (
	lowercaseCharactersRegex = `[a-z]{1,}`
	uppercaseCharactersRegex = `[A-Z]{1,}`
	baseNumberRegex          = `[0-9]{1,}`
	specialCharactersRegex   = `[!\"#$%&'()*+,\-. /:;<=>?@[\\\]^_\x60{|}~]{1,}`
	passWordRegex            = `^[a-zA-Z0-9!\"#$%&'()*+,\-. /:;<=>?@[\\\]^_\x60{|}~]{8,64}$`
	minComplexCount          = 2
)

// CheckPassWordComplexity check password complexity
func CheckPassWordComplexity(s []byte) error {
	complexCheckRegexArr := []string{
		lowercaseCharactersRegex,
		uppercaseCharactersRegex,
		baseNumberRegex,
		specialCharactersRegex,
	}
	complexCount := 0
	for _, pattern := range complexCheckRegexArr {
		if matched, err := regexp.Match(pattern, s); matched && err == nil {
			complexCount++
		}
	}
	if complexCount < minComplexCount {
		return errors.New("password complex not meet the requirement")
	}
	return nil
}

// ValidatePassWord validate password
func ValidatePassWord(userName string, passWord []byte) error {
	if err := commonCheckForPassWord(userName, passWord); err != nil {
		return err
	}
	return CheckPassWordComplexity(passWord)
}

func commonCheckForPassWord(userName string, passWord []byte) error {
	if matched, err := regexp.Match(passWordRegex, passWord); err != nil || !matched {
		return errors.New("password not meet requirement")
	}
	var userNameByte []byte = []byte(userName)
	if bytes.Equal(userNameByte, passWord) {
		return errors.New("password cannot equals username")
	}
	var reverseUserName = ReverseString(userName)
	var reverseUserNameByte []byte = []byte(reverseUserName)
	if bytes.Equal(reverseUserNameByte, passWord) {
		return errors.New("password cannot equal reversed username")
	}
	return nil
}
