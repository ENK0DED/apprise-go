package notify

import (
	"bytes"
	"encoding/binary"
	"net"
	"sync"
	"testing"
	"time"
)

func TestAMQPConfiguration(t *testing.T) {
	for _, tc := range []struct{ version, want string }{{"", "0.9.1"}, {"0", "0.9.1"}, {"v0-9-1", "0.9.1"}, {"0.9", "0.9.1"}, {"v1", "1.0"}, {"1.0", "1.0"}} {
		parsed, err := ParseURL("amqp://example.com/queue?version=" + tc.version)
		if err != nil {
			t.Fatal(err)
		}
		target, err := NewAMQPTarget(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if target.version != tc.want {
			t.Fatalf("version %q selected %q", tc.version, target.version)
		}
	}
	for _, query := range []string{"version=2", "version=v1junk", "version=1.0.1", "priority=256", "priority=-1", "priority=x", "ttl=4294967296", "ttl=-1", "ttl=x"} {
		parsed, err := ParseURL("amqp://example.com/queue?" + query)
		if err != nil {
			continue
		}
		if _, err := NewAMQPTarget(parsed); err == nil {
			t.Errorf("accepted invalid query %q", query)
		}
	}
}

func TestAMQP10ExplicitZeroTTL(t *testing.T) {
	parsed, err := ParseURL("amqp://example.com/queue?version=1&ttl=0&priority=0")
	if err != nil {
		t.Fatal(err)
	}
	target, err := NewAMQPTarget(parsed)
	if err != nil {
		t.Fatal(err)
	}
	message := target.message10("body")
	encoded, err := message.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	// A three-field header preserves the explicitly set zero TTL. Omitting
	// the third field would silently turn immediate expiry into no expiry.
	count := uint32(0)
	if len(encoded) >= 12 && bytes.Equal(encoded[:3], []byte{0, 0x53, 0x70}) {
		switch encoded[3] {
		case 0xc0:
			count = uint32(encoded[5])
		case 0xd0:
			count = binary.BigEndian.Uint32(encoded[8:12])
		}
	}
	if count < 3 {
		t.Fatalf("zero TTL omitted from AMQP header: %x", encoded)
	}
}

func TestAMQPSilentBrokerTimeout(t *testing.T) {
	for _, version := range []string{"0.9.1", "1.0"} {
		t.Run(version, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var connections []net.Conn
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					connection, err := listener.Accept()
					if err != nil {
						return
					}
					mu.Lock()
					connections = append(connections, connection)
					mu.Unlock()
				}
			}()
			defer func() {
				listener.Close()
				<-done
				mu.Lock()
				defer mu.Unlock()
				for _, connection := range connections {
					connection.Close()
				}
			}()
			parsed, err := ParseURL("amqp://" + listener.Addr().String() + "/queue?version=" + version + "&cto=5&rto=5")
			if err != nil {
				t.Fatal(err)
			}
			target, err := NewAMQPTarget(parsed)
			if err != nil {
				t.Fatal(err)
			}
			SetNotificationDeadline(parsed, time.Now().Add(100*time.Millisecond))
			started := time.Now()
			if err := DispatchSendWithInput(target, parsed, "body", "title", "", NotifyInfo, nil); err == nil {
				t.Fatal("silent broker reported success")
			}
			if elapsed := time.Since(started); elapsed > 2*time.Second {
				t.Fatalf("silent broker stalled for %v", elapsed)
			}
		})
	}
}
