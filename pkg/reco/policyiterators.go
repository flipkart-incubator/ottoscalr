package reco

type Policy struct {
	Name                    string `json:"name"`
	RiskIndex               string `json:"riskIndex"`
	MinReplicaPercentageCut int    `json:"minReplicaPercentageCut"`
	TargetUtilization       int    `json:"targetUtilization"`
}

// TODO(bharathguvvala): Implement PolicyCache which watches on the list of policies which the policy iterators treat as source of truth
// TODO(bharathguvvala): Implement policy iterators -- breach analyzer, default policy, Aging

type PolicyIterator interface {
	NextPolicy(wm WorkloadMeta) (*Policy, error)
}
