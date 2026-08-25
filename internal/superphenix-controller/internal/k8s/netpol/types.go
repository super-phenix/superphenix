package netpol

import (
	"errors"
	"fmt"
)

// MaxSubnets must stay in sync with the `max` validation tag on the SubnetEIds fields.
const MaxSubnets = 10

var (
	ErrUnknownSubnet  = errors.New("unknown subnet")
	ErrTooManySubnets = fmt.Errorf("a network policy cannot be scoped to more than %d subnets", MaxSubnets)
)

type SgPort struct {
	Port     int32  `json:"port"`
	EndPort  int32  `json:"endPort"`
	Protocol string `json:"protocol"`
}

type MatchLabel struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type MatchExpression struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values"`
}

type LabelSelector struct {
	MatchLabels      []MatchLabel      `json:"matchLabels"`
	MatchExpressions []MatchExpression `json:"matchExpressions"`
}

type IPBlock struct {
	CIDR   string   `json:"CIDR"`
	Except []string `json:"Except"`
}

type Peer struct {
	PodSelector LabelSelector `json:"podSelector"`
	IPBlock     IPBlock       `json:"IPBlock"`
}

type IngressRule struct {
	Ports    []SgPort `json:"ports"`
	From     []Peer   `json:"from"`
	AllowAll bool     `json:"allowAll"`
	DenyAll  bool     `json:"denyAll"`
}
type EgressRule struct {
	Ports    []SgPort `json:"ports"`
	To       []Peer   `json:"to"`
	AllowAll bool     `json:"allowAll"`
	DenyAll  bool     `json:"denyAll"`
}
