package common

import (
	"errors"
	"strings"
)

type NamespacedName struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

const (
	Separator = "/"
)

func NewNamespacedName(namespace, name string) NamespacedName {
	if len(namespace) == 0 || len(name) == 0 {
		return NamespacedName{}
	}
	return NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
}

// String returns the general purpose string representation
func (n NamespacedName) String() string {
	return n.Namespace + Separator + n.Name
}

func Parse(nString string) (NamespacedName, error) {
	if !strings.Contains(nString, Separator) {
		return NamespacedName{}, errors.New("invalid format")
	}
	strList := strings.Split(nString, Separator)
	if len(strList) != 2 {
		return NamespacedName{}, errors.New("too many separator")
	}
	return NamespacedName{
		Namespace: strList[0],
		Name:      strList[1],
	}, nil
}
