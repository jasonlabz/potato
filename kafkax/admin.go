package kafkax

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/segmentio/kafka-go"
)

type TopicConsumerGroup struct {
	GroupID string `json:"group_id"`
	State   string `json:"state,omitempty"`
	Members int    `json:"members,omitempty"`
	Lag     int64  `json:"lag,omitempty"`
}

type ConsumerGroupDescription struct {
	GroupID string
	State   string
	Members []ConsumerGroupMember
}

type ConsumerGroupMember struct {
	Topics      []string
	Assignments []TopicPartitionAssignment
}

type TopicPartitionAssignment struct {
	Topic      string
	Partitions []int
}

type ConsumerGroupOffset struct {
	Partition       int
	CommittedOffset int64
	Error           error
}

type PartitionOffset struct {
	Partition  int
	LastOffset int64
	Error      error
}

func (r *KafkaOperator) adminClient() *kafka.Client {
	return &kafka.Client{
		Addr:      kafka.TCP(r.config.BootstrapServers...),
		Timeout:   10 * time.Second,
		Transport: r.transport,
	}
}

func groupUsesTopic(group ConsumerGroupDescription, topic string) bool {
	for _, member := range group.Members {
		for _, t := range member.Topics {
			if t == topic {
				return true
			}
		}
		for _, assignment := range member.Assignments {
			if assignment.Topic == topic {
				return true
			}
		}
	}
	return false
}

func offsetRequests(partitions []kafka.Partition) []kafka.OffsetRequest {
	requests := make([]kafka.OffsetRequest, 0, len(partitions))
	for _, partition := range partitions {
		requests = append(requests, kafka.LastOffsetOf(partition.ID))
	}
	return requests
}

func partitionIDs(partitions []kafka.Partition) []int {
	ids := make([]int, 0, len(partitions))
	for _, partition := range partitions {
		ids = append(ids, partition.ID)
	}
	sort.Ints(ids)
	return ids
}

func describeGroup(group kafka.DescribeGroupsResponseGroup) ConsumerGroupDescription {
	description := ConsumerGroupDescription{GroupID: group.GroupID, State: group.GroupState, Members: make([]ConsumerGroupMember, 0, len(group.Members))}
	for _, member := range group.Members {
		m := ConsumerGroupMember{Topics: member.MemberMetadata.Topics}
		for _, topic := range member.MemberAssignments.Topics {
			m.Assignments = append(m.Assignments, TopicPartitionAssignment{Topic: topic.Topic, Partitions: topic.Partitions})
		}
		description.Members = append(description.Members, m)
	}
	return description
}

func consumerOffsets(partitions []kafka.OffsetFetchPartition) []ConsumerGroupOffset {
	offsets := make([]ConsumerGroupOffset, 0, len(partitions))
	for _, partition := range partitions {
		offsets = append(offsets, ConsumerGroupOffset{Partition: partition.Partition, CommittedOffset: partition.CommittedOffset, Error: partition.Error})
	}
	return offsets
}

func partitionOffsets(partitions []kafka.PartitionOffsets) []PartitionOffset {
	offsets := make([]PartitionOffset, 0, len(partitions))
	for _, partition := range partitions {
		offsets = append(offsets, PartitionOffset{Partition: partition.Partition, LastOffset: partition.LastOffset, Error: partition.Error})
	}
	return offsets
}

func calculateLag(committed []ConsumerGroupOffset, latest []PartitionOffset) int64 {
	latestByPartition := make(map[int]int64, len(latest))
	for _, partition := range latest {
		if partition.Error == nil {
			latestByPartition[partition.Partition] = partition.LastOffset
		}
	}
	var lag int64
	for _, partition := range committed {
		if partition.Error != nil || partition.CommittedOffset < 0 {
			continue
		}
		if latestOffset, ok := latestByPartition[partition.Partition]; ok && latestOffset > partition.CommittedOffset {
			lag += latestOffset - partition.CommittedOffset
		}
	}
	return lag
}

func (r *KafkaOperator) ListTopicConsumerGroups(ctx context.Context, topic string) ([]TopicConsumerGroup, error) {
	client := r.adminClient()
	metadata, err := client.Metadata(ctx, &kafka.MetadataRequest{Topics: []string{topic}})
	if err != nil {
		return nil, fmt.Errorf("kafka: topic metadata %q: %w", topic, err)
	}
	if len(metadata.Topics) == 0 {
		return nil, nil
	}
	partitions := metadata.Topics[0].Partitions
	ids := partitionIDs(partitions)
	latestOffsets, err := client.ListOffsets(ctx, &kafka.ListOffsetsRequest{Topics: map[string][]kafka.OffsetRequest{topic: offsetRequests(partitions)}})
	if err != nil {
		return nil, fmt.Errorf("kafka: list offsets %q: %w", topic, err)
	}
	groups, err := client.ListGroups(ctx, &kafka.ListGroupsRequest{})
	if err != nil {
		return nil, fmt.Errorf("kafka: list consumer groups: %w", err)
	}
	if groups.Error != nil {
		return nil, fmt.Errorf("kafka: list consumer groups: %w", groups.Error)
	}

	result := make([]TopicConsumerGroup, 0, len(groups.Groups))
	for _, group := range groups.Groups {
		if group.ProtocolType != "" && group.ProtocolType != "consumer" {
			continue
		}
		described, err := client.DescribeGroups(ctx, &kafka.DescribeGroupsRequest{GroupIDs: []string{group.GroupID}})
		if err != nil {
			return nil, fmt.Errorf("kafka: describe consumer group %q: %w", group.GroupID, err)
		}
		if len(described.Groups) == 0 || described.Groups[0].Error != nil {
			continue
		}
		description := describeGroup(described.Groups[0])
		if !groupUsesTopic(description, topic) {
			continue
		}
		committed, err := client.OffsetFetch(ctx, &kafka.OffsetFetchRequest{GroupID: group.GroupID, Topics: map[string][]int{topic: ids}})
		if err != nil {
			return nil, fmt.Errorf("kafka: fetch consumer group %q offsets: %w", group.GroupID, err)
		}
		if committed.Error != nil {
			continue
		}
		result = append(result, TopicConsumerGroup{
			GroupID: group.GroupID,
			State:   description.State,
			Members: len(description.Members),
			Lag:     calculateLag(consumerOffsets(committed.Topics[topic]), partitionOffsets(latestOffsets.Topics[topic])),
		})
	}
	return result, nil
}
