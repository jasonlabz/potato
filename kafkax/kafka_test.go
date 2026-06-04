package kafkax

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"

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

func TestKafkaUnsupportedSASLMechanismReturnsError(t *testing.T) {
	_, err := (&MQConfig{SaslMechanism: "OAUTHBEARER"}).getSASLMechanism()
	if err == nil {
		t.Fatal("expected unsupported sasl mechanism error")
	}
	if !strings.Contains(err.Error(), "unsupported sasl_mechanism") {
		t.Fatalf("error = %q, want unsupported sasl_mechanism", err.Error())
	}
}

func TestKafkaKerberosConfigReturnsExplicitError(t *testing.T) {
	_, err := (&MQConfig{SaslMechanism: "GSSAPI"}).getSASLMechanism()
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
