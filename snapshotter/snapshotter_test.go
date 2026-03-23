package snapshotter_test

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
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

func switchToTempWorkingDir(t *testing.T) {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %s", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to switch to temp directory: %s", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Errorf("failed to restore working directory: %s", err)
		}
	})
}

func setRewriteSnapshotsEnv(t *testing.T) {
	t.Helper()

	previousValue := os.Getenv("REWRITE_SNAPSHOTS")
	if err := os.Setenv("REWRITE_SNAPSHOTS", "1"); err != nil {
		t.Fatalf("failed to set REWRITE_SNAPSHOTS: %s", err)
	}
	t.Cleanup(func() {
		if err := os.Setenv("REWRITE_SNAPSHOTS", previousValue); err != nil {
			t.Errorf("failed to restore REWRITE_SNAPSHOTS: %s", err)
		}
	})
}

func tinyRenderFn(_ []interface{}) (image.Image, error) {
	return image.NewRGBA(image.Rect(0, 0, 1, 1)), nil
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

func TestVerifyWithImageSnapshotRemoved(t *testing.T) {
	switchToTempWorkingDir(t)
	setRewriteSnapshotsEnv(t)

	ss := snapshotter.New(t)
	ss.Snapshot("a", 1)
	ss.Snapshot("b", 2)
	ss.Snapshot("c", 3)
	ss.VerifyWithImage(tinyRenderFn)

	dir := strings.TrimSuffix(ss.SnapshotFileName(), ".snapshots.json")
	for _, fileName := range []string{"a.png", "b.png", "c.png"} {
		if _, err := os.Stat(filepath.Join(dir, fileName)); err != nil {
			t.Fatalf("expected %s to exist after first rewrite: %s", fileName, err)
		}
	}

	ss = snapshotter.New(t)
	ss.Snapshot("a", 1)
	ss.Snapshot("c", 3)
	ss.VerifyWithImage(tinyRenderFn)

	if _, err := os.Stat(filepath.Join(dir, "b.png")); !os.IsNotExist(err) {
		t.Fatalf("expected stale b.png to be removed, got err: %s", err)
	}
	for _, fileName := range []string{"a.png", "c.png"} {
		if _, err := os.Stat(filepath.Join(dir, fileName)); err != nil {
			t.Fatalf("expected %s to exist after second rewrite: %s", fileName, err)
		}
	}
}

func TestVerifyWithImageWhitespacesInSnapshotName(t *testing.T) {
	switchToTempWorkingDir(t)
	setRewriteSnapshotsEnv(t)

	ss := snapshotter.New(t)
	snapshotName := "name with spaces"
	ss.Snapshot(snapshotName, 1)
	ss.VerifyWithImage(tinyRenderFn)

	dir := strings.TrimSuffix(ss.SnapshotFileName(), ".snapshots.json")
	imagePath := filepath.Join(dir, "name_with_spaces.png")
	if _, err := os.Stat(imagePath); err != nil {
		t.Fatalf("expected %s to exist after rewrite: %s", imagePath, err)
	}
	if _, err := os.Stat(filepath.Join(dir, snapshotName+".png")); !os.IsNotExist(err) {
		t.Fatalf("expected image file with spaces to not exist, got err: %s", err)
	}
}

func TestVerifyWithImageNoSnapshots(t *testing.T) {
	switchToTempWorkingDir(t)
	setRewriteSnapshotsEnv(t)

	ss := snapshotter.New(t)
	ss.Snapshot("only", 1)
	ss.VerifyWithImage(tinyRenderFn)

	dir := strings.TrimSuffix(ss.SnapshotFileName(), ".snapshots.json")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("expected image directory to exist after first rewrite: %s", err)
	}

	ss = snapshotter.New(t)
	ss.VerifyWithImage(tinyRenderFn)

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("expected image directory to be removed, got err: %s", err)
	}
}

func TestSnapshotFileName(t *testing.T) {
	ss := snapshotter.New(t)
	expected := fmt.Sprintf("testdata/%s.snapshots.json", t.Name())
	if expected != ss.SnapshotFileName() {
		t.Errorf("expected %s, got %s", expected, ss.SnapshotFileName())
	}

	ss2 := snapshotter.NewNamed(t, "foo bar")
	expected = fmt.Sprintf("testdata/%s_foo_bar.snapshots.json", t.Name())
	if expected != ss2.SnapshotFileName() {
		t.Errorf("expected %s, got %s", expected, ss2.SnapshotFileName())
	}

}
