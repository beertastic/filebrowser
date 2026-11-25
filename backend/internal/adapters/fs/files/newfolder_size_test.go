package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// A folder created and filled after the index was built (create dir, then upload
// a file into it) must show its real size in the parent listing right away, not
// the 4 KB placeholder until the next scheduled scan.
func TestNewFolderSizeVisibleInParentListing(t *testing.T) {
	cacheDir := t.TempDir()
	originalCache := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = cacheDir
	t.Cleanup(func() { settings.Config.Server.CacheDir = originalCache })

	fileutils.SetFsPermissions(0o644, 0o755)

	if indexing.GetIndexDB() == nil {
		db, _, err := dbsql.NewIndexDB("test_newfolder_size", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
		indexing.SetIndexDBForTesting(db)
	}

	root := t.TempDir()
	// A folder that exists before the index is built, as in a running server.
	if err := os.MkdirAll(filepath.Join(root, "scope", "existing"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scope", "existing", "a.bin"), make([]byte, 100000), 0o644); err != nil {
		t.Fatal(err)
	}

	sourceName := "test_newfolder_size"
	indexing.Initialize(&settings.Source{Name: sourceName, Path: root, Config: settings.SourceConfig{ResolvedRules: settings.ResolvedRulesConfig{
				// What a single root rule {folderPath: "/", ignoreSymlinks: true} resolves to.
				IgnoreAllSymlinks: true,
				FolderPaths:       map[string]settings.ConditionalRule{"/": {FolderPath: "/", IgnoreSymlinks: true}},
			}}}, false, false)
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		t.Fatal("Failed to get test index")
	}
	stopPoll := startScannerStatusPoll(t, idx, 1)
	defer stopPoll()
	waitForScannerReady(t, idx)
	defer func() {
		indexing.StopAllScanners()
		time.Sleep(50 * time.Millisecond)
		indexing.ClearTestIndices()
	}()

	const payload = 20000
	if err := WriteDirectory("/scope", utils.FileOptions{Source: sourceName, Path: "/scope/newdir/", IsDir: true}); err != nil {
		t.Fatalf("WriteDirectory: %v", err)
	}
	if err := WriteFile(sourceName, "/scope", "/scope/newdir/blob.bin", strings.NewReader(strings.Repeat("x", payload))); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// The refresh after a write may run in the background; give it a moment.
	var got int64
	for i := 0; i < 40; i++ {
		got = idx.GetFolderSizeForDisplay("/scope/newdir/")
		if got >= payload {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got < payload {
		t.Errorf("size map has %d for the new folder, want at least %d (its contents)", got, payload)
	}

	// The listing the web page uses for the parent directory.
	info, err := idx.GetFileInfo(indexing.FileInfoRequest{IndexPath: "/scope/", BoundIndexPath: "/scope", Expand: true})
	if err != nil {
		t.Fatalf("GetFileInfo: %v", err)
	}
	found := false
	for _, f := range info.Folders {
		if f.Name == "newdir" {
			found = true
			if f.Size < payload {
				t.Errorf("parent listing shows %d for the new folder, want at least %d", f.Size, payload)
			}
		}
	}
	if !found {
		t.Errorf("new folder missing from the parent listing: %+v", info.Folders)
	}
}
