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
	"reflect"
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

const (
	decimal1A    = 26
	decimalFF    = 255
	decimalNeg10 = 16
	decimalNegFF = -255
)

func buildHexStringToIntTestCase() []struct {
	name     string
	input    []string
	expected map[int64]struct{}
} {
	return []struct {
		name     string
		input    []string
		expected map[int64]struct{}
	}{
		{
			name:  "01 - Valid hex strings",
			input: []string{"1A", "FF", "10"},
			expected: map[int64]struct{}{
				decimal1A:    {},
				decimalFF:    {},
				decimalNeg10: {},
			},
		},
		{
			name:     "02 - Invalid hex strings",
			input:    []string{"xyz", "ghijk"},
			expected: map[int64]struct{}{},
		},
		{
			name:     "03 - Empty input array",
			input:    []string{},
			expected: map[int64]struct{}{},
		},
		{
			name:  "04 - Duplicate values should be deduplicated",
			input: []string{"0x1A", "1A", "0x1a"}, // All represent 26 in decimal
			expected: map[int64]struct{}{
				decimal1A: {},
			},
		},
		{
			name:     "05 - Mixed valid and invalid inputs",
			input:    []string{"0x1A", "xyz", "0xFF", "invalid", "0x10"},
			expected: map[int64]struct{}{},
		},
		{
			name:  "06 - Negative hex numbers",
			input: []string{"-0x1A", "-FF"},
			expected: map[int64]struct{}{
				decimalNegFF: {},
			},
		},
	}
}

func TestHexStringToInt(t *testing.T) {
	for _, tt := range buildHexStringToIntTestCase() {
		t.Run(tt.name, func(t *testing.T) {
			result := StringTool.HexStringToInt(tt.input)
			for i := range tt.expected {
				fmt.Println(i)
			}
			if len(result) != len(tt.expected) {
				t.Errorf("Expected map length %d, but got %d", len(tt.expected), len(result))
				return
			}
			for key := range tt.expected {
				if _, exists := result[key]; !exists {
					t.Errorf("Expected key %d not found in result", key)
				}
			}
			for key := range result {
				if _, exists := tt.expected[key]; !exists {
					t.Errorf("Unexpected key %d found in result", key)
				}
			}
		})
	}
}

