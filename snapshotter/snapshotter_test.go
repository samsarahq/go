package snapshotter_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/samsarahq/go/snapshotter"
)

func TestSnapshotter(t *testing.T) {
	ss := snapshotter.New(t)
	defer ss.Verify()

	ss.Snapshot("first", 1, "uno")
	ss.Snapshot("second", 2, struct{ Foo string }{Foo: "Bar"})
}

type mockT struct {
	errors []string
}

func (m *mockT) Name() string {
	return "MockTest"
}

func (m *mockT) Helper() {
}

func (m *mockT) Errorf(format string, args ...interface{}) {
	m.errors = append(m.errors, fmt.Sprintf(format, args...))
}

func TestSnapshotterFailed(t *testing.T) {
	var m mockT
	ss := snapshotter.New(&m)
	ss.Snapshot("good", true)
	ss.Snapshot("bad", "i am not in the snapshot catch me")
	ss.Verify()

	if len(m.errors) == 0 || !strings.Contains(m.errors[0], "snapshot bad differs") {
		t.Errorf("expected error, got %v", m.errors)
	}
}

func TestSnapshotterNoSnapshots(t *testing.T) {
	ss := snapshotter.New(t)
	ss.Verify()
}
