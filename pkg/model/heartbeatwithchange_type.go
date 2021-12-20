package model

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type HeartbeatWithChangeRequest struct {
	Healthy    bool
	Addons     []Addon
	Conditions []Condition
}

type Condition struct {
	Timestamp metav1.Time
	Message   string
	Reason    string
	Type      string
}
