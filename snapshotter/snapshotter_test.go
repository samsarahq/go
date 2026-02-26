package snapshotter_test

import (
	"fmt"
	"image"
	"image/color"
	"os"
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

func (m *mockT) Error(args ...interface{}) {
	m.errors = append(m.errors, fmt.Sprint(args...))
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

func TestSnapshotterInvalidFlags(t *testing.T) {
	if err := os.Setenv("REWRITE_SNAPSHOTS", "1"); err != nil {
		t.Errorf("failed to set REWRITE_SNAPSHOTS")
	}
	if err := os.Setenv("REWRITE_WITH_FAIL_ON_DIFF", "1"); err != nil {
		t.Errorf("failed to set REWRITE_WITH_FAIL_ON_DIFF")
	}
	defer func() {
		if err := os.Setenv("REWRITE_SNAPSHOTS", "0"); err != nil {
			t.Errorf("failed to reset REWRITE_SNAPSHOTS")
		}
		if err := os.Setenv("REWRITE_WITH_FAIL_ON_DIFF", "0"); err != nil {
			t.Errorf("failed to reset REWRITE_WITH_FAIL_ON_DIFF")
		}
	}()

	var m mockT
	ss := snapshotter.New(&m)
	ss.Verify()

	if len(m.errors) == 0 || !strings.Contains(m.errors[0], "choose one of rewriteWithFailOnDiff and rewriteSnapshots") {
		t.Errorf("expected error, got %v", m.errors)
	}
}

func TestVerifyWithImage(t *testing.T) {
	renderFn := func(values []interface{}) (image.Image, error) {
		length := 256
		r := uint8(values[0].(float64))
		g := uint8(values[1].(float64))
		b := uint8(values[2].(float64))
		img := image.NewRGBA(image.Rect(0, 0, length, length))
		c := color.RGBA{R: r, G: g, B: b, A: 255}
		for x := 0; x < length; x++ {
			for y := 0; y < length; y++ {
				img.Set(x, y, c)
			}
		}
		return img, nil
	}

	ss := snapshotter.New(t)
	defer ss.VerifyWithImage(renderFn)

	ss.Snapshot("color1", 255, 0, 0)
	ss.Snapshot("color2", 0, 255, 0)
	ss.Snapshot("color3", 128, 0, 128)
}

func TestSnapshotFileName(t *testing.T) {
	ss := snapshotter.New(t)
	expected := fmt.Sprintf("testdata/%s.snapshots.json", t.Name())
	if expected != ss.SnapshotFileName() {
		t.Errorf("expected %s, got %s", expected, ss.SnapshotFileName())
	}

	ss2 := snapshotter.NewNamed(t, "foobar")
	expected = fmt.Sprintf("testdata/%s_foobar.snapshots.json", t.Name())
	if expected != ss2.SnapshotFileName() {
		t.Errorf("expected %s, got %s", expected, ss2.SnapshotFileName())
	}

}