func TestSameElementInMap(t *testing.T) {
	for _, tt := range buildSameElementInMapTestCase() {
		t.Run(tt.name, func(t *testing.T) {
			result := SameElementInMap(tt.sources, tt.targets)
			if result != tt.expected {
				t.Errorf("SameElementInMap() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func buildSameElementInMapTestCase() []struct {
	name     string
	sources  map[int]struct{}
	targets  []int
	expected bool
} {
	return []struct {
		name     string
		sources  map[int]struct{}
		targets  []int
		expected bool
	}{
		{
			name:     "01 There are identical elements present",
			sources:  map[int]struct{}{1: {}, 2: {}, 3: {}},
			targets:  []int{4, 5, 2},
			expected: true,
		},
		{
			name:     "02 There are no identical elements present\n",
			sources:  map[int]struct{}{1: {}, 2: {}, 3: {}},
			targets:  []int{4, 5, 6},
			expected: false,
		},
		{
			name:     "03 target is nil",
			sources:  map[int]struct{}{1: {}, 2: {}},
			targets:  []int{},
			expected: false,
		},
		{
			name:     "04 source is nil",
			sources:  map[int]struct{}{},
			targets:  []int{1, 2, 3},
			expected: false,
		},
		{
			name:     "05 source and target are both nil",
			sources:  map[int]struct{}{},
			targets:  []int{},
			expected: false,
		},
	}
}

func TestSameElementInMap_StringType(t *testing.T) {
	sources := map[string]struct{}{
		"apple":  {},
		"banana": {},
		"orange": {},
	}
	targets := []string{"grape", "apple", "kiwi"}
	result := SameElementInMap(sources, targets)
	if !result {
		t.Errorf("SameElementInMap() with string type should return true, got false")
	}
	targetsNoMatch := []string{"grape", "kiwi", "mango"}
	resultNoMatch := SameElementInMap(sources, targetsNoMatch)
	if resultNoMatch {
		t.Errorf("SameElementInMap() with string type should return false, got true")
	}
}

func TestContains(t *testing.T) {
	for _, tt := range buildContainsTestCase() {
		t.Run(tt.name, func(t *testing.T) {
			switch s1 := tt.source.(type) {
			case []int:
				s2 := tt.target.(int)
				result := Contains(s1, s2)
				if !reflect.DeepEqual(result, tt.expected) {
					t.Errorf("Contains() = %v, want %v", result, tt.expected)
				}
			case []string:
				s2 := tt.target.(string)
				result := Contains(s1, s2)
				if !reflect.DeepEqual(result, tt.expected) {
					t.Errorf("Contains() = %v, want %v", result, tt.expected)
				}
			default:
				t.Errorf("unsupported type")
			}
		})
	}
}

func buildContainsTestCase() []struct {
	name     string
	source   interface{}
	target   interface{}
	expected bool
} {
	return []struct {
		name     string
		source   interface{}
		target   interface{}
		expected bool
	}{
		{
			name:     "01 contains for int type",
			source:   []int{1, 2, 3, 4},
			target:   1,
			expected: true,
		},
		{
			name:     "02 not contains for int type",
			source:   []int{1, 2, 3, 4},
			target:   0,
			expected: false,
		},
		{
			name:     "03 contains for string type",
			source:   []string{"1", "2", "3", "4"},
			target:   "1",
			expected: true,
		},
		{
			name:     "04 not contains for string type",
			source:   []string{"1", "2", "3", "4"},
			target:   "0",
			expected: false,
		},
		{
			name:     "05 empty source slice",
			source:   []int{},
			target:   1,
			expected: false,
		},
	}
}

func TestRemove(t *testing.T) {
	for _, tt := range buildRemoveTestCase() {
		t.Run(tt.name, func(t *testing.T) {
			switch s1 := tt.source.(type) {
			case []int:
				s2 := tt.target.(int)
				result := Remove(s1, s2)
				expected := tt.expected.([]int)
				if !reflect.DeepEqual(result, expected) {
					t.Errorf("Contains() = %v, want %v", result, expected)
				}
			case []string:
				s2 := tt.target.(string)
				result := Remove(s1, s2)
				expected := tt.expected.([]string)
				if !reflect.DeepEqual(result, expected) {
					t.Errorf("RemoveElementsNotInSecond() = %v, want %v", result, expected)
				}
			default:
				t.Errorf("unsupported type")
			}
		})
	}
}

func buildRemoveTestCase() []struct {
	name     string
	source   interface{}
	target   interface{}
	expected interface{}
} {
	return []struct {
		name     string
		source   interface{}
		target   interface{}
		expected interface{}
	}{
		{
			name:     "01 contains for int type",
			source:   []int{1, 2, 3, 4},
			target:   1,
			expected: []int{2, 3, 4},
		},
		{
			name:     "02 not contains for int type",
			source:   []int{1, 2, 3, 4},
			target:   0,
			expected: []int{1, 2, 3, 4},
		},
		{
			name:     "03 contains for string type",
			source:   []string{"1", "2", "3", "4"},
			target:   "1",
			expected: []string{"2", "3", "4"},
		},
		{
			name:     "04 not contains for string type",
			source:   []string{"1", "2", "3", "4"},
			target:   "0",
			expected: []string{"1", "2", "3", "4"},
		},
		{
			name:     "05 empty source slice",
			source:   []int{},
			target:   1,
			expected: []int{},
		},
	}
}

func buildRemoveElementsNotInSecondTestCase() []struct {
	name     string
	slice1   interface{}
	slice2   interface{}
	expected interface{}
} {
	return []struct {
		name     string
		slice1   interface{}
		slice2   interface{}
		expected interface{}
	}{
		{
			name:     "01 Basic functionality - integer slices with partial overlap",
			slice1:   []int{1, 2, 3, 4},
			slice2:   []int{2, 4, 6, 8},
			expected: []int{2, 4},
		},
		{
			name:     "02 Empty first slice",
			slice1:   []int{},
			slice2:   []int{1, 2, 3},
			expected: []int{},
		},
		{
			name:     "03 Empty second slice",
			slice1:   []int{1, 2, 3},
			slice2:   []int{},
			expected: []int{},
		},
		{
			name:     "04 Both slices empty",
			slice1:   []int{},
			slice2:   []int{},
			expected: []int{},
		},
		{
			name:     "05 No intersection between slices",
			slice1:   []int{1, 2, 3},
			slice2:   []int{4, 5, 6},
			expected: []int{},
		},
		{
			name:     "06 String type test",
			slice1:   []string{"1", "2", "3"},
			slice2:   []string{"2", "3", "4"},
			expected: []string{"2", "3"},
		},
	}
}

func TestRemoveElementsNotInSecond(t *testing.T) {
	for _, tt := range buildRemoveElementsNotInSecondTestCase() {
		t.Run(tt.name, func(t *testing.T) {
			switch s1 := tt.slice1.(type) {
			case []int:
				s2 := tt.slice2.([]int)
				expected := tt.expected.([]int)
				result := RemoveElementsNotInSecond(s1, s2)
				if !reflect.DeepEqual(result, expected) {
					t.Errorf("RemoveElementsNotInSecond() = %v, want %v", result, expected)
				}
			case []string:
				s2 := tt.slice2.([]string)
				expected := tt.expected.([]string)
				result := RemoveElementsNotInSecond(s1, s2)
				if !reflect.DeepEqual(result, expected) {
					t.Errorf("RemoveElementsNotInSecond() = %v, want %v", result, expected)
				}
			default:
				t.Errorf("unsupported type")
			}
		})
	}
}

func buildRemoveEleSliTestCase() []struct {
	name     string
	source   interface{}
	target   interface{}
	expected interface{}
} {
	return []struct {
		name     string
		source   interface{}
		target   interface{}
		expected interface{}
	}{
		{
			name:     "01 int type",
			source:   []int{1, 2, 3, 4, 5},
			target:   []int{2, 4},
			expected: []int{1, 3, 5},
		},
		{
			name:     "02 source is empty for int type",
			source:   []int{},
			target:   []int{1, 2},
			expected: []int{},
		},
		{
			name:     "03 target is empty for int type",
			source:   []int{1, 2, 3},
			target:   []int{},
			expected: []int{1, 2, 3},
		},
		{
			name:     "04 source and target are both empty for int type",
			source:   []int{},
			target:   []int{},
			expected: []int{},
		},
		{
			name:     "05 string type",
			source:   []string{"a", "b", "c", "d"},
			target:   []string{"b", "d"},
			expected: []string{"a", "c"},
		},
	}
}

func TestRemoveEleSli(t *testing.T) {
	for _, tt := range buildRemoveEleSliTestCase() {
		t.Run(tt.name, func(t *testing.T) {
			switch s1 := tt.source.(type) {
			case []int:
				s2 := tt.target.([]int)
				expected := tt.expected.([]int)
				result := RemoveEleSli(s1, s2)
				if !reflect.DeepEqual(result, expected) {
					t.Errorf("RemoveEleSli() = %v, want %v", result, expected)
				}
			case []string:
				s2 := tt.target.([]string)
				expected := tt.expected.([]string)
				result := RemoveEleSli(s1, s2)
				if !reflect.DeepEqual(result, expected) {
					t.Errorf("RemoveEleSli() = %v, want %v", result, expected)
				}
			default:
				t.Errorf("unsupported type")
			}
		})
	}
}

func buildRemoveDuplicatesCase() []struct {
	name     string
	input    interface{}
	expected interface{}
} {
	return []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "01 empty slice for int type",
			input:    []int{},
			expected: []int{},
		},
		{
			name:     "02 no duplicates for int type",
			input:    []int{1, 2, 3},
			expected: []int{1, 2, 3},
		},
		{
			name:     "03 with duplicates for int type",
			input:    []int{1, 2, 2, 3, 1, 4},
			expected: []int{1, 2, 3, 4},
		},
		{
			name:     "04 with duplicates for string type",
			input:    []string{"1", "3", "3", "4"},
			expected: []string{"1", "3", "4"},
		},
	}
}

func TestRemoveDuplicates(t *testing.T) {
	for _, tt := range buildRemoveDuplicatesCase() {
		t.Run(tt.name, func(t *testing.T) {
			switch s1 := tt.input.(type) {
			case []int:
				expected := tt.expected.([]int)
				result := RemoveDuplicates(s1)
				if !reflect.DeepEqual(result, expected) {
					t.Errorf("RemoveDuplicates() = %v, want %v", result, expected)
				}
			case []string:
				expected := tt.expected.([]string)
				result := RemoveDuplicates(s1)
				if !reflect.DeepEqual(result, expected) {
					t.Errorf("RemoveDuplicates() = %v, want %v", result, expected)
				}
			default:
				t.Errorf("unsupported type")
			}
		})
	}
}

func TestCheckSliceSupport(t *testing.T) {
	convey.Convey("test TestCheckSliceSupport, check ok", t, func() {
		elements := []int64{1, 2}
		expects := []int64{1, 2, 3}
		err := CheckSliceSupport(elements, expects)
		convey.So(err, convey.ShouldBeNil)
	})
	convey.Convey("test TestCheckSliceSupport, check fail", t, func() {
		elements := []int64{1, 2, 4}
		expects := []int64{1, 2, 3}
		err := CheckSliceSupport(elements, expects)
		convey.So(err, convey.ShouldNotBeNil)
	})
}
