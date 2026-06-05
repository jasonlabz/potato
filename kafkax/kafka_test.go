package kafkax

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
	"testing"

	"github.com/segmentio/kafka-go/protocol"
	metadataAPI "github.com/segmentio/kafka-go/protocol/metadata"
	"github.com/segmentio/kafka-go/sasl"
)

type testSASLMechanism struct{}

func (testSASLMechanism) Name() string {
	return "GSSAPI"
}

func (testSASLMechanism) Start(context.Context) (sasl.StateMachine, []byte, error) {
	return testSASLStateMachine{}, nil, nil
}

type testSASLStateMachine struct{}

func (testSASLStateMachine) Next(context.Context, []byte) (bool, []byte, error) {
	return true, nil, nil
}

type metadataTransport struct {
	addr net.Addr
	req  protocol.Message
}

func (t *metadataTransport) RoundTrip(_ context.Context, addr net.Addr, req protocol.Message) (protocol.Message, error) {
	t.addr = addr
	t.req = req
	return &metadataAPI.Response{
		Brokers: []metadataAPI.ResponseBroker{{NodeID: 1, Host: "broker", Port: 9092}},
		Topics: []metadataAPI.ResponseTopic{
			{Name: "topic-b", Partitions: []metadataAPI.ResponsePartition{{PartitionIndex: 0, LeaderID: 1}}},
			{Name: "topic-a", Partitions: []metadataAPI.ResponsePartition{{PartitionIndex: 0, LeaderID: 1}}},
			{Name: "topic-b", Partitions: []metadataAPI.ResponsePartition{{PartitionIndex: 1, LeaderID: 1}}},
		},
	}, nil
}

func TestKafkaSASLMechanismNormalizesConfig(t *testing.T) {
	config := &MQConfig{
		BootstrapServers: []string{"localhost:9092"},
		SecurityProtocol: "sasl-plaintext",
		SaslMechanism:    "scram_sha_256",
		SaslUsername:     "user",
		SaslPassword:     "pass",
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	mechanism, err := config.getSASLMechanism()
	if err != nil {
		t.Fatalf("getSASLMechanism() error = %v", err)
	}
	if mechanism == nil {
		t.Fatal("getSASLMechanism() returned nil")
	}
	if mechanism.Name() != saslMechanismScramSHA256 {
		t.Fatalf("mechanism.Name() = %q, want %q", mechanism.Name(), saslMechanismScramSHA256)
	}
}

func TestKafkaSecurityProtocolNoneDisablesSecurity(t *testing.T) {
	config := &MQConfig{
		BootstrapServers: []string{"localhost:9092"},
		SecurityProtocol: "none",
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if config.usesSASL() {
		t.Fatal("usesSASL() = true, want false")
	}
	if config.usesTLS() {
		t.Fatal("usesTLS() = true, want false")
	}
}

func TestKafkaSASLMechanismEnablesSASLWithoutSecurityProtocol(t *testing.T) {
	config := &MQConfig{
		BootstrapServers: []string{"localhost:9092"},
		SaslMechanism:    "SCRAM-SHA-512",
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !config.usesSASL() {
		t.Fatal("usesSASL() = false, want true")
	}
}

func TestKafkaUnsupportedSASLMechanismReturnsError(t *testing.T) {
	_, err := (&MQConfig{SecurityProtocol: securityProtocolSASLPlaintext, SaslMechanism: "OAUTHBEARER"}).getSASLMechanism()
	if err == nil {
		t.Fatal("expected unsupported sasl mechanism error")
	}
	if !strings.Contains(err.Error(), "unsupported sasl_mechanism") {
		t.Fatalf("error = %q, want unsupported sasl_mechanism", err.Error())
	}
}

func TestKafkaKerberosConfigReturnsExplicitError(t *testing.T) {
	_, err := (&MQConfig{SecurityProtocol: securityProtocolSASLPlaintext, SaslMechanism: "GSSAPI"}).getSASLMechanism()
	if err == nil {
		t.Fatal("expected Kerberos config error")
	}
	if !strings.Contains(err.Error(), "GSSAPI/Kerberos") {
		t.Fatalf("error = %q, want GSSAPI/Kerberos", err.Error())
	}
}

func TestKafkaBuildDialerUsesCustomSASLAndTLS(t *testing.T) {
	tlsConfig := &tls.Config{ServerName: "broker.local", MinVersion: tls.VersionTLS12}
	operator := &KafkaOperator{
		config: &MQConfig{
			BootstrapServers: []string{"localhost:9092"},
			SecurityProtocol: securityProtocolSASLSSL,
		},
		connCfg: &ConnConfig{
			customSASLMechanism: testSASLMechanism{},
			customTLSConfig:     tlsConfig,
		},
	}

	dialer, err := operator.buildDialer()
	if err != nil {
		t.Fatalf("buildDialer() error = %v", err)
	}
	if dialer.SASLMechanism == nil || dialer.SASLMechanism.Name() != "GSSAPI" {
		t.Fatalf("dialer.SASLMechanism = %#v, want GSSAPI", dialer.SASLMechanism)
	}
	if dialer.TLS != tlsConfig {
		t.Fatalf("dialer.TLS = %#v, want custom TLS config", dialer.TLS)
	}
}

func TestKafkaListTopicsUsesMetadataClient(t *testing.T) {
	transport := &metadataTransport{}
	operator := &KafkaOperator{
		config:    &MQConfig{BootstrapServers: []string{"broker-a:9092", "broker-b:9092"}},
		connCfg:   DefaultConfig(),
		transport: transport,
	}

	topics, err := operator.ListTopics(context.Background())
	if err != nil {
		t.Fatalf("ListTopics() error = %v", err)
	}
	want := []string{"topic-a", "topic-b"}
	if len(topics) != len(want) {
		t.Fatalf("ListTopics() = %#v, want %#v", topics, want)
	}
	for i := range want {
		if topics[i] != want[i] {
			t.Fatalf("ListTopics() = %#v, want %#v", topics, want)
		}
	}
	if _, ok := transport.req.(*metadataAPI.Request); !ok {
		t.Fatalf("RoundTrip request = %T, want *metadata.Request", transport.req)
	}
	addr := transport.addr.String()
	if !strings.Contains(addr, "broker-a:9092") || !strings.Contains(addr, "broker-b:9092") {
		t.Fatalf("RoundTrip addr = %q, want full bootstrap list", addr)
	}
}
