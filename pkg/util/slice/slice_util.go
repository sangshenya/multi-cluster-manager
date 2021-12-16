package slice

import "encoding/json"

func ContainsString(slice []string, s string) bool {
	if len(slice) == 0 {
		return false
	}
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func RemoveString(slice []string, s string) (result []string) {
	if len(slice) == 0 {
		return
	}
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}

func GetIndexWithObject(slice []interface{}, obj interface{}) int {
	if len(slice) == 0 {
		return -1
	}
	objByte, err := json.Marshal(obj)
	if err != nil {
		return -1
	}
	index := -1
	for i := 0; i < len(slice); i++ {
		itemByte, err := json.Marshal(slice[i])
		if err != nil {
			continue
		}
		if string(objByte) == string(itemByte) {
			index = i
			break
		}
	}
	return index
}

func RemoveObjectWithIndex(array []interface{}, index int) []interface{} {
	if index == 0 {
		return array[1:]
	} else if index == 1 {
		return append(array[index+1:], array[0])
	} else if index >= len(array) {
		return array
	} else if index == len(array)-1 {
		return array[0:index]
	} else if index == len(array)-2 {
		return append(array[:index], array[index+1])
	}
	return append(array[:index], array[index+1:]...)
}
