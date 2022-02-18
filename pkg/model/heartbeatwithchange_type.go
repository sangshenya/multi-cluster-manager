package model

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type HeartbeatWithChangeRequest struct {
	Healthy    bool        `json:"healthy"`
	Addons     []Addon     `json:"addons"`
	Conditions []Condition `json:"conditions"`
}

type Condition struct {
	Timestamp metav1.Time `json:"timestamp"`
	Message   string      `json:"message"`
	Reason    string      `json:"reason"`
	Type      string      `json:"type"`
}
