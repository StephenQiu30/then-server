//go:build integration || container

package testcontainer

import (
	"context"
	"fmt"
	"testing"
	"time"

	dockercontainer "github.com/moby/moby/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const KafkaImage = "apache/kafka:4.3.1@sha256:77e3df9054047a88b520d0cc46e16696d3b22022e1d580aeccd2632df6532837"

// Kafka starts an owned KRaft broker. A shared network uses loopback inside the
// runtime container; host tests receive the Docker-assigned advertised port.
func Kafka(t *testing.T, ctx context.Context, sharedNetwork string) (testcontainers.Container, string, error) {
	t.Helper()
	address := "127.0.0.1:9092"
	request := testcontainers.ContainerRequest{
		Image: KafkaImage,
		Env: map[string]string{
			"CLUSTER_ID":    "MkU3OEVBNTcwNTJENDM2Qk",
			"KAFKA_NODE_ID": "1", "KAFKA_PROCESS_ROLES": "broker,controller",
			"KAFKA_LISTENERS":                        "PLAINTEXT://0.0.0.0:9092,CONTROLLER://127.0.0.1:9093",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "PLAINTEXT:PLAINTEXT,CONTROLLER:PLAINTEXT",
			"KAFKA_CONTROLLER_LISTENER_NAMES":        "CONTROLLER",
			"KAFKA_INTER_BROKER_LISTENER_NAME":       "PLAINTEXT",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":         "1@127.0.0.1:9093",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1", "KAFKA_OFFSETS_TOPIC_NUM_PARTITIONS": "1",
			"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR": "1", "KAFKA_TRANSACTION_STATE_LOG_MIN_ISR": "1",
			"KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS": "0", "KAFKA_AUTO_CREATE_TOPICS_ENABLE": "false",
			"KAFKA_HEAP_OPTS": "-Xms256m -Xmx512m",
		},
		Entrypoint: []string{"/bin/sh", "-c"},
		Cmd:        []string{"while [ ! -f /tmp/then-kafka-start.sh ]; do sleep 0.1; done; exec /bin/sh /tmp/then-kafka-start.sh"},
		LifecycleHooks: []testcontainers.ContainerLifecycleHooks{{PostStarts: []testcontainers.ContainerHook{
			func(ctx context.Context, c testcontainers.Container) error {
				if sharedNetwork == "" {
					var err error
					address, err = c.PortEndpoint(ctx, "9092/tcp", "")
					if err != nil {
						return err
					}
				}
				script := fmt.Sprintf("export KAFKA_ADVERTISED_LISTENERS='PLAINTEXT://%s'\nexec /etc/kafka/docker/run\n", address)
				if err := c.CopyToContainer(ctx, []byte(script), "/tmp/then-kafka-start.sh", 0o755); err != nil {
					return err
				}
				return wait.ForLog("Kafka Server started").WithStartupTimeout(time.Minute).WaitUntilReady(ctx, c)
			},
		}}},
	}
	if sharedNetwork == "" {
		request.ExposedPorts = []string{"9092/tcp"}
	} else {
		request.HostConfigModifier = func(h *dockercontainer.HostConfig) {
			h.NetworkMode = dockercontainer.NetworkMode("container:" + sharedNetwork)
		}
	}
	c, err := Create(t, ctx, testcontainers.GenericContainerRequest{ContainerRequest: request, Started: true})
	return c, address, err
}
