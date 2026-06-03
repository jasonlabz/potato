package kafkax

import "testing"

func TestKafkaLagIgnoresUncommittedOffsets(t *testing.T) {
	lag := calculateLag(
		[]ConsumerGroupOffset{{Partition: 0, CommittedOffset: 8}, {Partition: 1, CommittedOffset: -1}},
		[]PartitionOffset{{Partition: 0, LastOffset: 10}, {Partition: 1, LastOffset: 20}},
	)

	if lag != 2 {
		t.Fatalf("lag = %d, want 2", lag)
	}
}

func TestKafkaGroupUsesTopicFromMetadataOrAssignments(t *testing.T) {
	if !groupUsesTopic(ConsumerGroupDescription{Members: []ConsumerGroupMember{{Topics: []string{"orders"}}}}, "orders") {
		t.Fatal("expected metadata topic to match")
	}
	if !groupUsesTopic(ConsumerGroupDescription{Members: []ConsumerGroupMember{{Assignments: []TopicPartitionAssignment{{Topic: "payments", Partitions: []int{0}}}}}}, "payments") {
		t.Fatal("expected assignment topic to match")
	}
	if groupUsesTopic(ConsumerGroupDescription{Members: []ConsumerGroupMember{{Topics: []string{"orders"}}}}, "payments") {
		t.Fatal("unexpected topic match")
	}
}
