/* Copyright(C) 2025. Huawei Technologies Co.,Ltd. All rights reserved.
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

// Package utils this file for slice utils
package utils

import (
	"fmt"
	"slices"
	"strconv"
)

// hex hexadecimal
const hex = 16

type stringTool struct{}

// StringTool slice for string tool
var StringTool stringTool

// HexStringToInt hex string slice to int64 slice
func (s stringTool) HexStringToInt(sources []string) map[int64]struct{} {
	intMap := make(map[int64]struct{}, len(sources))
	for _, source := range sources {
		num, err := strconv.ParseInt(source, hex, 0)
		if err != nil {
			fmt.Printf("parse hex to int failed, skip it. error: %v\n", err)
			continue
		}
		intMap[num] = struct{}{}
	}
	return intMap
}

// Contains check whether slice contains target
func Contains[T comparable](sources []T, target T) bool {
	for _, v := range sources {
		if v == target {
			return true
		}
	}
	return false
}

// Remove delete the first matching element in the slice
func Remove[T comparable](slice []T, target T) []T {
	for i, v := range slice {
		if v == target {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// RemoveDuplicates remove duplicates from slice
func RemoveDuplicates[T comparable](slice []T) []T {
	existMap := make(map[T]struct{})
	result := make([]T, 0)
	for _, str := range slice {
		if _, ok := existMap[str]; !ok {
			existMap[str] = struct{}{}
			result = append(result, str)
		}
	}
	return result
}

// SameElementInMap whether map contains target
func SameElementInMap[T comparable](sources map[T]struct{}, targets []T) bool {
	for _, target := range targets {
		if _, ok := sources[target]; ok {
			return true
		}
	}
	return false
}

// RemoveEleSli remove element in sources which is in target
func RemoveEleSli[T comparable](source, target []T) []T {
	sliMap := make(map[T]struct{})
	for _, item := range target {
		sliMap[item] = struct{}{}
	}

	result := make([]T, 0)
	for _, ele := range source {
		if _, ok := sliMap[ele]; !ok {
			result = append(result, ele)
		}
	}
	return result
}

// RemoveElementsNotInSecond remove elements not in slice2
func RemoveElementsNotInSecond[T comparable](slice1, slice2 []T) []T {
	sliMap := make(map[T]struct{})
	for _, item := range slice2 {
		sliMap[item] = struct{}{}
	}

	result := make([]T, 0)
	for _, item := range slice1 {
		if _, ok := sliMap[item]; ok {
			result = append(result, item)
		}
	}
	return result
}

// CheckSliceSupport check elements is supported in expects
func CheckSliceSupport(elements []int64, expects []int64) error {
	for _, e := range elements {
		if !slices.Contains(expects, e) {
			return fmt.Errorf("element %v does not contain %v", e, expects)
		}
	}
	return nil
}
